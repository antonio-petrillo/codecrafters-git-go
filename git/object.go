package git

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
)

const (
	BlobType string = "blob"
	TreeType string = "tree"
)

var (
	ErrInvalidObj = errors.New("Invalid git object")
	ErrUnknownObj = errors.New("Unknown git object")
)

func WriteObject(path string, writeToFile bool) (empty [20]byte, _ error) {
	obj, err := os.Open(path)
	if err != nil {
		return empty, err
	}
	defer obj.Close()

	buf := bytes.Buffer{}
	size, err := buf.ReadFrom(obj)
	if err != nil {
		return empty, err
	}
	hash, content := HashContent(BlobType, size, buf.Bytes())

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

func ReadObject(sha string) ([]byte, error) {
	pathObj := path.Join(".git/objects", sha[:2], sha[2:])
	file, err := os.Open(pathObj)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	zReader, err := zlib.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer zReader.Close()

	return io.ReadAll(zReader)
}

func WriteTreeObject(dir string) (empty [20]byte, _ error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return empty, err
	}

	content := bytes.Buffer{}
	for _, entry := range entries {
		var sha [20]byte

		if entry.Name() == ".git" {
			continue
		}
		if entry.IsDir() { // tree
			// recursion
			content.WriteString("40000 ")
			sha, err = WriteTreeObject(path.Join(dir, entry.Name()))
		} else { // object
			content.WriteString("100644 ")
			// write blob
			sha, err = WriteObject(path.Join(dir, entry.Name()), true)
		}

		if err != nil {
			return empty, err
		}

		content.WriteString(entry.Name())
		content.WriteByte(0) // append \0
		content.Write(sha[:])
	}

	sha, byteContent := HashContent(TreeType, int64(content.Len()), content.Bytes())
	err = WriteObjectToFile(sha, byteContent)
	if err != nil {
		return empty, err
	}

	return sha, nil
}
