// Package usage reports sanitized gateway usage to an optional consumer.
package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Tokens contains normalized provider token counts.
type Tokens struct {
	Input  int
	Output int
	Total  int
}

// ParseJSON extracts OpenAI-compatible usage from a JSON response.
func ParseJSON(data []byte) Tokens {
	var envelope struct {
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
			Total      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return Tokens{}
	}
	tokens := Tokens{Input: envelope.Usage.Prompt, Output: envelope.Usage.Completion, Total: envelope.Usage.Total}
	if tokens.Total == 0 {
		tokens.Total = tokens.Input + tokens.Output
	}
	return tokens
}

// ParseSSE extracts the last usage-bearing event from an SSE response.
func ParseSSE(data []byte) Tokens {
	var result Tokens
	for _, event := range strings.Split(string(data), "\n\n") {
		for _, line := range strings.Split(event, "\n") {
			if strings.HasPrefix(line, "data:") {
				parsed := ParseJSON([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))))
				if parsed.Total > 0 {
					result = parsed
				}
			}
		}
	}
	return result
}

// Event is the prompt-free usage record sent to an internal consumer.
type Event struct {
	RequestID     string  `json:"request_id"`
	UserID        string  `json:"user_id,omitempty"`
	ProjectID     string  `json:"project_id,omitempty"`
	InstanceID    string  `json:"instance_id,omitempty"`
	Operation     string  `json:"operation,omitempty"`
	Gateway       string  `json:"gateway"`
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	TotalTokens   int     `json:"total_tokens"`
	EstimatedCost float64 `json:"estimated_cost_usd,omitempty"`
	Status        string  `json:"status"`
	DurationMs    int64   `json:"duration_ms"`
	FirstTokenMs  int64   `json:"first_token_ms,omitempty"`
	FallbackUsed  bool    `json:"fallback_used,omitempty"`
	AttemptCount  int     `json:"attempt_count"`
}

// Reporter delivers normalized usage events to an optional internal endpoint.
type Reporter struct {
	Endpoint string
	APIKey   string
	Timeout  time.Duration
	Client   *http.Client
}

// Report sends an event within the configured bounded callback deadline.
func (r Reporter) Report(event Event) {
	if r.Endpoint == "" || r.APIKey == "" || event.RequestID == "" {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	client := r.Client
	if client == nil {
		client = &http.Client{}
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, bytes.NewReader(data))
	if err != nil {
		return
	}
	request.Header.Set("Authorization", "Bearer "+r.APIKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err == nil {
		_ = response.Body.Close()
	}
}
