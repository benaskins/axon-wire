package wire

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestTransport_RoundTrip_GET(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("proxy method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/proxy" {
			t.Errorf("proxy path = %s, want /proxy", r.URL.Path)
		}
		if r.Header.Get("X-Wire-Token") != "test-token" {
			t.Errorf("token = %q", r.Header.Get("X-Wire-Token"))
		}

		var req proxyRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !strings.HasPrefix(req.URL, "http") {
			t.Errorf("url = %q", req.URL)
		}
		if req.Method != "GET" {
			t.Errorf("method = %q, want GET", req.Method)
		}

		w.Header().Set("X-Proxy-Status", "200")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"origin": "cloudflare"})
	}))
	defer proxy.Close()

	transport := &Transport{ProxyURL: proxy.URL, Token: "test-token"}
	client := &http.Client{Transport: transport}

	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["origin"] != "cloudflare" {
		t.Errorf("origin = %q, want cloudflare", body["origin"])
	}
}

func TestTransport_RoundTrip_POST(t *testing.T) {
	var gotReq proxyRequest
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("X-Proxy-Status", "201")
		w.Write([]byte(`{"id": "123"}`))
	}))
	defer proxy.Close()

	transport := &Transport{ProxyURL: proxy.URL, Token: "tok"}
	client := &http.Client{Transport: transport}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.example.com/items", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	defer resp.Body.Close()

	if gotReq.URL != "https://api.example.com/items" {
		t.Errorf("url = %q", gotReq.URL)
	}
	if gotReq.Method != "POST" {
		t.Errorf("method = %q", gotReq.Method)
	}
	if gotReq.Body != `{"name":"test"}` {
		t.Errorf("body = %q", gotReq.Body)
	}
	if gotReq.Headers["Authorization"] != "Bearer secret" {
		t.Errorf("auth header = %q", gotReq.Headers["Authorization"])
	}
	if resp.StatusCode != 201 {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
}

func TestTransport_RoundTrip_ProxyError(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error": "upstream failed"}`))
	}))
	defer proxy.Close()

	transport := &Transport{ProxyURL: proxy.URL, Token: "tok"}
	client := &http.Client{Transport: transport}

	resp, err := client.Get("https://example.com")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

func TestTransport_HeadersPassedThrough(t *testing.T) {
	var gotReq proxyRequest
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("X-Proxy-Status", "200")
		w.Write([]byte("ok"))
	}))
	defer proxy.Close()

	transport := &Transport{ProxyURL: proxy.URL, Token: "tok"}
	client := &http.Client{Transport: transport}

	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("X-Custom", "value")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	defer resp.Body.Close()

	if gotReq.Headers["Accept"] != "text/html" {
		t.Errorf("accept = %q", gotReq.Headers["Accept"])
	}
	if gotReq.Headers["X-Custom"] != "value" {
		t.Errorf("x-custom = %q", gotReq.Headers["X-Custom"])
	}
}

func TestTransport_NoToken(t *testing.T) {
	var gotToken string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Wire-Token")
		w.Header().Set("X-Proxy-Status", "200")
		w.Write([]byte("ok"))
	}))
	defer proxy.Close()

	transport := &Transport{ProxyURL: proxy.URL}
	client := &http.Client{Transport: transport}

	resp, err := client.Get("https://example.com")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	defer resp.Body.Close()

	if gotToken != "" {
		t.Errorf("token should be empty, got %q", gotToken)
	}
}

func TestTransport_NilBody(t *testing.T) {
	var gotReq proxyRequest
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("X-Proxy-Status", "200")
		w.Write([]byte("ok"))
	}))
	defer proxy.Close()

	transport := &Transport{ProxyURL: proxy.URL, Token: "tok"}
	client := &http.Client{Transport: transport}

	resp, err := client.Get("https://example.com/page")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	defer resp.Body.Close()

	if gotReq.Body != "" {
		t.Errorf("body should be empty for GET, got %q", gotReq.Body)
	}
}

func TestNewTransport_NoEnvVar(t *testing.T) {
	t.Setenv("AXON_WIRE_URL", "")
	transport := NewTransport()
	if transport != nil {
		t.Error("expected nil transport when AXON_WIRE_URL is not set")
	}
}

func TestNewTransport_WithEnvVar(t *testing.T) {
	t.Setenv("AXON_WIRE_URL", "https://proxy.example.workers.dev")
	t.Setenv("AXON_WIRE_TOKEN", "my-secret")

	transport := NewTransport()
	if transport == nil {
		t.Fatal("expected non-nil transport")
	}
	if transport.ProxyURL != "https://proxy.example.workers.dev" {
		t.Errorf("proxy url = %q", transport.ProxyURL)
	}
	if transport.Token != "my-secret" {
		t.Errorf("token = %q", transport.Token)
	}
}

func TestNewClient_FallsBackToDefault(t *testing.T) {
	t.Setenv("AXON_WIRE_URL", "")
	client := NewClient()
	if client.Transport != nil {
		t.Error("expected nil transport (default) when wire not configured")
	}
}

func TestNewClient_UsesWireTransport(t *testing.T) {
	t.Setenv("AXON_WIRE_URL", "https://proxy.example.com")
	t.Setenv("AXON_WIRE_TOKEN", "tok")

	client := NewClient()
	if _, ok := client.Transport.(*Transport); !ok {
		t.Errorf("expected *Transport, got %T", client.Transport)
	}
}

func TestTransport_LiveProxy(t *testing.T) {
	proxyURL := "https://wire-proxy.ben-askins.workers.dev"
	token := os.Getenv("AXON_WIRE_TOKEN")
	if token == "" {
		t.Skip("AXON_WIRE_TOKEN not set")
	}

	transport := &Transport{ProxyURL: proxyURL, Token: token}
	client := &http.Client{Transport: transport}

	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	json.Unmarshal(body, &result)

	origin, _ := result["origin"].(string)
	t.Logf("origin=%q (should be Cloudflare IP, not home IP)", origin)
	if origin == "" {
		t.Error("expected non-empty origin")
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}
