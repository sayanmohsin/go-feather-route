package router

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sayanmohsin/go-feather-route/internal/config"
)

func TestMetricsExposeTerminalCountersHistogramsAndBoundedLabels(t *testing.T) {
	attempts := 0
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			response.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = response.Write([]byte(`{"id":"chat","choices":[]}`))
	}))
	defer provider.Close()

	server := NewServer(config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"provider/with spaces": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"model with spaces": "provider/with spaces"},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := server.Handler()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"model with spaces","messages":[]}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := metrics.Body.String()
	for _, want := range []string{
		"go_feather_route_requests_total 1",
		"go_feather_route_provider_errors_total 1",
		"go_feather_route_retries_by_reason_total{reason=\"upstream_5xx\"} 1",
		"go_feather_route_upstream_status_total{provider=\"other\",model=\"other\",status=\"200\"} 1",
		"go_feather_route_request_duration_milliseconds_count 1",
		"go_feather_route_request_duration_milliseconds_bucket{le=\"+Inf\"} 1",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "provider/with spaces") || strings.Contains(body, "model with spaces") {
		t.Fatalf("unbounded metric labels leaked into metrics:\n%s", body)
	}
}

func TestMetricsExposeAuthenticationAndLimitRejections(t *testing.T) {
	server := NewServer(config.Config{
		Server: config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 8, MaxConcurrentRequests: 1},
		Auth:   config.AuthConfig{APIKey: "gateway-secret"},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := server.Handler()
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}
	oversized := httptest.NewRecorder()
	oversizedRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"too-large"}`))
	oversizedRequest.Header.Set("Authorization", "Bearer gateway-secret")
	handler.ServeHTTP(oversized, oversizedRequest)
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status=%d", oversized.Code)
	}
	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := metrics.Body.String()
	if !strings.Contains(body, "go_feather_route_auth_failures_total 1") || !strings.Contains(body, "go_feather_route_limit_rejections_total 1") {
		t.Fatalf("metrics = %s", body)
	}
}

func TestRequestLogsExcludePayloadsAndSanitizeRequestID(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"id":"chat","choices":[]}`))
	}))
	defer provider.Close()
	server := NewServer(config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}, logger)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[{"role":"user","content":"private prompt"}]}`))
	request.Header.Set("X-Request-ID", "request\nsecret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(logs.String(), "private prompt") || strings.Contains(logs.String(), "provider-secret") || strings.Contains(logs.String(), "request\nsecret") {
		t.Fatalf("sensitive payload leaked into logs: %s", logs.String())
	}
	if strings.Contains(logs.String(), "request\\nsecret") {
		t.Fatalf("request id was not sanitized: %s", logs.String())
	}
}

func TestStatusReportsActiveCountersAfterCompletion(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"id":"chat","choices":[]}`))
	}))
	defer provider.Close()
	server := NewServer(config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[]}`)))
	status := httptest.NewRecorder()
	server.Handler().ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/status", nil))
	var payload map[string]any
	if err := json.Unmarshal(status.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"active_chat", "active_embeddings", "streams"} {
		if payload[key] != float64(0) {
			t.Fatalf("%s remained active: %v", key, payload[key])
		}
	}
	if server.active.Load() != 0 {
		t.Fatalf("active requests remained after status completed: %d", server.active.Load())
	}
}
