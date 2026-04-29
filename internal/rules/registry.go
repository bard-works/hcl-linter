package rules

import (
	"sort"

	"github.com/bard-works/hcl-linter/internal/config"
)

var defaultRegistry = &Registry{}

// DefaultRegistry returns the global registry populated via init() calls.
func DefaultRegistry() *Registry { return defaultRegistry }

// Register adds a rule to the global default registry.
func Register(rule Rule) { defaultRegistry.Register(rule) }

// Registry holds all registered rules in declaration order.
type Registry struct {
	rules []Rule
}

// Register adds a rule to the registry.
func (r *Registry) Register(rule Rule) {
	r.rules = append(r.rules, rule)
}

// All returns every registered rule.
func (r *Registry) All() []Rule {
	return r.rules
}

// Enabled returns rules that are active for the given config.
func (r *Registry) Enabled(cfg *config.Rules) []Rule {
	var active []Rule
	for _, rule := range r.rules {
		if rule.Enabled(cfg) {
			active = append(active, rule)
		}
	}
	return active
}

// Sorted returns a copy of the registry's rules sorted by Priority ascending,
// preserving registration order within the same priority tier.
func (r *Registry) Sorted() []Rule {
	out := make([]Rule, len(r.rules))
	copy(out, r.rules)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority() < out[j].Priority()
	})
	return out
}
