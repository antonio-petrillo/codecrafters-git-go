package main

// https://stefan.saasen.me/articles/git-clone-in-haskell-from-the-bottom-up/#reimplementing-git-clone-in-haskell-from-the-bottom-up

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

func parsePacketLine(r io.Reader) ([]byte, error) {
	lengthBytes := make([]byte, 4)
	if _, err := r.Read(lengthBytes); err != nil {
		return nil, err
	}
	length, err := strconv.ParseInt(string(lengthBytes), 16, 64)
	if err != nil || length == 0 {
		return nil, err
	}
	line := make([]byte, length-4)
	n, err := r.Read(line)
	if err != nil {
		return nil, err
	}
	if n != int(length)-4 {
		return nil, fmt.Errorf("packet line doesnt' match declared length")
	}

	return line, nil
}

func serializePackeLine(line string) string {
	return fmt.Sprintf("%04x%s", len(line)+4, line)
}

// https://git-scm.com/docs/http-protocol
func GetLastHash(url string) ([]byte, error) {
	r, err := http.Get(fmt.Sprintf("%s/info/refs?service=git-upload-pack", url))
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("[GetLastHash]: Retrieving last hash return %d status code %q", r.StatusCode, r.Status)
	}

	// skip line: 001e# service=git-upload-pack\n
	_, err = parsePacketLine(r.Body)
	if err != nil {
		return nil, err
	}
	// skip line: 0000
	_, err = parsePacketLine(r.Body)
	if err != nil {
		return nil, err
	}

	// ideally here there can be multiple refs, the challenge put it easy on us because ensure only one refs in the request
	ref, err := parsePacketLine(r.Body)
	idx := bytes.IndexByte(ref, byte(' '))
	if idx == -1 {
		return nil, fmt.Errorf("[GetLastHash]: Refs is not in the expected form")
	}
	return ref[:idx], nil
}

// https://git-scm.com/docs/http-protocol
func UploadPack(url string, hash []byte) ([]byte, error) {
	body := &bytes.Buffer{}
	want := fmt.Sprintf("want %s no-progress\n", string(hash))
	// this can't actually fail
	body.WriteString(serializePackeLine(want))
	body.WriteString("0000")
	body.WriteString(serializePackeLine("done\n")) // flush and end request
	r, err := http.Post(fmt.Sprintf("%s/git-upload-pack", url), "application/x-git-upload-pack-request", body)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("[UploadPack]: url/git-upload-pack return %d status code %q", r.StatusCode, r.Status)
	}
	nak, err := parsePacketLine(r.Body)
	if err != nil {
		return nil, err
	}
	if !bytes.EqualFold(nak, []byte{'N', 'A', 'K', '\n'}) {
		return nil, fmt.Errorf("Expecting 'NAK' got %q", string(nak))
	}
	body.Reset()
	if _, err = io.Copy(body, r.Body); err != nil {
		return nil, err
	}

	return body.Bytes(), nil
}
