# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`httpmqb` is a lightweight, in-memory HTTP message queue broker. Clients push messages via HTTP PUT and consume them via HTTP GET (blocking until a message is available or a timeout expires).

## Commands

```bash
# Run application (default port 5000)
make run
# or: go run cmd/main.go -port 8080

# Build binary
make build              # outputs: httpmqb

# Run tests
go test -v ./...

# Run single package tests
go test -v ./httpmq/

# Docker
make docker             # build images
make dockerup           # docker-compose up
make dockerdown         # docker-compose down
make dockerdebug        # start with Delve debugger on port 2345
```

Port is configured via `-port` flag, `HTTPMQB_PORT` env var, or `.env` file (default: 5000).

## Architecture

### Concurrency Model

The broker uses a **single goroutine event loop** (`httpmq.start()`) to handle all queue state, avoiding mutexes entirely. HTTP handlers communicate with this loop via two channels:
- `pushCh chan pushOp` — for PUT requests
- `popCh chan popOp` — for GET requests

Each operation carries a response channel that the event loop sends the result back on.

### Key Packages

- **`queue/`** — Generic doubly-linked-list queue (`Queue[T]`), used for both message items and waiting listeners.
- **`httpmq/`** — Core broker: `topic` holds a message queue and a listener queue. When a GET arrives with no messages, a listener channel is enqueued; when a PUT arrives with no messages, any waiting listener is served immediately.
- **`logger/`** — Thin singleton wrapper around logrus (JSON output). Use `logger.Info/Warning/Error(msg, logger.Fields{...})`.
- **`cmd/main.go`** — Entry point; wires up the HTTP server and handles SIGINT / "q" for graceful shutdown.

### HTTP API

| Method | Path | Query Params | Behavior |
|--------|------|--------------|----------|
| PUT | `/{topic}` | `v=<message>` | Push message; 400 if topic/value missing |
| GET | `/{topic}` | `timeout=<seconds>` | Pop message; blocks until available or timeout (0 = infinite); 404 on timeout |
