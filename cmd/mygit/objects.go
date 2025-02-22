package main

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"os"
	"path"
)

type ObjectKind string

const (
	BlobKind ObjectKind = "blob"
)

var (
	InvalidObject = errors.New("Invalid Object")
)

type GitObject interface {
	Kind() ObjectKind
	Content() []byte
	String() string
}

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

// read file at sha and /parses/ into a gitobject
func ReadFromDisk(sha string) (GitObject, error) {
	path := path.Join(".git/objects", sha[:2], sha[2:])
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	zReader, err := zlib.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer zReader.Close()
	
	content, err := io.ReadAll(zReader)
	if err != nil {
		return nil, err
	}

	header, body, found := bytes.Cut(content, []byte{byte(0)})
	if !found {
		return nil, InvalidObject
	}

	if bytes.HasPrefix(header, []byte(BlobKind)) {
		return &Blob{
			content: body,
		}, nil
	}

	return nil, InvalidObject
}
