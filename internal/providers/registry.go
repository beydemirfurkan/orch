package providers

import (
	"fmt"
	"strings"
)

type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
	if r == nil || p == nil {
		return
	}
	r.providers[strings.ToLower(p.Name())] = p
}

func (r *Registry) Get(name string) (Provider, error) {
	if r == nil {
		return nil, fmt.Errorf("provider registry is nil")
	}
	p, ok := r.providers[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return p, nil
}
