package main

import (
	"bytes"
	"errors"
	"os"
)

var (
	InvalidBlob = errors.New("File at path cannot be parsed into a blob.")
)

type Blob struct {
	content []byte
}

func (b *Blob) Kind() ObjectKind {
	return BlobKind
}
func (b *Blob) Content() []byte {
	return b.content
}

func (b *Blob) String() string {
	return string(b.content)
}

func ReadBlobFromFile(path string) (*Blob, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, InvalidBlob
	}

	buf := bytes.Buffer{}

	_, err = buf.ReadFrom(file)
	if err != nil {
		return nil, err
	}

	return &Blob{
		content: buf.Bytes(),
	}, nil
}
