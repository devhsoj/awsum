package files

import (
    "errors"
    "fmt"
    "net"
    "os"
    "path"

    "golang.org/x/crypto/ssh"
    "golang.org/x/crypto/ssh/knownhosts"
)

func GetAssumedUserSSHDir() (string, error) {
    homeDir, err := os.UserHomeDir()

    if err != nil {
        return "", fmt.Errorf("failed to get assumed user ssh directory: %w", err)
    }

    return path.Join(homeDir, ".ssh"), nil
}

func GenerateHostKeyCallbackFromKnownHosts() (ssh.HostKeyCallback, error) {
    sshDir, err := GetAssumedUserSSHDir()

    if err != nil {
        return nil, fmt.Errorf("failed to get assumed user ssh directory while generating host key callback")
    }

    knownHostsFilename := path.Join(sshDir, "known_hosts")

    hostKeyCallback, err := knownhosts.New(knownHostsFilename)

    if err != nil {
        return nil, fmt.Errorf("failed to get create host key callback from known_hosts '%s': %w", knownHostsFilename, err)
    }

    return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
        if err = hostKeyCallback(hostname, remote, key); err != nil {
            var knownHostsErr *knownhosts.KeyError

            // is unknown host?
            if errors.As(err, &knownHostsErr) && len(knownHostsErr.Want) == 0 {
                line := knownhosts.Line([]string{
                    knownhosts.Normalize(hostname),
                }, key)

                return AppendToFile(knownHostsFilename, []byte(line+"\n"))
            }

            return err
        }

        return nil
    }, nil
}
