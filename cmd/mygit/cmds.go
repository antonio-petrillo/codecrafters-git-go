package main

import (
	"errors"
	"fmt"
	"os"
)

const (
	Init    = "init"
	CatFile = "cat-file"
)

type Handler func(name string, args []string) error
type Commands map[string]Handler

var (
	CommandNotFoundError = errors.New("Command not found")
	MismatchedError      = errors.New("Mismatched command")
	InvalidArgsError     = errors.New("Invalid arguments")
)

var availableCommands = Commands{
	Init: HandlerInit,
	CatFile: HandlerCatFile,
}

func GetCommand(cmd string) (Handler, error) {
	handler, ok := availableCommands[cmd]
	if !ok {
		return nil, CommandNotFoundError
	}
	return handler, nil
}

func HandlerInit(name string, args []string) error {
	if name != Init {
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
	if name != CatFile {
		return MismatchedError
	}

	if len(args) != 2 {
		return InvalidArgsError
	}

	switch verb := args[0]; verb {
	case "-p":

		gitObj, err := ReadFromDisk(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("%s", gitObj)

	default:
		return InvalidArgsError
	}

	return nil
}
