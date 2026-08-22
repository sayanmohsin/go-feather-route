package gateway

import "testing"

func TestRoutesCopyAndResolveAliases(t *testing.T) {
	configured := map[string]string{"gpt-test": "openai"}
	routes := NewRoutes(configured)
	configured["gpt-test"] = "changed"

	provider, ok := routes.ProviderFor("gpt-test")
	if !ok || provider != "openai" {
		t.Fatalf("provider=%q ok=%t", provider, ok)
	}
	provider, ok = routes.ProviderFor("openai/gpt-test")
	if !ok || provider != "openai" {
		t.Fatalf("provider fallback=%q ok=%t", provider, ok)
	}
}

func TestRoutesRejectUnknownProviderNotation(t *testing.T) {
	if _, ok := NewRoutes(map[string]string{"gpt-test": "openai"}).ProviderFor("unknown/gpt-test"); ok {
		t.Fatal("expected unknown provider notation to be rejected")
	}
}

func TestRoutesModelsAreSorted(t *testing.T) {
	models := NewRoutes(map[string]string{"z-model": "openai", "a-model": "openai"}).Models()
	if len(models) != 2 || models[0] != "a-model" || models[1] != "z-model" {
		t.Fatalf("models=%v", models)
	}
}
