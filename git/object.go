package git

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

const (
	BlobType   string = "blob"
	TreeType   string = "tree"
	CommitType string = "commit"
)

var (
	ErrInvalidObj = errors.New("Invalid git object")
	ErrUnknownObj = errors.New("Unknown git object")
)

func GetBlobSha(path string) (bad [20]byte, _ error) {
	obj, err := os.Open(path)
	if err != nil {
		return bad, err
	}
	defer obj.Close()

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(obj)
	if err != nil {
		return bad, err
	}

	sha, _ := prepareContent(BlobType, buf.Bytes())
	return sha, nil
}

func WriteBlob(path string) (bad [20]byte, _ error) {
	obj, err := os.Open(path)
	if err != nil {
		return bad, err
	}
	defer obj.Close()

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(obj)
	if err != nil {
		return bad, err
	}

	sha, content := prepareContent(BlobType, buf.Bytes())

	err = writeObject(fmt.Sprintf("%x", sha), content)
	if err != nil {
		return bad, err
	}
	return sha, nil
}

func WriteCommit(commit []byte) (bad [20]byte, _ error) {
	sha, content := prepareContent(CommitType, commit)
	err := writeObject(fmt.Sprintf("%x", sha), content)
	if err != nil {
		return bad, err
	}
	return sha, nil
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

func WriteTreeObject(dir string) (bad [20]byte, _ error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return bad, err
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
			sha, err = WriteBlob(path.Join(dir, entry.Name()))
		}

		if err != nil {
			return bad, err
		}

		content.WriteString(entry.Name())
		content.WriteByte(0) // append \0
		content.Write(sha[:])
	}

	sha, treeObj := prepareContent(TreeType, content.Bytes())
	err = writeObject(fmt.Sprintf("%x", sha), treeObj)

	if err != nil {
		return bad, err
	}

	return sha, nil
}

// private write object function, used to write Blob, Tree and commits
// due to go lacks of enum I can't model the `objectType string` as, well..., an enum
func prepareContent(objectType string, content []byte) ([20]byte, []byte) {
	objContent := []byte(fmt.Sprintf("%s %d\x00", objectType, len(content)))
	objContent = append(objContent, content...)

	return sha1.Sum(objContent), objContent
}

func writeObject(objPath string, content []byte) error {
	dir, sha := objPath[:2], objPath[2:]
	writePath := path.Join(".git/objects", dir)
	if err := os.Mkdir(writePath, 0o755); err != nil && !os.IsExist(err) { // rxwr-x---
		return err
	}

	writePath = path.Join(writePath, sha)
	file, err := os.OpenFile(writePath, os.O_CREATE|os.O_WRONLY, 0o644) // rw-r--r--
	if err != nil {
		return err
	}
	defer file.Close()

	compressedFile := zlib.NewWriter(file)
	defer compressedFile.Close()

	_, err = compressedFile.Write(content)
	if err != nil {
		return err
	}

	return nil
}
