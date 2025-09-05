package commands

import (
    "context"
    "encoding/csv"
    "fmt"
    "os"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/devhsoj/awsum/service"
    "github.com/devhsoj/awsum/util"
    "github.com/olekukonko/tablewriter"
)

func List(ctx context.Context, awsConfig aws.Config, format string) error {
    instances, err := service.GetInstances(ctx, awsConfig)

    if err != nil {
        return err
    }

    if format == "csv" {
        w := csv.NewWriter(os.Stdout)

        if err = w.Write([]string{
            "ID",
            "Name",
            "Type",
            "IP",
            "Key",
        }); err != nil {
            return fmt.Errorf("failed to write instance header csv record: %w", err)
        }

        for _, instance := range instances {
            if err = w.Write([]string{
                util.Unwrap(instance.EC2.InstanceId),
                instance.GetName(),
                instance.GetType(),
                instance.GetIpAddress(),
                util.Unwrap(instance.EC2.KeyName),
            }); err != nil {
                return fmt.Errorf("failed to write instance csv record: %w", err)
            }
        }

        w.Flush()
    } else if format == "pretty" {
        table := tablewriter.NewWriter(os.Stdout)

        table.Header([]string{
            "ID",
            "Name",
            "Type",
            "IP",
            "Key",
        })

        for _, instance := range instances {
            if err = table.Append([]string{
                util.Unwrap(instance.EC2.InstanceId),
                instance.GetName(),
                instance.GetType(),
                instance.GetIpAddress(),
                util.Unwrap(instance.EC2.KeyName),
            }); err != nil {
                return fmt.Errorf("failed to build instance list table: %w", err)
            }
        }

        return table.Render()
    }

    return nil
}
