package main

import (
	"errors"
	"fmt"
	"os"
)

const (
	Init string = "init"
)

type Handler func(name string, args []string) error
type Commands map[string]Handler

var (
	MismatchedError = errors.New("Mismatched command")
	InvalidArgs     = errors.New("Invalid arguments")
)

var AvailableCommands = Commands {
	Init: HandlerInit,
}

func HandlerInit(name string, args []string) error {
	if name != Init {
		return MismatchedError
	}

	if len(args) > 0 {
		return InvalidArgs
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
