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
                                Name:     "user",
                                Aliases:  []string{"as"},
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
                                User:    command.String("user"),
                                Command: strings.Join(command.Args().Slice(), " "),
                            })
                        },
                    },
                },
            },
            {
                Name: "file",
                Commands: []*cli.Command{
                    {
                        Name: "store",
                        Flags: []cli.Flag{
                            &cli.StringFlag{
                                Name:      "prefix",
                                Usage:     "",
                                Required:  false,
                                Aliases:   []string{"p"},
                                Validator: nil,
                            },
                        },
                        Action: func(ctx context.Context, command *cli.Command) error {
                            return commands.StoreFileItems(
                                ctx,
                                awsConfig,
                                command.String("prefix"),
                                command.Args().Slice()...,
                            )
                        },
                    },
                    {
                        Name:    "retrieve",
                        Aliases: []string{"get", "fetch"},
                        Arguments: []cli.Argument{
                            &cli.StringArg{
                                Name:      "key",
                                UsageText: "The key to match (which is the filename or directory path when files are stored from awsum) of the file s3 object(s) to be retrieved.",
                            },
                        },
                        Action: func(ctx context.Context, command *cli.Command) error {
                            return nil
                        },
                    },
                    {
                        Name: "delete",
                        Arguments: []cli.Argument{
                            &cli.StringArg{
                                Name:      "filename",
                                UsageText: "The filename of the file s3 object.",
                            },
                            &cli.StringArg{
                                Name:      "prefix",
                                UsageText: "The S3 prefix to prepend to the filename for the resulting S3 object. Commonly used to represent directories.",
                            },
                        },
                        Action: func(ctx context.Context, command *cli.Command) error {
                            return nil
                        },
                    },
                    {
                        Name: "list",
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
                            return nil
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
