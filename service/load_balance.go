package service

import (
    "context"
    "errors"
    "fmt"
    "maps"
    "slices"
    "strings"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
    elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
    "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
    "github.com/devhsoj/awsum/internal/memory"
)

var (
    ErrTargetGroupNotReturnedAfterCreation = errors.New("target group not returned after creation")
    ErrTargetInstancesMustAllBeInSameVPC   = errors.New("target instances must all be in the same vpc")
)

// AwsumILBService is a struct used to encompass all the logic of services on instances that are load balanced by
// resources created by awsum.
type AwsumILBService struct {
    EC2   *EC2
    ELBv2 *ELBv2
    ACM   *ACM
}

func NewAwsumILBService(awsConfig aws.Config) *AwsumILBService {
    return &AwsumILBService{
        EC2:   NewEC2(awsConfig),
        ELBv2: NewELBv2(awsConfig),
        ACM:   NewACM(awsConfig),
    }
}

type CreateInstanceTargetGroupOptions struct {
    Ctx             context.Context
    VpcId           string
    ServiceName     string
    TrafficPort     int32
    TrafficProtocol types.ProtocolEnum
}

func (svc *AwsumILBService) lazyCreateInstanceTargetGroup(opts CreateInstanceTargetGroupOptions) (string, error) {
    tgOutput, err := svc.ELBv2.Client().DescribeTargetGroups(opts.Ctx, &elbv2.DescribeTargetGroupsInput{
        Names: []string{opts.ServiceName},
    })

    if err != nil && !strings.Contains(err.Error(), "TargetGroupNotFound") {
        return "", err
    }

    if tgOutput != nil && len(tgOutput.TargetGroups) >= 1 {
        if tgOutput.TargetGroups[0].Protocol != opts.TrafficProtocol {
            return "", errors.New("")
        }
        return memory.Unwrap(tgOutput.TargetGroups[0].TargetGroupArn), nil
    }

    if tgOutput == nil || len(tgOutput.TargetGroups) == 0 {
        ctgOutput, err := svc.ELBv2.Client().CreateTargetGroup(opts.Ctx, &elbv2.CreateTargetGroupInput{
            Name:                    memory.Pointer(opts.ServiceName),
            Port:                    memory.Pointer(opts.TrafficPort),
            Protocol:                opts.TrafficProtocol,
            VpcId:                   memory.Pointer(opts.VpcId),
            TargetType:              types.TargetTypeEnumInstance,
            HealthCheckPath:         memory.Pointer("/"),
            HealthCheckProtocol:     types.ProtocolEnumHttp,
            HealthCheckPort:         memory.Pointer("traffic-port"),
            HealthyThresholdCount:   memory.Pointer(int32(3)),
            UnhealthyThresholdCount: memory.Pointer(int32(3)),
            Matcher:                 &types.Matcher{HttpCode: memory.Pointer("200,301,302,304")},
        })

        if err != nil {
            return "", err
        }

        if len(ctgOutput.TargetGroups) == 0 {
            return "", ErrTargetGroupNotReturnedAfterCreation
        }

        return memory.Unwrap(ctgOutput.TargetGroups[0].TargetGroupArn), nil
    }

    return "", nil
}

type SetupNewILBServiceOptions struct {
    Ctx                    context.Context
    ServiceName            string
    TargetInstanceFilters  InstanceFilters
    LoadBalancerIpProtocol string
    LoadBalancerPort       int32
    TrafficPort            int32
    TrafficProtocol        types.ProtocolEnum
    CertificateNames       []string
}

func (opts SetupNewILBServiceOptions) AwsumResourceName() string {
    return fmt.Sprintf("awsum-ilb-svc-%s", opts.ServiceName)
}

type ILBServiceResources struct {
    TargetGroupArn      string
    SecurityGroupId     string
    LoadBalancerArn     string
    LoadBalancerDNSName string
}

func (svc *AwsumILBService) SetupNewILBService(opts SetupNewILBServiceOptions) (*ILBServiceResources, error) {
    // target selection

    instances, err := svc.EC2.GetAllRunningInstances(opts.Ctx)

    if err != nil {
        return nil, err
    }

    targetInstances := opts.TargetInstanceFilters.Matches(instances)

    var (
        vpcIdMap    = make(map[string]struct{})
        subnetIdMap = make(map[string]struct{})
    )

    for _, instance := range targetInstances {
        vpcIdMap[memory.Unwrap(instance.Info.VpcId)] = struct{}{}
        subnetIdMap[memory.Unwrap(instance.Info.SubnetId)] = struct{}{}
    }

    instanceVPCs := slices.Collect(maps.Keys(vpcIdMap))
    instanceSubnets := slices.Collect(maps.Keys(subnetIdMap))

    if len(instanceVPCs) > 1 {
        return nil, ErrTargetInstancesMustAllBeInSameVPC
    }

    targetVPC := instanceVPCs[0]

    // remove old load balancing resources

    loadBalancer, err := svc.ELBv2.SearchForLoadBalancerByName(opts.Ctx, opts.AwsumResourceName())

    if err != nil {
        return nil, err
    }

    if loadBalancer != nil {
        if _, err = svc.ELBv2.Client().DeleteLoadBalancer(opts.Ctx, &elbv2.DeleteLoadBalancerInput{
            LoadBalancerArn: loadBalancer.LoadBalancerArn,
        }); err != nil {
            return nil, err
        }
    }

    // remove old target group resources

    targetGroup, err := svc.ELBv2.SearchForTargetGroupByName(opts.Ctx, opts.AwsumResourceName())

    if err != nil {
        return nil, err
    }

    if targetGroup != nil {
        if _, err = svc.ELBv2.Client().DeleteTargetGroup(opts.Ctx, &elbv2.DeleteTargetGroupInput{
            TargetGroupArn: targetGroup.TargetGroupArn,
        }); err != nil {
            return nil, err
        }
    }

    // remove old security group resources

    securityGroup, err := svc.EC2.SearchForSecurityGroupByName(opts.Ctx, opts.AwsumResourceName())

    if err != nil {
        return nil, err
    }

    if securityGroup != nil {
        _, err = svc.EC2.Client().DeleteSecurityGroup(opts.Ctx, &ec2.DeleteSecurityGroupInput{
            GroupId: securityGroup.GroupId,
        })

        if err != nil {
            return nil, err
        }
    }

    // setup service security group

    cesgOutput, err := svc.EC2.CreateEmptySecurityGroup(opts.Ctx, opts.AwsumResourceName())

    if err != nil {
        return nil, err
    }

    securityGroupId := memory.Unwrap(cesgOutput.GroupId)

    _, err = svc.EC2.Client().AuthorizeSecurityGroupIngress(opts.Ctx, &ec2.AuthorizeSecurityGroupIngressInput{
        GroupId: memory.Pointer(securityGroupId),
        IpPermissions: []ec2Types.IpPermission{
            {
                FromPort:   memory.Pointer(opts.TrafficPort),
                ToPort:     memory.Pointer(opts.TrafficPort),
                IpProtocol: memory.Pointer(opts.LoadBalancerIpProtocol),
                IpRanges: []ec2Types.IpRange{
                    {
                        CidrIp:      memory.Pointer("0.0.0.0/0"),
                        Description: memory.Pointer("all outbound"),
                    },
                },
            },
        },
    })

    if err != nil && !strings.Contains(err.Error(), "already exists") {
        return nil, err
    }

    _, err = svc.EC2.Client().AuthorizeSecurityGroupEgress(opts.Ctx, &ec2.AuthorizeSecurityGroupEgressInput{
        GroupId: memory.Pointer(securityGroupId),
        IpPermissions: []ec2Types.IpPermission{
            {
                FromPort:   memory.Pointer(opts.TrafficPort),
                ToPort:     memory.Pointer(opts.TrafficPort),
                IpProtocol: memory.Pointer(opts.LoadBalancerIpProtocol),
                IpRanges: []ec2Types.IpRange{
                    {
                        CidrIp:      memory.Pointer("0.0.0.0/0"),
                        Description: memory.Pointer("all outbound"),
                    },
                },
            },
        },
    })

    if err != nil && !strings.Contains(err.Error(), "already exists") {
        return nil, err
    }

    // load balancer creation

    allSubnets, err := svc.EC2.GetAllSubnets(opts.Ctx)

    if err != nil {
        return nil, err
    }

    subnetAzMap := make(map[string]string)

    for _, subnet := range allSubnets {
        subnetAzMap[memory.Unwrap(subnet.AvailabilityZone)] = memory.Unwrap(subnet.SubnetId)
    }

    azGroupedSubnets := slices.Collect(maps.Values(subnetAzMap))

    clbOutput, err := svc.ELBv2.Client().CreateLoadBalancer(opts.Ctx, &elbv2.CreateLoadBalancerInput{
        Name:           memory.Pointer(opts.AwsumResourceName()),
        Type:           types.LoadBalancerTypeEnumApplication,
        Scheme:         types.LoadBalancerSchemeEnumInternetFacing,
        SecurityGroups: []string{securityGroupId},
        Subnets:        append(instanceSubnets, azGroupedSubnets...),
        IpAddressType:  types.IpAddressTypeIpv4,
    })

    if err != nil {
        return nil, err
    }

    loadBalancerArn := memory.Unwrap(clbOutput.LoadBalancers[0].LoadBalancerArn)
    loadBalancerDNSName := memory.Unwrap(clbOutput.LoadBalancers[0].DNSName)

    // target group creation

    ctgOutput, err := svc.ELBv2.Client().CreateTargetGroup(opts.Ctx, &elbv2.CreateTargetGroupInput{
        Name:                    memory.Pointer(opts.AwsumResourceName()),
        Port:                    memory.Pointer(opts.TrafficPort),
        Protocol:                opts.TrafficProtocol,
        VpcId:                   memory.Pointer(targetVPC),
        TargetType:              types.TargetTypeEnumInstance,
        HealthCheckPath:         memory.Pointer("/"),
        HealthCheckProtocol:     opts.TrafficProtocol,
        HealthCheckPort:         memory.Pointer("traffic-port"),
        HealthyThresholdCount:   memory.Pointer(int32(3)),
        UnhealthyThresholdCount: memory.Pointer(int32(3)),
        Matcher:                 &types.Matcher{HttpCode: memory.Pointer("200,301,302,304")},
    })

    if err != nil {
        return nil, err
    }

    targetGroupArn := memory.Unwrap(ctgOutput.TargetGroups[0].TargetGroupArn)

    // target group registration

    for _, instance := range targetInstances {
        _, err = svc.ELBv2.Client().RegisterTargets(opts.Ctx, &elbv2.RegisterTargetsInput{
            TargetGroupArn: memory.Pointer(targetGroupArn),
            Targets: []types.TargetDescription{
                {Id: instance.Info.InstanceId, Port: memory.Pointer(opts.TrafficPort)},
            },
        })

        if err != nil {
            return nil, err
        }
    }

    // load-balancer certificate and listener setup

    var certs []types.Certificate

    if len(opts.CertificateNames) > 0 {
        certs, err = svc.ACM.GenerateLoadBalanceCertificateListFromCertificateNames(opts.Ctx, opts.CertificateNames)

        if err != nil {
            return nil, err
        }
    }

    // load-balancer listener creation

    _, err = svc.ELBv2.Client().CreateListener(opts.Ctx, &elbv2.CreateListenerInput{
        LoadBalancerArn: memory.Pointer(loadBalancerArn),
        Port:            memory.Pointer(opts.LoadBalancerPort),
        Protocol:        opts.TrafficProtocol,
        Certificates:    certs,
        DefaultActions: []types.Action{{
            Type: types.ActionTypeEnumForward,
            ForwardConfig: &types.ForwardActionConfig{
                TargetGroups: []types.TargetGroupTuple{
                    {
                        TargetGroupArn: memory.Pointer(targetGroupArn),
                    },
                },
            },
        }},
    })

    return &ILBServiceResources{
        TargetGroupArn:      targetGroupArn,
        SecurityGroupId:     securityGroupId,
        LoadBalancerArn:     loadBalancerArn,
        LoadBalancerDNSName: loadBalancerDNSName,
    }, err
}
