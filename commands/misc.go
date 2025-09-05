package commands

import (
    "fmt"

    "github.com/aws/aws-sdk-go-v2/aws"
)

func Intro(awsConfig aws.Config) error {
    _, err := fmt.Printf(
        "(aws configured ✓ %s) run `awsum help` for a guide on how to use awsum :)\n",
        awsConfig.Region,
    )

    return err
}

var helpText = `awsum's functionality is broken down into categories consisting of commands.

usage: awsum [category] (command)
example usage: awsum instance list
help with usage: awsum instance list --help

here is a list of all command categories:
- instance: commands that interact with EC2 instances`

func Help() error {
    _, err := fmt.Println(helpText)

    return err
}
