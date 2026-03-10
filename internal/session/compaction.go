package session

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/furkanbeydemir/orch/internal/storage"
)

type TokenBudget struct {
	ContextLimit   int
	ReservedOutput int
	SafetyMargin   float64
}

func (b TokenBudget) UsableInput() int {
	usable := b.ContextLimit - b.ReservedOutput
	if usable < 0 {
		return 0
	}
	return usable
}

func ResolveBudget(modelID string) TokenBudget {
	model := strings.ToLower(strings.TrimSpace(modelID))
	switch {
	case strings.Contains(model, "gpt-5"):
		return TokenBudget{ContextLimit: 200000, ReservedOutput: 16000, SafetyMargin: 0.12}
	case strings.Contains(model, "gpt-4"):
		return TokenBudget{ContextLimit: 128000, ReservedOutput: 12000, SafetyMargin: 0.15}
	default:
		return TokenBudget{ContextLimit: 64000, ReservedOutput: 8000, SafetyMargin: 0.18}
	}
}

type modelTokenProfile struct {
	CharPerToken float64
	BaseOverhead int
	RoleOverhead int
	PartOverhead map[string]int
}

func resolveTokenProfile(modelID string) modelTokenProfile {
	model := strings.ToLower(strings.TrimSpace(modelID))
	base := modelTokenProfile{
		CharPerToken: 4.0,
		BaseOverhead: 8,
		RoleOverhead: 2,
		PartOverhead: map[string]int{
			"text":       4,
			"tool":       6,
			"stage":      5,
			"compaction": 8,
			"error":      4,
			"file":       4,
		},
	}
	if strings.Contains(model, "gpt-5") {
		base.CharPerToken = 3.7
		base.BaseOverhead = 10
	}
	if strings.Contains(model, "gpt-4") {
		base.CharPerToken = 3.9
		base.BaseOverhead = 10
	}
	return base
}

func EstimateTokens(messages []MessageWithParts, modelID string) int {
	profile := resolveTokenProfile(modelID)
	totalTokens := 0
	for _, message := range messages {
		totalTokens += profile.BaseOverhead
		totalTokens += profile.RoleOverhead
		totalTokens += charsToTokens(len(strings.TrimSpace(message.Message.Role)), profile.CharPerToken)
		totalTokens += charsToTokens(len(strings.TrimSpace(message.Message.Error)), profile.CharPerToken)
		for _, part := range message.Parts {
			partType := strings.ToLower(strings.TrimSpace(part.Type))
			totalTokens += profile.PartOverhead[partType]
			totalTokens += charsToTokens(len(strings.TrimSpace(part.Payload)), profile.CharPerToken)
		}
	}
	if totalTokens < 0 {
		return 0
	}
	if totalTokens == 0 {
		return 1
	}
	return totalTokens
}

func charsToTokens(chars int, charPerToken float64) int {
	if chars <= 0 {
		return 0
	}
	if charPerToken <= 0 {
		charPerToken = 4.0
	}
	return int(math.Ceil(float64(chars) / charPerToken))
}

func (s *Service) MaybeCompact(sessionID, modelID string) (bool, string, error) {
	if s == nil || s.store == nil {
		return false, "", fmt.Errorf("session service is not initialized")
	}

	messages, err := s.ListMessagesWithParts(sessionID, 500)
	if err != nil {
		return false, "", err
	}
	if len(messages) == 0 {
		return false, "", nil
	}

	budget := ResolveBudget(modelID)
	estimated := EstimateTokens(messages, modelID)
	threshold := int(float64(budget.UsableInput()) * (1.0 - budget.SafetyMargin))
	if threshold < 1 {
		threshold = 1
	}
	if estimated < threshold {
		return false, "", nil
	}

	summary := summarizeForCompaction(messages, estimated, budget)
	summaryPayload, err := json.Marshal(map[string]any{
		"estimated_tokens": estimated,
		"usable_input":     budget.UsableInput(),
		"threshold":        threshold,
		"safety_margin":    budget.SafetyMargin,
		"summary":          summary,
	})
	if err != nil {
		return false, "", fmt.Errorf("serialize compaction payload: %w", err)
	}

	parts := []storage.SessionPart{{Type: "compaction", Payload: string(summaryPayload)}}
	if _, err := s.AppendMessage(MessageInput{
		SessionID:    sessionID,
		Role:         "assistant",
		FinishReason: "compacted",
		Text:         summary,
	}, parts); err != nil {
		return false, "", err
	}

	affected, err := s.store.CompactSessionParts(sessionID, 12)
	if err != nil {
		return false, "", err
	}
	if err := s.store.UpsertSessionSummary(sessionID, summary); err != nil {
		return false, "", err
	}

	message := fmt.Sprintf("session compaction applied (tokens=%d, threshold=%d, compacted_parts=%d)", estimated, threshold, affected)
	return true, message, nil
}

func summarizeForCompaction(messages []MessageWithParts, estimated int, budget TokenBudget) string {
	files := collectRelevantPaths(messages, 8)
	recent := collectRecentTexts(messages, 3, 220)

	lines := []string{
		"## Goal",
		"Continue the conversation with reduced context size.",
		"",
		"## Instructions",
		"Preserve user intent, prioritize latest requests, and avoid reprocessing compacted raw outputs.",
		"",
		"## Discoveries",
		fmt.Sprintf("Estimated context tokens reached %d (usable budget %d).", estimated, budget.UsableInput()),
	}
	if len(recent) > 0 {
		lines = append(lines, "Recent non-empty snippets:")
		for _, snippet := range recent {
			lines = append(lines, "- "+snippet)
		}
	}

	lines = append(lines,
		"",
		"## Accomplished",
		"Older session parts were compacted and replaced with short placeholders.",
		"",
		"## Next",
		"Continue from the latest user request while relying on this summary and recent messages.",
	)

	if len(files) > 0 {
		lines = append(lines,
			"",
			"## Relevant files/directories",
		)
		for _, file := range files {
			lines = append(lines, "- "+file)
		}
	}

	return strings.Join(lines, "\n")
}

func collectRecentTexts(messages []MessageWithParts, maxCount, maxLen int) []string {
	if maxCount <= 0 {
		return []string{}
	}
	out := make([]string, 0, maxCount)
	for i := len(messages) - 1; i >= 0 && len(out) < maxCount; i-- {
		for _, part := range messages[i].Parts {
			text := strings.TrimSpace(ExtractTextPart(part))
			if text == "" {
				continue
			}
			if len(text) > maxLen {
				text = text[:maxLen] + "..."
			}
			out = append(out, text)
			break
		}
	}
	return out
}

var pathPattern = regexp.MustCompile(`(?m)([A-Za-z0-9_./-]+\.[A-Za-z0-9]{1,8}|[A-Za-z0-9_./-]+/)`)

func collectRelevantPaths(messages []MessageWithParts, maxCount int) []string {
	seen := map[string]struct{}{}
	paths := make([]string, 0, maxCount)
	for i := len(messages) - 1; i >= 0 && len(paths) < maxCount; i-- {
		for _, part := range messages[i].Parts {
			payload := strings.TrimSpace(part.Payload)
			if payload == "" {
				continue
			}
			matches := pathPattern.FindAllString(payload, -1)
			for _, match := range matches {
				candidate := strings.TrimSpace(match)
				if candidate == "" || strings.HasPrefix(candidate, "http") {
					continue
				}
				if _, ok := seen[candidate]; ok {
					continue
				}
				seen[candidate] = struct{}{}
				paths = append(paths, candidate)
				if len(paths) >= maxCount {
					break
				}
			}
			if len(paths) >= maxCount {
				break
			}
		}
	}
	sort.Strings(paths)
	return paths
}
