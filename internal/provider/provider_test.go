package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientRetriesRetryableNonStreamingResponse(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			response.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = response.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := Client{Name: "test", BaseURL: server.URL, APIKey: "secret", HTTPClient: server.Client()}
	result, err := client.Chat(context.Background(), []byte(`{"model":"test"}`), false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = result.Body.Close() }()
	if attempts != 2 || result.StatusCode != http.StatusOK {
		t.Fatalf("attempts=%d status=%d", attempts, result.StatusCode)
	}
}

func TestClientDoesNotRetryStreaming(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		attempts++
		response.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := Client{Name: "test", BaseURL: server.URL, APIKey: "secret", HTTPClient: server.Client()}
	result, err := client.Chat(context.Background(), []byte(`{"model":"test","stream":true}`), true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = result.Body.Close() }()
	if attempts != 1 || result.StatusCode != http.StatusBadGateway {
		t.Fatalf("attempts=%d status=%d", attempts, result.StatusCode)
	}
}

func TestClientRequiresAPIKey(t *testing.T) {
	client := Client{Name: "test", BaseURL: "https://example.com", HTTPClient: http.DefaultClient}
	_, err := client.Chat(context.Background(), nil, false)
	if err == nil || !strings.Contains(err.Error(), "API key is empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientHonorsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_ = request
		time.Sleep(100 * time.Millisecond)
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := Client{Name: "test", BaseURL: server.URL, APIKey: "secret", HTTPClient: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result, err := client.Chat(ctx, []byte(`{"model":"test"}`), true)
	if result.Body != nil {
		_, _ = io.Copy(io.Discard, result.Body)
		_ = result.Body.Close()
	}
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}
