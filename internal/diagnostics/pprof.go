// Package diagnostics provides explicitly enabled, private runtime diagnostics.
package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"
)

// Server is an opt-in diagnostics HTTP server. It is deliberately separate
// from the public gateway handler so profiling routes cannot be exposed by
// accident through the OpenAI-compatible listener.
type Server struct {
	server   *http.Server
	listener net.Listener
	errors   chan error
}

// Start binds and starts a diagnostics server. An empty address disables it.
func Start(address string, logger *slog.Logger) (*Server, error) {
	if address == "" {
		return nil, nil
	}
	if err := validateLoopbackAddress(address); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen for diagnostics on %q: %w", address, err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /debug/pprof/", index)
	for _, name := range []string{"allocs", "block", "goroutine", "goroutineleak", "heap", "mutex", "threadcreate"} {
		mux.HandleFunc("GET /debug/pprof/"+name, profile(name))
	}
	server := &Server{
		server: &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       30 * time.Second,
		},
		listener: listener,
		errors:   make(chan error, 1),
	}
	go func() {
		logger.Info("diagnostics server listening", "address", listener.Addr().String())
		if serveErr := server.server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			server.errors <- serveErr
		}
		close(server.errors)
	}()
	return server, nil
}

func validateLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("diagnostics address %q must include a loopback host and port: %w", address, err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsLoopback() {
		return fmt.Errorf("diagnostics address %q must use a loopback host", address)
	}
	return nil
}

func index(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintln(response, "Go Feather Route runtime profiles:")
	for _, name := range []string{"allocs", "block", "goroutine", "goroutineleak", "heap", "mutex", "threadcreate"} {
		_, _ = fmt.Fprintf(response, "/debug/pprof/%s\n", name)
	}
}

func profile(name string) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		item := pprof.Lookup(name)
		if item == nil {
			http.NotFound(response, request)
			return
		}
		debug := 0
		if value, err := strconv.Atoi(request.URL.Query().Get("debug")); err == nil && value > 0 {
			debug = value
		}
		if debug > 0 {
			response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		} else {
			response.Header().Set("Content-Type", "application/octet-stream")
		}
		if err := item.WriteTo(response, debug); err != nil {
			return
		}
	}
}

// Address returns the bound diagnostics address.
func (s *Server) Address() string {
	if s == nil || s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Errors reports an asynchronous server error, if one occurs.
func (s *Server) Errors() <-chan error {
	if s == nil {
		return nil
	}
	return s.errors
}

// Shutdown stops the diagnostics listener.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}
