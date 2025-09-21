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

type SetupNewILBServiceOptions struct {
    Ctx                          context.Context
    ServiceName                  string
    TargetInstanceFilters        InstanceFilters
    LoadBalancerListenerProtocol types.ProtocolEnum
    LoadBalancerIpProtocol       string
    LoadBalancerPort             int32
    TrafficPort                  int32
    TrafficProtocol              types.ProtocolEnum
    CertificateNames             []string
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
    var resources ILBServiceResources

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

    // remove old security group resources

    securityGroup, err := svc.EC2.SearchForSecurityGroupByName(opts.Ctx, opts.AwsumResourceName())

    if err != nil {
        return nil, err
    }

    if securityGroup != nil {
        rules, err := svc.EC2.GetAllSecurityGroupRules(opts.Ctx)

        if err != nil {
            return nil, err
        }

        var (
            egressRuleIds  []string
            ingressRuleIds []string
        )

        for _, rule := range rules {
            if memory.Unwrap(rule.IsEgress) {
                egressRuleIds = append(egressRuleIds, memory.Unwrap(rule.SecurityGroupRuleId))
                continue
            }

            ingressRuleIds = append(ingressRuleIds, memory.Unwrap(rule.SecurityGroupRuleId))
        }

        if len(egressRuleIds) > 0 {
            if _, err = svc.EC2.Client().RevokeSecurityGroupEgress(opts.Ctx, &ec2.RevokeSecurityGroupEgressInput{
                SecurityGroupRuleIds: egressRuleIds,
            }); err != nil {
                return nil, err
            }
        }

        if len(ingressRuleIds) > 0 {
            if _, err = svc.EC2.Client().RevokeSecurityGroupIngress(opts.Ctx, &ec2.RevokeSecurityGroupIngressInput{
                SecurityGroupRuleIds: ingressRuleIds,
            }); err != nil {
                return nil, err
            }
        }

        resources.SecurityGroupId = memory.Unwrap(securityGroup.GroupId)
    } else {
        // setup service security group

        cesgOutput, err := svc.EC2.CreateEmptySecurityGroup(opts.Ctx, opts.AwsumResourceName())

        if err != nil {
            return nil, err
        }

        resources.SecurityGroupId = memory.Unwrap(cesgOutput.GroupId)
    }

    _, err = svc.EC2.Client().AuthorizeSecurityGroupIngress(opts.Ctx, &ec2.AuthorizeSecurityGroupIngressInput{
        GroupId: memory.Pointer(resources.SecurityGroupId),
        IpPermissions: []ec2Types.IpPermission{
            {
                FromPort:   memory.Pointer(opts.TrafficPort),
                ToPort:     memory.Pointer(opts.TrafficPort),
                IpProtocol: memory.Pointer(opts.LoadBalancerIpProtocol),
                IpRanges: []ec2Types.IpRange{
                    {
                        CidrIp:      memory.Pointer("0.0.0.0/0"),
                        Description: memory.Pointer("all inbound traffic"),
                    },
                },
            },
        },
    })

    if err != nil && !strings.Contains(err.Error(), "already exists") {
        return nil, err
    }

    _, err = svc.EC2.Client().AuthorizeSecurityGroupIngress(opts.Ctx, &ec2.AuthorizeSecurityGroupIngressInput{
        GroupId: memory.Pointer(resources.SecurityGroupId),
        IpPermissions: []ec2Types.IpPermission{
            {
                FromPort:   memory.Pointer(opts.LoadBalancerPort),
                ToPort:     memory.Pointer(opts.LoadBalancerPort),
                IpProtocol: memory.Pointer(opts.LoadBalancerIpProtocol),
                IpRanges: []ec2Types.IpRange{
                    {
                        CidrIp:      memory.Pointer("0.0.0.0/0"),
                        Description: memory.Pointer("all inbound traffic"),
                    },
                },
            },
        },
    })

    if err != nil && !strings.Contains(err.Error(), "already exists") {
        return nil, err
    }

    _, err = svc.EC2.Client().AuthorizeSecurityGroupEgress(opts.Ctx, &ec2.AuthorizeSecurityGroupEgressInput{
        GroupId: memory.Pointer(resources.SecurityGroupId),
        IpPermissions: []ec2Types.IpPermission{
            {
                FromPort:   memory.Pointer(opts.TrafficPort),
                ToPort:     memory.Pointer(opts.TrafficPort),
                IpProtocol: memory.Pointer(opts.LoadBalancerIpProtocol),
                IpRanges: []ec2Types.IpRange{
                    {
                        CidrIp:      memory.Pointer("0.0.0.0/0"),
                        Description: memory.Pointer("all outbound traffic"),
                    },
                },
            },
        },
    })

    if err != nil && !strings.Contains(err.Error(), "already exists") {
        return nil, err
    }

    // remove conflicting load balancing resources

    loadBalancer, err := svc.ELBv2.SearchForLoadBalancerByName(opts.Ctx, opts.AwsumResourceName())

    if err != nil {
        return nil, err
    }

    if loadBalancer != nil {
        listeners, err := svc.ELBv2.GetAllListenersInLoadBalancer(opts.Ctx, memory.Unwrap(loadBalancer.LoadBalancerArn))

        if err != nil {
            return nil, err
        }

        for _, listener := range listeners {
            if _, err = svc.ELBv2.Client().DeleteListener(opts.Ctx, &elbv2.DeleteListenerInput{
                ListenerArn: listener.ListenerArn,
            }); err != nil {
                return nil, err
            }
        }

        resources.LoadBalancerArn = memory.Unwrap(loadBalancer.LoadBalancerArn)
        resources.LoadBalancerDNSName = memory.Unwrap(loadBalancer.DNSName)
    } else {
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
            SecurityGroups: []string{resources.SecurityGroupId},
            Subnets:        append(instanceSubnets, azGroupedSubnets...),
            IpAddressType:  types.IpAddressTypeIpv4,
        })

        if err != nil {
            return nil, err
        }

        resources.LoadBalancerArn = memory.Unwrap(clbOutput.LoadBalancers[0].LoadBalancerArn)
        resources.LoadBalancerDNSName = memory.Unwrap(clbOutput.LoadBalancers[0].DNSName)
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

    // target group creation

    ctgInput := elbv2.CreateTargetGroupInput{
        Name:       memory.Pointer(opts.AwsumResourceName()),
        Port:       memory.Pointer(opts.TrafficPort),
        Protocol:   opts.TrafficProtocol,
        VpcId:      memory.Pointer(targetVPC),
        TargetType: types.TargetTypeEnumInstance,
    }

    ctgOutput, err := svc.ELBv2.Client().CreateTargetGroup(opts.Ctx, &ctgInput)

    if err != nil {
        return nil, err
    }

    resources.TargetGroupArn = memory.Unwrap(ctgOutput.TargetGroups[0].TargetGroupArn)

    // target group registration

    for _, instance := range targetInstances {
        _, err = svc.ELBv2.Client().RegisterTargets(opts.Ctx, &elbv2.RegisterTargetsInput{
            TargetGroupArn: memory.Pointer(resources.TargetGroupArn),
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

    _, err = svc.ELBv2.Client().CreateListener(opts.Ctx, &elbv2.CreateListenerInput{
        LoadBalancerArn: memory.Pointer(resources.LoadBalancerArn),
        Port:            memory.Pointer(opts.LoadBalancerPort),
        Protocol:        opts.LoadBalancerListenerProtocol,
        Certificates:    certs,
        DefaultActions: []types.Action{{
            Type: types.ActionTypeEnumForward,
            ForwardConfig: &types.ForwardActionConfig{
                TargetGroups: []types.TargetGroupTuple{
                    {
                        TargetGroupArn: memory.Pointer(resources.TargetGroupArn),
                    },
                },
            },
        }},
    })

    if err != nil {
        return nil, err
    }

    return &resources, err
}
