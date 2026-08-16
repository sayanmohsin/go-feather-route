package router

import (
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
		response.Header().Set("Content-Type", "application/json")
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

	request := httptest.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[]}`))
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
