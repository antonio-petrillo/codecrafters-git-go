package main

// https://stefan.saasen.me/articles/git-clone-in-haskell-from-the-bottom-up/#reimplementing-git-clone-in-haskell-from-the-bottom-up

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
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
func ParseObjects(raw []byte) error {
	if len(raw) < 12 {
		return fmt.Errorf("Pack file has incomplete header: expected len of at least 12, got %d", len(raw))
	}

	if !bytes.Equal([]byte{'P', 'A', 'C', 'K'}, raw[:4]) {
		return fmt.Errorf("Expected magic number 'PACK' got %x", raw[:4])
	}
	count := binary.BigEndian.Uint32(raw[8:12])
	reader := bytes.NewReader(raw[12:])
	for range count {
		err := ParseObject(reader)
		if err != nil {
			return err
		}
	}
	return nil
}

// https://codewords.recurse.com/issues/three/unpacking-git-packfiles
func ParseObject(r *bytes.Reader) error {
	kind, size, err := parseObjectHeader(r)
	if err != nil {
		return err
	}
	switch kind {
	case tag:
		return fmt.Errorf("Unsupported git object for now [TAG]")
	case ofsDelta:
		return fmt.Errorf("Unsupported git object for now [OFS_DELTA]")
	case refDelta:
		obj, err := parseRefDelta(r, size)
		if err != nil {
			return err
		}
		_, err = WriteContent(obj)
		return err

	default:
		data, err := decompress(r, size)
		if err != nil {
			return nil
		}
		var obj GitObject
		switch kind {
		case blob:
			obj = &Blob{content: data.Bytes()}
		case tree:
			obj = &Tree{content: data.Bytes()}
		case commit:
			obj = &CommitAsBytes{content: data.Bytes()}
		default:
			panic("unexpected object kind")
		}
		_, err = WriteContent(obj)
		return err
	}
}

func parseRefDelta(r *bytes.Reader, size int64) (GitObject, error) {
	var sha [20]byte
	n, err := r.Read(sha[:])
	if err != nil {
		return nil, err
	}
	if n != 20 {
		return nil, fmt.Errorf("Unvalid git sha in [REF_DELTA]")
	}
	baseObj, err := ReadGitObject(fmt.Sprintf("%x", sha))
	if err != nil {
		return nil, err
	}

	data, err := decompress(r, size)
	if err != nil {
		return nil, err
	}

	srcSize, err := binary.ReadUvarint(data)
	if err != nil {
		return nil, err
	}
	if srcSize != uint64(len(baseObj.Content())) {
		return nil, fmt.Errorf("Expected base object to have size %d, got %d", srcSize, uint32(len(baseObj.Content())))
	}
	dstSize, err := binary.ReadUvarint(data)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	for {
		b, err := data.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// read offset
		if b&0x80 != 0 { // COPY
			offset, size := 0, 0
			for i := range 4 {
				if b&(1<<i) != 0 {
					b_, err := data.ReadByte()
					if err != nil {
						return nil, err
					}
					offset |= int(b_) << (i * 8)
				}
			}

			for i := range 3 {
				if b&(0x10<<i) != 0 {
					b_, err := data.ReadByte()
					if err != nil {
						return nil, err
					}
					size |= int(b_) << (i * 8)
				}
			}
			buf.Write(baseObj.Content()[offset : offset+size])

		} else { // ADD
			_, err = io.CopyN(buf, data, int64(b&0x7f))
			if err != nil {
				return nil, err
			}
		}

	}

	if dstSize != uint64(buf.Len()) {
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
	return obj, err
}

func decompress(r io.Reader, size int64) (*bytes.Buffer, error) {
	zReader, err := zlib.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer zReader.Close()
	buf := &bytes.Buffer{}
	if _, err = io.CopyN(buf, zReader, size); err != nil {
		return nil, err
	}
	return buf, nil
}

func parseObjectHeader(r io.ByteReader) (packFileKind, int64, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	kind, size := (b&0x70)>>4, int64(b&0x0F)
	// if kind == 0 || kind == 5 {
	// 	return 0, 0, fmt.Errorf("Invalid obj type, got '%d'", kind)
	// }
	if b&0x80 == 0 {
		return packFileKind(kind), size, nil
	}
	shift := 4
	for i := 0; ; i++ {
		b_, err := r.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		size |= int64(b_&0x7f) << shift
		if b_&0x80 == 0 {
			break
		}
		shift += 7
	}
	return packFileKind(kind), size, err
}

func Checkout(hash string) error {
	obj, err := ReadGitObject(hash)
	if err != nil {
		return err
	}
	data := obj.Content()
	hash = string(data[bytes.IndexByte(data, '\x00')+6 : bytes.IndexByte(data, '\x0a')])

	return ParseTreeFromHash(".", hash)
}

func ParseTreeFromHash(basepath, hash string) error {
	obj, err := ReadGitObject(hash)
	if err != nil {
		return err
	}
	data := obj.Content()
	for len(data) > 0 {
		_, kind := toType(data[:6])
		if kind == BlobKind {
			data = data[7:] // skip mode
		} else {
			data = data[6:] // skip mode
		}
		idx := bytes.Index(data, []byte{'\x00'})
		filename := path.Join(basepath, string(data[:idx]))
		data = data[idx+1:]
		fileHash := fmt.Sprintf("%x", data[:20])
		data = data[20:]
		switch kind {
		case TreeKind:
			if err = os.Mkdir(filename, 0o755); err != nil && !os.IsExist(err) {
				return err
			}
			if err = ParseTreeFromHash(filename, fileHash); err != nil {
				return err
			}
		case BlobKind:
			blob, err := ReadGitObject(fileHash)
			if err != nil {
				return err
			}
			blobData := blob.Content()
			if err = os.WriteFile(filename, blobData[:len(blobData)-1], 0o644); err != nil {
				return err
			}
		default:
			fmt.Printf("Unknown %s\n", kind)
			panic("unsupported for now")
		}
	}

	return nil
}
