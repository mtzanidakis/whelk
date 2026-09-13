package proxy

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestNew(t *testing.T) {
	timeout := 30 * time.Second

	handler := New(timeout, testLogger())

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.timeout != timeout {
		t.Errorf("expected timeout=%v, got %v", timeout, handler.timeout)
	}
	if handler.transport == nil {
		t.Error("expected non-nil transport")
	}
	if handler.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestHandleHTTP_MissingHost(t *testing.T) {
	handler := New(30*time.Second, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleHTTP_ValidRequest(t *testing.T) {
	// Create a test upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "value")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	// Create proxy handler
	handler := New(30*time.Second, testLogger())

	// Create proxy request
	req := httptest.NewRequest(http.MethodGet, upstream.URL, http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if body != "upstream response" {
		t.Errorf("expected body 'upstream response', got '%s'", body)
	}

	if rec.Header().Get("X-Test") != "value" {
		t.Errorf("expected header X-Test=value, got %s", rec.Header().Get("X-Test"))
	}
}

func TestHandleHTTP_InvalidUpstream(t *testing.T) {
	handler := New(1*time.Second, testLogger())

	// Request to non-existent host
	req := httptest.NewRequest(http.MethodGet, "http://invalid-host-that-does-not-exist.local:9999/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should return either 502 Bad Gateway or 504 Gateway Timeout
	if rec.Code != http.StatusBadGateway && rec.Code != http.StatusGatewayTimeout {
		t.Errorf("expected status %d or %d, got %d", http.StatusBadGateway, http.StatusGatewayTimeout, rec.Code)
	}
}

func TestHandleConnect_MissingHost(t *testing.T) {
	handler := New(30*time.Second, testLogger())

	req := httptest.NewRequest(http.MethodConnect, "/", http.NoBody)
	// Explicitly clear the Host header that httptest sets
	req.Host = ""
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleConnect_Tunnel(t *testing.T) {
	// Upstream server reachable through the CONNECT tunnel.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tunneled"))
	}))
	defer upstream.Close()

	handler := New(5*time.Second, testLogger())
	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	proxyAddr := strings.TrimPrefix(proxyServer.URL, "http://")
	upstreamAddr := strings.TrimPrefix(upstream.URL, "http://")

	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		t.Fatalf("failed to dial proxy: %v", err)
	}
	defer func() { _ = conn.Close() }()

	connectReq := "CONNECT " + upstreamAddr + " HTTP/1.1\r\nHost: " + upstreamAddr + "\r\n\r\n"
	if _, err := io.WriteString(conn, connectReq); err != nil {
		t.Fatalf("failed to send CONNECT request: %v", err)
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read status line: %v", err)
	}
	if !strings.Contains(statusLine, "200") {
		t.Fatalf("expected 200 Connection Established, got %q", strings.TrimSpace(statusLine))
	}

	// Consume the remaining response headers up to the blank line.
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("failed to read response headers: %v", err)
		}
		if line == "\r\n" {
			break
		}
	}

	// Send a request through the established tunnel.
	request := "GET / HTTP/1.1\r\nHost: " + upstreamAddr + "\r\nConnection: close\r\n\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatalf("failed to send tunneled request: %v", err)
	}

	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("failed to read tunneled response: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read tunneled body: %v", err)
	}
	if string(body) != "tunneled" {
		t.Errorf("expected body 'tunneled', got %q", body)
	}
}

func TestRemoveHopByHopHeaders(t *testing.T) {
	headers := http.Header{
		"Connection":          []string{"keep-alive"},
		"Keep-Alive":          []string{"timeout=5"},
		"Proxy-Authorization": []string{"Basic abc123"},
		"Content-Type":        []string{"application/json"},
		"X-Custom":            []string{"value"},
	}

	removeHopByHopHeaders(headers)

	// Hop-by-hop headers should be removed
	if headers.Get("Connection") != "" {
		t.Error("Connection header should be removed")
	}
	if headers.Get("Keep-Alive") != "" {
		t.Error("Keep-Alive header should be removed")
	}
	if headers.Get("Proxy-Authorization") != "" {
		t.Error("Proxy-Authorization header should be removed")
	}

	// Other headers should remain
	if headers.Get("Content-Type") != "application/json" {
		t.Error("Content-Type header should remain")
	}
	if headers.Get("X-Custom") != "value" {
		t.Error("X-Custom header should remain")
	}
}

func TestCopyHeaders(t *testing.T) {
	src := http.Header{
		"Content-Type":  []string{"application/json"},
		"X-Multi":       []string{"value1", "value2"},
		"Authorization": []string{"Bearer token"},
	}

	dst := http.Header{}
	copyHeaders(dst, src)

	if dst.Get("Content-Type") != "application/json" {
		t.Error("Content-Type not copied correctly")
	}
	if len(dst["X-Multi"]) != 2 {
		t.Error("Multi-value header not copied correctly")
	}
	if dst.Get("Authorization") != "Bearer token" {
		t.Error("Authorization not copied correctly")
	}
}

func TestHandleHTTP_StreamingResponse(t *testing.T) {
	// Create a test server that streams data
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 5; i++ {
			_, _ = w.Write([]byte("chunk "))
		}
	}))
	defer upstream.Close()

	handler := New(30*time.Second, testLogger())

	req := httptest.NewRequest(http.MethodGet, upstream.URL, http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	expected := strings.Repeat("chunk ", 5)
	if body != expected {
		t.Errorf("expected body '%s', got '%s'", expected, body)
	}
}

func TestHandleHTTP_POSTWithBody(t *testing.T) {
	// Create a test server that echoes the body
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer upstream.Close()

	handler := New(30*time.Second, testLogger())

	requestBody := "test request body"
	req := httptest.NewRequest(http.MethodPost, upstream.URL, strings.NewReader(requestBody))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if body != requestBody {
		t.Errorf("expected body '%s', got '%s'", requestBody, body)
	}
}
