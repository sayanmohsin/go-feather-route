// Package provider contains small HTTP clients for OpenAI-compatible providers.
package provider

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"time"
)

// Client sends OpenAI-compatible requests to one provider.
type Client struct {
	Name         string
	BaseURL      string
	APIKey       string
	Kind         string
	ModelAliases map[string]string
	HTTPClient   *http.Client
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
	return Client{Name: name, BaseURL: baseURL, APIKey: apiKey, Kind: "openai-compatible", HTTPClient: httpClient}
}

// ProviderName identifies the configured provider for observability labels.
func (c Client) ProviderName() string {
	return c.Name
}

// Response is an upstream response whose body ownership belongs to the caller.
type Response struct {
	StatusCode            int
	Header                http.Header
	Body                  io.ReadCloser
	Attempts              int
	ConnectionDuration    time.Duration
	FirstResponseDuration time.Duration
	RetryReason           string
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
	retryReason := ""
	for attempt := 0; attempt < 2; attempt++ {
		preparedBody, err := c.prepareChatBody(body)
		if err != nil {
			return Response{}, err
		}
		response, err := c.doChat(ctx, endpoint, preparedBody)
		if err != nil {
			// A transport error is ambiguous: the provider may have accepted the
			// request before the connection failed. Do not duplicate a POST.
			return Response{}, err
		}
		response.Attempts = attempt + 1
		response.RetryReason = retryReason
		if response.StatusCode < 500 && response.StatusCode != http.StatusTooManyRequests {
			return response, nil
		}
		if attempt == 0 && !stream {
			retryReason = retryReasonForStatus(response.StatusCode)
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
	retryReason := ""
	for attempt := 0; attempt < 2; attempt++ {
		preparedBody, err := c.prepareModelBody(body)
		if err != nil {
			return Response{}, err
		}
		response, err := c.doJSON(ctx, endpoint, preparedBody, "application/json")
		if err != nil {
			// Embedding POSTs are also unsafe to replay after an ambiguous
			// transport failure.
			return Response{}, err
		}
		response.Attempts = attempt + 1
		response.RetryReason = retryReason
		if response.StatusCode < 500 && response.StatusCode != http.StatusTooManyRequests {
			return response, nil
		}
		if attempt == 0 {
			retryReason = retryReasonForStatus(response.StatusCode)
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

func (c Client) prepareChatBody(body []byte) ([]byte, error) {
	prepared, err := c.prepareModelBody(body)
	if err != nil {
		return nil, err
	}
	if c.Kind != "ollama" {
		return c.stripProviderPrefix(prepared), nil
	}
	var request map[string]json.RawMessage
	if err := json.Unmarshal(prepared, &request); err != nil {
		return nil, fmt.Errorf("prepare Ollama chat request: %w", err)
	}
	var reasoningEffort string
	if value, ok := request["reasoning_effort"]; ok {
		_ = json.Unmarshal(value, &reasoningEffort)
	}
	if reasoningEffort == "none" {
		request["think"] = json.RawMessage("false")
		delete(request, "reasoning_effort")
	}
	prepared, err = json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return c.stripProviderPrefix(prepared), nil
}

func (c Client) stripProviderPrefix(body []byte) []byte {
	var request map[string]json.RawMessage
	if err := json.Unmarshal(body, &request); err != nil {
		return body
	}
	value, ok := request["model"]
	if !ok {
		return body
	}
	var model string
	if err := json.Unmarshal(value, &model); err != nil {
		return body
	}
	prefix, upstream, ok := strings.Cut(model, "/")
	if !ok || prefix != c.Name || upstream == "" {
		return body
	}
	request["model"], _ = json.Marshal(upstream)
	prepared, err := json.Marshal(request)
	if err != nil {
		return body
	}
	return prepared
}

func (c Client) prepareModelBody(body []byte) ([]byte, error) {
	if len(c.ModelAliases) == 0 {
		return c.stripProviderPrefix(body), nil
	}
	var request map[string]json.RawMessage
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("prepare provider request: %w", err)
	}
	var model string
	if value, ok := request["model"]; ok {
		if err := json.Unmarshal(value, &model); err != nil {
			return nil, fmt.Errorf("prepare provider model: %w", err)
		}
	}
	if providerModel, ok := c.ModelAliases[model]; ok {
		encoded, err := json.Marshal(providerModel)
		if err != nil {
			return nil, fmt.Errorf("prepare provider model: %w", err)
		}
		request["model"] = encoded
		body, err = json.Marshal(request)
		if err != nil {
			return nil, fmt.Errorf("prepare provider model: %w", err)
		}
		return c.stripProviderPrefix(body), nil
	}
	return c.stripProviderPrefix(body), nil
}

func retryReasonForStatus(status int) string {
	if status == http.StatusTooManyRequests {
		return "rate_limited"
	}
	if status >= http.StatusInternalServerError {
		return "upstream_5xx"
	}
	return "other"
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
	started := time.Now()
	var connectStarted time.Time
	var connectionDuration time.Duration
	var firstResponseDuration time.Duration
	trace := &httptrace.ClientTrace{
		ConnectStart: func(_, _ string) { connectStarted = time.Now() },
		ConnectDone: func(_, _ string, _ error) {
			if !connectStarted.IsZero() {
				connectionDuration = time.Since(connectStarted)
			}
		},
		GotFirstResponseByte: func() { firstResponseDuration = time.Since(started) },
	}
	ctx = httptrace.WithClientTrace(ctx, trace)
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
	return Response{StatusCode: response.StatusCode, Header: response.Header, Body: response.Body, ConnectionDuration: connectionDuration, FirstResponseDuration: firstResponseDuration}, nil //nolint:bodyclose // ownership transfers to the caller.
}
