// Package provider contains small HTTP clients for OpenAI-compatible providers.
package provider

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Client sends OpenAI-compatible requests to one provider.
type Client struct {
	Name       string
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// ClientAPI is the provider capability required by the gateway.
type ClientAPI interface {
	ProviderName() string
	Chat(context.Context, []byte, bool) (Response, error)
	Embedding(context.Context, []byte) (Response, error)
}

var _ ClientAPI = Client{}

// NewHTTPClient creates the shared connection-reusing client for providers.
func NewHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 32
	transport.IdleConnTimeout = 90 * time.Second
	transport.TLSHandshakeTimeout = 5 * time.Second
	transport.ExpectContinueTimeout = time.Second
	return &http.Client{Transport: transport}
}

// NewClient constructs an OpenAI-compatible provider adapter.
func NewClient(name, baseURL, apiKey string, httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = NewHTTPClient()
	}
	return Client{Name: name, BaseURL: baseURL, APIKey: apiKey, HTTPClient: httpClient}
}

// ProviderName identifies the configured provider for observability labels.
func (c Client) ProviderName() string {
	return c.Name
}

// Response is an upstream response whose body ownership belongs to the caller.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
	Attempts   int
}

type requestIDContextKey struct{}

// WithRequestID attaches a gateway request ID to an upstream request context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// Chat sends a chat completion request and transfers response-body ownership to the caller.
func (c Client) Chat(ctx context.Context, body []byte, stream bool) (Response, error) {
	if c.APIKey == "" {
		return Response{}, fmt.Errorf("provider %s is not configured: API key is empty", c.Name)
	}
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		response, err := c.doChat(ctx, endpoint, body)
		if err != nil {
			lastErr = err
			if !stream && attempt == 0 && ctx.Err() == nil {
				if err := waitForRetry(ctx, http.Header{}, attempt); err != nil {
					return Response{}, err
				}
				continue
			}
			return Response{}, err
		}
		response.Attempts = attempt + 1
		if response.StatusCode < 500 && response.StatusCode != http.StatusTooManyRequests {
			return response, nil
		}
		if attempt == 0 && !stream {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if err := waitForRetry(ctx, response.Header, attempt); err != nil {
				return Response{}, err
			}
			continue
		}
		return response, nil
	}
	return Response{}, lastErr
}

// Embedding sends an OpenAI-compatible embeddings request and transfers
// response-body ownership to the caller.
func (c Client) Embedding(ctx context.Context, body []byte) (Response, error) {
	if c.APIKey == "" {
		return Response{}, fmt.Errorf("provider %s is not configured: API key is empty", c.Name)
	}
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/embeddings"
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		response, err := c.doJSON(ctx, endpoint, body, "application/json")
		if err != nil {
			lastErr = err
			if attempt == 0 && ctx.Err() == nil {
				if err := waitForRetry(ctx, http.Header{}, attempt); err != nil {
					return Response{}, err
				}
				continue
			}
			return Response{}, err
		}
		response.Attempts = attempt + 1
		if response.StatusCode < 500 && response.StatusCode != http.StatusTooManyRequests {
			return response, nil
		}
		if attempt == 0 {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if err := waitForRetry(ctx, response.Header, attempt); err != nil {
				return Response{}, err
			}
			continue
		}
		return response, nil
	}
	return Response{}, lastErr
}

func waitForRetry(ctx context.Context, headers http.Header, attempt int) error {
	delay := 100 * time.Millisecond
	if retryAfter := strings.TrimSpace(headers.Get("Retry-After")); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
			delay = time.Duration(seconds) * time.Second
		} else if when, err := http.ParseTime(retryAfter); err == nil {
			delay = time.Until(when)
		}
	} else if attempt > 0 {
		delay *= time.Duration(1 << min(attempt, 4))
	}
	if delay < 0 {
		delay = 0
	}
	const maxRetryDelay = 500 * time.Millisecond
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	if jitter, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(25*time.Millisecond))); err == nil {
		delay += time.Duration(jitter.Int64())
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c Client) doChat(ctx context.Context, endpoint string, body []byte) (Response, error) {
	return c.doJSON(ctx, endpoint, body, "application/json, text/event-stream")
}

func (c Client) doJSON(ctx context.Context, endpoint string, body []byte, accept string) (Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create provider request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.APIKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", accept)
	if requestID, ok := ctx.Value(requestIDContextKey{}).(string); ok && requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
	}
	response, err := c.HTTPClient.Do(request) //nolint:bodyclose // ownership transfers through Response.
	if err != nil {
		return Response{}, fmt.Errorf("call provider %s: %w", c.Name, err)
	}
	return Response{StatusCode: response.StatusCode, Header: response.Header, Body: response.Body}, nil //nolint:bodyclose // ownership transfers to the caller.
}
