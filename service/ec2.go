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
    "github.com/devhsoj/awsum/internal/memory"
    "golang.org/x/crypto/ssh"
    "golang.org/x/term"
)

type EC2 struct {
    client *ec2.Client
}

func (e *EC2) Client() *ec2.Client {
    if e == nil || e.client == nil {
        fmt.Printf("ec2 service not initialized!")
        os.Exit(1)
    }

    return e.client
}

func (e *EC2) GetRunningInstances(ctx context.Context) ([]*Instance, error) {
    var (
        instances []*Instance
        nextToken *string
    )

    for {
        output, err := DefaultEC2.Client().DescribeInstances(ctx, &ec2.DescribeInstancesInput{
            NextToken: nextToken,
        })

        if err != nil {
            return nil, fmt.Errorf("failed to get instances: %w", err)
        }

        for _, reservation := range output.Reservations {
            for _, instance := range reservation.Instances {
                // make sure the instance is absolutely running (16 is the instance state code for running)
                if instance.State != nil && memory.Unwrap(instance.State.Code) == 16 {
                    instances = append(instances, NewInstanceFromEC2(instance))
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

func (e *EC2) GetVPCs(ctx context.Context) ([]types.Vpc, error) {
    var (
        vpcs      []types.Vpc
        nextToken *string
    )

    for {
        output, err := DefaultEC2.Client().DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
            NextToken: nextToken,
        })

        if err != nil {
            return nil, fmt.Errorf("failed to get vpcs: %w", err)
        }

        for _, vpc := range output.Vpcs {
            vpcs = append(vpcs, vpc)
        }

        nextToken = output.NextToken

        if nextToken == nil {
            break
        }
    }

    return vpcs, nil
}

func (e *EC2) GetSubnets(ctx context.Context) ([]types.Subnet, error) {
    var (
        subnets   []types.Subnet
        nextToken *string
    )

    for {
        output, err := DefaultEC2.Client().DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
            NextToken: nextToken,
        })

        if err != nil {
            return nil, fmt.Errorf("failed to get subnets: %w", err)
        }

        for _, subnet := range output.Subnets {
            subnets = append(subnets, subnet)
        }

        nextToken = output.NextToken

        if nextToken == nil {
            break
        }
    }

    return subnets, nil
}

func (e *EC2) GetSecurityGroupByName(ctx context.Context, name string) (*types.SecurityGroup, error) {
    var (
        nextToken *string
    )

    for {
        sgOutput, err := e.Client().DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
            GroupNames: []string{name},
            NextToken:  nextToken,
        })

        if err != nil && !strings.Contains(err.Error(), "InvalidGroup.NotFound") {
            return nil, err
        }

        if sgOutput == nil {
            break
        }

        for _, securityGroup := range sgOutput.SecurityGroups {
            if memory.Unwrap(securityGroup.GroupName) == name {
                return &securityGroup, nil
            }
        }

        nextToken = sgOutput.NextToken

        if nextToken == nil {
            break
        }
    }

    return nil, nil
}

func (e *EC2) CreateSecurityGroup(ctx context.Context, name string) (*ec2.CreateSecurityGroupOutput, error) {
    return e.Client().CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
        GroupName:   memory.Pointer(name),
        Description: memory.Pointer("managed by awsum"),
        TagSpecifications: []types.TagSpecification{
            {
                ResourceType: types.ResourceTypeSecurityGroup,
                Tags: []types.Tag{
                    {
                        Key:   memory.Pointer("managed-by"),
                        Value: memory.Pointer("awsum"),
                    },
                },
            },
        },
    })
}

func NewEC2(awsConfig aws.Config) *EC2 {
    return &EC2{
        client: ec2.NewFromConfig(awsConfig),
    }
}

var DefaultEC2 *EC2

type Instance struct {
    Info    types.Instance
    Service *EC2
}

// GetFormattedBestIpAddress returns a string containing the 'best' ip address to display for the instance. By 'best',
// meaning return the EC2 instance's public ip address if it is available, if not, return the private ip address.
func (i *Instance) GetFormattedBestIpAddress() string {
    var ip = memory.Unwrap(i.Info.PublicIpAddress)

    if len(ip) == 0 {
        ip = memory.Unwrap(i.Info.PrivateIpAddress)
    }

    return ip
}

func (i *Instance) GetName() string {
    var name string

    for _, tag := range i.Info.Tags {
        if memory.Unwrap(tag.Key) == "Name" {
            name = memory.Unwrap(tag.Value)
            break
        }
    }

    return name
}

func (i *Instance) GetFormattedType() string {
    return fmt.Sprintf("%s (%s %s)", i.Info.InstanceType, i.Info.Architecture, memory.Unwrap(i.Info.PlatformDetails))
}

// GenerateSSHClientConfigFromAssumedUserKey generates an ssh client config with keys from the user's ssh directory.
// Assumed to be '~/.ssh'. The given user will be used in authentication.
func (i *Instance) GenerateSSHClientConfigFromAssumedUserKey(user string) (*ssh.ClientConfig, error) {
    homeDir, err := os.UserHomeDir()

    if err != nil {
        return nil, fmt.Errorf("failed to get user home dir while searching for private key: %w", err)
    }

    assumedSSHDirName := path.Join(homeDir, ".ssh")
    assumedKeyFilename := path.Join(assumedSSHDirName, fmt.Sprintf("%s.pem", memory.Unwrap(i.Info.KeyName)))

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

    client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", memory.Unwrap(i.Info.PublicDnsName)), config)

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

func NewInstanceFromEC2(ec2Instance types.Instance) *Instance {
    return &Instance{
        Info:    ec2Instance,
        Service: DefaultEC2,
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

func (f InstanceFilters) Matches(instances []*Instance) []*Instance {
    var matches []*Instance

    for _, instance := range instances {
        if f.DoesMatch(instance) {
            matches = append(matches, instance)
        }
    }

    return matches
}
