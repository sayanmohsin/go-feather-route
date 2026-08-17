// Package config loads and validates Go Feather Route configuration.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the validated application configuration.
type Config struct {
	Server    ServerConfig              `yaml:"server"`
	Auth      AuthConfig                `yaml:"auth"`
	Providers map[string]ProviderConfig `yaml:"providers"`
	Routes    map[string]string         `yaml:"routes"`
	// AllowInsecureHTTP is intended only for local benchmark fixtures.
	AllowInsecureHTTP bool `yaml:"-"`
}

// ServerConfig controls the HTTP server and resource limits.
type ServerConfig struct {
	Address               string        `yaml:"address"`
	LogLevel              string        `yaml:"log_level"`
	RequestTimeout        time.Duration `yaml:"-"`
	RequestTimeoutText    string        `yaml:"request_timeout"`
	StreamIdleTimeout     time.Duration `yaml:"-"`
	StreamIdleTimeoutText string        `yaml:"stream_idle_timeout"`
	MaxBodyBytes          int64         `yaml:"max_body_bytes"`
	MaxResponseBytes      int64         `yaml:"max_response_bytes"`
	MaxConcurrentRequests int           `yaml:"max_concurrent_requests"`
	MaxConcurrentStreams  int           `yaml:"max_concurrent_streams"`
}

// AuthConfig controls gateway authentication.
type AuthConfig struct {
	APIKeyEnv string `yaml:"api_key_env"`
	APIKey    string `yaml:"-"`
}

// ProviderConfig describes one upstream provider.
type ProviderConfig struct {
	BaseURL   string   `yaml:"base_url"`
	APIKeyEnv string   `yaml:"api_key_env"`
	Models    []string `yaml:"models"`
	APIKey    string   `yaml:"-"`
}

// Load reads a YAML configuration file and applies environment overrides.
func Load(path string, env map[string]string) (Config, error) {
	config := defaults()
	if path != "" {
		data, err := os.ReadFile(path) // #nosec G304 -- the operator explicitly selects the config path.
		if err != nil {
			return Config{}, fmt.Errorf("read config %q: %w", path, err)
		}
		if err := yaml.Unmarshal(data, &config); err != nil {
			return Config{}, fmt.Errorf("parse config %q: %w", path, err)
		}
	}
	if err := applyEnvironment(&config, env); err != nil {
		return Config{}, err
	}
	if err := validate(&config, env); err != nil {
		return Config{}, err
	}
	return config, nil
}

// LoadFromEnvironment loads configuration using the current process environment.
func LoadFromEnvironment() (Config, error) {
	env := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	path := env["GOFEATHERROUTE_CONFIG_FILE"]
	if path == "" {
		for _, candidate := range []string{"config/defaults.yaml", "/etc/go-feather-route/defaults.yaml"} {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}
	return Load(path, env)
}

func defaults() Config {
	return Config{
		Server: ServerConfig{
			Address:               ":4000",
			LogLevel:              "info",
			RequestTimeoutText:    "60s",
			StreamIdleTimeoutText: "30s",
			MaxBodyBytes:          1 << 20,
			MaxResponseBytes:      8 << 20,
			MaxConcurrentRequests: 16,
			MaxConcurrentStreams:  4,
		},
		Auth: AuthConfig{APIKeyEnv: "GOFEATHERROUTE_API_KEY"},
		Providers: map[string]ProviderConfig{
			"openai":   {BaseURL: "https://api.openai.com/v1", APIKeyEnv: "OPENAI_API_KEY", Models: []string{"gpt-4o-mini"}},
			"deepseek": {BaseURL: "https://api.deepseek.com/v1", APIKeyEnv: "DEEPSEEK_API_KEY", Models: []string{"deepseek-chat"}},
		},
		Routes: map[string]string{"gpt-4o-mini": "openai", "deepseek-chat": "deepseek"},
	}
}

func applyEnvironment(config *Config, env map[string]string) error {
	if value := env["GOFEATHERROUTE_ADDR"]; value != "" {
		config.Server.Address = value
	}
	if value := env["GOFEATHERROUTE_LOG_LEVEL"]; value != "" {
		config.Server.LogLevel = value
	}
	if value := env["GOFEATHERROUTE_REQUEST_TIMEOUT"]; value != "" {
		config.Server.RequestTimeoutText = value
	}
	if value := env["GOFEATHERROUTE_STREAM_IDLE_TIMEOUT"]; value != "" {
		config.Server.StreamIdleTimeoutText = value
	}
	if value := env["GOFEATHERROUTE_MAX_BODY_BYTES"]; value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("GOFEATHERROUTE_MAX_BODY_BYTES must be an integer: %w", err)
		}
		config.Server.MaxBodyBytes = parsed
	}
	if value := env["GOFEATHERROUTE_MAX_RESPONSE_BYTES"]; value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("GOFEATHERROUTE_MAX_RESPONSE_BYTES must be an integer: %w", err)
		}
		config.Server.MaxResponseBytes = parsed
	}
	if value := env["GOFEATHERROUTE_MAX_CONCURRENT_REQUESTS"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("GOFEATHERROUTE_MAX_CONCURRENT_REQUESTS must be an integer: %w", err)
		}
		config.Server.MaxConcurrentRequests = parsed
	}
	if value := env["GOFEATHERROUTE_MAX_CONCURRENT_STREAMS"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("GOFEATHERROUTE_MAX_CONCURRENT_STREAMS must be an integer: %w", err)
		}
		config.Server.MaxConcurrentStreams = parsed
	}
	config.Auth.APIKey = env[config.Auth.APIKeyEnv]
	config.AllowInsecureHTTP = env["GOFEATHERROUTE_ALLOW_INSECURE_HTTP"] == "1" || env["GOFEATHERROUTE_ALLOW_INSECURE_HTTP"] == "true"
	for name, provider := range config.Providers {
		provider.APIKey = env[provider.APIKeyEnv]
		config.Providers[name] = provider
	}
	return nil
}

func validate(config *Config, env map[string]string) error {
	if config.Server.Address == "" {
		return errors.New("server.address must not be empty")
	}
	if config.Server.MaxBodyBytes <= 0 {
		return errors.New("server.max_body_bytes must be positive")
	}
	if config.Server.MaxResponseBytes <= 0 {
		return errors.New("server.max_response_bytes must be positive")
	}
	if config.Server.MaxConcurrentRequests <= 0 {
		return errors.New("server.max_concurrent_requests must be positive")
	}
	if config.Server.MaxConcurrentStreams <= 0 {
		return errors.New("server.max_concurrent_streams must be positive")
	}
	parsed, err := time.ParseDuration(config.Server.RequestTimeoutText)
	if err != nil || parsed <= 0 {
		return fmt.Errorf("server.request_timeout must be a positive duration: %q", config.Server.RequestTimeoutText)
	}
	config.Server.RequestTimeout = parsed
	streamIdleTimeout, err := time.ParseDuration(config.Server.StreamIdleTimeoutText)
	if err != nil || streamIdleTimeout <= 0 {
		return fmt.Errorf("server.stream_idle_timeout must be a positive duration: %q", config.Server.StreamIdleTimeoutText)
	}
	config.Server.StreamIdleTimeout = streamIdleTimeout
	if !strings.Contains(config.Server.LogLevel, "debug") && config.Server.LogLevel != "info" && config.Server.LogLevel != "warn" && config.Server.LogLevel != "error" {
		return fmt.Errorf("server.log_level must be debug, info, warn, or error: %q", config.Server.LogLevel)
	}
	for name, provider := range config.Providers {
		parsedURL, err := url.Parse(provider.BaseURL)
		if err != nil || parsedURL.Host == "" {
			return fmt.Errorf("providers.%s.base_url must be a valid URL", name)
		}
		if parsedURL.Scheme == "http" && !config.AllowInsecureHTTP {
			return fmt.Errorf("providers.%s.base_url must be an https URL unless GOFEATHERROUTE_ALLOW_INSECURE_HTTP is enabled for a benchmark fixture", name)
		}
		if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
			return fmt.Errorf("providers.%s.base_url must be an https URL unless GOFEATHERROUTE_ALLOW_INSECURE_HTTP is enabled for a benchmark fixture", name)
		}
		if provider.APIKeyEnv == "" {
			return fmt.Errorf("providers.%s.api_key_env must not be empty", name)
		}
	}
	if env == nil {
		return errors.New("environment must not be nil")
	}
	return nil
}
