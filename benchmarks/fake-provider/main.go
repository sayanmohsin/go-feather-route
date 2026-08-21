// Command fake-provider serves a deterministic OpenAI-compatible benchmark upstream.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type chatRequest struct {
	Stream bool `json:"stream"`
}

type embeddingRequest struct {
	Input json.RawMessage `json:"input"`
}

func main() {
	port := os.Getenv("FAKE_PROVIDER_PORT")
	if port == "" {
		port = "8080"
	}
	delay := durationFromEnv("FAKE_PROVIDER_CHUNK_DELAY", time.Millisecond)
	server := http.Server{Addr: ":" + port, Handler: handler(delay), ReadHeaderTimeout: 5 * time.Second}
	slog.Info("fake provider listening", "addr", server.Addr, "chunk_delay", delay)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("fake provider stopped", "error", err)
		os.Exit(1)
	}
}

func handler(delay time.Duration) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("POST /v1/chat/completions", func(response http.ResponseWriter, request *http.Request) {
		defer func() { _ = request.Body.Close() }()
		var input chatRequest
		if err := json.NewDecoder(io.LimitReader(request.Body, 1<<20)).Decode(&input); err != nil {
			http.Error(response, "invalid request", http.StatusBadRequest)
			return
		}
		if input.Stream {
			streamResponse(response, delay)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"id":"fake-benchmark","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"benchmark response"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":2,"total_tokens":10}}`))
	})
	mux.HandleFunc("POST /v1/embeddings", func(response http.ResponseWriter, request *http.Request) {
		defer func() { _ = request.Body.Close() }()
		var input embeddingRequest
		if err := json.NewDecoder(io.LimitReader(request.Body, 1<<20)).Decode(&input); err != nil {
			http.Error(response, "invalid request", http.StatusBadRequest)
			return
		}
		count, err := embeddingInputCount(input.Input)
		if err != nil || count == 0 {
			http.Error(response, "invalid embedding input", http.StatusBadRequest)
			return
		}
		data := make([]map[string]any, count)
		for index := range count {
			data[index] = map[string]any{
				"object":    "embedding",
				"index":     index,
				"embedding": []float64{float64(index + 1), float64(count)},
			}
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"object": "list",
			"data":   data,
			"model":  "benchmark-model",
			"usage":  map[string]int{"prompt_tokens": count * 4, "total_tokens": count * 4},
		})
	})
	return mux
}

func embeddingInputCount(input json.RawMessage) (int, error) {
	var batch []string
	if err := json.Unmarshal(input, &batch); err == nil {
		return len(batch), nil
	}
	var single string
	if err := json.Unmarshal(input, &single); err != nil {
		return 0, err
	}
	return 1, nil
}

func streamResponse(response http.ResponseWriter, delay time.Duration) {
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	flusher, _ := response.(http.Flusher)
	for _, chunk := range []string{"benchmark ", "stream"} {
		_, _ = fmt.Fprintf(response, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", chunk)
		if flusher != nil {
			flusher.Flush()
		}
		time.Sleep(delay)
	}
	_, _ = fmt.Fprint(response, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func durationFromEnv(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
