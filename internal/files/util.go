package files

import (
    "fmt"
    "io"
    "os"
)

func ReadFileFull(filename string) ([]byte, error) {
    f, err := os.OpenFile(filename, os.O_RDONLY, 0644)

    if err != nil {
        return nil, fmt.Errorf("failed to open file '%s': %w", filename, err)
    }

    defer func() {
        if err = f.Close(); err != nil {
            fmt.Printf("failed to properly close file '%s': %s\n", filename, err)
        }
    }()

    buf, err := io.ReadAll(f)

    if err != nil {
        return nil, fmt.Errorf("failed to read from file '%s': %w", filename, err)
    }

    return buf, nil
}

func AppendToFile(filename string, buf []byte) error {
    f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

    if err != nil {
        return fmt.Errorf("failed to open file '%s': %w", filename, err)
    }

    defer func() {
        if err = f.Close(); err != nil {
            fmt.Printf("failed to properly close file '%s': %s\n", filename, err)
        }
    }()

    if _, err = f.Write(buf); err != nil {
        return fmt.Errorf("failed to append to file '%s': %w", filename, err)
    }

    return f.Sync()
}
