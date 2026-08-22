package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnvironmentOverrides(t *testing.T) {
	config, err := Load("", map[string]string{
		"GOFEATHERROUTE_ADDR":            ":4100",
		"GOFEATHERROUTE_REQUEST_TIMEOUT": "2s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.Server.Address != ":4100" {
		t.Fatalf("address = %q", config.Server.Address)
	}
	if config.Server.RequestTimeout.String() != "2s" {
		t.Fatalf("timeout = %s", config.Server.RequestTimeout)
	}
}

func TestCLIOverridesEnvironmentAndYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  address: :4200\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadWithOverrides(path, map[string]string{"GOFEATHERROUTE_ADDR": ":4300"}, Overrides{Address: ":4400"})
	if err != nil {
		t.Fatal(err)
	}
	if config.Server.Address != ":4400" {
		t.Fatalf("address = %q", config.Server.Address)
	}
}

func TestCLIConfigFileOverridesEnvironmentConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cli.yaml")
	if err := os.WriteFile(path, []byte("server:\n  address: :4500\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadFromEnvironmentWith(Overrides{ConfigFile: path})
	if err != nil {
		t.Fatal(err)
	}
	if config.Server.Address != ":4500" {
		t.Fatalf("address = %q", config.Server.Address)
	}
}

func TestLoadRejectsInvalidLimits(t *testing.T) {
	_, err := Load("", map[string]string{"GOFEATHERROUTE_MAX_BODY_BYTES": "0"})
	if err == nil {
		t.Fatal("expected invalid limit error")
	}
}

func TestLoadRejectsMalformedNumericEnvironment(t *testing.T) {
	_, err := Load("", map[string]string{"GOFEATHERROUTE_MAX_BODY_BYTES": "not-a-number"})
	if err == nil {
		t.Fatal("expected malformed numeric environment error")
	}
}

func TestEnvironmentExample(t *testing.T) {
	data, err := os.ReadFile("../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, name := range []string{
		"GOFEATHERROUTE_CONFIG_FILE",
		"GOFEATHERROUTE_ADDR",
		"GOFEATHERROUTE_LOG_LEVEL",
		"GOFEATHERROUTE_REQUEST_TIMEOUT",
		"GOFEATHERROUTE_STREAM_IDLE_TIMEOUT",
		"GOFEATHERROUTE_MAX_BODY_BYTES",
		"GOFEATHERROUTE_MAX_RESPONSE_BYTES",
		"GOFEATHERROUTE_MAX_CONCURRENT_REQUESTS",
		"GOFEATHERROUTE_MAX_CONCURRENT_STREAMS",
		"GOFEATHERROUTE_API_KEY",
		"OPENAI_API_KEY",
		"DEEPSEEK_API_KEY",
	} {
		if !strings.Contains(content, name+"=") {
			t.Fatalf(".env.example missing %s", name)
		}
	}
}

func TestHTTPProviderRequiresExplicitBenchmarkOptIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("providers:\n  fake:\n    base_url: http://fake-provider:8080/v1\n    api_key_env: FAKE_API_KEY\nroutes:\n  test-model: fake\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, map[string]string{"FAKE_API_KEY": "test"}); err == nil {
		t.Fatal("expected HTTP provider configuration to fail without benchmark opt-in")
	}
	config, err := Load(path, map[string]string{
		"FAKE_API_KEY":                       "test",
		"GOFEATHERROUTE_ALLOW_INSECURE_HTTP": "true",
	})
	if err != nil {
		t.Fatalf("expected benchmark HTTP provider to load: %v", err)
	}
	if !config.AllowInsecureHTTP {
		t.Fatal("expected benchmark HTTP opt-in to be enabled")
	}
}
