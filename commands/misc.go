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
