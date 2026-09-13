# Whelk - Simple Forward Proxy

A minimal, production-ready **non-caching forward HTTP/HTTPS proxy** written in Go. Designed for lightweight traffic forwarding, debugging, and controlled network routing.

## Features

- Forward HTTP requests transparently
- Support HTTPS via CONNECT tunneling (no TLS inspection)
- No caching
- Environment-based configuration only
- Structured JSON logging
- Graceful shutdown on SIGTERM/SIGINT
- Docker-first distribution
- Fast startup and low memory footprint

## Configuration

All configuration is done via environment variables:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `BIND_HOST` | string | `0.0.0.0` | Host/IP to bind the server to |
| `BIND_PORT` | int | `3128` | Port to listen on |
| `TIMEOUT` | duration | `60s` | Upstream request timeout |

## Usage

### Docker (Recommended)

Build the Docker image:

```bash
docker build -t whelk .
```

Run with default settings:

```bash
docker run -p 3128:3128 whelk
```

Run with custom configuration:

```bash
docker run -p 8080:8080 \
  -e BIND_HOST=0.0.0.0 \
  -e BIND_PORT=8080 \
  -e TIMEOUT=30s \
  whelk
```

### Local Build

Build the binary:

```bash
go build -o whelk ./cmd/whelk
```

Run:

```bash
./whelk
```

With custom configuration:

```bash
BIND_HOST=127.0.0.1 BIND_PORT=8080 TIMEOUT=30s ./whelk
```

## Testing the Proxy

### HTTP Request

```bash
# Set proxy for curl
export http_proxy=http://localhost:3128

# Make HTTP request through proxy
curl http://example.com
```

### HTTPS Request

```bash
# Set proxy for curl
export https_proxy=http://localhost:3128

# Make HTTPS request through proxy (CONNECT tunnel)
curl https://example.com
```

## Development

Tooling is pinned with [mise](https://mise.jdx.dev/) (Go 1.27.1 and golangci-lint 2.13.2). Install the tools and run the tasks:

```bash
mise install        # install pinned tool versions
mise run build      # build the whelk binary
mise run test       # run the unit tests
mise run test:race  # run the unit tests with the race detector
mise run test:cover # run the unit tests with coverage
mise run lint       # run golangci-lint
mise run vet        # run go vet
mise run fmt        # format the Go source
mise run ci         # run vet, lint, test and build
```

List every task with `mise tasks`.

The equivalent commands without mise are:

```bash
go build -o whelk ./cmd/whelk
go test ./...
go test -race ./...
go test -cover ./...
golangci-lint run
```

## Architecture

```
/
├── .github/
│   ├── dependabot.yml
│   └── workflows/
│       ├── ci.yml           # Lint, test and build
│       └── release.yml      # Container image release
├── cmd/
│   └── whelk/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/
│   │   ├── config.go        # Environment configuration
│   │   └── config_test.go   # Configuration tests
│   └── proxy/
│       ├── handler.go       # HTTP/HTTPS proxy handler
│       └── handler_test.go  # Proxy handler tests
├── .golangci.yml            # golangci-lint configuration
├── mise.toml                # Pinned tools and tasks
├── go.mod
├── go.sum
├── Dockerfile               # Multi-stage Docker build
└── README.md
```

## How It Works

### HTTP Forwarding

1. Receives standard HTTP proxy request
2. Forwards method, headers, and body to upstream server
3. Streams response back to client
4. No buffering or caching

### HTTPS Tunneling (CONNECT)

1. Receives CONNECT request with target host
2. Establishes TCP connection to target
3. Hijacks client connection
4. Sends "200 Connection Established"
5. Performs bidirectional data copy (no TLS inspection)

## Limitations

This is a simple forward proxy and intentionally excludes:

- Request/response caching
- Authentication
- Access control
- Rate limiting
- Metrics endpoints
- Request/response logging (only startup and errors)
- TLS termination (acts as forward proxy only)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Security Notes

- Runs as whelk user in Docker (uid=13128)
- No file writes
- No shell execution
- Minimal dependencies
- Static binary with no CGO dependencies
