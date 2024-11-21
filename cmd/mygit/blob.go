package main

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
)

var (
	InvalidBlobSize = errors.New("Invalid blob, can't parse blob")
)

type Blob struct {
	Size    int
	Content []byte
}

func ReadBlob(dir, sha string) (*Blob, error) {
	path := path.Join(".git/objects", dir, sha)
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

	raw, err := io.ReadAll(zReader)
	if err != nil {
		return nil, err
	}
	var head, content []byte
	for i, ch := range raw {
		if ch == 0 {
			head = raw[:i]
			content = raw[i+1:]
			break
		}
	}
	pow, size := 1, 0
	for i := len(head) - 1; i >= 0; i-- {
		if head[i] == ' ' || head[i] == '\t' {
			break
		}
		digit := 0
		switch head[i] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			digit = int(head[i] - '0')
		default:
			return nil, InvalidBlobSize
		}
		size += digit * pow
		pow *= 10
	}

	return &Blob{
		Size:    size,
		Content: content,
	}, nil
}

func WriteBlob(path string, writeToFile bool) (empty [20]byte, _ error) {
	file, err := os.Open(path)
	if err != nil {
		return empty, err
	}
	defer file.Close()

	buf := bytes.Buffer{}
	size, err := buf.ReadFrom(file)
	if err != nil {
		return empty, err
	}
	hash, content := HashContent(BlobHeader, size, buf.Bytes())

	if writeToFile {
		err = WriteObjectToFile(hash, content)
		if err != nil {
			return empty, err
		}
	}

	return hash, nil
}

func WriteObjectToFile(hash [20]byte, content []byte) error {
	hashStr := fmt.Sprintf("%x", hash)
	dir, sha := hashStr[:2], hashStr[2:]

	objPath := path.Join(".git/objects", dir)
	err := os.Mkdir(objPath, 0o755) // rxwr-x---
	if err != nil && !os.IsExist(err) {
		return err
	}

	objPath = path.Join(objPath, sha)

	file, err := os.OpenFile(objPath, os.O_CREATE|os.O_WRONLY, 0o644) // rw-r--r--
	if err != nil {
		return err
	}
	defer file.Close()

	compressedFile := zlib.NewWriter(file)
	defer compressedFile.Close()

	_, err = compressedFile.Write(content)

	return err
}
