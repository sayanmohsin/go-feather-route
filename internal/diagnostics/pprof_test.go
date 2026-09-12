package diagnostics

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStartDisabled(t *testing.T) {
	server, err := Start("", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil || server != nil {
		t.Fatalf("Start(empty) = server %v, error %v", server, err)
	}
}

func TestStartRejectsPublicDiagnosticsAddress(t *testing.T) {
	server, err := Start(":6060", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil || server != nil {
		t.Fatalf("Start(public address) = server %v, error %v", server, err)
	}
}

func TestDiagnosticsServerExposesProfilesSeparately(t *testing.T) {
	server, err := Start("127.0.0.1:0", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if shutdownErr := server.Shutdown(ctx); shutdownErr != nil {
			t.Fatal(shutdownErr)
		}
	}()

	client := http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + server.Address() + "/debug/pprof/goroutineleak?debug=1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("profile status = %d", response.StatusCode)
	}
	var body strings.Builder
	if _, err := io.Copy(&body, response.Body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.String(), "goroutineleak") {
		t.Fatalf("profile body did not identify goroutineleak: %q", body.String())
	}
}
