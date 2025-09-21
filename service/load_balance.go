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
}

type CreateInstanceTargetGroupOptions struct {
    Ctx             context.Context
    VpcId           string
    ServiceName     string
    TrafficPort     uint16
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
        return memory.Unwrap(tgOutput.TargetGroups[0].TargetGroupArn), nil
    }

    if tgOutput == nil || len(tgOutput.TargetGroups) == 0 {
        ctgOutput, err := svc.ELBv2.Client().CreateTargetGroup(opts.Ctx, &elbv2.CreateTargetGroupInput{
            Name:                    memory.Pointer(opts.ServiceName),
            Port:                    memory.Pointer(int32(opts.TrafficPort)),
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
    Ctx                   context.Context
    ServiceName           string
    TargetInstanceFilters InstanceFilters
    TrafficPort           uint16
    TrafficProtocol       types.ProtocolEnum
    IpProtocol            string
}

func (opts SetupNewILBServiceOptions) AwsumResourceName() string {
    return fmt.Sprintf("awsum-ilb-svc-%s", opts.ServiceName)
}

type InstanceLoadBalancedServiceResources struct {
    TargetGroupArn string
    SecurityGroup  *ec2Types.SecurityGroup
    LoadBalancer   *types.LoadBalancer
}

func (svc *AwsumILBService) SetupNewILBService(opts SetupNewILBServiceOptions) (*InstanceLoadBalancedServiceResources, error) {
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

    targetGroupArn, err := svc.lazyCreateInstanceTargetGroup(CreateInstanceTargetGroupOptions{
        Ctx:             opts.Ctx,
        VpcId:           targetVPC,
        ServiceName:     opts.AwsumResourceName(),
        TrafficPort:     opts.TrafficPort,
        TrafficProtocol: opts.TrafficProtocol,
    })

    if err != nil {
        return nil, err
    }

    if err = svc.ELBv2.DeregisterAllTargetsInTargetGroup(opts.Ctx, targetGroupArn); err != nil {
        return nil, err
    }

    for _, instance := range targetInstances {
        _, err = svc.ELBv2.Client().RegisterTargets(opts.Ctx, &elbv2.RegisterTargetsInput{
            TargetGroupArn: memory.Pointer(targetGroupArn),
            Targets: []types.TargetDescription{
                {Id: instance.Info.InstanceId, Port: memory.Pointer(int32(opts.TrafficPort))},
            },
        })

        if err != nil {
            return nil, err
        }
    }

    securityGroup, err := svc.EC2.SearchForSecurityGroupByName(opts.Ctx, opts.AwsumResourceName())

    if err != nil {
        return nil, err
    }

    for securityGroup == nil {
        if _, err = svc.EC2.CreateEmptySecurityGroup(opts.Ctx, opts.AwsumResourceName()); err != nil {
            if strings.Contains(err.Error(), "already exists") {
                break
            }

            return nil, err
        }

        securityGroup, err = svc.EC2.SearchForSecurityGroupByName(opts.Ctx, opts.AwsumResourceName())

        if err != nil {
            return nil, err
        }
    }

    _, err = svc.EC2.Client().AuthorizeSecurityGroupIngress(opts.Ctx, &ec2.AuthorizeSecurityGroupIngressInput{
        GroupId: securityGroup.GroupId,
        IpPermissions: []ec2Types.IpPermission{
            {
                FromPort:   memory.Pointer(int32(opts.TrafficPort)),
                ToPort:     memory.Pointer(int32(opts.TrafficPort)),
                IpProtocol: memory.Pointer(opts.IpProtocol),
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
        GroupId: securityGroup.GroupId,
        IpPermissions: []ec2Types.IpPermission{
            {
                FromPort:   memory.Pointer(int32(opts.TrafficPort)),
                ToPort:     memory.Pointer(int32(opts.TrafficPort)),
                IpProtocol: memory.Pointer(opts.IpProtocol),
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

    loadBalancer, err := svc.ELBv2.GetLoadBalancerByName(opts.Ctx, opts.AwsumResourceName())

    if err != nil {
        return nil, err
    }

    for loadBalancer == nil {
        allSubnets, err := svc.EC2.GetAllSubnets(opts.Ctx)

        if err != nil {
            return nil, err
        }

        subnetAzMap := make(map[string]string)

        for _, subnet := range allSubnets {
            subnetAzMap[memory.Unwrap(subnet.AvailabilityZone)] = memory.Unwrap(subnet.SubnetId)
        }

        azGroupedSubnets := slices.Collect(maps.Values(subnetAzMap))

        _, err = svc.ELBv2.Client().CreateLoadBalancer(opts.Ctx, &elbv2.CreateLoadBalancerInput{
            Name:           memory.Pointer(opts.AwsumResourceName()),
            Type:           types.LoadBalancerTypeEnumApplication,
            Scheme:         types.LoadBalancerSchemeEnumInternetFacing,
            SecurityGroups: []string{memory.Unwrap(securityGroup.GroupId)},
            Subnets:        append(instanceSubnets, azGroupedSubnets...),
            IpAddressType:  types.IpAddressTypeIpv4,
        })

        if err != nil {
            return nil, err
        }

        loadBalancer, err = svc.ELBv2.GetLoadBalancerByName(opts.Ctx, opts.AwsumResourceName())

        if err != nil {
            return nil, err
        }
    }

    err = svc.ELBv2.DeleteAllListenersInLoadBalancer(opts.Ctx, memory.Unwrap(loadBalancer.LoadBalancerArn))

    if err != nil {
        return nil, err
    }

    _, err = svc.ELBv2.Client().CreateListener(opts.Ctx, &elbv2.CreateListenerInput{
        LoadBalancerArn: loadBalancer.LoadBalancerArn,
        Port:            memory.Pointer(int32(opts.TrafficPort)),
        Protocol:        opts.TrafficProtocol,
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

    return &InstanceLoadBalancedServiceResources{
        TargetGroupArn: targetGroupArn,
        SecurityGroup:  securityGroup,
        LoadBalancer:   loadBalancer,
    }, err
}

func NewAwsumILBService(awsConfig aws.Config) *AwsumILBService {
    return &AwsumILBService{
        EC2:   NewEC2(awsConfig),
        ELBv2: NewELBv2(awsConfig),
    }
}
