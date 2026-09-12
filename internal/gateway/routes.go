// Package gateway contains provider-independent routing policy.
package gateway

import (
	"sort"
	"strings"
)

// Rule maps an exact or prefix model pattern to a provider.
type Rule struct {
	Match    string
	Provider string
}

// Routes is an immutable model-to-provider routing table.
type Routes struct {
	byModel map[string]string
	rules   []Rule
}

// NewRoutes copies the supplied route configuration so runtime routing cannot
// be changed by a caller retaining the original map.
func NewRoutes(routes map[string]string) Routes {
	return NewRoutesWithRules(routes, nil)
}

// NewRoutesWithRules copies route and pattern configuration into an immutable table.
func NewRoutesWithRules(routes map[string]string, rules []Rule) Routes {
	copyOfRoutes := make(map[string]string, len(routes))
	for model, provider := range routes {
		copyOfRoutes[model] = provider
	}
	copyOfRules := append([]Rule(nil), rules...)
	return Routes{byModel: copyOfRoutes, rules: copyOfRules}
}

// ProviderFor resolves an exact model alias, then accepts provider/model
// notation for explicitly configured provider names.
func (r Routes) ProviderFor(model string) (string, bool) {
	if provider, ok := r.byModel[model]; ok && provider != "" {
		return provider, true
	}
	provider, _, ok := strings.Cut(model, "/")
	if ok && provider != "" {
		for _, rule := range r.rules {
			if rule.Match == provider+"/*" {
				return rule.Provider, true
			}
		}
		if len(r.rules) == 0 {
			for _, configuredProvider := range r.byModel {
				if configuredProvider == provider {
					return provider, true
				}
			}
		}
	}
	for _, rule := range r.rules {
		if strings.HasSuffix(rule.Match, "*") && strings.HasPrefix(model, strings.TrimSuffix(rule.Match, "*")) {
			return rule.Provider, true
		}
	}
	return "", false
}

// Models returns sorted configured model aliases.
func (r Routes) Models() []string {
	models := make([]string, 0, len(r.byModel))
	for model := range r.byModel {
		models = append(models, model)
	}
	sort.Strings(models)
	return models
}

// ProviderForModel returns the configured provider for an exact model alias.
func (r Routes) ProviderForModel(model string) (string, bool) {
	if provider, ok := r.byModel[model]; ok && provider != "" {
		return provider, true
	}
	return r.ProviderFor(model)
}
