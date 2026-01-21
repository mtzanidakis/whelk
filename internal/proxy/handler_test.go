package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	timeout := 30 * time.Second

	handler := New(timeout, logger)

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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := New(30*time.Second, logger)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
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
		w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	// Create proxy handler
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := New(30*time.Second, logger)

	// Create proxy request
	req := httptest.NewRequest(http.MethodGet, upstream.URL, nil)
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := New(1*time.Second, logger)

	// Request to non-existent host
	req := httptest.NewRequest(http.MethodGet, "http://invalid-host-that-does-not-exist.local:9999/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should return either 502 Bad Gateway or 504 Gateway Timeout
	if rec.Code != http.StatusBadGateway && rec.Code != http.StatusGatewayTimeout {
		t.Errorf("expected status %d or %d, got %d", http.StatusBadGateway, http.StatusGatewayTimeout, rec.Code)
	}
}

func TestHandleConnect_MissingHost(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := New(30*time.Second, logger)

	req := httptest.NewRequest(http.MethodConnect, "/", nil)
	// Explicitly clear the Host header that httptest sets
	req.Host = ""
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
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
		"Content-Type":   []string{"application/json"},
		"X-Multi":        []string{"value1", "value2"},
		"Authorization":  []string{"Bearer token"},
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
			w.Write([]byte("chunk "))
		}
	}))
	defer upstream.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := New(30*time.Second, logger)

	req := httptest.NewRequest(http.MethodGet, upstream.URL, nil)
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
		w.Write(body)
	}))
	defer upstream.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := New(30*time.Second, logger)

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
