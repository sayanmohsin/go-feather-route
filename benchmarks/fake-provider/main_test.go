package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandlerEmbeddingsSupportsSingleAndBatchInputs(t *testing.T) {
	server := httptest.NewServer(handler(time.Millisecond))
	defer server.Close()

	for _, test := range []struct {
		name  string
		input string
		count int
	}{
		{name: "single", input: `"one"`, count: 1},
		{name: "batch", input: `["one","two"]`, count: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/embeddings", strings.NewReader(`{"model":"benchmark-model","input":`+test.input+`}`))
			if err != nil {
				t.Fatal(err)
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", response.StatusCode)
			}
			var payload struct {
				Data []struct {
					Index int       `json:"index"`
					Value []float64 `json:"embedding"`
				} `json:"data"`
			}
			if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Data) != test.count || len(payload.Data[0].Value) != 2 {
				t.Fatalf("data = %+v", payload.Data)
			}
			for index, item := range payload.Data {
				if item.Index != index {
					t.Fatalf("data[%d].index = %d", index, item.Index)
				}
			}
		})
	}
}
