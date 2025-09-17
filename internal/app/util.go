package app

import (
    "context"
    "fmt"
    "os"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/devhsoj/awsum/service"
)

var Ctx = context.Background()

func Setup() {
    awsConfig, err := config.LoadDefaultConfig(Ctx)

    if err != nil {
        fmt.Printf("failed to load aws config: %s\n", err)
        os.Exit(1)
    }

    service.Setup(awsConfig)
}
