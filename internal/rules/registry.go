package rules

import "github.com/bard-works/hcl-linter/internal/config"

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
