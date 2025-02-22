package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"errors"
	"fmt"
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

// read file at sha and /parses/ into a gitobject
func ReadGitObject(sha string) (GitObject, error) {
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

func HashObject(gitObj GitObject) ([20]byte, []byte) {
	obj := []byte(fmt.Sprintf("%s %d\x00", gitObj.Kind(), len(gitObj.Content())))
	obj = append(obj, gitObj.Content()...)

	return sha1.Sum(obj), obj
}

func WriteContent(objPath string, content []byte) error {
	dir, signature := objPath[:2], objPath[2:]
	objPath = path.Join(".git/objects", dir)
	if err := os.Mkdir(objPath, 0o755); err != nil && !os.IsExist(err) {
		return err
	}

	objPath = path.Join(objPath, signature)
	file, err := os.OpenFile(objPath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	compressed := zlib.NewWriter(file)
	defer compressed.Close()

	_, err = compressed.Write(content)
	if err != nil {
		return err
	}
	return nil
}
