package service

import (
    "context"
    "errors"
    "fmt"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/s3/types"
    "github.com/devhsoj/awsum/internal/mem"
)

const (
    S3FileBucketName string = "awsum-files"
)

type Bucket struct {
    S3        types.Bucket
    AWSConfig aws.Config
}

func NewBucketFromAWSBucket(output types.Bucket, awsConfig aws.Config) *Bucket {
    return &Bucket{
        S3:        output,
        AWSConfig: awsConfig,
    }
}

func GetBuckets(ctx context.Context, awsConfig aws.Config) ([]*Bucket, error) {
    svc := s3.NewFromConfig(awsConfig)

    var (
        buckets           []*Bucket
        continuationToken *string
    )

    for {
        output, err := svc.ListBuckets(ctx, &s3.ListBucketsInput{
            ContinuationToken: continuationToken,
        })

        if err != nil {
            return nil, fmt.Errorf("failed to get buckets: %w", err)
        }

        for _, bucket := range output.Buckets {
            buckets = append(buckets, NewBucketFromAWSBucket(bucket, awsConfig))
        }

        continuationToken = output.ContinuationToken

        if continuationToken == nil {
            break
        }
    }

    return buckets, nil
}

var (
    ErrNoBucketMatchesName = errors.New("no bucket matches given name")
)

func GetBucketByName(ctx context.Context, awsConfig aws.Config, name string) (*Bucket, error) {
    buckets, err := GetBuckets(ctx, awsConfig)

    if err != nil {
        return nil, err
    }

    for _, bucket := range buckets {
        if mem.Unwrap(bucket.S3.Name) == name {
            return bucket, nil
        }
    }

    return nil, ErrNoBucketMatchesName
}

func GetAllBucketObjects(ctx context.Context, awsConfig aws.Config, bucketName string) ([]types.Object, error) {
    svc := s3.NewFromConfig(awsConfig)

    var (
        objects           []types.Object
        continuationToken *string
    )

    for {
        output, err := svc.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
            Bucket:            mem.Pointer(bucketName),
            ContinuationToken: continuationToken,
        })

        if err != nil {
            return nil, fmt.Errorf("failed to get buckets: %w", err)
        }

        for _, object := range output.Contents {
            objects = append(objects, object)
        }

        continuationToken = output.ContinuationToken

        if continuationToken == nil {
            break
        }
    }

    return objects, nil
}
