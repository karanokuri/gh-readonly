package main

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"testing"
)

func makeProxyClient(port string, caCert *x509.Certificate) *http.Client {
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	proxyURL, _ := url.Parse("http://127.0.0.1:" + port)
	return &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}
}

func startTestProxy(t *testing.T) (*http.Client, func()) {
	t.Helper()
	port, caCert, cleanup, err := startProxy()
	if err != nil {
		t.Fatalf("startProxy: %v", err)
	}
	return makeProxyClient(port, caCert), cleanup
}

func ghToken(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		t.Skip("gh not authenticated")
	}
	return strings.TrimSpace(string(out))
}

// TestProxyDenyWrite はネットワーク不要（deny はプロキシ内で完結する）
func TestProxyDenyWrite(t *testing.T) {
	client, cleanup := startTestProxy(t)
	defer cleanup()

	req, _ := http.NewRequest("POST", "https://api.github.com/repos/owner/repo/issues", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "gh-readonly: denied") {
		t.Errorf("expected deny message, got: %s", body)
	}
}

// TestProxyAllowGet はGETリクエストが転送されることを確認（要gh認証）
func TestProxyAllowGet(t *testing.T) {
	token := ghToken(t)
	client, cleanup := startTestProxy(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "https://api.github.com/repos/karanokuri/gh-readonly", nil)
	req.Header.Set("Authorization", "token "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		t.Error("GET request should not be denied")
	}
}

// TestProxyDenyPatch はネットワーク不要（deny はプロキシ内で完結する）
func TestProxyDenyPatch(t *testing.T) {
	client, cleanup := startTestProxy(t)
	defer cleanup()

	req, _ := http.NewRequest("PATCH", "https://api.github.com/repos/owner/repo/issues/1", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "gh-readonly: denied") {
		t.Errorf("expected deny message, got: %s", body)
	}
}

// TestProxyDenyDelete はネットワーク不要（deny はプロキシ内で完結する）
func TestProxyDenyDelete(t *testing.T) {
	client, cleanup := startTestProxy(t)
	defer cleanup()

	req, _ := http.NewRequest("DELETE", "https://api.github.com/repos/owner/repo/issues/1", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "gh-readonly: denied") {
		t.Errorf("expected deny message, got: %s", body)
	}
}
