package util

import (
	"fmt"
	"io"
	"log"
	"rpg/files"
)

func MustOpen(filename string) io.ReaderAt {
	f, err := files.Open(filename)
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
	f, err := files.Open(filename)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to open file '%s': %w", filename, err))
	}
	return f
}
