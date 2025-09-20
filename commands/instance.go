package commands

import (
    "context"
    "encoding/csv"
    "fmt"
    "os"

    "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
    "github.com/devhsoj/awsum/internal/memory"
    "github.com/devhsoj/awsum/service"
    "github.com/olekukonko/tablewriter"
)

func InstanceList(ctx context.Context, format string) error {
    instances, err := service.DefaultEC2.GetAllRunningInstances(ctx)

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
                memory.Unwrap(instance.Info.InstanceId),
                instance.GetName(),
                instance.GetFormattedType(),
                instance.GetFormattedBestIpAddress(),
                memory.Unwrap(instance.Info.KeyName),
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
                memory.Unwrap(instance.Info.InstanceId),
                instance.GetName(),
                instance.GetFormattedType(),
                instance.GetFormattedBestIpAddress(),
                memory.Unwrap(instance.Info.KeyName),
            }); err != nil {
                return fmt.Errorf("failed to build instance list table: %w", err)
            }
        }

        return table.Render()
    }

    return nil
}

type InstanceShellOptions struct {
    Ctx             context.Context
    InstanceFilters service.InstanceFilters
    User            string
    Command         string
}

func InstanceShell(opts InstanceShellOptions) error {
    instances, err := service.DefaultEC2.GetAllRunningInstances(opts.Ctx)

    if err != nil {
        return err
    }

    for _, instance := range opts.InstanceFilters.Matches(instances) {
        if len(opts.Command) == 0 {
            return instance.AttachShell(opts.User)
        }

        fmt.Printf("--- '%s' SHELL START ---\n", instance.GetName())

        if err = instance.RunInteractiveCommand(opts.User, opts.Command); err != nil {
            return err
        }

        fmt.Printf("--- '%s' SHELL END ---\n", instance.GetName())
    }

    return nil
}

type InstanceLoadBalanceOptions struct {
    Ctx             context.Context
    ServiceName     string
    InstanceFilters service.InstanceFilters
    TrafficPort     uint16
    TrafficProtocol types.ProtocolEnum
    IpProtocol      string
}

func InstanceLoadBalance(opts InstanceLoadBalanceOptions) error {
    return service.DefaultAwsumILB.SetupNewILBService(service.SetupNewILBServiceOptions{
        Ctx:                   opts.Ctx,
        ServiceName:           opts.ServiceName,
        TargetInstanceFilters: opts.InstanceFilters,
        TrafficPort:           opts.TrafficPort,
        TrafficProtocol:       opts.TrafficProtocol,
        IpProtocol:            opts.IpProtocol,
    })
}
