// Package skills provides a composable capability system for orch agents.
// A Skill is a named bundle of: allowed tool names, a system prompt hint,
// and an optional model preference. Skills are assignable per-agent in config.
package skills

import (
	"fmt"
	"strings"
	"sync"
)

// Skill is a named capability bundle assignable to any agent.
type Skill struct {
	// Name is the unique skill identifier (e.g. "cartography", "simplify").
	Name string
	// Description is a human-readable explanation of what the skill does.
	Description string
	// Tools lists the tool names from tools.Registry that this skill requires.
	Tools []string
	// SystemHint is appended to the agent's system prompt when the skill is active.
	SystemHint string
	// ModelHint optionally suggests a preferred model ID for this skill.
	ModelHint string
	// Enabled controls whether the skill is active.
	Enabled bool
}

// Registry holds all registered skills.
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*Skill
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *Registry {
	return &Registry{skills: make(map[string]*Skill)}
}

// Register adds or replaces a skill in the registry.
func (r *Registry) Register(s *Skill) {
	if s == nil || strings.TrimSpace(s.Name) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[strings.ToLower(s.Name)] = s
}

// Get returns the skill with the given name, or an error if not found.
func (r *Registry) Get(name string) (*Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", name)
	}
	return s, nil
}

// List returns all registered skills.
func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	return out
}

// CollectHints returns a concatenated system hint string for the named skills.
// Skills not found in the registry are silently skipped.
func (r *Registry) CollectHints(names []string) string {
	var parts []string
	for _, name := range names {
		s, err := r.Get(name)
		if err != nil || !s.Enabled {
			continue
		}
		if strings.TrimSpace(s.SystemHint) != "" {
			parts = append(parts, strings.TrimSpace(s.SystemHint))
		}
	}
	return strings.Join(parts, "\n")
}

// CollectTools returns a deduplicated list of tool names required by the named skills.
func (r *Registry) CollectTools(names []string) []string {
	seen := make(map[string]struct{})
	var tools []string
	for _, name := range names {
		s, err := r.Get(name)
		if err != nil || !s.Enabled {
			continue
		}
		for _, t := range s.Tools {
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				tools = append(tools, t)
			}
		}
	}
	return tools
}
