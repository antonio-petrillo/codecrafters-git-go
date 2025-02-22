package main

import (
	"errors"
	"fmt"
	"os"
)

const (
	InitCmd       = "init"
	CatFileCmd    = "cat-file"
	HashObjectCmd = "hash-object"
	LsTreeCmd     = "ls-tree"
)

type Handler func(name string, args []string) error
type Commands map[string]Handler

var (
	CommandNotFoundError = errors.New("Command not found")
	MismatchedError      = errors.New("Mismatched command")
	InvalidArgsError     = errors.New("Invalid arguments")
)

var availableCommands = Commands{
	InitCmd:       HandlerInit,
	CatFileCmd:    HandlerCatFile,
	HashObjectCmd: HandlerHashObject,
	LsTreeCmd:     HandlerListTree,
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
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
		}
	}

	headFileContents := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
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
