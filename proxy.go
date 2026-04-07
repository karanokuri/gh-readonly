package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

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

func startProxy() (port string, caCert *x509.Certificate, cleanup func(), err error) {
	tlsCert, x509Cert, err := generateCert()
	if err != nil {
		return "", nil, nil, fmt.Errorf("cert generation failed: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, nil, fmt.Errorf("listen failed: %w", err)
	}

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodConnect {
				http.Error(w, "only CONNECT is supported", http.StatusMethodNotAllowed)
				return
			}

			hijack, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijacking not supported", http.StatusInternalServerError)
				return
			}

			clientConn, _, hijackErr := hijack.Hijack()
			if hijackErr != nil {
				return
			}

			clientConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))

			tlsServerConn := tls.Server(clientConn, &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
				NextProtos:   []string{"http/1.1"},
			})
			defer tlsServerConn.Close()

			if err := tlsServerConn.Handshake(); err != nil {
				return
			}

			serveConnections(tlsServerConn)
		}),
	}

	go server.Serve(ln)

	addr := ln.Addr().(*net.TCPAddr)
	port = fmt.Sprintf("%d", addr.Port)

	cleanup = func() { server.Close() }
	return port, x509Cert, cleanup, nil
}

func serveConnections(conn net.Conn) {
	defer conn.Close()

	upstream := &http.Transport{}
	br := bufio.NewReader(conn)

	for {
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}

		shouldBlock := writeMethods[req.Method]
		if shouldBlock && req.URL.Path == "/graphql" {
			body, err := io.ReadAll(req.Body)
			req.Body.Close()
			if err == nil {
				req.Body = io.NopCloser(bytes.NewReader(body))
				shouldBlock = isGraphQLMutation(body)
			}
		}

		if shouldBlock {
			io.Copy(io.Discard, req.Body)
			req.Body.Close()

			msg := fmt.Sprintf(
				"gh-readonly: denied — write operation (%s %s) is not allowed. Use 'gh' directly (requires user approval).\n",
				req.Method, req.URL.Path,
			)
			resp := &http.Response{
				StatusCode:    http.StatusForbidden,
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
				Body:          io.NopCloser(strings.NewReader(msg)),
				ContentLength: int64(len(msg)),
			}
			resp.Write(conn)

			if req.Close || strings.EqualFold(req.Header.Get("Connection"), "close") {
				return
			}
			continue
		}

		// forward to api.github.com
		req.URL = &url.URL{
			Scheme:   "https",
			Host:     "api.github.com",
			Path:     req.URL.Path,
			RawQuery: req.URL.RawQuery,
		}
		req.Host = "api.github.com"
		req.RequestURI = ""

		resp, err := upstream.RoundTrip(req)
		if err != nil {
			errMsg := fmt.Sprintf("proxy error: %v\n", err)
			errResp := &http.Response{
				StatusCode:    http.StatusBadGateway,
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
				Body:          io.NopCloser(strings.NewReader(errMsg)),
				ContentLength: int64(len(errMsg)),
			}
			errResp.Write(conn)
			return
		}

		// buffer body to get exact Content-Length and avoid HTTP/2 framing issues
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
		// downgrade to HTTP/1.1 so gh can parse the response correctly
		resp.Proto = "HTTP/1.1"
		resp.ProtoMajor = 1
		resp.ProtoMinor = 1

		respClose := resp.Close
		resp.Write(conn)

		if req.Close || respClose || strings.EqualFold(req.Header.Get("Connection"), "close") {
			return
		}
	}
}
