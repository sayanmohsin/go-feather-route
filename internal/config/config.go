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
	Server     ServerConfig              `yaml:"server"`
	Auth       AuthConfig                `yaml:"auth"`
	Usage      UsageConfig               `yaml:"usage"`
	Providers  map[string]ProviderConfig `yaml:"providers"`
	ModelList  []ModelRoute              `yaml:"model_list"`
	Routes     map[string]string         `yaml:"routes"`
	RouteRules []RouteRule               `yaml:"route_rules"`
	// AllowInsecureHTTP is intended only for local benchmark fixtures.
	AllowInsecureHTTP bool `yaml:"-"`
}

// ModelRoute is a LiteLLM-style public model name mapped to a provider model.
// Model names remain configuration data; the gateway does not hardcode them.
type ModelRoute struct {
	ModelName     string   `yaml:"model_name"`
	Provider      string   `yaml:"provider"`
	UpstreamModel string   `yaml:"upstream_model"`
	Fallbacks     []string `yaml:"fallbacks"`
	InputCost     float64  `yaml:"input_cost_per_million_tokens"`
	OutputCost    float64  `yaml:"output_cost_per_million_tokens"`
	EmbeddingCost float64  `yaml:"embedding_cost_per_million_tokens"`
}

// UsageConfig controls optional Cloud usage event delivery.
type UsageConfig struct {
	Endpoint    string        `yaml:"endpoint"`
	APIKeyEnv   string        `yaml:"api_key_env"`
	APIKey      string        `yaml:"-"`
	TimeoutText string        `yaml:"timeout"`
	Timeout     time.Duration `yaml:"-"`
}

// RouteRule maps arbitrary model names to a configured provider. A trailing
// '*' is treated as a prefix match so new provider models need no code change.
type RouteRule struct {
	Match    string `yaml:"match"`
	Provider string `yaml:"provider"`
}

// ServerConfig controls the HTTP server and resource limits.
type ServerConfig struct {
	Address                 string        `yaml:"address"`
	DiagnosticsAddress      string        `yaml:"diagnostics_address"`
	LogLevel                string        `yaml:"log_level"`
	RequestTimeout          time.Duration `yaml:"-"`
	RequestTimeoutText      string        `yaml:"request_timeout"`
	StreamIdleTimeout       time.Duration `yaml:"-"`
	StreamIdleTimeoutText   string        `yaml:"stream_idle_timeout"`
	MaxBodyBytes            int64         `yaml:"max_body_bytes"`
	MaxResponseBytes        int64         `yaml:"max_response_bytes"`
	MaxConcurrentRequests   int           `yaml:"max_concurrent_requests"`
	MaxConcurrentEmbeddings int           `yaml:"max_concurrent_embeddings"`
	MaxConcurrentStreams    int           `yaml:"max_concurrent_streams"`
}

// AuthConfig controls gateway authentication.
type AuthConfig struct {
	APIKeyEnv string `yaml:"api_key_env"`
	APIKey    string `yaml:"-"`
}

// ProviderConfig describes one upstream provider.
type ProviderConfig struct {
	BaseURL      string            `yaml:"base_url"`
	APIKeyEnv    string            `yaml:"api_key_env"`
	Kind         string            `yaml:"kind"`
	Models       []string          `yaml:"models"`
	ModelAliases map[string]string `yaml:"model_aliases"`
	APIKey       string            `yaml:"-"`
}

// Overrides contains command-line values that take precedence over the
// environment and YAML configuration.
type Overrides struct {
	ConfigFile string
	Address    string
}

// Load reads a YAML configuration file and applies environment overrides.
func Load(path string, env map[string]string) (Config, error) {
	return LoadWithOverrides(path, env, Overrides{})
}

// LoadWithOverrides loads configuration using CLI overrides above environment
// variables, YAML, and defaults.
func LoadWithOverrides(path string, env map[string]string, overrides Overrides) (Config, error) {
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
	normalizeModelRoutes(&config)
	if err := applyEnvironment(&config, env); err != nil {
		return Config{}, err
	}
	if overrides.Address != "" {
		config.Server.Address = overrides.Address
	}
	if err := validate(&config, env); err != nil {
		return Config{}, err
	}
	return config, nil
}

// LoadFromEnvironment loads configuration using the current process environment.
func LoadFromEnvironment() (Config, error) {
	return LoadFromEnvironmentWith(Overrides{})
}

// LoadFromEnvironmentWith loads process configuration with explicit CLI
// overrides applied at the highest precedence.
func LoadFromEnvironmentWith(overrides Overrides) (Config, error) {
	env := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	path := overrides.ConfigFile
	if path == "" {
		path = env["GOFEATHERROUTE_CONFIG_FILE"]
	}
	if path == "" {
		for _, candidate := range []string{"config/defaults.yaml", "/etc/go-feather-route/defaults.yaml"} {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}
	return LoadWithOverrides(path, env, overrides)
}

func defaults() Config {
	return Config{
		Server: ServerConfig{
			Address:                 ":4000",
			DiagnosticsAddress:      "",
			LogLevel:                "info",
			RequestTimeoutText:      "60s",
			StreamIdleTimeoutText:   "30s",
			MaxBodyBytes:            1 << 20,
			MaxResponseBytes:        8 << 20,
			MaxConcurrentRequests:   16,
			MaxConcurrentEmbeddings: 2,
			MaxConcurrentStreams:    4,
		},
		Auth:       AuthConfig{APIKeyEnv: "GOFEATHERROUTE_API_KEY"},
		Usage:      UsageConfig{APIKeyEnv: "GOFEATHERROUTE_USAGE_API_KEY", TimeoutText: "2s"},
		Providers:  map[string]ProviderConfig{},
		ModelList:  nil,
		Routes:     map[string]string{},
		RouteRules: nil,
	}
}

func normalizeModelRoutes(config *Config) {
	if config.Routes == nil {
		config.Routes = make(map[string]string)
	}
	for _, route := range config.ModelList {
		if route.ModelName == "" || route.Provider == "" {
			continue
		}
		config.Routes[route.ModelName] = route.Provider
		if provider, ok := config.Providers[route.Provider]; ok && route.UpstreamModel != "" {
			if provider.ModelAliases == nil {
				provider.ModelAliases = make(map[string]string)
			}
			provider.ModelAliases[route.ModelName] = route.UpstreamModel
			config.Providers[route.Provider] = provider
		}
	}
}

func applyEnvironment(config *Config, env map[string]string) error {
	if value := env["OLLAMA_API_BASE"]; value != "" {
		if provider, ok := config.Providers["ollama"]; ok {
			provider.BaseURL = strings.TrimRight(value, "/")
			config.Providers["ollama"] = provider
		}
	}
	if provider, ok := config.Providers["ollama"]; ok {
		if provider.ModelAliases == nil {
			provider.ModelAliases = make(map[string]string)
		}
		if value := env["OLLAMA_CHAT_MODEL"]; value != "" {
			provider.ModelAliases["ollama-qwen3"] = value
		}
		if value := env["OLLAMA_EMBEDDING_MODEL"]; value != "" {
			provider.ModelAliases["ollama-nomic-embed"] = value
		}
		config.Providers["ollama"] = provider
	}
	if value := env["GOFEATHERROUTE_ADDR"]; value != "" {
		config.Server.Address = value
	}
	if value := env["GOFEATHERROUTE_PPROF_ADDR"]; value != "" {
		config.Server.DiagnosticsAddress = value
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
	if value := env["GOFEATHERROUTE_USAGE_ENDPOINT"]; value != "" {
		config.Usage.Endpoint = strings.TrimRight(value, "/")
	}
	if value := env["GOFEATHERROUTE_USAGE_TIMEOUT"]; value != "" {
		config.Usage.TimeoutText = value
	}
	config.Usage.APIKey = env[config.Usage.APIKeyEnv]
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
	if value := env["GOFEATHERROUTE_MAX_CONCURRENT_EMBEDDINGS"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("GOFEATHERROUTE_MAX_CONCURRENT_EMBEDDINGS must be an integer: %w", err)
		}
		config.Server.MaxConcurrentEmbeddings = parsed
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
	if config.Server.MaxConcurrentEmbeddings <= 0 {
		return errors.New("server.max_concurrent_embeddings must be positive")
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
	usageTimeout, err := time.ParseDuration(config.Usage.TimeoutText)
	if err != nil || usageTimeout <= 0 {
		return fmt.Errorf("usage.timeout must be a positive duration: %q", config.Usage.TimeoutText)
	}
	config.Usage.Timeout = usageTimeout
	if !strings.Contains(config.Server.LogLevel, "debug") && config.Server.LogLevel != "info" && config.Server.LogLevel != "warn" && config.Server.LogLevel != "error" {
		return fmt.Errorf("server.log_level must be debug, info, warn, or error: %q", config.Server.LogLevel)
	}
	for name, provider := range config.Providers {
		if provider.Kind == "" {
			provider.Kind = "openai-compatible"
		}
		if provider.Kind != "openai-compatible" && provider.Kind != "ollama" {
			return fmt.Errorf("providers.%s.kind must be openai-compatible or ollama", name)
		}
		parsedURL, err := url.Parse(provider.BaseURL)
		if err != nil || parsedURL.Host == "" {
			return fmt.Errorf("providers.%s.base_url must be a valid URL", name)
		}
		localOllama := name == "ollama" && (parsedURL.Hostname() == "127.0.0.1" || parsedURL.Hostname() == "localhost" || parsedURL.Hostname() == "host.docker.internal")
		if parsedURL.Scheme == "http" && !config.AllowInsecureHTTP && !localOllama {
			return fmt.Errorf("providers.%s.base_url must be an https URL unless GOFEATHERROUTE_ALLOW_INSECURE_HTTP is enabled for a benchmark fixture", name)
		}
		if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
			return fmt.Errorf("providers.%s.base_url must be an https URL unless GOFEATHERROUTE_ALLOW_INSECURE_HTTP is enabled for a benchmark fixture", name)
		}
		if provider.APIKeyEnv == "" {
			return fmt.Errorf("providers.%s.api_key_env must not be empty", name)
		}
	}
	for _, route := range config.ModelList {
		if route.ModelName == "" || route.Provider == "" {
			return errors.New("model_list entries require model_name and provider")
		}
		if _, ok := config.Providers[route.Provider]; !ok {
			return fmt.Errorf("model_list.%s references unknown provider %q", route.ModelName, route.Provider)
		}
		for _, fallback := range route.Fallbacks {
			if _, ok := config.Providers[fallback]; !ok {
				return fmt.Errorf("model_list.%s references unknown fallback provider %q", route.ModelName, fallback)
			}
		}
		if route.InputCost < 0 || route.OutputCost < 0 || route.EmbeddingCost < 0 {
			return fmt.Errorf("model_list.%s cost values must not be negative", route.ModelName)
		}
	}
	for _, rule := range config.RouteRules {
		if rule.Match == "" || rule.Provider == "" {
			return errors.New("route_rules entries require match and provider")
		}
		if _, ok := config.Providers[rule.Provider]; !ok {
			return fmt.Errorf("route_rules.%s references unknown provider %q", rule.Match, rule.Provider)
		}
		if !strings.HasSuffix(rule.Match, "*") && !strings.Contains(rule.Match, "/") {
			return fmt.Errorf("route_rules.%s must use a provider-qualified pattern or trailing *", rule.Match)
		}
	}
	if env == nil {
		return errors.New("environment must not be nil")
	}
	return nil
}
