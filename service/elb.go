package service

import (
    "context"
    "errors"
    "fmt"
    "log"
    "maps"
    "os"
    "slices"
    "strings"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
    elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
    "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
    "github.com/devhsoj/awsum/internal/memory"
)

type ELBv2 struct {
    client *elbv2.Client
}

func (e *ELBv2) Client() *elbv2.Client {
    if e == nil || e.client == nil {
        fmt.Printf("elbv2 service not initialized!")
        os.Exit(1)
    }

    return e.client
}

func (e *ELBv2) GenerateAwsumServiceName(serviceName string) string {
    return fmt.Sprintf("awsum-service-%s", serviceName)
}

type ELBv2SetupInstanceTargetGroupOptions struct {
    Ctx             context.Context
    VpcId           string
    SubnetIds       []string
    ServiceName     string
    Instances       []*Instance
    TrafficPort     uint16
    TrafficProtocol types.ProtocolEnum
    IpProtocol      string
    EC2             *EC2
}

type ELBv2SetupInstanceServiceLoadBalanceResources struct {
    TargetGroupArn   string
    LoadBalancerArn  string
    SecurityGroupArn string
}

func (e *ELBv2) SetupInstanceServiceLoadBalanceResources(opts ELBv2SetupInstanceTargetGroupOptions) (*ELBv2SetupInstanceServiceLoadBalanceResources, error) {
    var (
        targetGroupArn  string
        loadBalancerArn string
        securityGroupId string
    )

    opts.ServiceName = e.GenerateAwsumServiceName(opts.ServiceName)

    tgOutput, err := e.Client().DescribeTargetGroups(opts.Ctx, &elbv2.DescribeTargetGroupsInput{
        Names: []string{opts.ServiceName},
    })

    if err != nil && !strings.Contains(err.Error(), "TargetGroupNotFound") {
        return nil, err
    }

    if tgOutput == nil || len(tgOutput.TargetGroups) == 0 {
        ctgOutput, err := e.Client().CreateTargetGroup(opts.Ctx, &elbv2.CreateTargetGroupInput{
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
            return nil, err
        }

        if len(ctgOutput.TargetGroups) == 0 {
            return nil, errors.New("target group not found after creation")
        }

        targetGroupArn = memory.Unwrap(ctgOutput.TargetGroups[0].TargetGroupArn)
    }

    if tgOutput != nil {
        targetGroupArn = memory.Unwrap(tgOutput.TargetGroups[0].TargetGroupArn)
    }

    dthOutput, err := e.Client().DescribeTargetHealth(opts.Ctx, &elbv2.DescribeTargetHealthInput{
        TargetGroupArn: memory.Pointer(targetGroupArn),
    })

    if err != nil {
        return nil, err
    }

    var targets []types.TargetDescription

    for _, desc := range dthOutput.TargetHealthDescriptions {
        if desc.Target == nil {
            continue
        }

        targets = append(targets, *desc.Target)
    }

    if len(targets) > 0 {
        _, err = e.Client().DeregisterTargets(opts.Ctx, &elbv2.DeregisterTargetsInput{
            TargetGroupArn: memory.Pointer(targetGroupArn),
            Targets:        targets,
        })

        if err != nil {
            return nil, err
        }
    }

    for _, instance := range opts.Instances {
        _, err = e.Client().RegisterTargets(opts.Ctx, &elbv2.RegisterTargetsInput{
            TargetGroupArn: memory.Pointer(targetGroupArn),
            Targets: []types.TargetDescription{
                {Id: instance.Info.InstanceId, Port: memory.Pointer(int32(opts.TrafficPort))},
            },
        })

        if err != nil {
            log.Printf("register target error: %+v\n", err)
            return nil, err
        }
    }

    sg, err := opts.EC2.GetSecurityGroupByName(opts.Ctx, opts.ServiceName)

    if err != nil {
        return nil, err
    }

    if sg != nil {
        securityGroupId = memory.Unwrap(sg.GroupId)
    } else {
        csgOutput, err := opts.EC2.CreateSecurityGroup(opts.Ctx, opts.ServiceName)

        if err != nil {
            return nil, err
        }

        securityGroupId = memory.Unwrap(csgOutput.GroupId)
    }

    _, err = opts.EC2.Client().AuthorizeSecurityGroupIngress(opts.Ctx, &ec2.AuthorizeSecurityGroupIngressInput{
        GroupId: memory.Pointer(securityGroupId),
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

    _, err = opts.EC2.Client().AuthorizeSecurityGroupEgress(opts.Ctx, &ec2.AuthorizeSecurityGroupEgressInput{
        GroupId: memory.Pointer(securityGroupId),
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

    dlbOutput, err := e.Client().DescribeLoadBalancers(opts.Ctx, &elbv2.DescribeLoadBalancersInput{
        Names: []string{opts.ServiceName},
    })

    if err != nil && !strings.Contains(err.Error(), "LoadBalancerNotFound") {
        return nil, err
    }

    if dlbOutput == nil || len(dlbOutput.LoadBalancers) == 0 {
        subnets, err := opts.EC2.GetSubnets(opts.Ctx)

        if err != nil {
            return nil, err
        }

        var subnetIds = make(map[string]ec2Types.Subnet)

        for _, subnet := range subnets {
            if memory.Unwrap(subnet.VpcId) == opts.VpcId {
                subnetIds[memory.Unwrap(subnet.SubnetId)] = subnet
            }
        }

        clbOutput, err := e.Client().CreateLoadBalancer(opts.Ctx, &elbv2.CreateLoadBalancerInput{
            Name:           memory.Pointer(opts.ServiceName),
            Type:           types.LoadBalancerTypeEnumApplication,
            Scheme:         types.LoadBalancerSchemeEnumInternetFacing,
            SecurityGroups: []string{securityGroupId},
            Subnets:        slices.Collect(maps.Keys(subnetIds)),
            IpAddressType:  types.IpAddressTypeIpv4,
        })

        if err != nil {
            return nil, err
        }

        if len(clbOutput.LoadBalancers) == 0 {
            return nil, errors.New("load balancer not found after creation")
        }

        loadBalancerArn = memory.Unwrap(clbOutput.LoadBalancers[0].LoadBalancerArn)
    }

    if dlbOutput != nil {
        loadBalancerArn = memory.Unwrap(dlbOutput.LoadBalancers[0].LoadBalancerArn)
    }

    _, err = e.Client().CreateListener(opts.Ctx, &elbv2.CreateListenerInput{
        LoadBalancerArn: memory.Pointer(loadBalancerArn),
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

    return &ELBv2SetupInstanceServiceLoadBalanceResources{
        TargetGroupArn:   targetGroupArn,
        LoadBalancerArn:  loadBalancerArn,
        SecurityGroupArn: securityGroupId,
    }, err
}

func NewELBv2(awsConfig aws.Config) *ELBv2 {
    return &ELBv2{
        client: elbv2.NewFromConfig(awsConfig),
    }
}
