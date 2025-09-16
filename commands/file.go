package commands

import (
    "bytes"
    "context"
    "fmt"
    "os"
    "path"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/s3/types"
    "github.com/devhsoj/awsum/internal/files"
    "github.com/devhsoj/awsum/internal/mem"
    "github.com/devhsoj/awsum/service"
)

func StoreFileItems(ctx context.Context, awsConfig aws.Config, prefix string, items ...string) error {
    workingPath := path.Dir(prefix)
    svc := s3.NewFromConfig(awsConfig)

    bucket, err := service.GetBucketByName(ctx, awsConfig, service.S3FileBucketName)

    if err != nil {
        return err
    }

    for _, item := range items {
        itemPath := path.Join(workingPath, item)
        itemInfo, err := os.Stat(itemPath)

        if err != nil {
            return fmt.Errorf("failed to stat item before storing in s3: %w", err)
        }

        if itemInfo.IsDir() {
            entries, err := os.ReadDir(itemInfo.Name())

            if err != nil {
                return fmt.Errorf("failed to read dir entries before storing in s3: %w", err)
            }

            var entryItems []string

            for _, entry := range entries {
                entryItems = append(entryItems, path.Join(itemInfo.Name(), entry.Name()))
            }

            if err = StoreFileItems(
                ctx,
                awsConfig,
                path.Join(prefix, itemInfo.Name()),
                entryItems...,
            ); err != nil {
                return err
            }

            continue
        }

        buf, err := files.ReadFileFull(itemPath)

        if err != nil {
            return err
        }

        _, err = svc.PutObject(ctx, &s3.PutObjectInput{
            Bucket: bucket.S3.Name,
            Key:    mem.Pointer(path.Join(prefix, itemInfo.Name())),
            ACL:    types.ObjectCannedACLPrivate,
            Body:   bytes.NewReader(buf),
        })

        if err != nil {
            return fmt.Errorf("failed to store s3 file object: %w", err)
        }
    }

    return nil
}
