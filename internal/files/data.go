package files

import (
    "os"
    "path"
)

func CreateDataDirectory() (string, error) {
    homeDir, err := os.UserHomeDir()

    if err != nil {
        return "", err
    }

    awsDir := path.Join(homeDir, ".aws")
    awsumDir := path.Join(awsDir, "awsum")

    if err = os.MkdirAll(awsumDir, 0755); err != nil {
        if os.IsExist(err) {
            return "", nil
        }

        return "", err
    }

    return awsumDir, nil
}

// OpenAwsumFile opens a file with the given parameters in the awsum data directory.
func OpenAwsumFile(filename string, flag int, perm os.FileMode) (*os.File, error) {
    dataDir, err := CreateDataDirectory()

    if err != nil {
        return nil, err
    }

    return os.OpenFile(path.Join(dataDir, filename), flag, perm)
}
