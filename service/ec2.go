package service

import (
    "context"
    "errors"
    "fmt"
    "io"
    "os"
    "path"
    "strings"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    "github.com/aws/aws-sdk-go-v2/service/ec2/types"
    "github.com/devhsoj/awsum/util"
    "golang.org/x/crypto/ssh"
)

type Instance struct {
    EC2       types.Instance
    AWSConfig aws.Config
}

// GetFormattedBestIpAddress returns a string containing the 'best' ip address to display for the instance. By 'best',
// meaning return the EC2 instance's public ip address if it is available, if not, return the private ip address.
func (i *Instance) GetFormattedBestIpAddress() string {
    var ip = util.Unwrap(i.EC2.PublicIpAddress)

    if len(ip) == 0 {
        ip = util.Unwrap(i.EC2.PrivateIpAddress)
    }

    return ip
}

func (i *Instance) GetName() string {
    var name string

    for _, tag := range i.EC2.Tags {
        if util.Unwrap(tag.Key) == "Name" {
            name = util.Unwrap(tag.Value)
            break
        }
    }

    return name
}

func (i *Instance) GetFormattedType() string {
    return fmt.Sprintf("%s (%s %s)", i.EC2.InstanceType, i.EC2.Architecture, util.Unwrap(i.EC2.PlatformDetails))
}

// GenerateSSHClientConfigFromAssumedUserKey generates an ssh client config with keys from the user's ssh directory.
// Assumed to be '~/.ssh'. The given user will be used in authentication.
func (i *Instance) GenerateSSHClientConfigFromAssumedUserKey(user string) (*ssh.ClientConfig, error) {
    homeDir, err := os.UserHomeDir()

    if err != nil {
        return nil, fmt.Errorf("failed to get user home dir while searching for private key: %w", err)
    }

    assumedSSHDirName := path.Join(homeDir, ".ssh")
    assumedKeyFilename := path.Join(assumedSSHDirName, fmt.Sprintf("%s.pem", util.Unwrap(i.EC2.KeyName)))

    keyFile, err := os.OpenFile(assumedKeyFilename, os.O_RDONLY, 0400)

    if err != nil {
        return nil, fmt.Errorf("failed to open ssh private key file: %w", err)
    }

    defer func() {
        if err := keyFile.Close(); err != nil {
            fmt.Printf("failed to properly close user ssh private key: %s\n", err)
        }
    }()

    keyBuf, err := io.ReadAll(keyFile)

    if err != nil {
        return nil, fmt.Errorf("failed to read user ssh private key: %w", err)
    }

    signer, err := ssh.ParsePrivateKey(keyBuf)

    if err != nil {
        return nil, fmt.Errorf("failed to parse user ssh private key (%s) as PEM: %w", assumedKeyFilename, err)
    }

    return &ssh.ClientConfig{
        User: user,
        Auth: []ssh.AuthMethod{
            ssh.PublicKeys(signer),
        },
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         0,
    }, nil
}

func (i *Instance) DialSSH(user string) (*ssh.Client, error) {
    config, err := i.GenerateSSHClientConfigFromAssumedUserKey(user)

    if err != nil {
        return nil, fmt.Errorf("failed to dial ssh: %w", err)
    }

    client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", util.Unwrap(i.EC2.PublicDnsName)), config)

    if err != nil {
        return nil, fmt.Errorf("failed to start ssh connection: %w", err)
    }

    return client, nil
}

func (i *Instance) StartShell(user string) error {
    client, err := i.DialSSH(user)

    if err != nil {
        return fmt.Errorf("failed to create ssh client while connecting to instance: %w", err)
    }

    defer func() {
        if err := client.Close(); err != nil && !errors.Is(err, io.EOF) {
            fmt.Printf("failed to properly close ssh client connection to instance: %s\n", err)
        }
    }()

    session, err := client.NewSession()

    if err != nil {
        return fmt.Errorf("failed to create ssh session while connecting to instance: %w", err)
    }

    defer func() {
        if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
            fmt.Printf("failed to properly close ssh client session to instance: %s\n", err)
        }
    }()

    session.Stdout = os.Stdout
    session.Stderr = os.Stderr
    session.Stdin = os.Stdin

    if err = session.RequestPty("xterm", 96, 24, ssh.TerminalModes{}); err != nil {
        return fmt.Errorf("failed to request pty from instance: %w", err)
    }

    if err = session.Shell(); err != nil {
        return fmt.Errorf("failed to open shell to instance: %w", err)
    }

    if err = session.Wait(); err != nil {
        return fmt.Errorf("failed to wait on session to instance: %w", err)
    }

    return nil
}

func (i *Instance) RunCommand(user string, command string) error {
    client, err := i.DialSSH(user)

    if err != nil {
        return fmt.Errorf("failed to create ssh client while connecting to instance: %w", err)
    }

    defer func() {
        if err := client.Close(); err != nil && !errors.Is(err, io.EOF) {
            fmt.Printf("failed to properly close ssh client connection to instance: %s\n", err)
        }
    }()

    session, err := client.NewSession()

    if err != nil {
        return fmt.Errorf("failed to create ssh session while connecting to instance: %w", err)
    }

    defer func() {
        if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
            fmt.Printf("failed to properly close ssh client session to instance: %s\n", err)
        }
    }()

    session.Stdout = os.Stdout
    session.Stderr = os.Stderr
    session.Stdin = os.Stdin

    if err = session.Run(command); err != nil {
        return fmt.Errorf("failed to run command on instance: %w", err)
    }

    return nil
}

func NewInstanceFromEC2(ec2Instance types.Instance, awsConfig aws.Config) *Instance {
    return &Instance{
        EC2:       ec2Instance,
        AWSConfig: awsConfig,
    }
}

type InstanceFilters struct {
    Name string
}

func (f InstanceFilters) DoesMatch(instance *Instance) bool {
    if instance == nil {
        return false
    }

    if len(f.Name) > 0 {
        if strings.Contains(instance.GetName(), f.Name) {
            return true
        }
    }

    return false
}

func GetInstances(ctx context.Context, awsConfig aws.Config) ([]*Instance, error) {
    svc := ec2.NewFromConfig(awsConfig)

    var (
        instances []*Instance
        nextToken *string
    )

    for {
        output, err := svc.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
            NextToken: nextToken,
        })

        if err != nil {
            return nil, fmt.Errorf("failed to get instances: %w", err)
        }

        for _, reservation := range output.Reservations {
            for _, instance := range reservation.Instances {
                // make sure the instance is absolutely running (16 is the instance state code for running)
                if instance.State != nil && util.Unwrap(instance.State.Code) == 16 {
                    instances = append(instances, NewInstanceFromEC2(instance, awsConfig))
                }
            }
        }

        nextToken = output.NextToken

        if nextToken == nil {
            break
        }
    }

    return instances, nil
}
