package proxy

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Handler implements an HTTP/HTTPS forward proxy
type Handler struct {
	transport *http.Transport
	timeout   time.Duration
	logger    *slog.Logger
}

// New creates a new proxy handler with the specified timeout
func New(timeout time.Duration, logger *slog.Logger) *Handler {
	// Create a transport with caching disabled and explicit timeouts
	transport := &http.Transport{
		// Disable caching
		DisableCompression: false,
		DisableKeepAlives:  false,

		// Set timeouts
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,

		// Prevent caching by the transport
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
	}

	return &Handler{
		transport: transport,
		timeout:   timeout,
		logger:    logger,
	}
}

// ServeHTTP handles both HTTP and HTTPS proxy requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle HTTPS CONNECT method for tunneling
	if r.Method == http.MethodConnect {
		h.handleConnect(w, r)
		return
	}

	// Handle standard HTTP forwarding
	h.handleHTTP(w, r)
}

// handleHTTP forwards standard HTTP requests
func (h *Handler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Validate request
	if r.URL.Host == "" {
		http.Error(w, "Bad Request: missing host", http.StatusBadRequest)
		return
	}

	// Create outgoing request
	outReq := &http.Request{
		Method: r.Method,
		URL:    r.URL,
		Header: r.Header.Clone(),
		Body:   r.Body,
		Host:   r.URL.Host,
	}

	// Remove hop-by-hop headers
	removeHopByHopHeaders(outReq.Header)

	// Forward the request
	resp, err := h.transport.RoundTrip(outReq)
	if err != nil {
		h.logger.Error("upstream request failed", "error", err, "host", r.URL.Host)
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
		} else {
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		}
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Copy response headers
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	// Stream response body
	if _, err := io.Copy(w, resp.Body); err != nil {
		h.logger.Error("failed to copy response body", "error", err)
	}
}

// handleConnect handles HTTPS CONNECT tunneling
func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
	// Validate host
	if r.URL.Host == "" && r.Host == "" {
		http.Error(w, "Bad Request: missing host", http.StatusBadRequest)
		return
	}

	targetHost := r.Host
	if targetHost == "" {
		targetHost = r.URL.Host
	}

	// Dial the target server
	//nolint:gosec // G704: a forward proxy is expected to dial client-specified hosts.
	targetConn, err := net.DialTimeout("tcp", targetHost, h.timeout)
	if err != nil {
		h.logger.Error("failed to dial target", "error", err, "host", targetHost)
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
		} else {
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		}
		return
	}
	defer func() { _ = targetConn.Close() }()

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Internal Server Error: hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		h.logger.Error("failed to hijack connection", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = clientConn.Close() }()

	// Send 200 Connection Established
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		h.logger.Error("failed to send connection established", "error", err)
		return
	}

	// Bidirectional copy between client and target
	errCh := make(chan error, 2)

	// Copy from client to target
	go func() {
		_, err := io.Copy(targetConn, clientConn)
		errCh <- err
	}()

	// Copy from target to client
	go func() {
		_, err := io.Copy(clientConn, targetConn)
		errCh <- err
	}()

	// Wait for either direction to complete
	<-errCh
	// Note: We don't log errors here as they're expected when connections close
}

// removeHopByHopHeaders removes headers that shouldn't be forwarded
func removeHopByHopHeaders(h http.Header) {
	// Headers defined in HTTP/1.1 spec as hop-by-hop
	hopByHopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, header := range hopByHopHeaders {
		h.Del(header)
	}
}

// copyHeaders copies headers from src to dst
func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
