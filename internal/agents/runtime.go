package agents

import (
	"context"

	"github.com/furkanbeydemir/orch/internal/providers"
)

type LLMRuntime struct {
	Router *providers.Router
}

func (r *LLMRuntime) Chat(ctx context.Context, req providers.ChatRequest) (providers.ChatResponse, error) {
	if r == nil || r.Router == nil {
		return providers.ChatResponse{}, nil
	}

	provider, model, err := r.Router.Resolve(req.Role)
	if err != nil {
		return providers.ChatResponse{}, err
	}

	req.Model = model
	return provider.Chat(ctx, req)
}
