package files

import (
	"fmt"
	"io"
	"io/fs"
	"log"
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
	if err != nil {
		return nil, err // otherwise the nil f fails the assertion below and buries this in fs.ErrInvalid
	}
	readCloser, ok := f.(io.ReadCloser)
	if !ok {
		return nil, fs.ErrInvalid
	}
	return readCloser, nil
}

func ReadFile(filename string) ([]byte, error) {
	return sourceFileSystem.ReadFile(filename)
}

func MustOpen(filename string) io.ReaderAt {
	f, err := Open(filename)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to open file '%s': %w", filename, err))
	}
	readerAt, ok := f.(io.ReaderAt)
	if !ok {
		return nil
	}
	return readerAt
}

func MustOpenReader(filename string) io.ReadCloser {
	f, err := Open(filename)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to open file '%s': %w", filename, err))
	}
	return f
}
