package main

import (
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/devhsoj/awsum/commands"
    "github.com/devhsoj/awsum/internal/app"
    "github.com/devhsoj/awsum/service"
    "github.com/urfave/cli/v3"
)

func main() {
    app.Setup()

    cmd := &cli.Command{
        Name:        "awsum",
        Usage:       "a fun CLI tool for working with AWS infra",
        Description: "awsum allows you to rapidly develop with your own infra via the command line",
        Commands: []*cli.Command{
            {
                Name: "instance",
                Commands: []*cli.Command{
                    {
                        Name:  "list",
                        Usage: "display a formatted list of Info instances",
                        Flags: []cli.Flag{
                            &cli.StringFlag{
                                Name:     "format",
                                Usage:    "pretty|csv",
                                Value:    "pretty",
                                OnlyOnce: true,
                                Validator: func(s string) error {
                                    if s != "pretty" && s != "csv" {
                                        return fmt.Errorf("invalid format, must be pretty or csv")
                                    }

                                    return nil
                                },
                                ValidateDefaults: true,
                            },
                        },
                        Action: func(ctx context.Context, command *cli.Command) error {
                            return commands.InstanceList(ctx, command.String("format"))
                        },
                    },
                    {
                        Name:    "shell",
                        Usage:   "run a command or start a shell (via SSH) on ec2 instance(s) matched by the given filters",
                        Suggest: true,
                        Flags: []cli.Flag{
                            &cli.StringFlag{
                                Name:     "user",
                                Aliases:  []string{"as"},
                                Usage:    "which ssh user to connect as",
                                Value:    "ec2-user",
                                OnlyOnce: true,
                            },
                            &cli.StringFlag{
                                Name:     "name",
                                Usage:    "a fuzzy filter that matches against ec2 instance names (from tags)",
                                OnlyOnce: true,
                            },
                        },
                        Action: func(ctx context.Context, command *cli.Command) error {
                            return commands.InstanceShell(commands.InstanceShellOptions{
                                Ctx: ctx,
                                InstanceFilters: service.InstanceFilters{
                                    Name: command.String("name"),
                                },
                                User:    command.String("user"),
                                Command: strings.Join(command.Args().Slice(), " "),
                            })
                        },
                    },
                    {
                        Name:    "load-balance",
                        Usage:   "create or update load balancer resources for a service on desired instances",
                        Suggest: true,
                        Flags: []cli.Flag{
                            &cli.StringFlag{
                                Name:     "service",
                                Usage:    "the name of the service you wish to load-balance",
                                OnlyOnce: true,
                                Required: true,
                            },
                            &cli.StringFlag{
                                Name:     "name",
                                Usage:    "a fuzzy filter that matches against ec2 instance names (from tags)",
                                OnlyOnce: true,
                            },
                            &cli.Uint16Flag{
                                Name:     "traffic-port",
                                Usage:    "the traffic port to load-balance your service between your desired instances",
                                Required: true,
                            },
                        },
                        Action: func(ctx context.Context, command *cli.Command) error {
                            return commands.InstanceLoadBalance(commands.InstanceLoadBalanceOptions{
                                Ctx:         ctx,
                                ServiceName: command.String("service"),
                                InstanceFilters: service.InstanceFilters{
                                    Name: command.String("name"),
                                },
                                TrafficPort: command.Uint16("traffic-port"),
                            })
                        },
                    },
                },
            },
        },
    }

    if err := cmd.Run(app.Ctx, os.Args); err != nil {
        fmt.Printf("failed to run command: %s\n", err)
        os.Exit(1)
    }
}
