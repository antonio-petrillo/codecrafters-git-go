package main

import (
	"crypto/sha1"
	"fmt"
)

const (
	BlobHeader = "blob"
)

func HashContent(header string, size int64, content []byte) (empty [20]byte, _ []byte) {

	objContent := []byte(fmt.Sprintf("%s %d\x00", header, size))
	objContent = append(objContent, content...)

	return sha1.Sum(objContent), objContent
}
