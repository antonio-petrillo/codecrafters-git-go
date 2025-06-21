package main

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"path"
	"time"
)

const (
	InitCmd       = "init"
	CatFileCmd    = "cat-file"
	HashObjectCmd = "hash-object"
	LsTreeCmd     = "ls-tree"
	WriteTreeCmd  = "write-tree"
	CommitTreeCmd = "commit-tree"
	CloneCmd      = "clone"
)

type Handler func(name string, args []string) error
type Commands map[string]Handler

var (
	CommandNotFoundError    = errors.New("Command not found")
	MismatchedError         = errors.New("Mismatched command")
	InvalidArgsError        = errors.New("Invalid arguments")
	MismatchedChecksumError = errors.New("Packfile checksum doesn't mach")
	InvalidPackError        = errors.New("Invalid Packfile")
)

var availableCommands = Commands{
	InitCmd:       HandlerInit,
	CatFileCmd:    HandlerCatFile,
	HashObjectCmd: HandlerHashObject,
	LsTreeCmd:     HandlerListTree,
	WriteTreeCmd:  HandlerWriteTree,
	CommitTreeCmd: HandlerCommitTree,
	CloneCmd:      HandlerClone,
}

func GetCommand(cmd string) (Handler, error) {
	handler, ok := availableCommands[cmd]
	if !ok {
		return nil, CommandNotFoundError
	}
	return handler, nil
}

func HandlerInit(name string, args []string) error {
	if name != InitCmd {
		return MismatchedError
	}

	if len(args) > 0 {
		return InvalidArgsError
	}

	for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
		}
	}

	headFileContents := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(".git/HEAD", headFileContents, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
	}
	fmt.Println("Initialized git directory")

	return nil
}

func HandlerCatFile(name string, args []string) error {
	if name != CatFileCmd {
		return MismatchedError
	}

	if len(args) != 2 {
		return InvalidArgsError
	}

	switch verb := args[0]; verb {
	case "-p":

		gitObj, err := ReadGitObject(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("%s", gitObj)

	default:
		return InvalidArgsError
	}

	return nil
}

func HandlerHashObject(name string, args []string) error {
	if name != HashObjectCmd {
		return MismatchedError
	}

	if l := len(args); l < 1 || l > 2 {
		return InvalidArgsError
	}
	writeToFile := false
	objIndex := 0

	switch verb := args[0]; verb {
	case "-w":
		objIndex = 1
		writeToFile = true

	default:
		// nothing special for now
	}

	blob, err := ReadBlobFromFile(args[objIndex])
	if err != nil {
		return err
	}

	var hash [20]byte

	if writeToFile {
		hash, err = WriteContent(blob)
		if err != nil {
			return err
		}
	} else {
		hash, _ = HashObject(blob)
	}
	fmt.Printf("%x\n", hash)

	return nil
}

func HandlerListTree(name string, args []string) error {
	if name != LsTreeCmd {
		return MismatchedError
	}

	if l := len(args); l < 1 || l > 2 {
		return InvalidArgsError
	}

	sha, onlyName := args[0], false
	if len(args) == 2 {
		if args[0] == "--name-only" {
			sha, onlyName = args[1], true
		} else if args[1] == "--name-only" {
			sha, onlyName = args[0], true
		} else {
			return InvalidArgsError
		}
	}

	gitObj, err := ReadGitObject(sha)
	if err != nil {
		return err
	}

	tree, ok := gitObj.(*Tree)
	if !ok {
		return InvalidTree
	}

	fmt.Printf("%s", tree.Format(onlyName))

	return nil
}

func HandlerWriteTree(name string, args []string) error {
	if name != WriteTreeCmd {
		return MismatchedError
	}

	if len(args) != 0 {
		return InvalidArgsError
	}

	curDir, err := os.Getwd()
	if err != nil {
		return err
	}

	_, sha, err := BuildTreeFromDir(curDir)
	if err != nil {
		return err
	}

	fmt.Printf("%x\n", sha)

	return nil
}

func detectParam(args []string, target string) *string {
	size := len(args)
	for i, arg := range args {
		if arg == target && i != size-1 {
			return &args[i+1]
		}
	}
	return nil
}

func HandlerCommitTree(name string, args []string) error {
	if name != CommitTreeCmd {
		return MismatchedError
	}

	if len(args) < 1 {
		return InvalidArgsError
	}

	tree := args[0]
	args = args[1:]
	parent := detectParam(args, "-p")
	msg := detectParam(args, "-m")

	commit := &Commit{
		timestamp: time.Now().Local(),
		parent:    parent,
		tree:      tree,
		author:    "Antonio Petrillo",
		email:     "Antonio Petrillo",
		message:   msg,
	}

	sha, err := WriteContent(commit)
	if err != nil {
		return err
	}

	fmt.Printf("%x\n", sha)

	return nil
}

func HandlerClone(name string, args []string) error {
	if name != CloneCmd {
		return MismatchedError
	}

	if len(args) != 2 {
		return InvalidArgsError
	}

	repo, dir := args[0], args[1]

	curDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if !path.IsAbs(dir) {
		dir = path.Join(curDir, dir)
	}

	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}

	// switch to directory
	if err := os.Chdir(dir); err != nil {
		return err
	}

	// init repo
	if err := HandlerInit(InitCmd, nil); err != nil {
		return err
	}

	// clone repo
	if err := clonePlumbing(repo); err != nil {
		return err
	}

	// go to prev directory
	if err := os.Chdir(curDir); err != nil {
		return err
	}

	return nil
}

func clonePlumbing(url string) error {
	// get last commit from main refs
	hash, err := GetLastHash(url)
	if err != nil {
		return err
	}
	// create .git/refs/heads/main file containing the hash of the last commit
	refPath := ".git/refs/heads"
	err = os.MkdirAll(refPath, 0o755)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path.Join(refPath, "main"), os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = file.Write(hash); err != nil {
		return err
	}

	data, err := UploadPack(url, hash)
	if err != nil {
		return err
	}
	checksum := sha1.Sum(data[:len(data)-20])
	if !bytes.Equal(checksum[:], data[len(data)-20:]) {
		return fmt.Errorf("Mismatched hashes, want '%x' got '%x'", data[len(data)-20:], checksum)
	}

	err = ParseObjects(data[:len(data)-20])
	if err != nil {
		return err
	}

	err = Checkout(string(hash))
	if err != nil {
		return err
	}

	return nil
}
