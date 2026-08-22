// Package router exposes the OpenAI-compatible HTTP gateway.
package router

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sayanmohsin/go-feather-route/internal/config"
	"github.com/sayanmohsin/go-feather-route/internal/contract"
	"github.com/sayanmohsin/go-feather-route/internal/gateway"
	"github.com/sayanmohsin/go-feather-route/internal/provider"
)

// Server routes authenticated client requests to configured providers.
type Server struct {
	config          config.Config
	routes          gateway.Routes
	providers       map[string]provider.ClientAPI
	semaphore       chan struct{}
	streamSemaphore chan struct{}
	logger          *slog.Logger
	requests        atomic.Uint64
	errors          atomic.Uint64
	active          atomic.Int64
	activeStreams   atomic.Int64
	streamsTotal    atomic.Uint64
	streamCompleted atomic.Uint64
	streamAborted   atomic.Uint64
	authFailures    atomic.Uint64
	retries         atomic.Uint64
	bytes           atomic.Uint64
	durationMs      atomic.Uint64
	routeMu         sync.Mutex
	routeStats      map[routeKey]*routeMetric
}

type routeKey struct {
	provider string
	model    string
}

type routeMetric struct {
	requests atomic.Uint64
	errors   atomic.Uint64
	retries  atomic.Uint64
}

// NewServer constructs a router server from validated configuration.
func NewServer(cfg config.Config, logger *slog.Logger) *Server {
	if cfg.Server.MaxConcurrentRequests <= 0 {
		cfg.Server.MaxConcurrentRequests = 16
	}
	if cfg.Server.MaxConcurrentStreams <= 0 {
		cfg.Server.MaxConcurrentStreams = 4
	}
	if cfg.Server.MaxResponseBytes <= 0 {
		cfg.Server.MaxResponseBytes = 8 << 20
	}
	if cfg.Server.StreamIdleTimeout <= 0 {
		cfg.Server.StreamIdleTimeout = 30 * time.Second
	}
	httpClient := provider.NewHTTPClient()
	providers := make(map[string]provider.ClientAPI, len(cfg.Providers))
	for name, item := range cfg.Providers {
		providers[name] = provider.NewClient(name, item.BaseURL, item.APIKey, httpClient)
	}
	return &Server{
		config:          cfg,
		routes:          gateway.NewRoutes(cfg.Routes),
		providers:       providers,
		semaphore:       make(chan struct{}, cfg.Server.MaxConcurrentRequests),
		streamSemaphore: make(chan struct{}, cfg.Server.MaxConcurrentStreams),
		logger:          logger,
		routeStats:      make(map[routeKey]*routeMetric),
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
	mux.HandleFunc("POST /v1/embeddings", s.embeddings)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		tracked := &metricsResponseWriter{ResponseWriter: response}
		s.active.Add(1)
		defer func() {
			duration := time.Since(started)
			s.active.Add(-1)
			s.requests.Add(1)
			if tracked.statusCode >= http.StatusBadRequest {
				s.errors.Add(1)
			}
			if tracked.bytes > 0 {
				s.bytes.Add(uint64(tracked.bytes)) // #nosec G115 -- bytes is checked positive and uint64 has a wider positive range.
			}
			durationMs := duration.Milliseconds()
			if durationMs > 0 {
				s.durationMs.Add(uint64(durationMs)) // #nosec G115 -- duration is non-negative and bounded by the process lifetime.
			}
			s.logger.Info("request", "method", request.Method, "path", request.URL.Path, "request_id", request.Header.Get("X-Request-ID"), "status", tracked.statusCode, "bytes", tracked.bytes, "duration_ms", durationMs)
		}()
		requestID := requestID(request.Header.Get("X-Request-ID"))
		request.Header.Set("X-Request-ID", requestID)
		tracked.Header().Set("Server", "Go-Feather-Route")
		tracked.Header().Set("X-Request-ID", requestID)
		if !isPublicPath(request.URL.Path) && !s.authorized(request) {
			s.authFailures.Add(1)
			s.writeError(tracked, http.StatusUnauthorized, "invalid or missing bearer token")
			return
		}
		mux.ServeHTTP(tracked, request)
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
	for _, model := range s.routes.Models() {
		providerName, _ := s.routes.ProviderForModel(model)
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
		"object":            "gateway_status",
		"status":            "ok",
		"requests":          s.requests.Load(),
		"errors":            s.errors.Load(),
		"models":            len(s.routes.Models()),
		"active":            s.active.Load(),
		"streams":           s.activeStreams.Load(),
		"streams_total":     s.streamsTotal.Load(),
		"streams_completed": s.streamCompleted.Load(),
		"streams_aborted":   s.streamAborted.Load(),
		"auth_failures":     s.authFailures.Load(),
		"retries":           s.retries.Load(),
		"bytes":             s.bytes.Load(),
		"duration_ms":       s.durationMs.Load(),
	})
}

func (s *Server) models(response http.ResponseWriter, _ *http.Request) {
	modelNames := s.routes.Models()
	data := make([]map[string]any, 0, len(modelNames))
	for _, model := range modelNames {
		providerName, _ := s.routes.ProviderForModel(model)
		data = append(data, map[string]any{"id": model, "object": "model", "owned_by": providerName})
	}
	s.writeJSON(response, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (s *Server) modelStatus(response http.ResponseWriter, request *http.Request) {
	model := request.PathValue("model")
	providerName, ok := s.routes.ProviderForModel(model)
	if !ok {
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
	_, _ = fmt.Fprintf(response, "go_feather_route_active_requests %d\n", s.active.Load())
	_, _ = fmt.Fprintf(response, "go_feather_route_active_streams %d\n", s.activeStreams.Load())
	_, _ = fmt.Fprintf(response, "go_feather_route_streams_total %d\n", s.streamsTotal.Load())
	_, _ = fmt.Fprintf(response, "go_feather_route_streams_completed_total %d\n", s.streamCompleted.Load())
	_, _ = fmt.Fprintf(response, "go_feather_route_streams_aborted_total %d\n", s.streamAborted.Load())
	_, _ = fmt.Fprintf(response, "go_feather_route_auth_failures_total %d\n", s.authFailures.Load())
	_, _ = fmt.Fprintf(response, "go_feather_route_retries_total %d\n", s.retries.Load())
	_, _ = fmt.Fprintf(response, "go_feather_route_response_bytes_total %d\n", s.bytes.Load())
	_, _ = fmt.Fprintf(response, "go_feather_route_request_duration_milliseconds_total %d\n", s.durationMs.Load())
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	for key, metric := range s.routeStats {
		labels := fmt.Sprintf(`provider="%s",model="%s"`, escapeMetricLabel(key.provider), escapeMetricLabel(key.model))
		_, _ = fmt.Fprintf(response, "go_feather_route_upstream_requests_total{%s} %d\n", labels, metric.requests.Load())
		_, _ = fmt.Fprintf(response, "go_feather_route_upstream_errors_total{%s} %d\n", labels, metric.errors.Load())
		_, _ = fmt.Fprintf(response, "go_feather_route_upstream_retries_total{%s} %d\n", labels, metric.retries.Load())
	}
}

func escapeMetricLabel(value string) string {
	return strings.NewReplacer(`\\`, `\\\\`, `"`, `\\"`, "\n", `\\n`).Replace(value)
}

func (s *Server) chat(response http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(response, request.Body, s.config.Server.MaxBodyBytes)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		s.writeError(response, http.StatusRequestEntityTooLarge, "request body is too large or unreadable")
		return
	}
	envelope, err := contract.DecodeChatRequest(body)
	if err != nil {
		s.writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	client, err := s.clientFor(envelope.Model)
	if err != nil {
		s.writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	concurrency := s.semaphore
	if envelope.Stream {
		concurrency = s.streamSemaphore
	}
	select {
	case concurrency <- struct{}{}:
		defer func() { <-concurrency }()
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
	s.recordRouteMetric(client.ProviderName(), envelope.Model, upstream.StatusCode, upstream.Attempts)
	if envelope.Stream {
		s.activeStreams.Add(1)
		s.streamsTotal.Add(1)
		defer s.activeStreams.Add(-1)
	}
	s.retries.Add(retryCount(upstream.Attempts))
	if upstream.StatusCode >= http.StatusBadRequest {
		s.copyUpstreamError(response, upstream)
		return
	}
	if envelope.Stream {
		if s.streamResponse(response, upstream) {
			s.streamCompleted.Add(1)
		} else {
			s.streamAborted.Add(1)
		}
		return
	}
	responseBody, ok := readBounded(upstream.Body, s.config.Server.MaxResponseBytes)
	if !ok {
		s.writeError(response, http.StatusBadGateway, "upstream response exceeded configured limit")
		return
	}
	copyHeaders(response.Header(), upstream.Header)
	response.WriteHeader(upstream.StatusCode)
	_, _ = response.Write(responseBody)
}

func (s *Server) embeddings(response http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(response, request.Body, s.config.Server.MaxBodyBytes)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		s.writeError(response, http.StatusRequestEntityTooLarge, "request body is too large or unreadable")
		return
	}
	envelope, err := contract.DecodeEmbeddingRequest(body)
	if err != nil {
		s.writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	client, err := s.clientFor(envelope.Model)
	if err != nil {
		s.writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	concurrency := s.semaphore
	select {
	case concurrency <- struct{}{}:
		defer func() { <-concurrency }()
	case <-request.Context().Done():
		s.writeError(response, http.StatusRequestTimeout, "request canceled")
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), s.config.Server.RequestTimeout)
	defer cancel()
	ctx = provider.WithRequestID(ctx, request.Header.Get("X-Request-ID"))
	upstream, err := client.Embedding(ctx, body)
	if err != nil {
		s.writeError(response, http.StatusBadGateway, err.Error())
		return
	}
	defer func() { _ = upstream.Body.Close() }()
	s.recordRouteMetric(client.ProviderName(), envelope.Model, upstream.StatusCode, upstream.Attempts)
	s.retries.Add(retryCount(upstream.Attempts))
	if upstream.StatusCode >= http.StatusBadRequest {
		s.copyUpstreamError(response, upstream)
		return
	}
	responseBody, ok := readBounded(upstream.Body, s.config.Server.MaxResponseBytes)
	if !ok {
		s.writeError(response, http.StatusBadGateway, "upstream response exceeded configured limit")
		return
	}
	if err := validateEmbeddingResponse(body, responseBody); err != nil {
		s.writeError(response, http.StatusBadGateway, err.Error())
		return
	}
	copyHeaders(response.Header(), upstream.Header)
	response.WriteHeader(upstream.StatusCode)
	_, _ = response.Write(responseBody)
}

func (s *Server) recordRouteMetric(providerName, model string, status, attempts int) {
	s.routeMu.Lock()
	key := routeKey{provider: providerName, model: model}
	metric := s.routeStats[key]
	if metric == nil {
		metric = &routeMetric{}
		s.routeStats[key] = metric
	}
	s.routeMu.Unlock()
	metric.requests.Add(1)
	if status >= http.StatusBadRequest {
		metric.errors.Add(1)
	}
	metric.retries.Add(retryCount(attempts))
}

func (s *Server) copyUpstreamError(response http.ResponseWriter, upstream provider.Response) {
	body, ok := readBounded(upstream.Body, s.config.Server.MaxResponseBytes)
	if !ok {
		s.writeError(response, http.StatusBadGateway, "upstream error response exceeded configured limit")
		return
	}
	copyHeaders(response.Header(), upstream.Header)
	response.WriteHeader(upstream.StatusCode)
	_, _ = response.Write(body)
}

func (s *Server) streamResponse(response http.ResponseWriter, upstream provider.Response) bool {
	copyHeaders(response.Header(), upstream.Header)
	if response.Header().Get("Content-Type") == "" {
		response.Header().Set("Content-Type", "text/event-stream")
	}
	response.Header().Set("Cache-Control", "no-cache, no-transform")
	response.Header().Set("Connection", "keep-alive")
	response.Header().Set("X-Accel-Buffering", "no")
	response.WriteHeader(upstream.StatusCode)
	flusher, canFlush := response.(http.Flusher)
	if canFlush {
		flusher.Flush()
	}
	buffer := make([]byte, 32*1024)
	for {
		count, err := readWithIdleTimeout(upstream.Body, buffer, s.config.Server.StreamIdleTimeout)
		if count > 0 {
			if _, writeErr := response.Write(buffer[:count]); writeErr != nil {
				return false
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if err != nil {
			return errors.Is(err, io.EOF)
		}
	}
}

func readWithIdleTimeout(reader io.ReadCloser, buffer []byte, timeout time.Duration) (int, error) {
	if timeout <= 0 {
		return reader.Read(buffer)
	}
	type readResult struct {
		count int
		err   error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		count, err := reader.Read(buffer)
		resultCh <- readResult{count: count, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-resultCh:
		return result.count, result.err
	case <-timer.C:
		_ = reader.Close()
		return 0, context.DeadlineExceeded
	}
}

func readBounded(reader io.Reader, limit int64) ([]byte, bool) {
	if limit <= 0 {
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, false
	}
	return data, true
}

func validateEmbeddingResponse(requestBody, responseBody []byte) error {
	request, err := contract.DecodeEmbeddingRequest(requestBody)
	if err != nil {
		return err
	}
	expected, err := request.InputCount()
	if err != nil {
		return err
	}
	var response contract.EmbeddingResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("provider returned invalid embedding JSON: %w", err)
	}
	if len(response.Data) != expected {
		return fmt.Errorf("provider returned %d embeddings for %d inputs", len(response.Data), expected)
	}
	dimension := 0
	seen := make(map[int]struct{}, len(response.Data))
	for _, item := range response.Data {
		if item.Index < 0 || item.Index >= expected {
			return fmt.Errorf("provider returned invalid embedding index %d", item.Index)
		}
		if _, ok := seen[item.Index]; ok {
			return fmt.Errorf("provider returned duplicate embedding index %d", item.Index)
		}
		seen[item.Index] = struct{}{}
		if len(item.Embedding) == 0 {
			return fmt.Errorf("provider returned an empty embedding at index %d", item.Index)
		}
		if dimension == 0 {
			dimension = len(item.Embedding)
		} else if len(item.Embedding) != dimension {
			return errors.New("provider returned embeddings with inconsistent dimensions")
		}
		for _, value := range item.Embedding {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("provider returned a non-finite embedding value at index %d", item.Index)
			}
		}
	}
	return nil
}

func (s *Server) clientFor(model string) (provider.ClientAPI, error) {
	providerName, ok := s.routes.ProviderFor(model)
	if !ok {
		return nil, fmt.Errorf("no provider route configured for model %q", model)
	}
	client, ok := s.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("no provider route configured for model %q", model)
	}
	return client, nil
}

func (s *Server) authorized(request *http.Request) bool {
	if s.config.Auth.APIKey == "" {
		return true
	}
	expected := []byte("Bearer " + s.config.Auth.APIKey)
	actual := []byte(request.Header.Get("Authorization"))
	return subtle.ConstantTimeCompare(actual, expected) == 1
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

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bytes      int64
}

func (w *metricsResponseWriter) WriteHeader(statusCode int) {
	if w.statusCode != 0 {
		return
	}
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *metricsResponseWriter) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}
	count, err := w.ResponseWriter.Write(data)
	w.bytes += int64(count)
	return count, err
}

func (w *metricsResponseWriter) Flush() {
	if w.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func requestID(candidate string) string {
	if isSafeRequestID(candidate) {
		return candidate
	}
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func retryCount(attempts int) uint64 {
	if attempts <= 1 {
		return 0
	}
	return uint64(attempts - 1) // #nosec G115 -- attempts is checked above and bounded by the provider retry policy.
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
