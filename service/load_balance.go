package service

import (
    "github.com/aws/aws-sdk-go-v2/aws"
)

// AwsumILBService is a struct used to encompass all the logic of services on instances that are load balanced by
// resources created by awsum.
type AwsumILBService struct {
    EC2   *EC2
    ELBv2 *ELBv2
}

func NewAwsumILBService(awsConfig aws.Config) *AwsumILBService {
    return &AwsumILBService{
        EC2:   NewEC2(awsConfig),
        ELBv2: NewELBv2(awsConfig),
    }
}
