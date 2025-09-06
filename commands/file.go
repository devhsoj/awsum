package commands

import (
    "context"
    "fmt"
    "io"
    "os"
    "path"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/s3/types"
    "github.com/devhsoj/awsum/service"
    "github.com/devhsoj/awsum/util"
)

func StoreFile(ctx context.Context, awsConfig aws.Config, filename string, prefix string) error {
    svc := s3.NewFromConfig(awsConfig)

    f, err := os.OpenFile(filename, os.O_RDONLY, 0400)

    if err != nil {
        return fmt.Errorf("failed to open file '%s' to store in bucket: %w", filename, err)
    }

    defer func() {
        if err := f.Close(); err != nil {
            fmt.Printf("failed to properly close file to be stored in s3: %s\n", err)
        }
    }()

    bucket, err := service.GetBucketByName(ctx, awsConfig, service.S3FileBucketName)

    if err != nil {
        return err
    }

    _, err = svc.PutObject(ctx, &s3.PutObjectInput{
        Bucket: bucket.S3.Name,
        Key:    util.Pointer(path.Join(prefix, path.Base(filename))),
        ACL:    types.ObjectCannedACLPrivate,
        Body:   f,
    })

    if err != nil {
        return fmt.Errorf("failed to put file object: %w", err)
    }

    return nil
}

func GetFile(ctx context.Context, awsConfig aws.Config, filename string, prefix string) error {
    svc := s3.NewFromConfig(awsConfig)

    bucket, err := service.GetBucketByName(ctx, awsConfig, service.S3FileBucketName)

    if err != nil {
        return err
    }

    output, err := svc.GetObject(ctx, &s3.GetObjectInput{
        Bucket: bucket.S3.Name,
        Key:    util.Pointer(path.Join(prefix, path.Base(filename))),
    })

    if err != nil {
        return fmt.Errorf("failed to get file s3 object: %w", err)
    }

    if _, err := io.Copy(os.Stdout, output.Body); err != nil {
        return fmt.Errorf("failed to write file s3 object content to stdout: %w", err)
    }

    return nil
}

func DeleteFile(ctx context.Context, awsConfig aws.Config, filename string, prefix string) error {
    svc := s3.NewFromConfig(awsConfig)

    bucket, err := service.GetBucketByName(ctx, awsConfig, service.S3FileBucketName)

    if err != nil {
        return err
    }

    _, err = svc.DeleteObject(ctx, &s3.DeleteObjectInput{
        Bucket: bucket.S3.Name,
        Key:    util.Pointer(path.Join(prefix, path.Base(filename))),
    })

    if err != nil {
        return fmt.Errorf("failed to delete file s3 object: %w", err)
    }

    return nil
}
