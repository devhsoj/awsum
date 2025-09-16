package service

import (
    "context"
    "errors"
    "fmt"
    "io"
    "os"
    "os/signal"
    "path"
    "strings"
    "syscall"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    "github.com/aws/aws-sdk-go-v2/service/ec2/types"
    "github.com/devhsoj/awsum/internal/files"
    "github.com/devhsoj/awsum/internal/mem"
    "golang.org/x/crypto/ssh"
    "golang.org/x/term"
)

type Instance struct {
    EC2       types.Instance
    AWSConfig aws.Config
}

// GetFormattedBestIpAddress returns a string containing the 'best' ip address to display for the instance. By 'best',
// meaning return the EC2 instance's public ip address if it is available, if not, return the private ip address.
func (i *Instance) GetFormattedBestIpAddress() string {
    var ip = mem.Unwrap(i.EC2.PublicIpAddress)

    if len(ip) == 0 {
        ip = mem.Unwrap(i.EC2.PrivateIpAddress)
    }

    return ip
}

func (i *Instance) GetName() string {
    var name string

    for _, tag := range i.EC2.Tags {
        if mem.Unwrap(tag.Key) == "Name" {
            name = mem.Unwrap(tag.Value)
            break
        }
    }

    return name
}

func (i *Instance) GetFormattedType() string {
    return fmt.Sprintf("%s (%s %s)", i.EC2.InstanceType, i.EC2.Architecture, mem.Unwrap(i.EC2.PlatformDetails))
}

// GenerateSSHClientConfigFromAssumedUserKey generates an ssh client config with keys from the user's ssh directory.
// Assumed to be '~/.ssh'. The given user will be used in authentication.
func (i *Instance) GenerateSSHClientConfigFromAssumedUserKey(user string) (*ssh.ClientConfig, error) {
    homeDir, err := os.UserHomeDir()

    if err != nil {
        return nil, fmt.Errorf("failed to get user home dir while searching for private key: %w", err)
    }

    assumedSSHDirName := path.Join(homeDir, ".ssh")
    assumedKeyFilename := path.Join(assumedSSHDirName, fmt.Sprintf("%s.pem", mem.Unwrap(i.EC2.KeyName)))

    privateKeyBuf, err := files.ReadFileFull(assumedKeyFilename)

    if err != nil {
        return nil, fmt.Errorf("failed to parse user ssh private key '%s': %w", assumedKeyFilename, err)
    }

    hostKeyCallback, err := files.GenerateHostKeyCallbackFromKnownHosts()

    signer, err := ssh.ParsePrivateKey(privateKeyBuf)

    if err != nil {
        return nil, fmt.Errorf("failed to parse user ssh private key '%s' as PEM: %w", assumedKeyFilename, err)
    }

    return &ssh.ClientConfig{
        User: user,
        Auth: []ssh.AuthMethod{
            ssh.PublicKeys(signer),
        },
        HostKeyCallback: hostKeyCallback,
        Timeout:         time.Second * 10,
    }, nil
}

func (i *Instance) DialSSH(user string) (*ssh.Client, error) {
    config, err := i.GenerateSSHClientConfigFromAssumedUserKey(user)

    if err != nil {
        return nil, fmt.Errorf("failed to dial ssh: %w", err)
    }

    client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", mem.Unwrap(i.EC2.PublicDnsName)), config)

    if err != nil {
        return nil, fmt.Errorf("failed to start ssh connection: %w", err)
    }

    return client, nil
}

func (i *Instance) AttachShell(sshUser string) error {
    client, err := i.DialSSH(sshUser)

    if err != nil {
        return fmt.Errorf("failed to create ssh client while connecting to instance: %w", err)
    }

    defer func() {
        if err = client.Close(); err != nil && !errors.Is(err, io.EOF) {
            fmt.Printf("failed to properly close ssh client connection to instance: %s\n", err)
        }
    }()

    session, err := client.NewSession()

    if err != nil {
        return fmt.Errorf("failed to create ssh session while connecting to instance: %w", err)
    }

    defer func() {
        if err = session.Close(); err != nil && !errors.Is(err, io.EOF) {
            fmt.Printf("failed to properly close ssh client session to instance: %s\n", err)
        }
    }()

    session.Stdout = os.Stdout
    session.Stderr = os.Stderr
    session.Stdin = os.Stdin

    fd := int(os.Stdin.Fd())

    var (
        width  = 80
        height = 24
        inTerm = term.IsTerminal(fd)
    )

    if inTerm {
        width, height, err = term.GetSize(fd)

        if err != nil {
            width, height = 80, 24
        }
    }

    desiredTerm := os.Getenv("TERM")

    if len(desiredTerm) == 0 {
        desiredTerm = "xterm-256color"
    }

    if inTerm {
        oldState, err := term.MakeRaw(fd)

        if err == nil && oldState != nil {
            defer func() {
                if err = term.Restore(fd, oldState); err != nil {
                    fmt.Printf("failed to restore old local terminal state while disconnecting from instance: %s", err)
                }
            }()
        }
    }

    resize := make(chan os.Signal, 1)

    signal.Notify(resize, syscall.SIGWINCH)

    go func() {
        for range resize {
            if width, height, err = term.GetSize(fd); err == nil {
                _ = session.WindowChange(height, width)
            }
        }
    }()

    quitSignals := make(chan os.Signal, 1)
    signal.Notify(quitSignals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

    go func() {
        for s := range quitSignals {
            switch s {
            case syscall.SIGINT:
                _ = session.Signal(ssh.SIGINT)
            case syscall.SIGTERM:
                _ = session.Signal(ssh.SIGTERM)
            case syscall.SIGQUIT:
                _ = session.Signal(ssh.SIGQUIT)
            }
        }
    }()

    if err = session.RequestPty(desiredTerm, height, width, ssh.TerminalModes{
        ssh.ECHO:          1,
        ssh.IUTF8:         1,
        ssh.TTY_OP_ISPEED: 115_200,
        ssh.TTY_OP_OSPEED: 115_200,
    }); err != nil {
        return fmt.Errorf("failed to request pty while connecting to instance: %w", err)
    }

    if err = session.Shell(); err != nil {
        return fmt.Errorf("failed to open shell to instance: %w", err)
    }

    if err = session.Wait(); err != nil {
        return fmt.Errorf("failed to wait on session to instance: %w", err)
    }

    return nil
}

func (i *Instance) RunInteractiveCommand(sshUser string, command string) error {
    client, err := i.DialSSH(sshUser)

    if err != nil {
        return fmt.Errorf("failed to create ssh client while connecting to instance: %w", err)
    }

    defer func() {
        if err = client.Close(); err != nil && !errors.Is(err, io.EOF) {
            fmt.Printf("failed to properly close ssh client connection to instance: %s\n", err)
        }
    }()

    session, err := client.NewSession()

    if err != nil {
        return fmt.Errorf("failed to create ssh session while connecting to instance: %w", err)
    }

    defer func() {
        if err = session.Close(); err != nil && !errors.Is(err, io.EOF) {
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

func GetRunningInstances(ctx context.Context, awsConfig aws.Config) ([]*Instance, error) {
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
                if instance.State != nil && mem.Unwrap(instance.State.Code) == 16 {
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
