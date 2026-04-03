package council

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
)

func TestParseDecisionStrictFirstLine(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		want   models.ReviewDecision
		parsed bool
	}{
		{
			name:   "acceptWithReasoning",
			text:   "ACCEPT: patch looks good\nReasoning...",
			want:   models.ReviewAccept,
			parsed: true,
		},
		{
			name:   "acceptWithPeriod",
			text:   "ACCEPT. patch looks good",
			want:   models.ReviewAccept,
			parsed: true,
		},
		{
			name:   "reviseLowercase",
			text:   "revise - missing tests",
			want:   models.ReviewRevise,
			parsed: true,
		},
		{
			name:   "skipBlankLinesUntilDecision",
			text:   "\n\n  ACCEPT\nreasoning",
			want:   models.ReviewAccept,
			parsed: true,
		},
		{
			name:   "rejectWhenFirstLineIsNotDecision",
			text:   "Here is my verdict:\nACCEPT",
			parsed: false,
		},
		{
			name:   "malformedOutput",
			text:   "APPROVE",
			parsed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, ok := parseDecision(tt.text)
			if ok != tt.parsed {
				t.Fatalf("parseDecision parsed=%v want=%v", ok, tt.parsed)
			}
			if ok && decision != tt.want {
				t.Fatalf("parseDecision decision=%v want=%v", decision, tt.want)
			}
		})
	}
}

func TestSynthesiseMajorityTieDefaultsToRevise(t *testing.T) {
	verdict, err := synthesiseMajority([]MemberVote{
		{ProviderName: "p1", ModelID: "m1", Decision: models.ReviewAccept, Weight: 1},
		{ProviderName: "p2", ModelID: "m2", Decision: models.ReviewRevise, Weight: 1},
	})
	if err != nil {
		t.Fatalf("synthesiseMajority error: %v", err)
	}
	if verdict.Decision != models.ReviewRevise {
		t.Fatalf("decision=%v want=%v", verdict.Decision, models.ReviewRevise)
	}
	if verdict.Consensus {
		t.Fatalf("expected no consensus on tie")
	}
}

func TestSynthesiseMajorityConfidenceAndConsensus(t *testing.T) {
	tests := []struct {
		name          string
		votes         []MemberVote
		wantDecision  models.ReviewDecision
		wantConf      float64
		wantConsensus bool
	}{
		{
			name: "strongConsensus",
			votes: []MemberVote{
				{ProviderName: "p1", ModelID: "m1", Decision: models.ReviewAccept, Weight: 2},
				{ProviderName: "p2", ModelID: "m2", Decision: models.ReviewAccept, Weight: 1},
				{ProviderName: "p3", ModelID: "m3", Decision: models.ReviewRevise, Weight: 1},
			},
			wantDecision:  models.ReviewAccept,
			wantConf:      0.75,
			wantConsensus: true,
		},
		{
			name: "dissent",
			votes: []MemberVote{
				{ProviderName: "p1", ModelID: "m1", Decision: models.ReviewAccept, Weight: 1},
				{ProviderName: "p2", ModelID: "m2", Decision: models.ReviewRevise, Weight: 2},
				{ProviderName: "p3", ModelID: "m3", Decision: models.ReviewRevise, Weight: 0}, // defaults to 1
			},
			wantDecision:  models.ReviewRevise,
			wantConf:      0.75,
			wantConsensus: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict, err := synthesiseMajority(tt.votes)
			if err != nil {
				t.Fatalf("synthesiseMajority error: %v", err)
			}
			if verdict.Decision != tt.wantDecision {
				t.Fatalf("decision=%v want=%v", verdict.Decision, tt.wantDecision)
			}
			if math.Abs(verdict.Confidence-tt.wantConf) > 1e-9 {
				t.Fatalf("confidence=%.4f want=%.4f", verdict.Confidence, tt.wantConf)
			}
			if verdict.Consensus != tt.wantConsensus {
				t.Fatalf("consensus=%v want=%v", verdict.Consensus, tt.wantConsensus)
			}
			if verdict.Dissent != !tt.wantConsensus {
				t.Fatalf("dissent=%v want=%v", verdict.Dissent, !tt.wantConsensus)
			}
			if len(verdict.MemberVotes) != len(tt.votes) {
				t.Fatalf("member votes copied incorrectly: got=%d want=%d", len(verdict.MemberVotes), len(tt.votes))
			}
		})
	}
}

func TestSynthesiseMetaFallsBackOnChatFailure(t *testing.T) {
	votes := []MemberVote{
		{ProviderName: "p1", ModelID: "m1", Decision: models.ReviewAccept, Weight: 1, Reasoning: "ACCEPT", Usage: providers.Usage{}},
		{ProviderName: "p2", ModelID: "m2", Decision: models.ReviewRevise, Weight: 2, Reasoning: "REVISE", Usage: providers.Usage{}},
	}
	majority, err := synthesiseMajority(votes)
	if err != nil {
		t.Fatalf("majority fallback prep error: %v", err)
	}
	called := 0
	verdict, err := synthesiseMeta(context.Background(), votes, "prompt", func(context.Context, providers.ChatRequest) (providers.ChatResponse, error) {
		called++
		return providers.ChatResponse{}, errors.New("chat failure")
	}, 128)
	if err != nil {
		t.Fatalf("synthesiseMeta error: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected synth chat to be invoked once, got %d", called)
	}
	assertVerdictEquals(t, verdict, majority)
}

func TestSynthesiseMetaFallsBackOnMalformedOutput(t *testing.T) {
	votes := []MemberVote{
		{ProviderName: "p1", ModelID: "m1", Decision: models.ReviewAccept, Weight: 3, Reasoning: "ACCEPT"},
		{ProviderName: "p2", ModelID: "m2", Decision: models.ReviewRevise, Weight: 1, Reasoning: "REVISE"},
	}
	majority, err := synthesiseMajority(votes)
	if err != nil {
		t.Fatalf("majority fallback prep error: %v", err)
	}
	verdict, err := synthesiseMeta(context.Background(), votes, "prompt", func(context.Context, providers.ChatRequest) (providers.ChatResponse, error) {
		return providers.ChatResponse{Text: "Maybe accept after more tests"}, nil
	}, 256)
	if err != nil {
		t.Fatalf("synthesiseMeta error: %v", err)
	}
	assertVerdictEquals(t, verdict, majority)
}

func assertVerdictEquals(t *testing.T, got, want *CouncilVerdict) {
	t.Helper()
	if got == nil || want == nil {
		t.Fatalf("verdicts must be non-nil")
	}
	if got.Decision != want.Decision {
		t.Fatalf("decision=%v want=%v", got.Decision, want.Decision)
	}
	if math.Abs(got.Confidence-want.Confidence) > 1e-9 {
		t.Fatalf("confidence mismatch got=%.4f want=%.4f", got.Confidence, want.Confidence)
	}
	if got.Consensus != want.Consensus {
		t.Fatalf("consensus=%v want=%v", got.Consensus, want.Consensus)
	}
	if got.Dissent != want.Dissent {
		t.Fatalf("dissent=%v want=%v", got.Dissent, want.Dissent)
	}
	if got.Reasoning != want.Reasoning {
		t.Fatalf("reasoning mismatch got=%q want=%q", got.Reasoning, want.Reasoning)
	}
}
