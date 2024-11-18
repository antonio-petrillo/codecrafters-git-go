package main

import (
	"compress/zlib"
	"errors"
	"io"
	"os"
	"path"
)

var (
	InvalidBlob     = errors.New("Invalid size blob, cannot parse blob")
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

	if size == 0 {
		return nil, InvalidBlob
	}

	return &Blob{
		Size:    size,
		Content: content,
	}, nil
}
