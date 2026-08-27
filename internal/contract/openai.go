// Package contract contains the provider-neutral OpenAI-compatible wire types.
package contract

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ChatRequest contains the fields required by the gateway while retaining
// unknown provider-specific fields in the original request body for passthrough.
type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       json.RawMessage `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// ResponseFormat describes structured-output requests supported by providers.
type ResponseFormat struct {
	Type string `json:"type"`
}

// EmbeddingRequest contains the model and one or more input values.
type EmbeddingRequest struct {
	Model string          `json:"model"`
	Input json.RawMessage `json:"input"`
}

// EmbeddingData is one indexed embedding vector.
type EmbeddingData struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// EmbeddingResponse is the validated provider response envelope.
type EmbeddingResponse struct {
	Data []EmbeddingData `json:"data"`
}

// DecodeChatRequest validates the gateway-required chat fields.
func DecodeChatRequest(data []byte) (ChatRequest, error) {
	var request ChatRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return ChatRequest{}, fmt.Errorf("invalid chat request JSON: %w", err)
	}
	if request.Model == "" {
		return ChatRequest{}, errors.New("request must contain a valid model")
	}
	return request, nil
}

// DecodeEmbeddingRequest validates the gateway-required embedding fields.
func DecodeEmbeddingRequest(data []byte) (EmbeddingRequest, error) {
	var request EmbeddingRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return EmbeddingRequest{}, fmt.Errorf("invalid embedding request JSON: %w", err)
	}
	if request.Model == "" {
		return EmbeddingRequest{}, errors.New("request must contain a valid model")
	}
	if len(request.Input) == 0 || string(request.Input) == "null" {
		return EmbeddingRequest{}, errors.New("embedding request is missing input")
	}
	return request, nil
}

// InputCount returns the number of vectors required for the request.
func (request EmbeddingRequest) InputCount() (int, error) {
	var batch []json.RawMessage
	if len(request.Input) > 0 && request.Input[0] == '[' {
		if err := json.Unmarshal(request.Input, &batch); err != nil {
			return 0, fmt.Errorf("embedding input is invalid: %w", err)
		}
		if len(batch) == 0 {
			return 0, errors.New("embedding input batch must not be empty")
		}
		return len(batch), nil
	}
	return 1, nil
}

// ValidateChatResponse checks that a successful non-streaming response is a
// JSON object. Provider-specific fields remain untouched by the gateway.
func ValidateChatResponse(data []byte) error {
	var response map[string]json.RawMessage
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("provider returned invalid chat JSON: %w", err)
	}
	if response == nil {
		return errors.New("provider returned an empty chat response")
	}
	return nil
}
