package providers

import "context"

type Role string

const (
	RolePlanner  Role = "planner"
	RoleCoder    Role = "coder"
	RoleReviewer Role = "reviewer"
)

type ChatRequest struct {
	Role            Role
	Model           string
	SystemPrompt    string
	UserPrompt      string
	MaxTokens       int
	Temperature     float64
	ReasoningEffort string
}

type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type ChatResponse struct {
	Text             string
	FinishReason     string
	Usage            Usage
	ProviderMetadata map[string]string
}

type StreamEvent struct {
	Type     string
	Text     string
	Metadata map[string]string
}

type Provider interface {
	Name() string
	Validate(ctx context.Context) error
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, <-chan error)
}
