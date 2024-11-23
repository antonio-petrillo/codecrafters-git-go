package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

var (
	nullByte    = []byte{byte(0)}
	newLineByte = []byte{byte('\n')}
)

func Init() error {
	for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	headFileContents := []byte("ref: refs/heads/main\n")
	err := os.WriteFile(".git/HEAD", headFileContents, 0644)
	if err == nil {
		fmt.Println("Initialized git directory")
	}
	return err
}

func CatFile(args []string) error {
	if len(args) < 2 {
		return errors.New("Missing params to cat-file '-p sha_commit'")
	}
	switch verb := args[0]; verb {
	case "-p":
		blob, err := ReadObject(args[1])
		if err != nil {
			return err
		}
		_, blob, found := bytes.Cut(blob, nullByte)
		if !found {
			return errors.New("Invalid Blob Object")
		}
		fmt.Printf("%s", string(blob))
	default:
		return errors.New("Unknown verb for cat-file")
	}
	return nil
}

func HashObject(args []string) error {
	if len(args) == 0 {
		return errors.New("Missing filename or verb in git hash-object")
	}
	path := args[0]
	var writeToFile bool
	if len(args) == 2 && args[0] == "-w" {
		writeToFile = true
		path = args[1]
	} else if len(args) == 2 && args[1] == "-w" {
		writeToFile = true
	}

	hash, err := WriteObject(path, writeToFile)
	if err != nil {
		return err
	}
	fmt.Printf("%x\n", hash)
	return nil
}

func ListTree(args []string) error {
	if len(args) == 0 {
		return errors.New("Missing filename or verb in git ls-tree")
	}
	path := args[0]
	var nameOnly bool
	if len(args) == 2 && args[0] == "--name-only" {
		nameOnly = true
		path = args[1]
	} else if len(args) == 2 && args[1] == "--name-only" {
		nameOnly = true
	}
	tree, err := ReadObject(path)
	if err != nil {
		return err
	}
	return printTree(tree, nameOnly)
}

func toType(mode []byte) (string, string, error) {
	if bytes.HasPrefix(mode, []byte("40000")) {
		return "040000", "tree", nil
	} else if bytes.HasPrefix(mode, []byte("100644")) {
		return "100644", "blob", nil
	} else {
		return "", "", errors.New("Unknow Object type")
	}
}

func printTree(content []byte, nameOnly bool) error {
	_, content, found := bytes.Cut(content, nullByte)
	if !found { // skip first line
		return errors.New("Fatal: not a tree object")
	}

	lines := [][4]string{}

	for start, size := 0, len(content); start < size; {
		// parse mode
		line := [4]string{}
		for end := start; end < size; end++ {
			if content[end] == ' ' {
				mode, objType, err := toType(content[start:end])
				if err != nil {
					return errors.New("Cannot parse tree object")
				}
				line[0] = mode
				line[1] = objType
				start = end + 1
				break
			}
		}
		// parse name
		for end := start; end < size; end++ {
			if content[end] == 0 {
				line[3] = string(content[start:end])
				start = end + 1
				break
			}
		}

		// parse hash
		line[2] = fmt.Sprintf("%x", content[start:start+20])
		start += 20

		lines = append(lines, line)
	}

	out := []string{}

	if nameOnly {
		for _, line := range lines {
			out = append(out, line[3])
		}
	} else {
		for _, line := range lines {
			s := strings.Builder{}
			s.WriteString(strings.Join(line[:3], " "))
			s.WriteRune('\t')
			s.WriteString(line[3])
			out = append(out, s.String())
		}
	}

	fmt.Printf("%s\n", strings.Join(out, "\n"))
	return nil
}

func WriteTree() error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	sha, err := WriteTreeObject(pwd)
	if err != nil {
		return err
	}
	fmt.Printf("%x\n", sha)
	return nil
}

func detectParam(args []string, target string) (string, bool) {
	size := len(args)
	for i, arg := range args {
		if arg == target && i != size-1 {
			return args[i+1], true
		}
	}
	return "", false
}

func CommitTree(hash string, args []string) error {
	parent, hasParent := detectParam(args, "-p")
	msg, hasMsg := detectParam(args, "-m")

	curr := time.Now().Local()

	timestamp := fmt.Sprintf("%d %s", curr.Unix(), curr.Format("-0700"))

	buf := bytes.Buffer{}

	buf.WriteString(fmt.Sprintf("tree %s\n", hash))
	if hasParent {
		buf.WriteString(fmt.Sprintf("parent %s\n", parent))
	}
	buf.WriteString(fmt.Sprintf("author Antonio Petrillo <myfake@mail.com> %s\n", timestamp))
	buf.WriteString(fmt.Sprintf("committer Antonio Petrillo <myfake@mail.com> %s\n", timestamp))
	buf.WriteString("\n")
	if hasMsg {
		buf.WriteString(msg)
	}
	buf.WriteString("\n")

	sha, err := WriteObjectContent(buf.Len(), buf.Bytes())
	if err != nil {
		return err
	}

	fmt.Printf("%x\n", sha)

	return nil
}
