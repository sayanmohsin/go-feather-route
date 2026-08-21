// Command runner sends repeatable requests and emits sanitized benchmark JSON.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

type result struct {
	DurationMS  float64 `json:"duration_ms"`
	FirstByteMS float64 `json:"first_byte_ms,omitempty"`
	Status      int     `json:"status"`
	Bytes       int64   `json:"bytes"`
	Error       string  `json:"error,omitempty"`
}

type report struct {
	Gateway     string   `json:"gateway"`
	URL         string   `json:"url"`
	Model       string   `json:"model"`
	Operation   string   `json:"operation"`
	Requests    int      `json:"requests"`
	Concurrency int      `json:"concurrency"`
	Streaming   bool     `json:"streaming"`
	StartedAt   string   `json:"started_at"`
	Results     []result `json:"results"`
	Summary     summary  `json:"summary"`
}

type summary struct {
	Completed      int     `json:"completed"`
	Errors         int     `json:"errors"`
	P50MS          float64 `json:"p50_ms"`
	P95MS          float64 `json:"p95_ms"`
	P99MS          float64 `json:"p99_ms"`
	RequestsPerSec float64 `json:"requests_per_sec"`
	TotalBytes     int64   `json:"total_bytes"`
	P50FirstByteMS float64 `json:"p50_first_byte_ms,omitempty"`
}

func main() {
	url := flag.String("url", "http://127.0.0.1:4000", "gateway base URL")
	key := flag.String("api-key", "benchmark-key", "gateway key")
	model := flag.String("model", "benchmark-model", "model alias")
	gateway := flag.String("gateway", "unknown", "gateway label")
	requests := flag.Int("requests", 32, "number of requests")
	concurrency := flag.Int("concurrency", 1, "maximum concurrent requests")
	streaming := flag.Bool("stream", false, "use SSE streaming")
	operation := flag.String("operation", "chat", "operation: chat or embeddings")
	output := flag.String("output", "benchmark.json", "output JSON path")
	flag.Parse()
	if *requests < 1 || *concurrency < 1 || (*operation != "chat" && *operation != "embeddings") || (*operation == "embeddings" && *streaming) {
		fatal("requests and concurrency must be positive")
	}

	client := &http.Client{Timeout: 60 * time.Second}
	started := time.Now()
	results := make([]result, *requests)
	jobs := make(chan int)
	var wait sync.WaitGroup
	for worker := 0; worker < *concurrency; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := range jobs {
				results[index] = request(context.Background(), client, *url, *key, *model, *operation, *streaming)
			}
		}()
	}
	for index := range *requests {
		jobs <- index
	}
	close(jobs)
	wait.Wait()

	outputReport := report{
		Gateway: *gateway, URL: *url, Model: *model, Operation: *operation, Requests: *requests, Concurrency: *concurrency,
		Streaming: *streaming, StartedAt: started.UTC().Format(time.RFC3339), Results: results,
		Summary: summarize(results, time.Since(started)),
	}
	data, err := json.MarshalIndent(outputReport, "", "  ")
	if err != nil {
		fatal(err.Error())
	}
	if err := os.WriteFile(*output, append(data, '\n'), 0o600); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("%s p50=%.2fms p95=%.2fms p99=%.2fms rps=%.2f errors=%d\n", *gateway, outputReport.Summary.P50MS, outputReport.Summary.P95MS, outputReport.Summary.P99MS, outputReport.Summary.RequestsPerSec, outputReport.Summary.Errors)
}

func request(ctx context.Context, client *http.Client, baseURL, key, model, operation string, streaming bool) result {
	endpoint := "/v1/chat/completions"
	body := fmt.Sprintf(`{"model":%q,"stream":%t,"max_tokens":64,"response_format":{"type":"json_object"},"messages":[{"role":"user","content":"Return a short benchmark response as JSON."}]}`, model, streaming)
	if operation == "embeddings" {
		endpoint = "/v1/embeddings"
		body = fmt.Sprintf(`{"model":%q,"input":["Return a short benchmark embedding.","Measure deterministic routing."]}`, model)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+endpoint, bytes.NewBufferString(body))
	if err != nil {
		return result{Error: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	response, err := client.Do(req)
	if err != nil {
		return result{DurationMS: elapsedMS(started), Error: err.Error()}
	}
	defer func() { _ = response.Body.Close() }()
	firstByte := time.Time{}
	var bytesRead int64
	buffer := make([]byte, 32*1024)
	for {
		count, readErr := response.Body.Read(buffer)
		if count > 0 && firstByte.IsZero() {
			firstByte = time.Now()
		}
		bytesRead += int64(count)
		if readErr != nil {
			if readErr != io.EOF {
				return result{DurationMS: elapsedMS(started), Status: response.StatusCode, Bytes: bytesRead, Error: readErr.Error()}
			}
			break
		}
	}
	item := result{DurationMS: elapsedMS(started), Status: response.StatusCode, Bytes: bytesRead}
	if !firstByte.IsZero() {
		item.FirstByteMS = firstByte.Sub(started).Seconds() * 1000
	}
	if response.StatusCode >= http.StatusBadRequest {
		item.Error = fmt.Sprintf("upstream status %d", response.StatusCode)
	}
	return item
}

func summarize(results []result, elapsed time.Duration) summary {
	latencies := make([]float64, 0, len(results))
	firstBytes := make([]float64, 0, len(results))
	var totalBytes int64
	item := summary{}
	for _, result := range results {
		if result.Error != "" {
			item.Errors++
			continue
		}
		item.Completed++
		latencies = append(latencies, result.DurationMS)
		totalBytes += result.Bytes
		if result.FirstByteMS > 0 {
			firstBytes = append(firstBytes, result.FirstByteMS)
		}
	}
	sort.Float64s(latencies)
	sort.Float64s(firstBytes)
	item.P50MS, item.P95MS, item.P99MS = percentiles(latencies)
	if len(firstBytes) > 0 {
		item.P50FirstByteMS = percentile(firstBytes, 0.50)
	}
	item.TotalBytes = totalBytes
	item.RequestsPerSec = float64(item.Completed) / elapsed.Seconds()
	return item
}

func percentiles(values []float64) (float64, float64, float64) {
	return percentile(values, 0.50), percentile(values, 0.95), percentile(values, 0.99)
}

func percentile(values []float64, fraction float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * fraction)
	return values[index]
}

func elapsedMS(start time.Time) float64 { return time.Since(start).Seconds() * 1000 }

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
