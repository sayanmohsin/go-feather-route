package compatibility_test

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sayanmohsin/go-feather-route/internal/config"
	"github.com/sayanmohsin/go-feather-route/internal/router"
)

type fixture struct {
	Name      string          `json:"name"`
	Operation string          `json:"operation"`
	Model     string          `json:"model"`
	Body      json.RawMessage `json:"body"`
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name)) // #nosec G304 -- fixture names are fixed by repository tests.
	if err != nil {
		t.Fatal(err)
	}
	var value fixture
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func newGateway(t *testing.T, providerHandler http.Handler) *httptest.Server {
	t.Helper()
	provider := httptest.NewServer(providerHandler)
	t.Cleanup(provider.Close)
	cfg := config.Config{
		Server: config.ServerConfig{
			RequestTimeout:        time.Second,
			StreamIdleTimeout:     time.Second,
			MaxBodyBytes:          1 << 20,
			MaxResponseBytes:      1 << 20,
			MaxConcurrentRequests: 4,
			MaxConcurrentStreams:  2,
		},
		Auth:      config.AuthConfig{APIKey: "gateway-key"},
		Providers: map[string]config.ProviderConfig{"fake": {BaseURL: provider.URL + "/v1", APIKey: "provider-key"}},
		Routes:    map[string]string{"benchmark-model": "fake", "embedding-model": "fake"},
	}
	return httptest.NewServer(router.NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler())
}

func TestFixturesReplayChatAndEmbeddings(t *testing.T) {
	chat := loadFixture(t, "chat.json")
	embeddings := loadFixture(t, "embeddings.json")
	provider := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer provider-key" {
			t.Fatalf("provider authorization = %q", request.Header.Get("Authorization"))
		}
		if request.URL.Path == "/v1/chat/completions" {
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"id":"chat_fixture","choices":[{"index":0,"message":{"role":"assistant","content":"{\"answer\":\"ok\"}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":3,"total_tokens":11}}`))
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"object":"list","data":[{"index":1,"embedding":[0,1]},{"index":0,"embedding":[1,0]}],"model":"embedding-model","usage":{"prompt_tokens":4,"total_tokens":4}}`))
	})
	gateway := newGateway(t, provider)
	defer gateway.Close()

	for _, value := range []fixture{chat, embeddings} {
		t.Run(value.Name, func(t *testing.T) {
			endpoint := "/v1/" + value.Operation
			if value.Operation == "chat" {
				endpoint = "/v1/chat/completions"
			}
			request, err := http.NewRequest(http.MethodPost, gateway.URL+endpoint, strings.NewReader(string(value.Body)))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Authorization", "Bearer gateway-key")
			request.Header.Set("X-Request-ID", "fixture-"+value.Name)
			response, err := gateway.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "usage") {
				t.Fatalf("status=%d body=%s", response.StatusCode, body)
			}
		})
	}
}

func TestStreamingFixturePreservesSSEContract(t *testing.T) {
	fixture := loadFixture(t, "stream.json")
	provider := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		response.WriteHeader(http.StatusOK)
		flusher := response.(http.Flusher)
		_, _ = response.Write([]byte(": keep-alive\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		flusher.Flush()
		_, _ = response.Write([]byte("data: {\"choices\":[],\"usage\":{\"total_tokens\":3}}\n\ndata: [DONE]\n\n"))
	})
	gateway := newGateway(t, provider)
	defer gateway.Close()
	request, err := http.NewRequest(http.MethodPost, gateway.URL+"/v1/chat/completions", strings.NewReader(string(fixture.Body)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer gateway-key")
	response, err := gateway.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	scanner := bufio.NewScanner(response.Body)
	var stream strings.Builder
	for scanner.Scan() {
		stream.WriteString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(stream.String(), "keep-alive") || !strings.Contains(stream.String(), "[DONE]") || !strings.Contains(stream.String(), "usage") {
		t.Fatalf("status=%d stream=%q", response.StatusCode, stream.String())
	}
}

func TestCompatibilityFixtureRejectsMissingGatewayAuthentication(t *testing.T) {
	gateway := newGateway(t, http.NotFoundHandler())
	defer gateway.Close()
	request, err := http.NewRequest(http.MethodGet, gateway.URL+"/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := gateway.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d", response.StatusCode)
	}
}
