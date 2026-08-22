// Package gateway contains provider-independent routing policy.
package gateway

import (
	"sort"
	"strings"
)

// Routes is an immutable model-to-provider routing table.
type Routes struct {
	byModel map[string]string
}

// NewRoutes copies the supplied route configuration so runtime routing cannot
// be changed by a caller retaining the original map.
func NewRoutes(routes map[string]string) Routes {
	copyOfRoutes := make(map[string]string, len(routes))
	for model, provider := range routes {
		copyOfRoutes[model] = provider
	}
	return Routes{byModel: copyOfRoutes}
}

// ProviderFor resolves an exact model alias, then accepts provider/model
// notation for explicitly configured provider names.
func (r Routes) ProviderFor(model string) (string, bool) {
	if provider, ok := r.byModel[model]; ok && provider != "" {
		return provider, true
	}
	provider, _, ok := strings.Cut(model, "/")
	if !ok || provider == "" {
		return "", false
	}
	for _, configuredProvider := range r.byModel {
		if configuredProvider == provider {
			return provider, true
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
	provider, ok := r.byModel[model]
	return provider, ok && provider != ""
}
