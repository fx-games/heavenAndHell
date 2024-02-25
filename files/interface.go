package files

import (
    "io"
    "io/fs"
)

var sourceFileSystem FileSystem
type FileSystem interface {
    fs.ReadFileFS
    fs.ReadDirFS
}
func GetFileSystem() FileSystem {
    return sourceFileSystem
}
func Open(filename string) (io.ReadCloser, error) {
    f, err := sourceFileSystem.Open(filename)
    readCloser, ok := f.(io.ReadCloser)
    if !ok {
        return nil, fs.ErrInvalid
    }
    return readCloser, err
}

func ReadFile(filename string) ([]byte, error) {
    return sourceFileSystem.ReadFile(filename)
}
