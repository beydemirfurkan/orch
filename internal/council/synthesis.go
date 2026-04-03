package council

import (
	"context"
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
)

// parseDecision extracts an ACCEPT or REVISE decision from the first non-empty line.
func parseDecision(text string) (models.ReviewDecision, bool) {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(strings.ToLower(line))
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		switch strings.Trim(fields[0], ".,:;- ") {
		case "accept":
			return models.ReviewAccept, true
		case "revise":
			return models.ReviewRevise, true
		default:
			return "", false
		}
	}
	return "", false
}

// synthesiseMajority tallies weighted votes and returns a verdict without extra tokens.
func synthesiseMajority(votes []MemberVote) (*CouncilVerdict, error) {
	var acceptWeight, reviseWeight float64
	for _, v := range votes {
		w := v.Weight
		if w <= 0 {
			w = 1
		}
		if v.Decision == models.ReviewAccept {
			acceptWeight += w
		} else {
			reviseWeight += w
		}
	}

	total := acceptWeight + reviseWeight
	var decision models.ReviewDecision
	var confidence float64
	if acceptWeight > reviseWeight {
		decision = models.ReviewAccept
		confidence = acceptWeight / total
	} else {
		decision = models.ReviewRevise
		confidence = reviseWeight / total
	}

	consensus := confidence >= 0.75
	dissent := !consensus

	// Build a combined reasoning string.
	parts := make([]string, 0, len(votes))
	for _, v := range votes {
		parts = append(parts, fmt.Sprintf("[%s/%s → %s]", v.ProviderName, v.ModelID, v.Decision))
	}

	return &CouncilVerdict{
		Decision:    decision,
		Confidence:  confidence,
		Consensus:   consensus,
		Dissent:     dissent,
		MemberVotes: votes,
		Reasoning:   strings.Join(parts, " "),
	}, nil
}

// synthesiseMeta sends all member responses to a cheap synthesiser model for final verdict.
func synthesiseMeta(
	ctx context.Context,
	votes []MemberVote,
	originalPrompt string,
	synthChat func(context.Context, providers.ChatRequest) (providers.ChatResponse, error),
	maxTokens int,
) (*CouncilVerdict, error) {
	if synthChat == nil {
		// Fallback to majority if no synthesiser is configured.
		return synthesiseMajority(votes)
	}

	var b strings.Builder
	b.WriteString("You are a meta-reviewer. Below are reviews from a panel of senior engineers.\n")
	b.WriteString("Synthesise them into a single ACCEPT or REVISE decision with a one-paragraph rationale.\n\n")
	b.WriteString("Original review context:\n")
	b.WriteString(originalPrompt)
	b.WriteString("\n\nPanel reviews:\n")
	for i, v := range votes {
		b.WriteString(fmt.Sprintf("%d. [%s/%s] %s\n\n", i+1, v.ProviderName, v.ModelID, v.Reasoning))
	}
	b.WriteString("\nRespond with ACCEPT or REVISE on the first line, then your rationale.")

	if maxTokens <= 0 {
		maxTokens = 1024
	}

	resp, err := synthChat(ctx, providers.ChatRequest{
		Role:         providers.RoleReviewer,
		SystemPrompt: "You are a senior engineering meta-reviewer.",
		UserPrompt:   b.String(),
		MaxTokens:    maxTokens,
	})
	if err != nil {
		// Fallback to majority on synthesiser failure.
		return synthesiseMajority(votes)
	}

	decision, ok := parseDecision(resp.Text)
	if !ok {
		return synthesiseMajority(votes)
	}
	// Compute majority confidence for reference.
	var acceptW, reviseW float64
	for _, v := range votes {
		w := v.Weight
		if w <= 0 {
			w = 1
		}
		if v.Decision == models.ReviewAccept {
			acceptW += w
		} else {
			reviseW += w
		}
	}
	total := acceptW + reviseW
	var conf float64
	if decision == models.ReviewAccept {
		conf = acceptW / total
	} else {
		conf = reviseW / total
	}
	consensus := conf >= 0.75

	return &CouncilVerdict{
		Decision:    decision,
		Confidence:  conf,
		Consensus:   consensus,
		Dissent:     !consensus,
		MemberVotes: votes,
		Reasoning:   resp.Text,
	}, nil
}
