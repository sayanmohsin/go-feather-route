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

type Client struct {
	Name       string
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type Response struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}

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
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return Response{}, fmt.Errorf("call provider %s: %w", c.Name, err)
	}
	return Response{StatusCode: response.StatusCode, Header: response.Header, Body: response.Body}, nil
}
