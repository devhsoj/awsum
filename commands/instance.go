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
    Ctx                    context.Context
    ServiceName            string
    InstanceFilters        service.InstanceFilters
    LoadBalancerIpProtocol string
    LoadBalancerPort       int32
    TrafficPort            int32
    TrafficProtocol        types.ProtocolEnum
    CertificateNames       []string
}

func InstanceLoadBalance(opts InstanceLoadBalanceOptions) error {
    resources, err := service.DefaultAwsumILB.SetupNewILBService(service.SetupNewILBServiceOptions{
        Ctx:                    opts.Ctx,
        ServiceName:            opts.ServiceName,
        TargetInstanceFilters:  opts.InstanceFilters,
        LoadBalancerPort:       opts.LoadBalancerPort,
        LoadBalancerIpProtocol: opts.LoadBalancerIpProtocol,
        TrafficPort:            opts.TrafficPort,
        TrafficProtocol:        opts.TrafficProtocol,
        CertificateNames:       opts.CertificateNames,
    })

    if err != nil {
        return err
    }

    output := memory.Unwrap(resources.LoadBalancer.DNSName)

    if len(opts.CertificateNames) > 0 {
        output = opts.CertificateNames[0]
    }

    switch opts.TrafficProtocol {
    case types.ProtocolEnumTcp:
        fmt.Printf("tcp://%s\n", output)
    case types.ProtocolEnumUdp:
        fmt.Printf("udp://%s\n", output)
    case types.ProtocolEnumHttp:
        fmt.Printf("http://%s\n", output)
    case types.ProtocolEnumHttps:
        fmt.Printf("https://%s\n", output)
    default:
        fmt.Println(output)
    }

    return nil
}
