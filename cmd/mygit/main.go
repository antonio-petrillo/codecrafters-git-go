package main

import (
	"fmt"
	"github.com/codecrafters-io/git-starter-go/git"
	"os"
)

// Usage: your_program.sh <command> <arg1> <arg2> ...
func main() {
	fmt.Fprintf(os.Stderr, "Logs from your program will appear here!\n")

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		err := git.Init()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing repository: %s\n", err)
			os.Exit(1)
		}

	case "cat-file":
		err := git.CatFile(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fatal error in git cat-file: %q\n", err)
			os.Exit(1)
		}

	case "hash-object":
		err := git.HashObject(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fatal error in git hash-object: %q\n", err)
			os.Exit(1)
		}

	case "ls-tree":
		err := git.ListTree(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fatal error in git ls-tree: %q\n", err)
			os.Exit(1)
		}

	case "write-tree":
		err := git.WriteTree()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fatal error in git write-tree: %q\n", err)
			os.Exit(1)
		}

	case "commit-tree":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Fatal error in git commit-tree: Missing SHA param\n")
			os.Exit(1)
		}
		err := git.CommitTree(os.Args[2], os.Args[3:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fatal error in git commit-tree: %q\n", err)
			os.Exit(1)
		}

	case "clone":
		panic("Clone still not implemented")

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
