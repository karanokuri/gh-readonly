package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

func ghConfigDir() (string, error) {
	if d := os.Getenv("GH_CONFIG_DIR"); d != "" {
		return d, nil
	}
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "gh"), nil
	}
	if runtime.GOOS == "windows" {
		if d := os.Getenv("AppData"); d != "" {
			return filepath.Join(d, "GitHub CLI"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gh"), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func setupConfigDir(ghPath, socketPath string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "gh-readonly-cfg-")
	if err != nil {
		return "", fmt.Errorf("mkdir temp: %w", err)
	}

	srcDir, err := ghConfigDir()
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("resolve gh config dir: %w", err)
	}

	for _, name := range []string{"config.yml", "hosts.yml"} {
		src := filepath.Join(srcDir, name)
		dst := filepath.Join(tmpDir, name)
		if err := copyFile(src, dst); err != nil && !os.IsNotExist(err) {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("copy %s: %w", name, err)
		}
	}

	cmd := exec.Command(ghPath, "config", "set", "http_unix_socket", socketPath)
	cmd.Env = append(os.Environ(), "GH_CONFIG_DIR="+tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		if len(output) > 0 {
			return "", fmt.Errorf("gh config set: %w: %s", err, output)
		}
		return "", fmt.Errorf("gh config set: %w", err)
	}

	return tmpDir, nil
}

func runGH(ghPath string, args []string, socketPath string) int {
	tmpDir, err := setupConfigDir(ghPath, socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gh-readonly: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command(ghPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "GH_CONFIG_DIR="+tmpDir)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig, ok := <-sigCh
		if ok && cmd.Process != nil {
			cmd.Process.Signal(sig)
		}
	}()

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}
