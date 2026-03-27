---
module: github.com/benaskins/axon-wire
kind: library
---

# axon-wire

HTTP transport that routes outbound requests through a Cloudflare Worker proxy. Your IP is never exposed to the target.

## What it does

- `Transport` implements `http.RoundTripper` — drop into any `*http.Client`
- `NewClient()` returns a proxied client when `AXON_WIRE_URL` is set, plain client otherwise
- Serialises requests as JSON, POSTs to `/proxy` on the worker, returns the response

## Architecture

- `wire.go` — Transport, NewTransport, NewClient
- `wire_test.go` — unit tests (no network required)

## Running tests

```bash
go test ./...              # Unit tests (no network)
go test -v -run TestLive   # Live test (requires AXON_WIRE_TOKEN)
go vet ./...
```

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `AXON_WIRE_URL` | Yes | Base URL of the wire proxy worker |
| `AXON_WIRE_TOKEN` | No | Shared secret for `X-Wire-Token` auth |

## Dependencies

- No external dependencies — stdlib only
