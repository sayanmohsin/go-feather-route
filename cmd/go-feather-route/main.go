// Command go-feather-route starts the model-routing gateway.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sayanmohsin/go-feather-route/internal/config"
	"github.com/sayanmohsin/go-feather-route/internal/diagnostics"
	"github.com/sayanmohsin/go-feather-route/internal/router"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		checkHealth()
		return
	}
	configFile, address := commandLineOverrides()
	cfg, err := config.LoadFromEnvironmentWith(config.Overrides{ConfigFile: configFile, Address: address})
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	if cfg.Auth.APIKey == "" {
		slog.Error("configuration failed", "error", "GOFEATHERROUTE_API_KEY must be set")
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           router.NewServer(cfg, logger).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	diagnosticsServer, err := diagnostics.Start(cfg.Server.DiagnosticsAddress, logger)
	if err != nil {
		logger.Error("diagnostics server failed", "error", err)
		os.Exit(1)
	}
	go func() {
		logger.Info("server starting", "address", cfg.Server.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()
	var diagnosticsErrors <-chan error
	if diagnosticsServer != nil {
		diagnosticsErrors = diagnosticsServer.Errors()
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-signals:
	case err := <-diagnosticsErrors:
		if err != nil {
			logger.Error("diagnostics server stopped", "error", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := diagnosticsServer.Shutdown(ctx); err != nil {
		logger.Error("diagnostics shutdown failed", "error", err)
		os.Exit(1)
	}
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}
}

func commandLineOverrides() (string, string) {
	flags := flag.NewFlagSet("go-feather-route", flag.ExitOnError)
	configFile := flags.String("config", "", "path to the YAML configuration file")
	address := flags.String("addr", "", "HTTP listen address")
	_ = flags.Parse(os.Args[1:])
	return *configFile, *address
}

func checkHealth() {
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:4000/health/liveliness")
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			_ = response.Body.Close()
		}
		os.Exit(1)
	}
	_ = response.Body.Close()
}
