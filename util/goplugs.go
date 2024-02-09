package util

import (
	"io"
	"log"
	"os"
)

func MustLoad(open *os.File, err error) io.ReaderAt {
	if err != nil {
		panic(err)
	}
	return open
}

func MustOpen(filename string) io.ReaderAt {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

func MustOpenReader(filename string) io.Reader {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	return f
}
