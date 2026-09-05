package router

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sayanmohsin/go-feather-route/internal/config"
	"github.com/sayanmohsin/go-feather-route/internal/provider"
)

type chunkReadCloser struct {
	chunks [][]byte
	index  int
}

func (r *chunkReadCloser) Read(destination []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	r.index++
	return copy(destination, chunk), nil
}

func (r *chunkReadCloser) Close() error { return nil }

func TestStreamForwardsSplitDoneMarkerAndCompletes(t *testing.T) {
	server := &Server{
		config: config.Config{Server: config.ServerConfig{StreamIdleTimeout: time.Second}},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	response := httptest.NewRecorder()
	upstream := provider.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: &chunkReadCloser{chunks: [][]byte{
			[]byte("data: {\"choices\":[]}\n\ndata: [DO"),
			[]byte("NE]\n\n"),
		}},
	}

	completed, firstByte := server.streamResponse(response, upstream)
	if !completed {
		t.Fatal("stream with split [DONE] marker was marked aborted")
	}
	if firstByte <= 0 {
		t.Fatal("expected first-byte timing")
	}
	if body := response.Body.String(); !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("stream body = %q", body)
	}
}

type closeAwareReader struct {
	closed chan struct{}
	once   sync.Once
}

func (r *closeAwareReader) Read(_ []byte) (int, error) {
	<-r.closed
	return 0, errors.New("reader closed")
}

func (r *closeAwareReader) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

func TestStreamWatchdogClosesStalledReader(t *testing.T) {
	reader := &closeAwareReader{closed: make(chan struct{})}
	watchdog := newStreamWatchdog(reader, 10*time.Millisecond)
	defer watchdog.Stop()

	select {
	case <-reader.closed:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("watchdog did not close stalled reader")
	}
	if !watchdog.Expired() {
		t.Fatal("watchdog did not record expiration")
	}
}

func TestStreamWatchdogStopDoesNotCloseActiveReader(t *testing.T) {
	reader := &closeAwareReader{closed: make(chan struct{})}
	watchdog := newStreamWatchdog(reader, 100*time.Millisecond)
	watchdog.Stop()

	select {
	case <-reader.closed:
		t.Fatal("stopping watchdog closed active reader")
	case <-time.After(150 * time.Millisecond):
	}
	if watchdog.Expired() {
		t.Fatal("stopped watchdog expired")
	}
}
