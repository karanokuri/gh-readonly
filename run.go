package main

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func runGH(ghPath string, args []string, port string, caCert *x509.Certificate) int {
	tmp, err := os.CreateTemp("", "gh-readonly-cert-*.pem")
	if err != nil {
		return 1
	}
	defer os.Remove(tmp.Name())

	if err := pem.Encode(tmp, &pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw}); err != nil {
		tmp.Close()
		return 1
	}
	tmp.Close()

	cmd := exec.Command(ghPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"HTTPS_PROXY=http://127.0.0.1:"+port,
		"SSL_CERT_FILE="+tmp.Name(),
	)

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
