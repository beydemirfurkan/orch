package providers

import (
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/config"
)

type Router struct {
	cfg      *config.Config
	registry *Registry
}

func NewRouter(cfg *config.Config, registry *Registry) *Router {
	return &Router{cfg: cfg, registry: registry}
}

// Resolve returns the provider and model ID for the given role.
// Priority: RoleAssignments (providerName:modelID) > per-provider Models config.
func (r *Router) Resolve(role Role) (Provider, string, error) {
	if r == nil || r.cfg == nil || r.registry == nil {
		return nil, "", fmt.Errorf("provider router is not initialized")
	}

	// Check RoleAssignments first — allows mixing providers per role.
	if ra := r.cfg.Provider.RoleAssignments; len(ra) > 0 {
		if assignment, ok := ra[string(role)]; ok && strings.TrimSpace(assignment) != "" {
			providerName, modelID, err := parseRoleAssignment(assignment)
			if err != nil {
				return nil, "", fmt.Errorf("invalid role assignment for %s: %w", role, err)
			}
			provider, err := r.registry.Get(providerName)
			if err != nil {
				return nil, "", fmt.Errorf("provider %q not registered (role %s): %w", providerName, role, err)
			}
			return provider, modelID, nil
		}
	}

	// Fall back to default provider + per-provider model config.
	providerName := r.cfg.Provider.Default
	model := modelForRole(r.cfg, role)

	provider, err := r.registry.Get(providerName)
	if err != nil {
		return nil, "", err
	}

	if model == "" {
		return nil, "", fmt.Errorf("model not configured for role %s", role)
	}

	return provider, model, nil
}

// parseRoleAssignment splits "providerName:modelID" into its two components.
func parseRoleAssignment(assignment string) (providerName, modelID string, err error) {
	idx := strings.Index(assignment, ":")
	if idx <= 0 {
		return "", "", fmt.Errorf("expected format providerName:modelID, got %q", assignment)
	}
	return strings.ToLower(strings.TrimSpace(assignment[:idx])), strings.TrimSpace(assignment[idx+1:]), nil
}

func modelForRole(cfg *config.Config, role Role) string {
	if cfg == nil {
		return ""
	}
	switch role {
	case RolePlanner:
		return cfg.Provider.OpenAI.Models.Planner
	case RoleCoder:
		return cfg.Provider.OpenAI.Models.Coder
	case RoleReviewer:
		return cfg.Provider.OpenAI.Models.Reviewer
	case RoleExplorer:
		if m := cfg.Provider.OpenAI.Models.Explorer; m != "" {
			return m
		}
		return cfg.Provider.OpenAI.Models.Planner
	case RoleOracle:
		if m := cfg.Provider.OpenAI.Models.Oracle; m != "" {
			return m
		}
		return cfg.Provider.OpenAI.Models.Planner
	case RoleFixer:
		if m := cfg.Provider.OpenAI.Models.Fixer; m != "" {
			return m
		}
		return cfg.Provider.OpenAI.Models.Planner
	default:
		return ""
	}
}
