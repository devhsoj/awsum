package main

import (
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/devhsoj/awsum/commands"
    "github.com/devhsoj/awsum/service"
    "github.com/urfave/cli/v3"
)

func main() {
    ctx := context.Background()

    awsConfig, err := config.LoadDefaultConfig(ctx)

    if err != nil {
        fmt.Printf("failed to load aws config: %s\n", err)
        os.Exit(1)
    }

    cmd := &cli.Command{
        Name:        "awsum",
        Description: "awsum is a fun CLI tool for managing AWS infrastructure at a high level",
        HideHelp:    true,
        Action: func(ctx context.Context, command *cli.Command) error {
            return commands.Intro(awsConfig)
        },
        Commands: []*cli.Command{
            {
                Name:        "help",
                Description: "show helpful information about awsum and how it works",
                Action: func(ctx context.Context, command *cli.Command) error {
                    return commands.Help()
                },
            },
            {
                Name: "instance",
                Commands: []*cli.Command{
                    {
                        Name:        "list",
                        Description: "list EC2 instances",
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
                            return commands.List(ctx, awsConfig, command.String("format"))
                        },
                    },
                    {
                        Name:        "shell",
                        Description: "run a command or start a shell (via SSH) on EC2 instances matched by the given filters",
                        Flags: []cli.Flag{
                            &cli.StringFlag{
                                Name:     "as",
                                Usage:    "which ssh user to connect as",
                                Value:    "ec2-user",
                                OnlyOnce: true,
                            },
                            &cli.StringFlag{
                                Name:     "name",
                                Usage:    "a fuzzy filter that matches against EC2 instance names (from tags)",
                                OnlyOnce: true,
                            },
                        },
                        Action: func(ctx context.Context, command *cli.Command) error {
                            return commands.StartShell(commands.StartShellOptions{
                                Ctx:       ctx,
                                AWSConfig: awsConfig,
                                InstanceFilters: service.InstanceFilters{
                                    Name: command.String("name"),
                                },
                                User:    command.String("as"),
                                Command: strings.Join(command.Args().Slice(), " "),
                            })
                        },
                    },
                },
            },
        },
    }

    if err = cmd.Run(ctx, os.Args); err != nil {
        fmt.Printf("failed to run command: %s\n", err)
        os.Exit(1)
    }
}
