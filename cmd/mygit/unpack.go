package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const (
	ObjCommit   int = 1
	ObjTree     int = 2
	ObjBlob     int = 3
	ObjTag      int = 4
	ObjOfsDelta int = 6
	ObjRefDelta int = 7
)

const (
	msgMask    = 0x80
	typeMask   = 0x70
	vleNumMask = 0x7f // variable length mask
)

func GetGitUploadPack(url string) ([]byte, error) {
	r, err := http.Get(fmt.Sprintf("%s/info/refs?service=git-upload-pack", url))
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	// for simplicity I just care about the _sha_, I will discard all the other data
	// I also assume that there is only one ref
	// Discard: 001e
	_, err = ParsePacketLine(r.Body)
	if err != nil {
		return nil, err
	}
	// Discard: 0000
	_, err = ParsePacketLine(r.Body)
	if err != nil {
		return nil, err
	}

	// read ref (as I said above, I assume there is only one ref)
	data, err := ParsePacketLine(r.Body)
	index := bytes.IndexByte(data, byte(' '))

	return data[:index], nil
}

func ParsePacketLine(reader io.Reader) ([]byte, error) {
	lengthSlice := make([]byte, 4)
	_, err := reader.Read(lengthSlice)
	if err != nil {
		return nil, err
	}
	length, err := strconv.ParseInt(string(lengthSlice), 16, 64)
	if err != nil {
		return nil, err
	}
	if length == 0 { // end of packet line
		return []byte{}, nil
	}
	data := make([]byte, length-4) // remove the 4 bytes used for the length
	_, err = reader.Read(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func PostGitUploadPack(url string, hash []byte) ([]byte, error) {
	buf := &bytes.Buffer{}
	// List all _wants_
	buf.WriteString(fmt.Sprintf("0032want %s\n00000009done\n", hash))

	r, err := http.Post(
		fmt.Sprintf("%s/git-upload-pack", url),
		"application/x-git-upload-pack-request",
		buf,
	)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, errors.New("request POST git-upload-pack not 200-ok")
	}
	pack, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	return pack[8:], nil // discard NAK (this is *just a clone*)
}

func UnpackPackfile(data []byte, numOfObjects uint32) error {
	// Oh boy this is hard to do
	return nil
}
