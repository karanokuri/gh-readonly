package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func findRealGH() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine own path: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return "", fmt.Errorf("cannot resolve own path: %w", err)
	}

	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		candidate := filepath.Join(dir, "gh")
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			continue
		}
		if resolved != self {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("real gh binary not found in PATH")
}
