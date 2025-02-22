package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
)

var (
	InvalidTree = errors.New("File at path cannot be parsed into a tree.")
)

type Tree struct {
	content []byte
}

type entry struct {
	mode string
	kind ObjectKind
	hash string
	name string
}

func (e *entry) String() string {
	return fmt.Sprintf("%s %s %s\t%s", e.mode, e.kind, e.hash, e.name)
}

func (t *Tree) Kind() ObjectKind {
	return TreeKind
}
func (t *Tree) Content() []byte {
	return t.content
}

func toType(mode []byte) (string, ObjectKind) {
	if bytes.HasPrefix(mode, []byte("40000")) {
		return "040000", TreeKind
	} else if bytes.HasPrefix(mode, []byte("100644")) {
		return "100644", BlobKind
	} else {
		return "", ""
	}
}

func (t *Tree) Format(onlyName bool) string {
	lines := []entry{}

	for start, size := 0, len(t.content); start < size; {
		// parse mode
		line := entry{}
		for end := start; end < size; end++ {
			if t.content[end] == ' ' {
				mode, objType := toType(t.content[start:end])
				line.mode = mode
				line.kind = objType
				start = end + 1
				break
			}
		}
		// parse name
		for end := start; end < size; end++ {
			if t.content[end] == 0 {
				line.name = string(t.content[start:end])
				start = end + 1
				break
			}
		}

		// parse hash
		line.hash = fmt.Sprintf("%x", t.content[start:start+20])
		start += 20

		lines = append(lines, line)
	}

	formatted := []string{}
	if onlyName {
		for _, line := range lines {
			formatted = append(formatted, line.name)
		}
	} else {
		for _, line := range lines {
			formatted = append(formatted, line.String())
		}
	}
	formatted = append(formatted, "")
	return strings.Join(formatted, "\n")
}

func (t *Tree) String() string {
	return t.Format(false)
}

func BuildTreeFromDir(dir string) (_ *Tree, nilSha [20]byte, _ error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nilSha, err
	}

	content := bytes.Buffer{}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		var gitObj GitObject
		var sha [20]byte

		next := path.Join(dir, entry.Name())
		if entry.IsDir() { // tree
			content.WriteString("40000 ")
			gitObj, _, err = BuildTreeFromDir(next)
		} else {// obj
			content.WriteString("100644 ")
			gitObj, err = ReadBlobFromFile(next)
		}
		if err != nil {
			return nil, nilSha, err
		}
		sha, err = WriteContent(gitObj)
		if err != nil {
			return nil, nilSha, err
		}

		content.WriteString(entry.Name())
		content.WriteByte(0)
		content.Write(sha[:])
	} 

	tree := &Tree{
		content: content.Bytes(),
	}
	sha, err := WriteContent(tree)
	if err != nil {
		return nil, nilSha, err
	}

	return tree, sha, nil
} 
