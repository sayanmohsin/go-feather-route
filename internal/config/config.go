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

type Config struct {
	Server    ServerConfig              `yaml:"server"`
	Auth      AuthConfig                `yaml:"auth"`
	Providers map[string]ProviderConfig `yaml:"providers"`
}

type ServerConfig struct {
	Address               string        `yaml:"address"`
	LogLevel              string        `yaml:"log_level"`
	RequestTimeout        time.Duration `yaml:"-"`
	RequestTimeoutText    string        `yaml:"request_timeout"`
	MaxBodyBytes          int64         `yaml:"max_body_bytes"`
	MaxConcurrentRequests int           `yaml:"max_concurrent_requests"`
}

type AuthConfig struct {
	APIKeyEnv string `yaml:"api_key_env"`
}

type ProviderConfig struct {
	BaseURL   string   `yaml:"base_url"`
	APIKeyEnv string   `yaml:"api_key_env"`
	Models    []string `yaml:"models"`
}

func Load(path string, env map[string]string) (Config, error) {
	config := defaults()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("read config %q: %w", path, err)
		}
		if err := yaml.Unmarshal(data, &config); err != nil {
			return Config{}, fmt.Errorf("parse config %q: %w", path, err)
		}
	}
	applyEnvironment(&config, env)
	if err := validate(&config, env); err != nil {
		return Config{}, err
	}
	return config, nil
}

func LoadFromEnvironment() (Config, error) {
	env := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	return Load(env["GOFEATHERROUTE_CONFIG_FILE"], env)
}

func defaults() Config {
	return Config{
		Server: ServerConfig{
			Address:               ":4000",
			LogLevel:              "info",
			RequestTimeoutText:    "60s",
			MaxBodyBytes:          1 << 20,
			MaxConcurrentRequests: 16,
		},
		Auth: AuthConfig{APIKeyEnv: "GOFEATHERROUTE_API_KEY"},
		Providers: map[string]ProviderConfig{
			"openai":   {BaseURL: "https://api.openai.com/v1", APIKeyEnv: "OPENAI_API_KEY", Models: []string{"gpt-4o-mini"}},
			"deepseek": {BaseURL: "https://api.deepseek.com/v1", APIKeyEnv: "DEEPSEEK_API_KEY", Models: []string{"deepseek-chat"}},
		},
	}
}

func applyEnvironment(config *Config, env map[string]string) {
	if value := env["GOFEATHERROUTE_ADDR"]; value != "" {
		config.Server.Address = value
	}
	if value := env["GOFEATHERROUTE_LOG_LEVEL"]; value != "" {
		config.Server.LogLevel = value
	}
	if value := env["GOFEATHERROUTE_REQUEST_TIMEOUT"]; value != "" {
		config.Server.RequestTimeoutText = value
	}
	if value := env["GOFEATHERROUTE_MAX_BODY_BYTES"]; value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			config.Server.MaxBodyBytes = parsed
		}
	}
	if value := env["GOFEATHERROUTE_MAX_CONCURRENT_REQUESTS"]; value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			config.Server.MaxConcurrentRequests = parsed
		}
	}
}

func validate(config *Config, env map[string]string) error {
	if config.Server.Address == "" {
		return errors.New("server.address must not be empty")
	}
	if config.Server.MaxBodyBytes <= 0 {
		return errors.New("server.max_body_bytes must be positive")
	}
	if config.Server.MaxConcurrentRequests <= 0 {
		return errors.New("server.max_concurrent_requests must be positive")
	}
	parsed, err := time.ParseDuration(config.Server.RequestTimeoutText)
	if err != nil || parsed <= 0 {
		return fmt.Errorf("server.request_timeout must be a positive duration: %q", config.Server.RequestTimeoutText)
	}
	config.Server.RequestTimeout = parsed
	if !strings.Contains(config.Server.LogLevel, "debug") && config.Server.LogLevel != "info" && config.Server.LogLevel != "warn" && config.Server.LogLevel != "error" {
		return fmt.Errorf("server.log_level must be debug, info, warn, or error: %q", config.Server.LogLevel)
	}
	for name, provider := range config.Providers {
		parsedURL, err := url.Parse(provider.BaseURL)
		if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
			return fmt.Errorf("providers.%s.base_url must be an https URL", name)
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
