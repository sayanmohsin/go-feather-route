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
	b.Helper()
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		if streaming {
			response.Header().Set("Content-Type", "text/event-stream")
			_, _ = response.Write([]byte("data: {}\n\ndata: [DONE]\n\n"))
			return
		}
		_, _ = response.Write([]byte(`{"id":"bench","choices":[]}`))
	}))
	b.Cleanup(provider.Close)
	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1 << 20, MaxConcurrentRequests: 64},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL, APIKey: "secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	return NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func BenchmarkRouteRequest(b *testing.B) {
	handler := benchmarkServer(b, false).Handler()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[]}`))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
}

func BenchmarkProviderSelection(b *testing.B) {
	server := benchmarkServer(b, false)
	for b.Loop() {
		if _, err := server.clientFor("test-model"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuth(b *testing.B) {
	handler := benchmarkServer(b, false).Handler()
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	for b.Loop() {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
}

func BenchmarkNonStreamingProxy(b *testing.B) {
	handler := benchmarkServer(b, false).Handler()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[]}`))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
}

func BenchmarkStreamingProxy(b *testing.B) {
	handler := benchmarkServer(b, true).Handler()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","stream":true,"messages":[]}`))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
}

func BenchmarkConcurrentRequests(b *testing.B) {
	handler := benchmarkServer(b, false).Handler()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[]}`))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
		}
	})
}
