package router

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sayanmohsin/go-feather-route/internal/config"
)

func TestEmbeddingAdmissionIsBoundedAndCancellationReleasesWaiter(t *testing.T) {
	firstEmbeddingStarted := make(chan struct{})
	releaseFirstEmbedding := make(chan struct{})
	var embeddingRequests atomic.Int32
	var startedOnce sync.Once
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/embeddings" {
			response.WriteHeader(http.StatusNotFound)
			return
		}
		requestNumber := embeddingRequests.Add(1)
		if requestNumber == 1 {
			startedOnce.Do(func() { close(firstEmbeddingStarted) })
			select {
			case <-releaseFirstEmbedding:
			case <-request.Context().Done():
				return
			}
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"object":"list","data":[{"index":0,"embedding":[1,0]}]}`))
	}))
	defer provider.Close()

	server := httptest.NewServer(NewServer(config.Config{
		Server: config.ServerConfig{
			RequestTimeout:          time.Second,
			MaxBodyBytes:            1024,
			MaxResponseBytes:        1024,
			MaxConcurrentRequests:   4,
			MaxConcurrentEmbeddings: 1,
		},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"embedding-model": "openai"},
	}, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler())
	defer server.Close()

	client := server.Client()
	firstRequest, err := newEmbeddingRequest(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	firstResponse := make(chan *http.Response, 1)
	firstError := make(chan error, 1)
	go func() {
		response, err := client.Do(firstRequest) //nolint:bodyclose // the receiving test goroutine closes the body.
		firstResponse <- response
		firstError <- err
	}()
	select {
	case <-firstEmbeddingStarted:
	case <-time.After(time.Second):
		t.Fatal("first embedding did not reach provider")
	}

	queuedContext, cancelQueued := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelQueued()
	queuedRequest, err := newEmbeddingRequest(queuedContext, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if response, err := client.Do(queuedRequest); err == nil {
		_ = response.Body.Close()
		t.Fatal("queued embedding request unexpectedly acquired the saturated slot")
	}
	if got := embeddingRequests.Load(); got != 1 {
		t.Fatalf("provider received %d embeddings while the slot was saturated", got)
	}

	close(releaseFirstEmbedding)
	select {
	case err := <-firstError:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("first embedding did not finish")
	}
	if response := <-firstResponse; response != nil {
		_ = response.Body.Close()
	}

	thirdRequest, err := newEmbeddingRequest(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	thirdResponse, err := client.Do(thirdRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = thirdResponse.Body.Close() }()
	if thirdResponse.StatusCode != http.StatusOK || embeddingRequests.Load() != 2 {
		t.Fatalf("released embedding slot was not reusable: status=%d requests=%d", thirdResponse.StatusCode, embeddingRequests.Load())
	}
}

func newEmbeddingRequest(ctx context.Context, baseURL string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/embeddings", strings.NewReader(`{"model":"embedding-model","input":"hello"}`))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

func TestStreamAdmissionIsIndependentFromChatAdmission(t *testing.T) {
	streamStarted := make(chan struct{})
	releaseStream := make(chan struct{})
	var streamOnce sync.Once
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return
		}
		if strings.Contains(string(body), `"stream":true`) {
			response.Header().Set("Content-Type", "text/event-stream")
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write([]byte("data: {}\n\n"))
			response.(http.Flusher).Flush()
			streamOnce.Do(func() { close(streamStarted) })
			select {
			case <-releaseStream:
			case <-request.Context().Done():
				return
			}
			_, _ = response.Write([]byte("data: [DONE]\n\n"))
			return
		}
		_, _ = response.Write([]byte(`{"id":"chat","choices":[]}`))
	}))
	defer provider.Close()

	server := httptest.NewServer(NewServer(config.Config{
		Server: config.ServerConfig{
			RequestTimeout:        time.Second,
			StreamIdleTimeout:     time.Second,
			MaxBodyBytes:          1024,
			MaxResponseBytes:      1024,
			MaxConcurrentRequests: 1,
			MaxConcurrentStreams:  1,
		},
		Providers: map[string]config.ProviderConfig{"openai": {BaseURL: provider.URL + "/v1", APIKey: "provider-secret"}},
		Routes:    map[string]string{"test-model": "openai"},
	}, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler())
	defer server.Close()
	client := server.Client()

	streamRequest, err := http.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(`{"model":"test-model","stream":true,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	streamResponse := make(chan *http.Response, 1)
	streamError := make(chan error, 1)
	go func() {
		response, requestErr := client.Do(streamRequest) //nolint:bodyclose // the receiving test goroutine drains and closes the body.
		streamResponse <- response
		streamError <- requestErr
	}()
	select {
	case <-streamStarted:
	case <-time.After(time.Second):
		t.Fatal("stream did not reach provider")
	}
	firstStreamResponse := <-streamResponse
	if err := <-streamError; err != nil {
		t.Fatal(err)
	}
	defer func() { _ = firstStreamResponse.Body.Close() }()

	chatRequest, err := http.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(`{"model":"test-model","messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	chatResponse, err := client.Do(chatRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = chatResponse.Body.Close()
	if chatResponse.StatusCode != http.StatusOK {
		t.Fatalf("chat status while stream was active = %d", chatResponse.StatusCode)
	}

	queuedContext, cancelQueued := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelQueued()
	queuedStream, err := http.NewRequestWithContext(queuedContext, http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(`{"model":"test-model","stream":true,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if response, err := client.Do(queuedStream); err == nil {
		_ = response.Body.Close()
		t.Fatal("second stream unexpectedly acquired the saturated stream slot")
	}

	close(releaseStream)
	firstBody, err := io.ReadAll(firstStreamResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(firstBody), "data: [DONE]") {
		t.Fatalf("released stream body = %q", firstBody)
	}

	thirdStream, err := http.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(`{"model":"test-model","stream":true,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	thirdResponse, err := client.Do(thirdStream)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = thirdResponse.Body.Close() }()
	thirdBody, err := io.ReadAll(thirdResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if thirdResponse.StatusCode != http.StatusOK || !strings.Contains(string(thirdBody), "data: [DONE]") {
		t.Fatalf("reusable stream slot status=%d body=%q", thirdResponse.StatusCode, thirdBody)
	}
}
