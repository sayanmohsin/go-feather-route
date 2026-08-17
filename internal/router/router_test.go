package router

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sayanmohsin/go-feather-route/internal/config"
)

func TestChatProxiesNonStreamingRequest(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("provider request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer provider-secret" {
			t.Fatal("provider authorization was not forwarded")
		}
		if request.Header.Get("X-Request-ID") != "request-123" {
			t.Fatalf("request id = %q", request.Header.Get("X-Request-ID"))
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"model":"test-model","messages":[],"response_format":{"type":"json_object"}}` {
			t.Fatalf("provider body = %s", body)
		}
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("X-Request-ID", "provider-id")
		response.Header().Set("X-RateLimit-Limit", "10")
		_, _ = response.Write([]byte(`{"id":"chatcmpl_test","choices":[]}`))
	}))
	defer provider.Close()

	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Auth:      config.AuthConfig{APIKey: "gateway-secret"},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	server := httptest.NewServer(NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler())
	defer server.Close()

	request := httptest.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[],"response_format":{"type":"json_object"}}`))
	request.Header.Set("Authorization", "Bearer gateway-secret")
	request.Header.Set("X-Request-ID", "request-123")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "chatcmpl_test") {
		t.Fatalf("body = %s", response.Body.String())
	}
	if response.Header().Get("Server") != "Go-Feather-Route" || response.Header().Get("X-Request-ID") != "request-123" {
		t.Fatalf("gateway headers = server=%q request-id=%q", response.Header().Get("Server"), response.Header().Get("X-Request-ID"))
	}
	if response.Header().Get("X-RateLimit-Limit") != "10" {
		t.Fatalf("rate limit header = %q", response.Header().Get("X-RateLimit-Limit"))
	}
}

func TestChatRequiresAuthentication(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Auth:   config.AuthConfig{APIKey: "gateway-secret"},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model"}`))
	response := httptest.NewRecorder()
	NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestEmbeddingsProxiesBatchRequestAndUsage(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/embeddings" {
			t.Fatalf("provider request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer provider-secret" {
			t.Fatal("provider authorization was not forwarded")
		}
		if request.Header.Get("X-Request-ID") != "embedding-request" {
			t.Fatalf("request id = %q", request.Header.Get("X-Request-ID"))
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"object":"list","data":[{"index":0,"embedding":[1,0]},{"index":1,"embedding":[0,1]}],"model":"embedding-test","usage":{"prompt_tokens":8,"total_tokens":8}}`))
	}))
	defer provider.Close()

	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Auth:      config.AuthConfig{APIKey: "gateway-secret"},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"embedding-test": "openai"},
	}
	server := httptest.NewServer(NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler())
	defer server.Close()

	request := httptest.NewRequest(http.MethodPost, server.URL+"/v1/embeddings", strings.NewReader(`{"model":"embedding-test","input":["first","second"]}`))
	request.Header.Set("Authorization", "Bearer gateway-secret")
	request.Header.Set("X-Request-ID", "embedding-request")
	response := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"total_tokens":8`) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestChatStreamsProviderResponse(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		response.WriteHeader(http.StatusOK)
		flusher := response.(http.Flusher)
		_, _ = response.Write([]byte(`data: {"choices":[{"delta":{"content":"hi"}}]}` + "\n\n"))
		flusher.Flush()
		_, _ = response.Write([]byte("data: [DONE]\n\n"))
	}))
	defer provider.Close()

	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	handler := NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","stream":true,"messages":[]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
	if response.Header().Get("Cache-Control") != "no-cache, no-transform" || response.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("stream headers = cache=%q buffering=%q", response.Header().Get("Cache-Control"), response.Header().Get("X-Accel-Buffering"))
	}
	if !strings.Contains(response.Body.String(), "[DONE]") {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestOperationalEndpoints(t *testing.T) {
	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: "https://api.openai.com/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	handler := NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	for _, test := range []struct {
		path       string
		wantStatus int
	}{
		{path: "/health/live", wantStatus: http.StatusOK},
		{path: "/health/liveliness", wantStatus: http.StatusOK},
		{path: "/ready", wantStatus: http.StatusOK},
		{path: "/status", wantStatus: http.StatusOK},
		{path: "/status/models/test-model", wantStatus: http.StatusOK},
		{path: "/metrics", wantStatus: http.StatusOK},
	} {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestReadinessReportsMissingProviderCredentials(t *testing.T) {
	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: "https://api.openai.com/v1"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()
	NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestGatewayGeneratesRequestID(t *testing.T) {
	cfg := config.Config{Server: config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1}}
	response := httptest.NewRecorder()
	NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	requestID := response.Header().Get("X-Request-ID")
	if len(requestID) != 32 || !isSafeRequestID(requestID) {
		t.Fatalf("generated request id = %q", requestID)
	}
	if response.Header().Get("Server") != "Go-Feather-Route" {
		t.Fatalf("server header = %q", response.Header().Get("Server"))
	}
}

func TestMetricsAccountForUnauthorizedRequests(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Auth:   config.AuthConfig{APIKey: "gateway-secret"},
	}
	handler := NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}
	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(metrics.Body.String(), "go_feather_route_errors_total 1") {
		t.Fatalf("metrics = %s", metrics.Body.String())
	}
}

func TestStreamingCancellationReachesProvider(t *testing.T) {
	providerCanceled := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(`data: {"choices":[]}` + "\n\n"))
		response.(http.Flusher).Flush()
		<-request.Context().Done()
		close(providerCanceled)
	}))
	defer provider.Close()

	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: 5 * time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	gateway := httptest.NewServer(NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler())
	defer gateway.Close()

	ctx, cancel := context.WithCancel(context.Background())
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, gateway.URL+"/v1/chat/completions", strings.NewReader(`{"model":"test-model","stream":true,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	_ = response.Body.Close()

	select {
	case <-providerCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("provider request was not canceled")
	}
}
