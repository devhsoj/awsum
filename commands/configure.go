package commands

import (
    "fmt"
    "os"
    "path"

    "github.com/devhsoj/awsum/internal/files"
)

const basicDefaultAwsCLIConfigContents = `[default]
region = %s
`

const basicDefaultAwsCLICredentialsContents = `[default]
aws_access_key_id = %s
aws_secret_access_key = %s
`

func input[T any](prompt string, inputFormat string) (T, error) {
    var val T

    fmt.Print(prompt)

    if _, err := fmt.Scanf(inputFormat, &val); err != nil {
        return val, err
    }

    return val, nil
}

func Configure() error {
    homeDir, err := os.UserHomeDir()

    if err != nil {
        return err
    }

    awsPath := path.Join(homeDir, ".aws")

    if err = os.MkdirAll(awsPath, 0755); err != nil {
        return err
    }

    fmt.Println(awsPath)

    if _, err = files.CreateAwsumDataDirectory(); err != nil {
        return err
    }

    awsConfigPath := path.Join(awsPath, "config")
    awsCredentialsPath := path.Join(awsPath, "credentials")

    configFile, err := os.Open(awsConfigPath)

    if os.IsNotExist(err) {
        awsRegion, err := input[string]("AWS Region: ", "%s")

        if err != nil {
            return err
        }

        if err = files.WriteToFile(
            awsConfigPath,
            []byte(fmt.Sprintf(basicDefaultAwsCLIConfigContents, awsRegion)),
        ); err != nil {
            return err
        }
    }

    defer func() {
        if configFile != nil {
            if err = configFile.Close(); err != nil {
                fmt.Printf("failed to properly close aws config file '%s': %s", awsConfigPath, err)
            }
        }
    }()

    credentialsFile, err := os.Open(awsCredentialsPath)

    if os.IsNotExist(err) {
        awsAccessKeyId, err := input[string]("AWS Access Key ID: ", "%s")

        if err != nil {
            return err
        }

        awsSecretAccessKey, err := input[string]("AWS Secret Access Key: ", "%s")

        if err != nil {
            return err
        }

        if err = files.WriteToFile(
            awsCredentialsPath,
            []byte(
                fmt.Sprintf(
                    basicDefaultAwsCLICredentialsContents,
                    awsAccessKeyId,
                    awsSecretAccessKey,
                ),
            ),
        ); err != nil {
            return err
        }
    }

    defer func() {
        if credentialsFile != nil {
            if err = credentialsFile.Close(); err != nil {
                fmt.Printf("failed to properly close aws credentials file '%s': %s", awsCredentialsPath, err)
            }
        }
    }()

    return nil
}
