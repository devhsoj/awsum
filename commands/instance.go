package commands

import (
    "context"
    "encoding/csv"
    "errors"
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
                instance.GetFormattedType(),
                instance.GetFormattedBestIpAddress(),
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
                instance.GetFormattedType(),
                instance.GetFormattedBestIpAddress(),
                util.Unwrap(instance.EC2.KeyName),
            }); err != nil {
                return fmt.Errorf("failed to build instance list table: %w", err)
            }
        }

        return table.Render()
    }

    return nil
}

func Shell(ctx context.Context, awsConfig aws.Config, user string, filters service.InstanceFilters) error {
    instances, err := service.GetInstances(ctx, awsConfig)

    if err != nil {
        return err
    }

    var selectedInstance service.Instance

    for _, instance := range instances {
        if filters.DoesMatch(instance) {
            selectedInstance = instance
            break
        }
    }

    if !selectedInstance.IsValid() {
        return errors.New("failed to find an instance to start a shell based on the given filters")
    }

    return selectedInstance.StartShell(user)
}
