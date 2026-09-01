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
	if response.Header().Get("Server") != "Go-Feather-Route" || response.Header().Get("X-Go-Feather-Route") != "go-feather-route" || response.Header().Get("X-Request-ID") != "request-123" {
		t.Fatalf("gateway headers = server=%q marker=%q request-id=%q", response.Header().Get("Server"), response.Header().Get("X-Go-Feather-Route"), response.Header().Get("X-Request-ID"))
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

func TestChatPreservesProviderErrorBodyAndStatus(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("Retry-After", "3")
		response.WriteHeader(http.StatusTooManyRequests)
		_, _ = response.Write([]byte(`{"error":{"message":"provider quota","type":"rate_limit_error"}}`))
	}))
	defer provider.Close()

	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[]}`))
	response := httptest.NewRecorder()
	NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Retry-After") != "3" || !strings.Contains(response.Body.String(), "provider quota") {
		t.Fatalf("provider error was not preserved: headers=%v body=%s", response.Header(), response.Body.String())
	}
}

func TestEmbeddingsPreserveProviderErrorBodyAndStatus(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusBadRequest)
		_, _ = response.Write([]byte(`{"error":{"message":"invalid input","type":"invalid_request_error"}}`))
	}))
	defer provider.Close()

	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"embedding-test": "openai"},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{"model":"embedding-test","input":["hello"]}`))
	response := httptest.NewRecorder()
	NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid input") {
		t.Fatalf("provider error was not preserved: status=%d body=%s", response.Code, response.Body.String())
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

func TestEmbeddingResponseValidationRejectsInvalidVectors(t *testing.T) {
	request := []byte(`{"model":"embedding-test","input":["first","second"]}`)
	tests := []struct {
		name     string
		response string
		want     string
	}{
		{name: "missing vector", response: `{"data":[{"index":0,"embedding":[1,0]}]}`, want: "returned 1 embeddings"},
		{name: "duplicate index", response: `{"data":[{"index":0,"embedding":[1,0]},{"index":0,"embedding":[0,1]}]}`, want: "duplicate"},
		{name: "inconsistent dimensions", response: `{"data":[{"index":0,"embedding":[1,0]},{"index":1,"embedding":[0]}]}`, want: "inconsistent"},
		{name: "non-finite value", response: `{"data":[{"index":0,"embedding":[1e999]},{"index":1,"embedding":[0,1]}]}`, want: "invalid embedding data"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateEmbeddingResponse(request, []byte(test.response))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestEmbeddingResponseValidationAcceptsSingleInput(t *testing.T) {
	err := validateEmbeddingResponse(
		[]byte(`{"model":"embedding-test","input":"hello"}`),
		[]byte(`{"object":"list","data":[{"index":0,"embedding":[1,0]}]}`),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestOversizedProviderResponseReturnsGatewayError(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"id":"chat","choices":[],"padding":"this response is too large"}`))
	}))
	defer provider.Close()
	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxResponseBytes: 16, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL, APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[]}`))
	response := httptest.NewRecorder()
	NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), "exceeded configured limit") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestChatStreamsProviderResponse(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		response.WriteHeader(http.StatusOK)
		flusher := response.(http.Flusher)
		_, _ = response.Write([]byte(": keep-alive\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
		flusher.Flush()
		_, _ = response.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":1,\"total_tokens\":3}}\n\ndata: [DONE]\n\n"))
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
	if !strings.Contains(response.Body.String(), ": keep-alive") || !strings.Contains(response.Body.String(), `"total_tokens":3`) || !strings.Contains(response.Body.String(), "[DONE]") {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestChatMarksStreamWithoutDoneAsAborted(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("data: {\"choices\":[]}" + "\n\n"))
	}))
	defer provider.Close()

	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, StreamIdleTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	server := NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","stream":true,"messages":[]}`))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "choices") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	metrics := httptest.NewRecorder()
	server.Handler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/status", nil))
	if !strings.Contains(metrics.Body.String(), `"streams_completed":0`) || !strings.Contains(metrics.Body.String(), `"streams_aborted":1`) {
		t.Fatalf("status metrics = %s", metrics.Body.String())
	}
}

func TestRequestBodyLimitReturnsOpenAIError(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 8, MaxConcurrentRequests: 1},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model"}`))
	response := httptest.NewRecorder()
	NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), "go_feather_route_error") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestEmbeddingsPreserveBatchOrdering(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"object":"list","data":[{"index":1,"embedding":[0,1]},{"index":0,"embedding":[1,0]}],"model":"embedding-test"}`))
	}))
	defer provider.Close()
	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"embedding-test": "openai"},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{"model":"embedding-test","input":["first","second"]}`))
	response := httptest.NewRecorder()
	NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Index(body, `"index":0`) < strings.Index(body, `"index":1`) {
		return
	}
	t.Fatalf("provider response was not passed through with its indexes: %s", body)
}

func TestOrderedEmbeddingResponseIsForwardedWithoutReencoding(t *testing.T) {
	responseBody := `{"object":"list","data":[{"index":0,"embedding":[1,0]},{"index":1,"embedding":[0,1]}],"model":"embedding-test","usage":{"prompt_tokens":2,"total_tokens":2}}`
	if normalized, err := normalizeEmbeddingResponse([]byte(`{"model":"embedding-test","input":["first","second"]}`), []byte(responseBody)); err != nil {
		t.Fatal(err)
	} else if string(normalized) != responseBody {
		t.Fatalf("ordered response was re-encoded: %s", normalized)
	}
}

func TestEmbeddingLimitDoesNotBlockChat(t *testing.T) {
	chatStarted := make(chan struct{})
	embeddingStarted := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/embeddings" {
			close(embeddingStarted)
			<-chatStarted
			_, _ = response.Write([]byte(`{"object":"list","data":[{"index":0,"embedding":[1]}]}`))
			return
		}
		close(chatStarted)
		_, _ = response.Write([]byte(`{"id":"chat","choices":[]}`))
	}))
	defer provider.Close()
	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxResponseBytes: 1024, MaxConcurrentRequests: 1, MaxConcurrentEmbeddings: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"chat-model": "openai", "embedding-model": "openai"},
	}
	server := httptest.NewServer(NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler())
	defer server.Close()
	client := server.Client()
	embeddingDone := make(chan error, 1)
	go func() {
		request, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/embeddings", strings.NewReader(`{"model":"embedding-model","input":"hello"}`))
		request.Header.Set("Authorization", "Bearer provider-secret")
		response, err := client.Do(request)
		if response != nil {
			_ = response.Body.Close()
		}
		embeddingDone <- err
	}()
	select {
	case <-embeddingStarted:
	case <-time.After(time.Second):
		t.Fatal("embedding request did not reach provider")
	}
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(`{"model":"chat-model","messages":[]}`))
	request.Header.Set("Authorization", "Bearer provider-secret")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("chat status = %d", response.StatusCode)
	}
	if err := <-embeddingDone; err != nil {
		t.Fatal(err)
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
	if response.Header().Get("X-Go-Feather-Route") != "go-feather-route" {
		t.Fatalf("gateway marker = %q", response.Header().Get("X-Go-Feather-Route"))
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
	if !strings.Contains(metrics.Body.String(), "go_feather_route_auth_failures_total 1") {
		t.Fatalf("auth metrics = %s", metrics.Body.String())
	}
	if !strings.Contains(metrics.Body.String(), "go_feather_route_upstream_duration_milliseconds_total") || !strings.Contains(metrics.Body.String(), "go_feather_route_first_byte_milliseconds_total") {
		t.Fatalf("latency metrics = %s", metrics.Body.String())
	}
}

func TestMetricsExposeProviderAndModelLabels(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"id":"chat","choices":[]}`))
	}))
	defer provider.Close()
	cfg := config.Config{
		Server:    config.ServerConfig{RequestTimeout: time.Second, MaxBodyBytes: 1024, MaxConcurrentRequests: 1},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	server := NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[]}`))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	metrics := httptest.NewRecorder()
	server.Handler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(metrics.Body.String(), `go_feather_route_upstream_requests_total{provider="openai",model="test-model"} 1`) {
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

func TestStreamingIdleTimeoutStopsStalledProvider(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		response.WriteHeader(http.StatusOK)
		response.(http.Flusher).Flush()
		time.Sleep(100 * time.Millisecond)
	}))
	defer provider.Close()

	cfg := config.Config{
		Server: config.ServerConfig{
			RequestTimeout:        time.Second,
			StreamIdleTimeout:     10 * time.Millisecond,
			MaxBodyBytes:          1024,
			MaxConcurrentRequests: 1,
			MaxConcurrentStreams:  1,
		},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test-model","stream":true,"messages":[]}`))
	response := httptest.NewRecorder()
	NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}
