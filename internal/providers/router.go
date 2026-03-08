package providers

import (
	"fmt"

	"github.com/furkanbeydemir/orch/internal/config"
)

type Router struct {
	cfg      *config.Config
	registry *Registry
}

func NewRouter(cfg *config.Config, registry *Registry) *Router {
	return &Router{cfg: cfg, registry: registry}
}

func (r *Router) Resolve(role Role) (Provider, string, error) {
	if r == nil || r.cfg == nil || r.registry == nil {
		return nil, "", fmt.Errorf("provider router is not initialized")
	}

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
	default:
		return ""
	}
}
