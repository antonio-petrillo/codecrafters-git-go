package main

import (
	"fmt"
	"os"
)

func failOnErr(cmd string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error for %s: %v\n", cmd, err)
		os.Exit(1)
	}
}

func main() {

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

	command, args := os.Args[1], os.Args[2:]
	handler, err := GetCommand(command)

	failOnErr(command, err)

	failOnErr(command, handler(command, args))
}
