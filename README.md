# axon-wire

HTTP transport that routes outbound requests through a Cloudflare Worker proxy. Your IP is never exposed to the target.

## Usage

```go
// Automatically routes through the proxy when AXON_WIRE_URL is set.
// Falls back to direct HTTP when not configured.
client := wire.NewClient()

resp, err := client.Get("https://api.example.com/data")
```

### Environment variables

| Variable | Required | Description |
|---|---|---|
| `AXON_WIRE_URL` | Yes | Base URL of the wire proxy worker |
| `AXON_WIRE_TOKEN` | No | Shared secret for `X-Wire-Token` auth |

When `AXON_WIRE_URL` is not set, `NewClient()` returns a default `*http.Client` with no proxy — zero cost to opt out.

### Plug into an existing client

```go
transport := wire.NewTransport()
if transport != nil {
    myClient.Transport = transport
}
```

### Direct construction

```go
transport := &wire.Transport{
    ProxyURL: "https://wire-proxy.example.workers.dev",
    Token:    "secret",
}
client := &http.Client{Transport: transport}
```

## How it works

`wire.Transport` implements `http.RoundTripper`. Instead of connecting directly to the target URL, it serialises the request (URL, method, headers, body) as JSON and POSTs it to the proxy worker at `/proxy`. The proxy forwards the request from Cloudflare's network and returns the response.

```
your service → wire.Transport → POST /proxy → Cloudflare Worker → target
                                                (CF IP exposed, not yours)
```

## Install

```
go get github.com/benaskins/axon-wire
```

## License

MIT
