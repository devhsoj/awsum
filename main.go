package main

import (
    "context"
    "fmt"
    "os"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/devhsoj/awsum/commands"
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
                        Action: func(ctx context.Context, command *cli.Command) error {
                            return commands.List(ctx, awsConfig)
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
