// Package council implements a multi-model consensus mechanism for high-stakes reviews.
package council

import (
	"context"
	"fmt"
	"sync"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
)

// MemberVote captures a single council member's verdict.
type MemberVote struct {
	ProviderName string
	ModelID      string
	Decision     models.ReviewDecision
	Reasoning    string
	Weight       float64
	Usage        providers.Usage
}

// CouncilVerdict is the aggregated result of a council deliberation.
type CouncilVerdict struct {
	Decision    models.ReviewDecision
	Confidence  float64
	Consensus   bool
	Dissent     bool
	MemberVotes []MemberVote
	Reasoning   string
}

// CouncilMember describes a single participant in the council.
type CouncilMember struct {
	ProviderName string
	ModelID      string
	Weight       float64
	// Chat is a function that sends a prompt and returns the response text.
	// The orchestrator wires this up from its provider registry.
	Chat func(ctx context.Context, req providers.ChatRequest) (providers.ChatResponse, error)
}

// Council fans a review prompt out to multiple LLM members and synthesises a verdict.
type Council struct {
	Members              []CouncilMember
	SynthesisMode        string // "majority" or "meta"
	SynthesizerChat      func(ctx context.Context, req providers.ChatRequest) (providers.ChatResponse, error)
	MaxTokens            int
	MinSuccessfulMembers int
}

// Deliberate runs all council members concurrently and returns a synthesised verdict.
func (c *Council) Deliberate(ctx context.Context, prompt string) (*CouncilVerdict, error) {
	if len(c.Members) == 0 {
		return nil, fmt.Errorf("council has no members")
	}

	type result struct {
		vote MemberVote
		err  error
	}

	results := make([]result, len(c.Members))
	var wg sync.WaitGroup

	for i, member := range c.Members {
		wg.Add(1)
		go func(idx int, m CouncilMember) {
			defer wg.Done()
			if m.Chat == nil {
				results[idx] = result{err: fmt.Errorf("member %s has no chat function", m.ProviderName)}
				return
			}
			maxTok := c.MaxTokens
			if maxTok <= 0 {
				maxTok = 2048
			}
			resp, err := m.Chat(ctx, providers.ChatRequest{
				Role:         providers.RoleReviewer,
				SystemPrompt: "You are a senior code reviewer on a consensus panel. Respond with ACCEPT or REVISE on the first line, followed by brief reasoning grounded in the patch and validation context.",
				UserPrompt:   prompt,
				MaxTokens:    maxTok,
			})
			if err != nil {
				results[idx] = result{err: fmt.Errorf("member %s call failed: %w", m.ProviderName, err)}
				return
			}
			decision, ok := parseDecision(resp.Text)
			if !ok {
				results[idx] = result{err: fmt.Errorf("member %s returned unparseable decision", m.ProviderName)}
				return
			}
			results[idx] = result{vote: MemberVote{
				ProviderName: m.ProviderName,
				ModelID:      m.ModelID,
				Decision:     decision,
				Reasoning:    resp.Text,
				Weight:       m.Weight,
				Usage:        resp.Usage,
			}}
		}(i, member)
	}

	wg.Wait()

	var votes []MemberVote
	var errs []string
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err.Error())
			continue
		}
		votes = append(votes, r.vote)
	}

	minSuccessful := c.MinSuccessfulMembers
	if minSuccessful <= 0 {
		minSuccessful = len(c.Members)/2 + 1
	}

	if len(votes) < minSuccessful {
		return nil, fmt.Errorf("insufficient council quorum: got %d successful vote(s), need %d; errors=%v", len(votes), minSuccessful, errs)
	}

	// Require at least one successful vote.
	if len(votes) == 0 {
		return nil, fmt.Errorf("all council members failed: %v", errs)
	}

	switch c.SynthesisMode {
	case "meta":
		return synthesiseMeta(ctx, votes, prompt, c.SynthesizerChat, c.MaxTokens)
	default:
		return synthesiseMajority(votes)
	}
}
