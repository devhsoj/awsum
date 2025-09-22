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

func WriteToFile(filename string, buf []byte, append ...bool) error {
    flags := os.O_CREATE | os.O_WRONLY

    if len(append) > 0 && append[0] {
        flags |= os.O_APPEND
    } else {
        flags |= os.O_TRUNC
    }

    f, err := os.OpenFile(filename, flags, 0644)

    if err != nil {
        return fmt.Errorf("failed to open file '%s': %w", filename, err)
    }

    defer func() {
        if err = f.Close(); err != nil {
            fmt.Printf("failed to properly close file '%s': %s\n", filename, err)
        }
    }()

    if _, err = f.Write(buf); err != nil {
        return fmt.Errorf("failed to write to file '%s': %w", filename, err)
    }

    return f.Sync()
}
