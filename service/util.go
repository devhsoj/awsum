package service

import "github.com/aws/aws-sdk-go-v2/aws"

func Setup(awsConfig aws.Config) {
    DefaultEC2 = NewEC2(awsConfig)
}
