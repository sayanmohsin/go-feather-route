package usage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseSSEUsesFinalUsageEvent(t *testing.T) {
	got := ParseSSE([]byte("data: {\"choices\":[]}\n\ndata: {\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":3}}\n\ndata: [DONE]\n\n"))
	if got.Input != 4 || got.Output != 3 || got.Total != 7 {
		t.Fatalf("tokens=%+v", got)
	}
}

func TestReporterSendsOnlyNormalizedEvent(t *testing.T) {
	called := make(chan Event, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer callback-secret" {
			t.Fatal("missing callback authorization")
		}
		var event Event
		if err := json.NewDecoder(request.Body).Decode(&event); err != nil {
			t.Fatal(err)
		}
		called <- event
		response.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	Reporter{Endpoint: server.URL, APIKey: "callback-secret", Timeout: time.Second}.Report(Event{
		RequestID: "request-1",
		Gateway:   "go-feather-route",
		Provider:  "arbitrary-provider",
		Model:     "arbitrary-model",
		Status:    "completed",
	})
	select {
	case event := <-called:
		if event.RequestID != "request-1" || event.Provider != "arbitrary-provider" {
			t.Fatalf("event=%+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("usage event was not delivered")
	}
}
