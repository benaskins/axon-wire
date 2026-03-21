// Package wire provides an http.RoundTripper that routes outbound HTTP
// through a Cloudflare Worker proxy. axon services use this to ensure
// all internet-bound traffic flows through the Cloudflare edge — the
// caller's IP is never exposed to the target.
//
// Set AXON_WIRE_URL and AXON_WIRE_TOKEN to enable. When AXON_WIRE_URL
// is unset, NewTransport returns nil and NewClient returns a default
// http.Client — zero cost to opt out.
package wire

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

// Transport is an http.RoundTripper that routes requests through a
// proxy worker. Instead of connecting directly to the target URL,
// it sends the request details as JSON to the proxy, which forwards
// it on behalf of the caller.
type Transport struct {
	// ProxyURL is the base URL of the proxy worker
	// (e.g. https://wire-proxy.ben-askins.workers.dev).
	ProxyURL string
	// Token is the shared secret for authenticating with the proxy.
	Token string
	// Inner is the transport used to reach the proxy itself.
	// Defaults to http.DefaultTransport.
	Inner http.RoundTripper
}

type proxyRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// validateProxyURL checks that the proxy URL uses HTTPS to prevent
// sending the wire token in cleartext. HTTP is allowed only for
// localhost and 127.0.0.1 to support local development.
func validateProxyURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("wire: invalid proxy URL: %w", err)
	}
	if u.Scheme == "https" {
		return nil
	}
	host := u.Hostname()
	if u.Scheme == "http" && (host == "localhost" || host == "127.0.0.1") {
		return nil
	}
	return fmt.Errorf("wire: proxy URL must use HTTPS to protect the wire token (got %s://%s)", u.Scheme, u.Host)
}

// NewTransport creates a Transport from the AXON_WIRE_URL and
// AXON_WIRE_TOKEN environment variables. Returns nil if AXON_WIRE_URL
// is not set, allowing callers to fall back to direct HTTP.
func NewTransport() *Transport {
	proxyURL := os.Getenv("AXON_WIRE_URL")
	if proxyURL == "" {
		return nil
	}
	if err := validateProxyURL(proxyURL); err != nil {
		return nil
	}
	token := os.Getenv("AXON_WIRE_TOKEN")
	return &Transport{
		ProxyURL: proxyURL,
		Token:    token,
	}
}

// RoundTrip implements http.RoundTripper. It serialises the outgoing request
// into a JSON payload, sends it to the proxy, and reconstructs the
// proxied response.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := validateProxyURL(t.ProxyURL); err != nil {
		return nil, err
	}

	inner := t.Inner
	if inner == nil {
		inner = http.DefaultTransport
	}

	var bodyStr string
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("wire: read request body: %w", err)
		}
		req.Body.Close()
		bodyStr = string(data)
	}

	headers := make(map[string]string, len(req.Header))
	for k, v := range req.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	proxyBody := proxyRequest{
		URL:     req.URL.String(),
		Method:  req.Method,
		Headers: headers,
		Body:    bodyStr,
	}

	jsonBody, err := json.Marshal(proxyBody)
	if err != nil {
		return nil, fmt.Errorf("wire: marshal proxy request: %w", err)
	}

	proxyReq, err := http.NewRequestWithContext(req.Context(), http.MethodPost, t.ProxyURL+"/proxy", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("wire: create proxy request: %w", err)
	}
	proxyReq.Header.Set("Content-Type", "application/json")
	if t.Token != "" {
		proxyReq.Header.Set("X-Wire-Token", t.Token)
	}

	resp, err := inner.RoundTrip(proxyReq)
	if err != nil {
		return nil, fmt.Errorf("wire: proxy request failed: %w", err)
	}

	if proxyStatus := resp.Header.Get("X-Proxy-Status"); proxyStatus != "" {
		if code, err := strconv.Atoi(proxyStatus); err == nil {
			resp.StatusCode = code
			resp.Status = http.StatusText(code)
		}
	}

	return resp, nil
}

// NewClient returns an *http.Client that routes through the proxy
// if AXON_WIRE_URL is set, or a default client if not.
func NewClient() *http.Client {
	transport := NewTransport()
	if transport == nil {
		return &http.Client{}
	}
	return &http.Client{Transport: transport}
}
