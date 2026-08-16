// Package router exposes the OpenAI-compatible HTTP gateway.
package router

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sayanmohsin/go-feather-route/internal/config"
	"github.com/sayanmohsin/go-feather-route/internal/provider"
)

// Server routes authenticated client requests to configured providers.
type Server struct {
	config    config.Config
	providers map[string]provider.Client
	semaphore chan struct{}
	logger    *slog.Logger
	requests  atomic.Uint64
	errors    atomic.Uint64
}

// NewServer constructs a router server from validated configuration.
func NewServer(cfg config.Config, logger *slog.Logger) *Server {
	providers := make(map[string]provider.Client, len(cfg.Providers))
	for name, item := range cfg.Providers {
		providers[name] = provider.Client{
			Name:       name,
			BaseURL:    item.BaseURL,
			APIKey:     item.APIKey,
			HTTPClient: &http.Client{},
		}
	}
	return &Server{
		config:    cfg,
		providers: providers,
		semaphore: make(chan struct{}, cfg.Server.MaxConcurrentRequests),
		logger:    logger,
	}
}

// Handler returns the HTTP handler for the gateway endpoints.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.health)
	mux.HandleFunc("GET /health/liveliness", s.health)
	mux.HandleFunc("GET /ready", s.ready)
	mux.HandleFunc("GET /status", s.status)
	mux.HandleFunc("GET /status/models", s.models)
	mux.HandleFunc("GET /status/models/{model}", s.modelStatus)
	mux.HandleFunc("GET /metrics", s.metrics)
	mux.HandleFunc("GET /v1/models", s.models)
	mux.HandleFunc("POST /v1/chat/completions", s.chat)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		requestID := requestID(request.Header.Get("X-Request-ID"))
		request.Header.Set("X-Request-ID", requestID)
		response.Header().Set("Server", "Go-Feather-Route")
		response.Header().Set("X-Request-ID", requestID)
		if !isPublicPath(request.URL.Path) && !s.authorized(request) {
			s.errors.Add(1)
			s.writeError(response, http.StatusUnauthorized, "invalid or missing bearer token")
			return
		}
		mux.ServeHTTP(response, request)
		s.requests.Add(1)
		s.logger.Info("request", "method", request.Method, "path", request.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}

func isPublicPath(path string) bool {
	return path == "/health/live" || path == "/health/liveliness" || path == "/ready" || path == "/status" || path == "/metrics"
}

func (s *Server) health(response http.ResponseWriter, _ *http.Request) {
	s.writeJSON(response, http.StatusOK, map[string]any{"object": "health", "status": "ok"})
}

func (s *Server) ready(response http.ResponseWriter, _ *http.Request) {
	ready := false
	for model, providerName := range s.config.Routes {
		if providerConfig, ok := s.config.Providers[providerName]; ok && providerConfig.APIKey != "" && model != "" {
			ready = true
			break
		}
	}
	if !ready {
		s.writeJSON(response, http.StatusServiceUnavailable, map[string]any{"object": "ready", "status": "degraded", "reason": "no configured provider credentials"})
		return
	}
	s.writeJSON(response, http.StatusOK, map[string]any{"object": "ready", "status": "ready"})
}

func (s *Server) status(response http.ResponseWriter, _ *http.Request) {
	s.writeJSON(response, http.StatusOK, map[string]any{
		"object":   "gateway_status",
		"status":   "ok",
		"requests": s.requests.Load(),
		"errors":   s.errors.Load(),
		"models":   len(s.config.Routes),
	})
}

func (s *Server) models(response http.ResponseWriter, _ *http.Request) {
	modelNames := make([]string, 0, len(s.config.Routes))
	for model := range s.config.Routes {
		modelNames = append(modelNames, model)
	}
	sort.Strings(modelNames)
	data := make([]map[string]any, 0, len(modelNames))
	for _, model := range modelNames {
		providerName := s.config.Routes[model]
		data = append(data, map[string]any{"id": model, "object": "model", "owned_by": providerName})
	}
	s.writeJSON(response, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (s *Server) modelStatus(response http.ResponseWriter, request *http.Request) {
	model := request.PathValue("model")
	providerName := s.config.Routes[model]
	if providerName == "" {
		s.writeError(response, http.StatusNotFound, fmt.Sprintf("model %q is not configured", model))
		return
	}
	providerConfig := s.config.Providers[providerName]
	status := "configured"
	if providerConfig.APIKey == "" {
		status = "missing_credentials"
	}
	s.writeJSON(response, http.StatusOK, map[string]any{"object": "model_status", "id": model, "provider": providerName, "status": status})
}

func (s *Server) metrics(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(response, "go_feather_route_requests_total %d\n", s.requests.Load())
	_, _ = fmt.Fprintf(response, "go_feather_route_errors_total %d\n", s.errors.Load())
	_, _ = fmt.Fprintf(response, "go_feather_route_models_total %d\n", len(s.config.Routes))
}

func (s *Server) chat(response http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(response, request.Body, s.config.Server.MaxBodyBytes)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		s.writeError(response, http.StatusRequestEntityTooLarge, "request body is too large or unreadable")
		return
	}
	var envelope struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Model == "" {
		s.writeError(response, http.StatusBadRequest, "request must contain a valid model")
		return
	}
	client, err := s.clientFor(envelope.Model)
	if err != nil {
		s.writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	case <-request.Context().Done():
		s.writeError(response, http.StatusRequestTimeout, "request canceled")
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), s.config.Server.RequestTimeout)
	defer cancel()
	ctx = provider.WithRequestID(ctx, request.Header.Get("X-Request-ID"))
	upstream, err := client.Chat(ctx, body, envelope.Stream)
	if err != nil {
		s.writeError(response, http.StatusBadGateway, err.Error())
		return
	}
	defer func() { _ = upstream.Body.Close() }()
	if envelope.Stream {
		s.streamResponse(response, upstream)
		return
	}
	copyHeaders(response.Header(), upstream.Header)
	response.WriteHeader(upstream.StatusCode)
	_, _ = io.Copy(response, upstream.Body)
}

func (s *Server) streamResponse(response http.ResponseWriter, upstream provider.Response) {
	copyHeaders(response.Header(), upstream.Header)
	if response.Header().Get("Content-Type") == "" {
		response.Header().Set("Content-Type", "text/event-stream")
	}
	response.Header().Set("Cache-Control", "no-cache, no-transform")
	response.Header().Set("Connection", "keep-alive")
	response.Header().Set("X-Accel-Buffering", "no")
	response.WriteHeader(upstream.StatusCode)
	flusher, canFlush := response.(http.Flusher)
	buffer := make([]byte, 32*1024)
	for {
		count, err := upstream.Body.Read(buffer)
		if count > 0 {
			if _, writeErr := response.Write(buffer[:count]); writeErr != nil {
				return
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

func (s *Server) clientFor(model string) (provider.Client, error) {
	providerName := s.config.Routes[model]
	if providerName == "" {
		if name, _, ok := strings.Cut(model, "/"); ok {
			providerName = name
		}
	}
	client, ok := s.providers[providerName]
	if !ok {
		return provider.Client{}, fmt.Errorf("no provider route configured for model %q", model)
	}
	return client, nil
}

func (s *Server) authorized(request *http.Request) bool {
	if s.config.Auth.APIKey == "" {
		return true
	}
	return request.Header.Get("Authorization") == "Bearer "+s.config.Auth.APIKey
}

func (s *Server) writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func (s *Server) writeError(response http.ResponseWriter, status int, message string) {
	s.writeJSON(response, status, map[string]any{"error": map[string]string{"message": message, "type": "go_feather_route_error"}})
}

func copyHeaders(destination, source http.Header) {
	for _, name := range []string{"Content-Type", "Retry-After", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if value := source.Get(name); value != "" {
			destination.Set(name, value)
		}
	}
}

func requestID(candidate string) string {
	if isSafeRequestID(candidate) {
		return candidate
	}
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return fmt.Sprintf("%x", value)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func isSafeRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}
