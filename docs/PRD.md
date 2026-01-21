# PRD: Simple Non-Caching Forward Web Proxy (Go)

## 1. Overview

### Product Name
Simple Forward Proxy

### Purpose
Build a minimal, production-ready **non-caching forward HTTP/HTTPS proxy** written in **Go**, intended for lightweight traffic forwarding, debugging, or controlled network routing.

The proxy must:
- Forward requests transparently
- Perform **no caching**
- Be configurable **only via environment variables**
- Be easy to distribute and run via **Docker**

### Target Audience
- Developers
- DevOps / SREs
- Security engineers
- CI/CD environments
- Lightweight infrastructure setups

---

## 2. Goals & Non-Goals

### Goals
- Simple, readable Go codebase
- Forward HTTP and HTTPS traffic
- No request/response modification
- Environment-based configuration
- Docker-first distribution
- Fast startup and low memory footprint

### Non-Goals
- No caching
- No authentication
- No request logging beyond basic startup/errors
- No TLS termination (acts as a forward proxy, not reverse proxy)
- No rate limiting
- No metrics endpoint

---

## 3. Technical Requirements

### Language & Runtime
- **Go (latest stable version at implementation time)**  
  Example: Go 1.25+ (do not hardcode version unless required)

### Configuration
- Use **github.com/caarlos0/env/v11**
- All configuration via **environment variables only**
- No config files, no CLI flags

#### Required Environment Variables
| Variable | Type | Default | Description |
|--------|------|---------|-------------|
| `BIND_HOST` | string | `0.0.0.0` | Host/IP to bind |
| `BIND_PORT` | int | `3128` | Port to listen on |
| `TIMEOUT` | duration | `60s` | Upstream request timeout |

---

## 4. Functional Requirements

### 4.1 HTTP Forwarding
- Accept standard HTTP proxy requests
- Forward method, headers, body as-is
- Stream responses without buffering
- Preserve request/response headers

### 4.2 HTTPS Forwarding
- Support HTTPS via `CONNECT` method
- Establish TCP tunnel
- No TLS inspection
- No MITM

### 4.3 Error Handling
- Graceful handling of:
  - Invalid requests
  - Upstream timeouts
  - Network errors
- Return appropriate HTTP error codes (e.g. `502 Bad Gateway`, `504 Gateway Timeout`)

---

## 5. Non-Functional Requirements

### Performance
- Streaming I/O (no full buffering)
- No request caching
- Minimal allocations

### Security
- No open file writes
- No shell execution
- No dependency bloat

### Observability
- Log:
  - Use log/slog for structured logging in json format
  - Startup configuration
  - Fatal errors
- Do NOT log request bodies or headers

---

## 6. Testing

-   Unit tests

---

## 7. Project Structure

```
/
├── cmd/
│   └── whelk/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   └── proxy/
│       └── handler.go
├── go.mod
├── go.sum
├── Dockerfile
└── README.md
```

---

## 8. Implementation Notes

### HTTP Server
- Use `net/http`
- Use a custom `http.Transport`
- Disable:
  - Response caching
  - Compression (optional)
- Set timeouts explicitly

### CONNECT Handling
- Hijack connection using `http.Hijacker`
- Dial upstream using `net.DialTimeout`
- Bidirectional `io.Copy`

---

## 9. Docker Requirements

### Dockerfile Requirements
- Multi-stage build
- Final image:
  - Minimal (e.g. `scratch` or `distroless`)
  - Non-root user (preferred)
- Expose configured port
- Binary-only runtime

### Dockerfile Example

```dockerfile
# -------- Build stage --------
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o whelk ./cmd/whelk

# -------- Runtime stage --------
FROM scratch
WORKDIR /app
COPY --from=builder /app/whelk /whelk
USER nonroot:nonroot
ENTRYPOINT ["/whelk"]
```

---

## 10. Acceptance Criteria

- [ ] Builds with latest Go version
- [ ] Reads configuration from environment variables
- [ ] Successfully forwards HTTP requests
- [ ] Successfully tunnels HTTPS via CONNECT
- [ ] No caching behavior
- [ ] Runs successfully via Docker
- [ ] Clean shutdown on SIGTERM/SIGINT

---

## 11. Out of Scope (Explicit)

- Authentication
- Access control lists
- Traffic shaping
- Metrics
- UI
- Persistent storage

---

## 12. Deliverables

- Go source code with tests
- Dockerfile
- Minimal README with usage example
- Clear comments explaining proxy logic

---

## 13. Usage Example

```bash
docker run -p 3128:3128 \
  -e BIND_HOST=0.0.0.0 \
  -e BIND_PORT=3128 \
  -e TIMEOUT=30s \
  whelk
```

---

End of PRD.
