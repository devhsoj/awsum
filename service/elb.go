package service

import (
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/aws/aws-sdk-go-v2/aws"
    elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
    "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
    "github.com/devhsoj/awsum/internal/memory"
)

type ELBv2 struct {
    client *elbv2.Client
}

func (svc *ELBv2) Client() *elbv2.Client {
    if svc == nil || svc.client == nil {
        fmt.Printf("elbv2 service not initialized!")
        os.Exit(1)
    }

    return svc.client
}

func (svc *ELBv2) GenerateAwsumServiceName(serviceName string) string {
    return fmt.Sprintf("awsum-service-%s", serviceName)
}

func (svc *ELBv2) DeregisterAllTargetsInTargetGroup(ctx context.Context, targetGroupArn string) error {
    dthOutput, err := svc.Client().DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{
        TargetGroupArn: memory.Pointer(targetGroupArn),
    })

    if err != nil {
        return err
    }

    var targets []types.TargetDescription

    for _, desc := range dthOutput.TargetHealthDescriptions {
        if desc.Target == nil {
            continue
        }

        targets = append(targets, *desc.Target)
    }

    // to prevent annoying client error
    if len(targets) > 0 {
        _, err = svc.Client().DeregisterTargets(ctx, &elbv2.DeregisterTargetsInput{
            TargetGroupArn: memory.Pointer(targetGroupArn),
            Targets:        targets,
        })
    }

    return err
}

func (svc *ELBv2) GetAllListenersInLoadBalancer(ctx context.Context, loadBalancerArn string) ([]types.Listener, error) {
    var (
        dlOutput  *elbv2.DescribeListenersOutput
        arn       = memory.Pointer(loadBalancerArn)
        listeners []types.Listener
        marker    *string
        err       error
    )

    for {
        dlOutput, err = svc.Client().DescribeListeners(ctx, &elbv2.DescribeListenersInput{
            LoadBalancerArn: arn,
            Marker:          marker,
            PageSize:        nil,
        })

        if err != nil {
            return nil, err
        }

        listeners = append(listeners, dlOutput.Listeners...)
        marker = dlOutput.NextMarker

        if marker == nil {
            break
        }
    }

    return listeners, err
}

func (svc *ELBv2) DeleteAllListenersInLoadBalancer(ctx context.Context, loadBalancerArn string) error {
    listeners, err := svc.GetAllListenersInLoadBalancer(ctx, loadBalancerArn)

    if err != nil {
        return err
    }

    for _, listener := range listeners {
        _, err = svc.Client().DeleteListener(ctx, &elbv2.DeleteListenerInput{
            ListenerArn: listener.ListenerArn,
        })

        if err != nil {
            return err
        }
    }

    return nil
}

func (svc *ELBv2) GetLoadBalancerByName(ctx context.Context, name string) (*types.LoadBalancer, error) {
    dlbOutput, err := svc.Client().DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{
        Names: []string{name},
    })

    if err != nil {
        if strings.Contains(err.Error(), "LoadBalancerNotFound") {
            return nil, nil
        }

        return nil, err
    }

    return &dlbOutput.LoadBalancers[0], nil
}

func NewELBv2(awsConfig aws.Config) *ELBv2 {
    return &ELBv2{
        client: elbv2.NewFromConfig(awsConfig),
    }
}
