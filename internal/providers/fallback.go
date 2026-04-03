package providers

import (
	"context"
	"fmt"
	"strings"
)

// FallbackProvider wraps an ordered chain of providers. On rate-limit or auth
// errors it advances to the next provider in the chain. The first successful
// response wins. Register it as a named provider in the registry like any other.
type FallbackProvider struct {
	name  string
	chain []Provider
}

// NewFallbackProvider creates a FallbackProvider with the given name and chain.
// At least one provider must be supplied.
func NewFallbackProvider(name string, chain []Provider) (*FallbackProvider, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("fallback provider name cannot be empty")
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("fallback chain must have at least one provider")
	}
	return &FallbackProvider{name: name, chain: chain}, nil
}

func (f *FallbackProvider) Name() string { return f.name }

func (f *FallbackProvider) Validate(ctx context.Context) error {
	var errs []string
	for _, p := range f.chain {
		if err := p.Validate(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
		}
	}
	if len(errs) == len(f.chain) {
		return fmt.Errorf("all providers in fallback chain failed validation: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (f *FallbackProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	var lastErr error
	for _, p := range f.chain {
		resp, err := p.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryableError(err) {
			return ChatResponse{}, err
		}
	}
	return ChatResponse{}, fmt.Errorf("all providers in fallback chain failed: %w", lastErr)
}

func (f *FallbackProvider) Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, <-chan error) {
	events := make(chan StreamEvent, 1)
	errs := make(chan error, 1)
	go func() {
		defer close(events)
		defer close(errs)
		resp, err := f.Chat(ctx, req)
		if err != nil {
			errs <- err
			return
		}
		events <- StreamEvent{Type: "text", Text: resp.Text}
	}()
	return events, errs
}

// isRetryableError returns true for rate-limit (429) and server (5xx) errors,
// which warrant trying the next provider in the chain.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "http 5") ||
		strings.Contains(msg, "http 503") ||
		strings.Contains(msg, "http 502")
}
