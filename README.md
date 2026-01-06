# Shannon

[![Go Reference](https://pkg.go.dev/badge/github.com/savaki/shannon.svg)](https://pkg.go.dev/github.com/savaki/shannon)
[![Go Report Card](https://goreportcard.com/badge/github.com/savaki/shannon)](https://goreportcard.com/report/github.com/savaki/shannon)
[![CI](https://github.com/savaki/shannon/actions/workflows/ci.yml/badge.svg)](https://github.com/savaki/shannon/actions/workflows/ci.yml)

Shannon is a Go server that enables remote control of [Claude Code](https://claude.ai/claude-code) sessions from mobile devices. It wraps the Claude Code CLI as a subprocess and exposes a RESTful HTTP API for session management, real-time streaming, and mobile app pairing.

## Features

- **Session Management**: Create, list, stop, and delete Claude Code sessions
- **Real-time Streaming**: Server-Sent Events (SSE) for live output streaming
- **Mobile Pairing**: QR code generation for easy mobile app connection
- **Git Integration**: Automatic repository cloning with worktree support for session isolation
- **Secure Authentication**: Pre-shared key (PSK) authentication with constant-time comparison
- **Pure Go**: No CGO dependencies (uses modernc.org/sqlite)

## Installation

```bash
go install github.com/savaki/shannon/cmd/shannon@latest
```

Or build from source:

```bash
git clone https://github.com/savaki/shannon.git
cd shannon
go build -o shannon ./cmd/shannon
```

## Quick Start

```bash
# Start the server with default settings
shannon serve

# Start with custom port and data directory
shannon serve --port 9090 --data-dir /var/lib/shannon

# Start with a git repository for sessions
shannon serve --repo https://github.com/user/project.git --worktree
```

## Usage

```
NAME:
   shannon serve - Start the shannon server

USAGE:
   shannon serve [options]

OPTIONS:
   --port value, -p value       HTTP server port (default: 8080) [$SHANNON_PORT]
   --data-dir value, -d value   Data directory for SQLite and sessions (default: ~/.shannon) [$SHANNON_DATA_DIR]
   --repo value, -r value       Git repository URL to clone for sessions [$SHANNON_REPO]
   --worktree                   Use git worktrees instead of full clones (default: true) [$SHANNON_WORKTREE]
   --firebase-creds value       Path to Firebase service account JSON [$SHANNON_FIREBASE_CREDS]
   --tailscale-authkey value    Tailscale auth key for tsnet [$SHANNON_TAILSCALE_AUTHKEY]
   --log-level value            Log verbosity: debug, info, warn, error (default: info) [$SHANNON_LOG_LEVEL]
   --help, -h                   show help
```

## API

Shannon exposes a RESTful HTTP API. See [PROTOCOL.md](PROTOCOL.md) for complete API documentation.

### Quick Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check (no auth required) |
| POST | `/sessions` | Create a new session |
| GET | `/sessions` | List all sessions |
| GET | `/sessions/{id}` | Get session details |
| DELETE | `/sessions/{id}` | Delete a session |
| POST | `/sessions/{id}/input` | Send input to a session |
| GET | `/sessions/{id}/history` | Get conversation history |
| GET | `/sessions/{id}/stream` | SSE stream of session events |

### Authentication

All endpoints except `/health` require a Bearer token:

```bash
curl -H "Authorization: Bearer $PSK" http://localhost:8080/sessions
```

The PSK is displayed via QR code on server startup for mobile app pairing.

## Architecture

```
┌─────────────────┐
│  Mobile App     │
│  (iOS/Android)  │
└────────┬────────┘
         │ HTTP/SSE
         ▼
┌─────────────────┐
│    Shannon      │
│   (Go Server)   │
├─────────────────┤
│  HTTP API       │
│  Session Mgmt   │
│  Git Manager    │
│  SQLite Store   │
└────────┬────────┘
         │ stdin/stdout
         ▼
┌─────────────────┐
│  Claude Code    │
│  (subprocess)   │
└─────────────────┘
```

## Project Structure

```
shannon/
├── cmd/shannon/          # CLI entry point
├── internal/
│   ├── api/              # HTTP handlers and server
│   ├── auth/             # PSK and QR code generation
│   ├── claude/           # Claude Code wrapper
│   ├── config/           # Configuration management
│   ├── git/              # Git/worktree operations
│   ├── session/          # Session lifecycle management
│   └── storage/          # SQLite persistence
├── PROTOCOL.md           # API documentation
└── README.md
```

## Development

### Prerequisites

- Go 1.24 or later
- Claude Code CLI installed and configured

### Building

```bash
go build -o shannon ./cmd/shannon
```

### Running All Checks

```bash
# Install staticcheck (one time)
go install honnef.co/go/tools/cmd/staticcheck@latest

# Run all checks (vet, staticcheck, tests)
./check.sh
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./...
```

## Configuration

Shannon can be configured via command-line flags or environment variables:

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--port` | `SHANNON_PORT` | 8080 | HTTP server port |
| `--data-dir` | `SHANNON_DATA_DIR` | ~/.shannon | Data directory |
| `--repo` | `SHANNON_REPO` | - | Git repository URL |
| `--worktree` | `SHANNON_WORKTREE` | true | Use git worktrees |
| `--log-level` | `SHANNON_LOG_LEVEL` | info | Log verbosity |

## Dependencies

- [urfave/cli/v2](https://github.com/urfave/cli) - CLI framework
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) - Pure Go SQLite
- [skip2/go-qrcode](https://github.com/skip2/go-qrcode) - QR code generation
- [google/uuid](https://github.com/google/uuid) - UUID generation

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.

## Related Projects

- [Claude Code](https://claude.ai/claude-code) - AI coding assistant
- shannon-ios - iOS companion app (coming soon)
- shannon-android - Android companion app (coming soon)
