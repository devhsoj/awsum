package commands

import (
    "context"
    "fmt"
    "os"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/devhsoj/awsum/service"
    "github.com/devhsoj/awsum/util"
    "github.com/olekukonko/tablewriter"
)

func List(ctx context.Context, awsConfig aws.Config) error {
    instances, err := service.GetInstances(ctx, awsConfig)

    if err != nil {
        return err
    }

    table := tablewriter.NewTable(os.Stdout)

    table.Header("ID", "Name", "Type", "IP", "SSH Keypair")

    for _, instance := range instances {
        if err = table.Append(
            util.Unwrap(instance.EC2.InstanceId),
            instance.GetName(),
            instance.GetType(),
            instance.GetIpAddress(),
            util.Unwrap(instance.EC2.KeyName),
        ); err != nil {
            return fmt.Errorf("failed to build instance list table: %w", err)
        }
    }

    return table.Render()
}
