package providers

import (
	"context"
	"testing"

	"github.com/furkanbeydemir/orch/internal/config"
)

type fakeProvider struct{ name string }

func (f fakeProvider) Name() string { return f.name }

func (f fakeProvider) Validate(ctx context.Context) error { return nil }

func (f fakeProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	return ChatResponse{Text: "ok"}, nil
}

func (f fakeProvider) Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, <-chan error) {
	ev := make(chan StreamEvent)
	err := make(chan error)
	close(ev)
	close(err)
	return ev, err
}

func TestRegistryGetProvider(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeProvider{name: "openai"})

	got, err := reg.Get("openai")
	if err != nil {
		t.Fatalf("get provider: %v", err)
	}
	if got.Name() != "openai" {
		t.Fatalf("unexpected provider: %s", got.Name())
	}
}

func TestRouterResolveRoleModel(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry()
	reg.Register(fakeProvider{name: "openai"})

	router := NewRouter(cfg, reg)
	provider, model, err := router.Resolve(RoleCoder)
	if err != nil {
		t.Fatalf("resolve route: %v", err)
	}
	if provider.Name() != "openai" {
		t.Fatalf("unexpected provider: %s", provider.Name())
	}
	if model != cfg.Provider.OpenAI.Models.Coder {
		t.Fatalf("unexpected model: got=%s want=%s", model, cfg.Provider.OpenAI.Models.Coder)
	}
}
