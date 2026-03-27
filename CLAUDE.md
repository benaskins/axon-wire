@AGENTS.md

## Conventions
- `Transport` implements `http.RoundTripper` — used as a drop-in for `http.Client.Transport`
- `NewClient()` returns a proxied client when `AXON_WIRE_URL` is set, plain client otherwise
- Stdlib only — no external dependencies

## Constraints
- No dependencies on any axon-* module — this is fully standalone
- HTTPS is enforced for the proxy URL in production; do not weaken this
- `AXON_WIRE_TOKEN` must never appear in logs, errors, or test output
- Do not add request/response body inspection — the transport is opaque

## Testing
- `go test ./...` — unit tests with no network required
- `go test -v -run TestLive` — live tests require `AXON_WIRE_TOKEN` env var
- `go vet ./...` for lint
