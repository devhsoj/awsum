package main

import (
    "context"
    "fmt"
    "os"

    "github.com/aws/aws-sdk-go-v2/config"
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
        Description: "a fun tool for managing AWS infrastructure",
        HideHelp:    true,
        Action: func(ctx context.Context, command *cli.Command) error {
            fmt.Printf(
                "(%s) run `awsum help` for a guide on how to use awsum!\n",
                awsConfig.Region,
            )
            return nil
        },
    }

    if err = cmd.Run(ctx, os.Args); err != nil {
        fmt.Printf("failed to run command: %s\n", err)
        os.Exit(1)
    }
}
