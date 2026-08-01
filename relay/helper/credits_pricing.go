package helper

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

// Prices in this file are the customer-facing Credits target: official USD
// list price multiplied by the approved category percentage. The conversion
// is exact because 1 USD = 20,000 centi-Credits (200 Credits).
const (
	gptTargetPercent     int64 = 5
	claudeTargetPercent  int64 = 22
	geminiTargetPercent  int64 = 6
	longContextGPTMin          = 272001
	longContextGeminiMin       = 200001
)

type creditsTextRate struct {
	inputCentiCredits        int64
	outputCentiCredits       int64
	cachedInputCentiCredits  int64
	cacheWriteCentiCredits   int64
	cacheWrite5mCentiCredits int64
	cacheWrite1hCentiCredits int64
	tiers                    []creditsTextRateTier
}

type creditsTextRateTier struct {
	label                    string
	minPromptTokens          int
	maxPromptTokens          int
	inputCentiCredits        int64
	outputCentiCredits       int64
	cachedInputCentiCredits  int64
	cacheWriteCentiCredits   int64
	cacheWrite5mCentiCredits int64
	cacheWrite1hCentiCredits int64
}

func quotaPerMillionFromCentiCredits(centiCredits int64) int64 {
	return centiCredits * int64(common.CreditsQuotaUnit) / 100
}

// targetCentiCredits converts a published USD/MTok price represented in
// milli-dollars into integer centi-Credits at the approved percentage.
func targetCentiCredits(officialMilliUSD, targetPercent int64) int64 {
	return officialMilliUSD * 20 * targetPercent / 100
}

func gptRate(input, cachedInput, cacheWrite, output int64) creditsTextRate {
	return creditsTextRate{
		inputCentiCredits:       targetCentiCredits(input, gptTargetPercent),
		cachedInputCentiCredits: targetCentiCredits(cachedInput, gptTargetPercent),
		cacheWriteCentiCredits:  targetCentiCredits(cacheWrite, gptTargetPercent),
		outputCentiCredits:      targetCentiCredits(output, gptTargetPercent),
	}
}

func claudeRate(input, cacheWrite5m, cacheWrite1h, cachedInput, output int64) creditsTextRate {
	return creditsTextRate{
		inputCentiCredits:        targetCentiCredits(input, claudeTargetPercent),
		cacheWrite5mCentiCredits: targetCentiCredits(cacheWrite5m, claudeTargetPercent),
		cacheWrite1hCentiCredits: targetCentiCredits(cacheWrite1h, claudeTargetPercent),
		cachedInputCentiCredits:  targetCentiCredits(cachedInput, claudeTargetPercent),
		outputCentiCredits:       targetCentiCredits(output, claudeTargetPercent),
	}
}

func geminiRate(input, cachedInput, output int64) creditsTextRate {
	return creditsTextRate{
		inputCentiCredits:       targetCentiCredits(input, geminiTargetPercent),
		cachedInputCentiCredits: targetCentiCredits(cachedInput, geminiTargetPercent),
		outputCentiCredits:      targetCentiCredits(output, geminiTargetPercent),
	}
}

func rateTier(label string, minPromptTokens, maxPromptTokens int, rate creditsTextRate) creditsTextRateTier {
	return creditsTextRateTier{
		label:                    label,
		minPromptTokens:          minPromptTokens,
		maxPromptTokens:          maxPromptTokens,
		inputCentiCredits:        rate.inputCentiCredits,
		outputCentiCredits:       rate.outputCentiCredits,
		cachedInputCentiCredits:  rate.cachedInputCentiCredits,
		cacheWriteCentiCredits:   rate.cacheWriteCentiCredits,
		cacheWrite5mCentiCredits: rate.cacheWrite5mCentiCredits,
		cacheWrite1hCentiCredits: rate.cacheWrite1hCentiCredits,
	}
}

func withContextTiers(short, long creditsTextRate, minPromptTokens int) creditsTextRate {
	short.tiers = []creditsTextRateTier{
		rateTier("short", 0, minPromptTokens-1, short),
		rateTier("long", minPromptTokens, 0, long),
	}
	return short
}

func pricingFromRate(rate creditsTextRate) *types.CreditsTextPricing {
	pricing := &types.CreditsTextPricing{
		InputQuotaPerMillion:        quotaPerMillionFromCentiCredits(rate.inputCentiCredits),
		OutputQuotaPerMillion:       quotaPerMillionFromCentiCredits(rate.outputCentiCredits),
		CachedInputQuotaPerMillion:  quotaPerMillionFromCentiCredits(rate.cachedInputCentiCredits),
		CacheWriteQuotaPerMillion:   quotaPerMillionFromCentiCredits(rate.cacheWriteCentiCredits),
		CacheWrite5mQuotaPerMillion: quotaPerMillionFromCentiCredits(rate.cacheWrite5mCentiCredits),
		CacheWrite1hQuotaPerMillion: quotaPerMillionFromCentiCredits(rate.cacheWrite1hCentiCredits),
		PricingSource:               "geili",
	}
	if len(rate.tiers) == 0 {
		return pricing
	}
	pricing.Tiers = make([]types.CreditsTextPricingTier, 0, len(rate.tiers))
	for _, tier := range rate.tiers {
		pricing.Tiers = append(pricing.Tiers, types.CreditsTextPricingTier{
			Label:                       tier.label,
			MinPromptTokens:             tier.minPromptTokens,
			MaxPromptTokens:             tier.maxPromptTokens,
			InputQuotaPerMillion:        quotaPerMillionFromCentiCredits(tier.inputCentiCredits),
			OutputQuotaPerMillion:       quotaPerMillionFromCentiCredits(tier.outputCentiCredits),
			CachedInputQuotaPerMillion:  quotaPerMillionFromCentiCredits(tier.cachedInputCentiCredits),
			CacheWriteQuotaPerMillion:   quotaPerMillionFromCentiCredits(tier.cacheWriteCentiCredits),
			CacheWrite5mQuotaPerMillion: quotaPerMillionFromCentiCredits(tier.cacheWrite5mCentiCredits),
			CacheWrite1hQuotaPerMillion: quotaPerMillionFromCentiCredits(tier.cacheWrite1hCentiCredits),
		})
	}
	return pricing
}

func creditsV1TextPricing(modelName string) (*types.CreditsTextPricing, bool) {
	if !common.CreditsV1Enabled() {
		return nil, false
	}
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	var rate creditsTextRate
	switch {
	case normalized == "gpt-5.2" || strings.HasPrefix(normalized, "gpt-5.2-") && !strings.Contains(normalized, "pro"):
		rate = gptRate(1750, 175, 0, 14000)
	case normalized == "gpt-5.2-pro" || strings.HasPrefix(normalized, "gpt-5.2-pro-"):
		rate = gptRate(21000, 0, 0, 168000)
	case strings.HasPrefix(normalized, "gpt-5.3-codex"):
		rate = gptRate(1750, 175, 0, 14000)
	case normalized == "gpt-5.4" || strings.HasPrefix(normalized, "gpt-5.4-2026-"):
		rate = withContextTiers(
			gptRate(2500, 250, 0, 15000),
			gptRate(5000, 500, 0, 22500),
			longContextGPTMin,
		)
	case normalized == "gpt-5.4-mini" || strings.HasPrefix(normalized, "gpt-5.4-mini-"):
		rate = gptRate(750, 75, 0, 4500)
	case normalized == "gpt-5.4-nano" || strings.HasPrefix(normalized, "gpt-5.4-nano-"):
		rate = gptRate(200, 20, 0, 1250)
	case normalized == "gpt-5.4-pro" || strings.HasPrefix(normalized, "gpt-5.4-pro-"):
		rate = withContextTiers(
			gptRate(30000, 0, 0, 180000),
			gptRate(60000, 0, 0, 270000),
			longContextGPTMin,
		)
	case normalized == "gpt-5.5" || strings.HasPrefix(normalized, "gpt-5.5-2026-"):
		rate = withContextTiers(
			gptRate(5000, 500, 0, 30000),
			gptRate(10000, 1000, 0, 45000),
			longContextGPTMin,
		)
	case normalized == "gpt-5.5-pro" || strings.HasPrefix(normalized, "gpt-5.5-pro-"):
		rate = withContextTiers(
			gptRate(30000, 0, 0, 180000),
			gptRate(60000, 0, 0, 270000),
			longContextGPTMin,
		)
	case normalized == "gpt-5.6-sol":
		rate = withContextTiers(
			gptRate(5000, 500, 6250, 30000),
			gptRate(10000, 1000, 12500, 45000),
			longContextGPTMin,
		)
	case normalized == "gpt-5.6-terra":
		rate = withContextTiers(
			gptRate(2000, 200, 2500, 12000),
			gptRate(4000, 400, 5000, 18000),
			longContextGPTMin,
		)
	case normalized == "gpt-5.6-luna":
		rate = withContextTiers(
			gptRate(200, 20, 250, 1200),
			gptRate(400, 40, 500, 1800),
			longContextGPTMin,
		)
	case normalized == "gemini-2.5-flash" || normalized == "gemini-2.5-flash-nothinking":
		rate = geminiRate(300, 30, 2500)
	case normalized == "gemini-2.5-flash-lite":
		rate = geminiRate(100, 10, 400)
	case normalized == "gemini-2.5-pro" || strings.HasPrefix(normalized, "gemini-2.5-pro-"):
		rate = withContextTiers(
			geminiRate(1250, 125, 10000),
			geminiRate(2500, 250, 15000),
			longContextGeminiMin,
		)
	case normalized == "gemini-3-flash-preview" || normalized == "gemini-3-flash":
		rate = geminiRate(500, 50, 3000)
	case normalized == "gemini-3.1-flash-lite" || normalized == "gemini-3.1-flash-lite-preview":
		rate = geminiRate(250, 25, 1500)
	case normalized == "gemini-3.1-pro-preview" || normalized == "gemini-3.1-pro" || normalized == "gemini-3.1-pro-preview-low":
		rate = withContextTiers(
			geminiRate(2000, 200, 12000),
			geminiRate(4000, 400, 18000),
			longContextGeminiMin,
		)
	case normalized == "gemini-3.5-flash":
		rate = geminiRate(1500, 150, 9000)
	case normalized == "gemini-3.5-flash-lite":
		rate = geminiRate(300, 30, 2500)
	case normalized == "gemini-3.6-flash":
		rate = geminiRate(1500, 150, 7500)
	case isCreditsClaudeAlias(normalized, "claude-opus-4-5") ||
		isCreditsClaudeAlias(normalized, "claude-opus-4-6") ||
		isCreditsClaudeAlias(normalized, "claude-opus-4-7") ||
		isCreditsClaudeAlias(normalized, "claude-opus-4-8") ||
		isCreditsClaudeAlias(normalized, "claude-opus-5"):
		rate = claudeRate(5000, 6250, 10000, 500, 25000)
	case isCreditsClaudeAlias(normalized, "claude-sonnet-4-5") ||
		isCreditsClaudeAlias(normalized, "claude-sonnet-4-6"):
		rate = claudeRate(3000, 3750, 6000, 300, 15000)
	case isCreditsClaudeAlias(normalized, "claude-sonnet-5"):
		// Anthropic's introductory $2/$10 price is valid through 2026-08-31.
		rate = claudeRate(2000, 2500, 4000, 200, 10000)
	case isCreditsClaudeAlias(normalized, "claude-haiku-4-5"):
		rate = claudeRate(1000, 1250, 2000, 100, 5000)
	case isCreditsClaudeAlias(normalized, "claude-fable-5"):
		rate = claudeRate(10000, 12500, 20000, 1000, 50000)
	default:
		return nil, false
	}
	return pricingFromRate(rate), true
}

// CreditsV1TextPricingForPublicCatalog exposes the same immutable official
// target used by settlement. Public catalog pricing must not maintain a
// second table.
func CreditsV1TextPricingForPublicCatalog(modelName string) (*types.CreditsTextPricing, bool) {
	return creditsV1TextPricing(modelName)
}

func isCreditsClaudeAlias(modelName, base string) bool {
	if modelName == base {
		return true
	}
	for _, suffix := range []string{"-thinking", "-max", "-xhigh", "-high", "-medium", "-low"} {
		if modelName == base+suffix {
			return true
		}
	}
	// Anthropic publishes dated model IDs such as -20251001. Keep the
	// customer-facing aliases and dated channel IDs on one price.
	prefix := base + "-"
	if !strings.HasPrefix(modelName, prefix) {
		return false
	}
	suffix := strings.TrimPrefix(modelName, prefix)
	if len(suffix) != 8 {
		return false
	}
	for _, ch := range suffix {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
