package main

import (
	"fmt"
	"os"
)

var version = "devel"

func main() {
	os.Exit(run())
}

func run() int {
	realGH, err := findRealGH()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gh-readonly: %v\n", err)
		return 1
	}

	socketPath, cleanup, err := startProxy()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gh-readonly: %v\n", err)
		return 1
	}
	defer cleanup()

	return runGH(realGH, os.Args[1:], socketPath)
}
