package config

import (
	"os"
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
		"GOFEATHERROUTE_MAX_BODY_BYTES",
		"GOFEATHERROUTE_MAX_CONCURRENT_REQUESTS",
		"GOFEATHERROUTE_API_KEY",
		"OPENAI_API_KEY",
		"DEEPSEEK_API_KEY",
	} {
		if !strings.Contains(content, name+"=") {
			t.Fatalf(".env.example missing %s", name)
		}
	}
}
