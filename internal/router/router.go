package router

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sayanmohsin/go-feather-route/internal/config"
	"github.com/sayanmohsin/go-feather-route/internal/provider"
)

type Server struct {
	config    config.Config
	providers map[string]provider.Client
	semaphore chan struct{}
	logger    *slog.Logger
}

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

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/liveliness", s.health)
	mux.HandleFunc("GET /v1/models", s.models)
	mux.HandleFunc("POST /v1/chat/completions", s.chat)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		if request.URL.Path != "/health/liveliness" && !s.authorized(request) {
			s.writeError(response, http.StatusUnauthorized, "invalid or missing bearer token")
			return
		}
		mux.ServeHTTP(response, request)
		s.logger.Info("request", "method", request.Method, "path", request.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}

func (s *Server) health(response http.ResponseWriter, _ *http.Request) {
	s.writeJSON(response, http.StatusOK, map[string]any{"object": "health", "status": "ok"})
}

func (s *Server) models(response http.ResponseWriter, _ *http.Request) {
	data := make([]map[string]any, 0, len(s.config.Routes))
	for model, providerName := range s.config.Routes {
		data = append(data, map[string]any{"id": model, "object": "model", "owned_by": providerName})
	}
	s.writeJSON(response, http.StatusOK, map[string]any{"object": "list", "data": data})
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
	upstream, err := client.Chat(ctx, body, envelope.Stream)
	if err != nil {
		s.writeError(response, http.StatusBadGateway, err.Error())
		return
	}
	defer upstream.Body.Close()
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
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
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
	for _, name := range []string{"Content-Type", "X-Request-ID"} {
		if value := source.Get(name); value != "" {
			destination.Set(name, value)
		}
	}
}
