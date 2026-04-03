package providers

import "strings"

// modelPricing holds per-token pricing in USD per 1M tokens.
type modelPricing struct {
	InputPerM  float64
	OutputPerM float64
}

// pricingTable maps model ID substrings (lowercased) to pricing.
// Prices are approximate and should be updated as providers change rates.
var pricingTable = []struct {
	match   string
	pricing modelPricing
}{
	// OpenAI
	{"gpt-5.3-codex", modelPricing{InputPerM: 15.00, OutputPerM: 60.00}},
	{"o3", modelPricing{InputPerM: 10.00, OutputPerM: 40.00}},
	{"gpt-4o-mini", modelPricing{InputPerM: 0.15, OutputPerM: 0.60}},
	{"gpt-4o", modelPricing{InputPerM: 5.00, OutputPerM: 15.00}},
	{"gpt-4", modelPricing{InputPerM: 30.00, OutputPerM: 60.00}},
	{"gpt-3.5", modelPricing{InputPerM: 0.50, OutputPerM: 1.50}},
	// Anthropic
	{"claude-opus-4", modelPricing{InputPerM: 15.00, OutputPerM: 75.00}},
	{"claude-sonnet-4", modelPricing{InputPerM: 3.00, OutputPerM: 15.00}},
	{"claude-haiku-4", modelPricing{InputPerM: 0.80, OutputPerM: 4.00}},
	{"claude-opus", modelPricing{InputPerM: 15.00, OutputPerM: 75.00}},
	{"claude-sonnet", modelPricing{InputPerM: 3.00, OutputPerM: 15.00}},
	{"claude-haiku", modelPricing{InputPerM: 0.25, OutputPerM: 1.25}},
	// Generic fallback for unknown models
	{"", modelPricing{InputPerM: 5.00, OutputPerM: 15.00}},
}

// EstimateCostUSD returns an approximate USD cost for the given token counts and model ID.
// Uses a static pricing table; the empty-string entry acts as a catch-all fallback.
func EstimateCostUSD(modelID string, inputTokens, outputTokens int) float64 {
	lower := strings.ToLower(strings.TrimSpace(modelID))
	var p modelPricing
	for _, entry := range pricingTable {
		if entry.match == "" || strings.Contains(lower, entry.match) {
			p = entry.pricing
			break
		}
	}
	if p.InputPerM == 0 && p.OutputPerM == 0 {
		return 0
	}
	inputCost := float64(inputTokens) / 1_000_000 * p.InputPerM
	outputCost := float64(outputTokens) / 1_000_000 * p.OutputPerM
	return inputCost + outputCost
}
