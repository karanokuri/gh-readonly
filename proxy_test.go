package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"testing"
)

func makeProxyClient(socketPath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
	}
}

func startTestProxy(t *testing.T) (*http.Client, func()) {
	t.Helper()
	socketPath, cleanup, err := startProxy()
	if err != nil {
		t.Fatalf("startProxy: %v", err)
	}
	return makeProxyClient(socketPath), cleanup
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

	req, _ := http.NewRequest("POST", "http://api.github.com/repos/owner/repo/issues", nil)
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

	req, _ := http.NewRequest("GET", "http://api.github.com/repos/karanokuri/gh-readonly", nil)
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

	req, _ := http.NewRequest("PATCH", "http://api.github.com/repos/owner/repo/issues/1", nil)
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

// TestProxyDenyGraphQLMutation はネットワーク不要（deny はプロキシ内で完結する）
func TestProxyDenyGraphQLMutation(t *testing.T) {
	client, cleanup := startTestProxy(t)
	defer cleanup()

	body := strings.NewReader(`{"query":"mutation { createIssue(input:{repositoryId:\"1\",title:\"t\"}) { issue { id } } }"}`)
	req, _ := http.NewRequest("POST", "http://api.github.com/graphql", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

// TestProxyAllowGraphQLQuery はGETリクエストと同様にGraphQL queryが通ることを確認（要gh認証）
func TestProxyAllowGraphQLQuery(t *testing.T) {
	token := ghToken(t)
	client, cleanup := startTestProxy(t)
	defer cleanup()

	body := strings.NewReader(`{"query":"query { viewer { login } }"}`)
	req, _ := http.NewRequest("POST", "http://api.github.com/graphql", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		t.Error("GraphQL query should not be denied")
	}
}

// TestProxyDenyDelete はネットワーク不要（deny はプロキシ内で完結する）
func TestProxyDenyDelete(t *testing.T) {
	client, cleanup := startTestProxy(t)
	defer cleanup()

	req, _ := http.NewRequest("DELETE", "http://api.github.com/repos/owner/repo/issues/1", nil)
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
