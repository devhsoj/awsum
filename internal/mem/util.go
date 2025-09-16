package mem

func Pointer[T any](v T) *T {
    return &v
}

func Unwrap[T any](v *T) T {
    if v == nil {
        var ret T

        return ret
    }

    return *v
}
