package main

import (
	"fmt"
	"os"
)

var version = "devel"

func main() {
	realGH, err := findRealGH()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gh-readonly: %v\n", err)
		os.Exit(1)
	}

	port, caCert, cleanup, err := startProxy()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gh-readonly: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	exitCode := runGH(realGH, os.Args[1:], port, caCert)
	os.Exit(exitCode)
}
