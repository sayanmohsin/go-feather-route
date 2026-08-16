// Package provider contains small HTTP clients for OpenAI-compatible providers.
package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client sends OpenAI-compatible chat requests to one provider.
type Client struct {
	Name       string
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
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
			select {
			case <-time.After(100 * time.Millisecond):
			case <-ctx.Done():
				return Response{}, ctx.Err()
			}
			continue
		}
		return response, nil
	}
	return Response{}, lastErr
}

func (c Client) doChat(ctx context.Context, endpoint string, body []byte) (Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create provider request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.APIKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if requestID, ok := ctx.Value(requestIDContextKey{}).(string); ok && requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
	}
	response, err := c.HTTPClient.Do(request) //nolint:bodyclose // ownership transfers through Response.
	if err != nil {
		return Response{}, fmt.Errorf("call provider %s: %w", c.Name, err)
	}
	return Response{StatusCode: response.StatusCode, Header: response.Header, Body: response.Body}, nil //nolint:bodyclose // ownership transfers to the caller.
}
