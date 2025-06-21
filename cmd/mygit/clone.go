package main

// https://stefan.saasen.me/articles/git-clone-in-haskell-from-the-bottom-up/#reimplementing-git-clone-in-haskell-from-the-bottom-up

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

type packFileKind byte

const (
	commit   packFileKind = 1
	tree     packFileKind = 2
	blob     packFileKind = 3
	tag      packFileKind = 4
	ofsDelta packFileKind = 6
	refDelta packFileKind = 7
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
// return the bytes, when parsed we obtain the git objects
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

// https://codewords.recurse.com/issues/three/unpacking-git-packfiles
func ParseObjects(raw []byte) ([]GitObject, error) {
	objs := []GitObject{}
	if len(raw) < 12 {
		return nil, fmt.Errorf("Pack file has incomplete header: expected len of at least 12, got %d", len(raw))
	}

	if !bytes.Equal([]byte{'P', 'A', 'C', 'K'}, raw[:4]) {
		return nil, fmt.Errorf("Expected magic number 'PACK' got %x", raw[:4])
	}
	count := binary.BigEndian.Uint32(raw[8:12])
	reader := bytes.NewReader(raw[12:])

	store := make(map[[20]byte]GitObject)

	for range count {
		obj, err := ParseObject(reader, store)
		if err != nil {
			return objs, err
		}
		objs = append(objs, obj)
	}
	return objs, nil
}

// https://codewords.recurse.com/issues/three/unpacking-git-packfiles
func ParseObject(r *bytes.Reader, store map[[20]byte]GitObject) (GitObject, error) {
	kind, size, err := parseObjectHeader(r)
	_ = size
	if err != nil {
		return nil, err
	}
	switch kind {
	case tag:
		return nil, fmt.Errorf("Unsupported git object for now [TAG]")
	case ofsDelta:
		return nil, fmt.Errorf("Unsupported git object for now [OFS_DELTA]")
	case refDelta:
		return parseRefDelta(r, store, size)

	default:
		data, err := decompress(r)
		if err != nil {
			return nil, err
		}
		var obj GitObject
		switch kind {
		case blob:
			obj = &Blob{content: data}
		case tree:
			obj = &Tree{content: data}
		case commit:
			obj = &CommitAsBytes{content: data}
		default:
			panic("unexpected object kind")
		}
		hash, _ := HashObject(obj)
		if _, ok := store[hash]; ok {
			return nil, fmt.Errorf("Duplicated obj sha %x", hash[:])
		}
		store[hash] = obj
		return obj, nil
	}
}

func parseRefDelta(r *bytes.Reader, store map[[20]byte]GitObject, size uint64) (GitObject, error) {
	var sha [20]byte
	n, err := r.Read(sha[:])
	if err != nil {
		return nil, err
	}
	if n != 20 {
		return nil, fmt.Errorf("Unvalid git sha in [REF_DELTA]")
	}
	baseObj, ok := store[sha]
	if !ok {
		return nil, fmt.Errorf("Invalid base obj in [REF_DELTA]")
	}
	data, err := decompress(r)
	if err != nil {
		return nil, err
	}

	srcSize, err := parseSizeHeader(r)
	if err != nil {
		return nil, err
	}
	if srcSize != uint32(len(baseObj.Content())) {
		return nil, fmt.Errorf("Expected base object to have size %d, got %d", srcSize, uint32(len(baseObj.Content())))
	}

	buf := &bytes.Buffer{}
	for _, b := range data {
		if b&0x80 != 0 { // COPY

		} else { // ADD
			numBytes := b & 0x7f
			buf_ := make([]byte, numBytes)
			n, err := r.Read(buf_)
			if err != nil {
				return nil, err
			}
			if n != int(numBytes) {
				return nil, fmt.Errorf("ADD command required %d bytes, readed only %d\n", numBytes, n)
			}
			buf.Write(buf_)
		}
	}

	dstSize, err := parseSizeHeader(r)
	if err != nil {
		return nil, err
	}

	if dstSize != uint32(buf.Len()) {
		return nil, fmt.Errorf("Expected final object to have size %d, got %d", dstSize, buf.Len())
	}

	var obj GitObject
	switch baseObj.(type) {
	case *Blob:
		obj = &Blob{content: buf.Bytes()}
	case *Tree:
		obj = &Tree{content: buf.Bytes()}
	case *CommitAsBytes:
		obj = &CommitAsBytes{content: buf.Bytes()}
	default:
		panic("Unknown obj type switch")
	}
	hash, _ := HashObject(obj)
	if _, ok := store[hash]; ok {
		return nil, fmt.Errorf("Duplicate obj hash [REF_DELTA]")
	}
	store[hash] = obj

	return obj, nil
}

func parseSizeHeader(r io.ByteReader) (uint32, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}

	size := uint32(b & 0x7f)
	if b&0x80 == 0 {
		return size, nil
	}
	shift := 7
	for b, err = r.ReadByte(); err != nil; b, err = r.ReadByte() {
		size |= uint32(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
	}
	if err != nil {
		return 0, nil
	}

	return size, nil
}

func decompress(r io.Reader) ([]byte, error) {
	zReader, err := zlib.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer zReader.Close()
	buf := &bytes.Buffer{}
	if _, err = io.Copy(buf, zReader); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseObjectHeader(r io.ByteReader) (packFileKind, uint64, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	kind, size := (b&0x70)>>4, uint64(b&0x0F)
	if kind == 0 || kind == 5 {
		return 0, 0, fmt.Errorf("Invalid obj type, got '%d'", kind)
	}
	if b&0x80 == 0 {
		return packFileKind(kind), size, nil
	}
	shift := 4
	for b, err = r.ReadByte(); err != nil; b, err = r.ReadByte() {
		// instruction unclear: should I keep the last msb or not?
		size |= uint64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
	}
	if err != nil {
		return 0, 0, err
	}

	return packFileKind(kind), size, nil
}
