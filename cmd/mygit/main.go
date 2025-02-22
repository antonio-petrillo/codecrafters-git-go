package main

import (
	"fmt"
	"os"
)



func main() {

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

	command, args := os.Args[1], os.Args[2:]
	handler, ok := AvailableCommands[command]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknow command %q", command)
		os.Exit(1)
	}
	handler(command, args)
}
