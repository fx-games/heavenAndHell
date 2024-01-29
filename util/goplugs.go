package util

import (
    "io"
    "os"
)

func MustLoad(open *os.File, err error) io.ReaderAt {
    if err != nil {
        panic(err)
    }
    return open
}
