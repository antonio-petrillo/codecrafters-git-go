package main

import (
	"fmt"
	"os"
	"strings"
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

	case "cat-file":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Missing SHA in git cat-file\n")
			os.Exit(1)
		}
		switch verb := os.Args[2]; verb {
		case "-p":
			shaArg := os.Args[3]
			dir, sha := shaArg[:2], shaArg[2:]
			blob, err := ReadBlob(dir, sha)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Can't read blob file %q\n", shaArg)
				os.Exit(1)
			}
			fmt.Print(string(blob.Content))
		default:
			fmt.Fprintf(os.Stderr, "Unknown subcommand for cat-file %q\n", command)
			os.Exit(1)
		}

	case "hash-object":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Missing filename or verb in git hash-object\n")
			os.Exit(1)
		}
		writeToFile := strings.TrimSpace(os.Args[2]) == "-w"
		var path string
		if writeToFile {
			path = os.Args[3]
		} else {
			path = os.Args[2]
		}

		hash, err := WriteBlob(path, writeToFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't hash object\n")
			os.Exit(1)
		}
		fmt.Printf("%x\n", hash)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
