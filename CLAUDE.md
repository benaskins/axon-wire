# CLAUDE.md

Read [AGENTS.md](./AGENTS.md) for project context.

## Build & Test

```bash
go test ./...              # Unit tests (no network)
go test -v -run TestLive   # Live test (requires AXON_WIRE_TOKEN)
```

## Architecture

Single file module. `Transport` implements `http.RoundTripper`, `NewClient()` is the convenience wrapper. No dependencies beyond stdlib.
