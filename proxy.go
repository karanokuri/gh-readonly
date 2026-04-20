package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func isGraphQLMutation(body []byte) bool {
	var payload struct {
		Query         string `json:"query"`
		OperationName string `json:"operationName"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return true // パース不能は安全側に倒して遮断
	}
	doc, err := parser.ParseQuery(&ast.Source{Input: payload.Query})
	if err != nil {
		return true
	}
	for _, op := range doc.Operations {
		if payload.OperationName == "" || op.Name == payload.OperationName {
			if op.Operation != ast.Query {
				return true
			}
		}
	}
	return false
}

var writeMethods = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

func shouldBlockRequest(r *http.Request) bool {
	blocked := writeMethods[r.Method]
	if blocked && r.URL.Path == "/graphql" {
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err == nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
			blocked = isGraphQLMutation(body)
		}
	}
	return blocked
}

func startProxy() (socketPath string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "gh-readonly-")
	if err != nil {
		return "", nil, fmt.Errorf("tempdir failed: %w", err)
	}
	socketPath = filepath.Join(dir, "gh.sock")

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		os.RemoveAll(dir)
		return "", nil, fmt.Errorf("listen failed: %w", err)
	}

	reverseProxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme = "https"
			pr.Out.URL.Host = pr.In.Host
			pr.Out.Host = pr.In.Host
		},
	}

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldBlockRequest(r) {
				io.Copy(io.Discard, r.Body)
				msg := fmt.Sprintf(
					"gh-readonly: denied — write operation (%s %s) is not allowed. Use 'gh' directly (requires user approval).\n",
					r.Method, r.URL.Path,
				)
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(msg))
				return
			}
			reverseProxy.ServeHTTP(w, r)
		}),
	}

	go server.Serve(ln)

	cleanup = func() {
		server.Close()
		os.RemoveAll(dir)
	}
	return socketPath, cleanup, nil
}
