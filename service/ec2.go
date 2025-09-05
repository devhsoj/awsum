package service

import (
    "context"
    "fmt"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    "github.com/aws/aws-sdk-go-v2/service/ec2/types"
    "github.com/devhsoj/awsum/util"
)

type Instance struct {
    EC2 types.Instance
}

func (i Instance) GetIpAddress() string {
    var ip = util.Unwrap(i.EC2.PublicIpAddress)

    if len(ip) == 0 {
        ip = util.Unwrap(i.EC2.PrivateIpAddress)
    }

    return ip
}

func (i Instance) GetName() string {
    var name string

    for _, tag := range i.EC2.Tags {
        if util.Unwrap(tag.Key) == "Name" {
            name = util.Unwrap(tag.Value)
            break
        }
    }

    return name
}

func (i Instance) GetType() string {
    return fmt.Sprintf("%s (%s %s)", i.EC2.InstanceType, i.EC2.Architecture, util.Unwrap(i.EC2.PlatformDetails))
}

func NewInstanceFromEC2(ec2Instance types.Instance) Instance {
    return Instance{
        EC2: ec2Instance,
    }
}

func GetInstances(ctx context.Context, awsConfig aws.Config) ([]Instance, error) {
    svc := ec2.NewFromConfig(awsConfig)

    var (
        instances []Instance
        nextToken *string
    )

    for {
        output, err := svc.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
            NextToken: nextToken,
        })

        if err != nil {
            return nil, fmt.Errorf("failed to get instances: %w", err)
        }

        for _, reservation := range output.Reservations {
            for _, instance := range reservation.Instances {
                instances = append(instances, Instance{
                    EC2: instance,
                })
            }
        }

        nextToken = output.NextToken

        if nextToken == nil {
            break
        }
    }

    return instances, nil
}
