package main

import (
	"bytes"
	"os"
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

	buf := bytes.Buffer{}

	_, err = buf.ReadFrom(file)
	if err != nil {
		return nil, err
	}

	return &Blob{
		content: buf.Bytes(),
	}, nil
}
