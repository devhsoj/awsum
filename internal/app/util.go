package app

import (
    "context"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/smithy-go/logging"
    "github.com/devhsoj/awsum/internal/files"
    "github.com/devhsoj/awsum/service"
)

var Ctx = context.Background()

type Resources struct {
    Files []*os.File
}

func (r *Resources) Cleanup() []error {
    var errors []error

    for _, file := range r.Files {
        if err := file.Close(); err != nil {
            errors = append(errors, err)
        }
    }

    return errors
}

func Setup() *Resources {
    sessionStartTime := time.Now()
    awsOutputLogFilename := "awsum-global-aws-log-output"

    sessionAwsOutputLogFilename := fmt.Sprintf(
        "awsum-session-aws-log-output-%s",
        strings.ReplaceAll(strings.ReplaceAll(sessionStartTime.Format(time.DateTime), " ", "__"), ":", "-"),
    )

    globalAwsLogFile, err := files.OpenAwsumFile(awsOutputLogFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

    if err != nil {
        fmt.Printf("failed to open global awsum aws log file: %s\n", err)
        os.Exit(1)
    }

    sessionAwsLogFile, err := files.OpenAwsumFile(sessionAwsOutputLogFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

    if err != nil {
        fmt.Printf("failed to open global awsum aws log file: %s\n", err)
        os.Exit(1)
    }

    awsConfig, err := config.LoadDefaultConfig(
        Ctx,
        config.WithLogger(logging.NewStandardLogger(io.MultiWriter(globalAwsLogFile, sessionAwsLogFile))),
        config.WithClientLogMode(
            aws.LogRetries|aws.LogRequest|aws.LogResponse|aws.LogRequestWithBody|aws.LogResponseWithBody,
        ),
    )

    if err != nil {
        fmt.Printf("failed to load aws config: %s\n", err)
        os.Exit(1)
    }

    service.Setup(awsConfig)

    return &Resources{
        Files: []*os.File{
            globalAwsLogFile,
            sessionAwsLogFile,
        },
    }
}
