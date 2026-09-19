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

func benchmarkServer(b *testing.B, streaming bool) *Server {
	return benchmarkServerWithOptions(b, streaming, false, false)
}

func benchmarkServerWithOptions(b *testing.B, streaming, embeddings, authenticated bool) *Server {
	b.Helper()
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		if embeddings {
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"object":"list","data":[{"object":"embedding","embedding":[0.1,0.2,0.3],"index":0},{"object":"embedding","embedding":[0.4,0.5,0.6],"index":1}],"model":"test-model","usage":{"prompt_tokens":2,"total_tokens":2}}`))
			return
		}
		if streaming {
			response.Header().Set("Content-Type", "text/event-stream")
			_, _ = response.Write([]byte("data: {}\n\ndata: [DONE]\n\n"))
			return
		}
		_, _ = response.Write([]byte(`{"id":"bench","choices":[]}`))
	}))
	b.Cleanup(provider.Close)
	cfg := config.Config{
		Server: config.ServerConfig{
			RequestTimeout:          time.Second,
			MaxBodyBytes:            1 << 20,
			MaxResponseBytes:        1 << 20,
			MaxConcurrentRequests:   64,
			MaxConcurrentEmbeddings: 64,
		},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL, APIKey: "secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	if authenticated {
		cfg.Auth.APIKey = "gateway-secret"
	}
	return NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func BenchmarkRouteRequest(b *testing.B) {
	handler := benchmarkServer(b, false).Handler()
	b.ReportAllocs()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[],"max_tokens":32,"response_format":{"type":"json_object"}}`))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
}

func BenchmarkProviderSelection(b *testing.B) {
	server := benchmarkServer(b, false)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := server.clientFor("test-model"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuth(b *testing.B) {
	handler := benchmarkServerWithOptions(b, false, false, true).Handler()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		request.Header.Set("Authorization", "Bearer gateway-secret")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
}

func BenchmarkNonStreamingProxy(b *testing.B) {
	handler := benchmarkServer(b, false).Handler()
	b.ReportAllocs()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[],"max_tokens":32,"response_format":{"type":"json_object"}}`))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
}

func BenchmarkRequestIDPropagation(b *testing.B) {
	handler := benchmarkServer(b, false).Handler()
	b.ReportAllocs()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[]}`))
		request.Header.Set("X-Request-ID", "benchmark-request-id")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
}

func BenchmarkEmbeddingProxy(b *testing.B) {
	handler := benchmarkServerWithOptions(b, false, true, false).Handler()
	b.ReportAllocs()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{"model":"test-model","input":["first","second"]}`))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
}

func BenchmarkMetricsEndpoint(b *testing.B) {
	handler := benchmarkServer(b, false).Handler()
	b.ReportAllocs()
	for b.Loop() {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	}
}

func BenchmarkStreamingProxy(b *testing.B) {
	handler := benchmarkServer(b, true).Handler()
	b.ReportAllocs()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","stream":true,"messages":[]}`))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
}

func BenchmarkConcurrentRequests(b *testing.B) {
	handler := benchmarkServer(b, false).Handler()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[]}`))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
		}
	})
}
