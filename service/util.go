package service

import "github.com/aws/aws-sdk-go-v2/aws"

var (
    DefaultEC2      *EC2
    DefaultELBv2    *ELBv2
    DefaultACM      *ACM
    DefaultAwsumILB *AwsumILBService
)

func Setup(awsConfig aws.Config) {
    DefaultEC2 = NewEC2(awsConfig)
    DefaultELBv2 = NewELBv2(awsConfig)
    DefaultACM = NewACM(awsConfig)
    DefaultAwsumILB = NewAwsumILBService(awsConfig)
}
