package helper

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

type creditsTextRate struct {
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

func creditsV1TextPricing(modelName string) (*types.CreditsTextPricing, bool) {
	if !common.CreditsV1Enabled() {
		return nil, false
	}
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	var rate creditsTextRate
	switch {
	case normalized == "gpt-5.4" || strings.HasPrefix(normalized, "gpt-5.4-2026-"):
		rate = creditsTextRate{inputCentiCredits: 7000, outputCentiCredits: 56000}
	case normalized == "gpt-5.5" || strings.HasPrefix(normalized, "gpt-5.5-2026-"):
		rate = creditsTextRate{cachedInputCentiCredits: 1400, inputCentiCredits: 14000, outputCentiCredits: 84000}
	case normalized == "gpt-5.6-sol":
		rate = creditsTextRate{
			inputCentiCredits: 28000, outputCentiCredits: 168000,
			cachedInputCentiCredits: 2800, cacheWriteCentiCredits: 35000,
		}
	case normalized == "gpt-5.6-terra":
		rate = creditsTextRate{
			inputCentiCredits: 14000, outputCentiCredits: 84000,
			cachedInputCentiCredits: 1400, cacheWriteCentiCredits: 17500,
		}
	case normalized == "gpt-5.6-luna":
		rate = creditsTextRate{
			inputCentiCredits: 5600, outputCentiCredits: 33600,
			cachedInputCentiCredits: 560, cacheWriteCentiCredits: 7000,
		}
	case normalized == "gemini-2.5-flash":
		rate = creditsTextRate{inputCentiCredits: 900, outputCentiCredits: 7500}
	case normalized == "gemini-3.1-pro-preview":
		rate = creditsTextRate{inputCentiCredits: 10000, outputCentiCredits: 70000}
	case isCreditsClaudeAlias(normalized, "claude-opus-4-6"):
		rate = creditsTextRate{
			inputCentiCredits: 28500, outputCentiCredits: 143000,
			cachedInputCentiCredits: 2850, cacheWrite5mCentiCredits: 35625, cacheWrite1hCentiCredits: 57000,
		}
	case isCreditsClaudeAlias(normalized, "claude-opus-4-8"):
		rate = creditsTextRate{
			inputCentiCredits: 40000, outputCentiCredits: 200000,
			cachedInputCentiCredits: 4000, cacheWrite5mCentiCredits: 50000, cacheWrite1hCentiCredits: 80000,
		}
	case isCreditsClaudeAlias(normalized, "claude-opus-5"):
		rate = creditsTextRate{
			inputCentiCredits: 40000, outputCentiCredits: 200000,
			cachedInputCentiCredits: 4000, cacheWrite5mCentiCredits: 50000, cacheWrite1hCentiCredits: 80000,
		}
	case isCreditsClaudeAlias(normalized, "claude-sonnet-4-6"):
		rate = creditsTextRate{
			inputCentiCredits: 17000, outputCentiCredits: 85500,
			cachedInputCentiCredits: 1700, cacheWrite5mCentiCredits: 21250, cacheWrite1hCentiCredits: 34000,
		}
	case isCreditsClaudeAlias(normalized, "claude-sonnet-5"):
		rate = creditsTextRate{
			inputCentiCredits: 17000, outputCentiCredits: 85500,
			cachedInputCentiCredits: 1700, cacheWrite5mCentiCredits: 21250, cacheWrite1hCentiCredits: 34000,
		}
	case isCreditsClaudeAlias(normalized, "claude-fable-5"):
		rate = creditsTextRate{
			inputCentiCredits: 80000, outputCentiCredits: 400000,
			cachedInputCentiCredits: 8000, cacheWrite5mCentiCredits: 100000, cacheWrite1hCentiCredits: 160000,
		}
	default:
		return nil, false
	}
	return &types.CreditsTextPricing{
		InputQuotaPerMillion:        quotaPerMillionFromCentiCredits(rate.inputCentiCredits),
		OutputQuotaPerMillion:       quotaPerMillionFromCentiCredits(rate.outputCentiCredits),
		CachedInputQuotaPerMillion:  quotaPerMillionFromCentiCredits(rate.cachedInputCentiCredits),
		CacheWriteQuotaPerMillion:   quotaPerMillionFromCentiCredits(rate.cacheWriteCentiCredits),
		CacheWrite5mQuotaPerMillion: quotaPerMillionFromCentiCredits(rate.cacheWrite5mCentiCredits),
		CacheWrite1hQuotaPerMillion: quotaPerMillionFromCentiCredits(rate.cacheWrite1hCentiCredits),
		PricingSource:               "kie",
	}, true
}

// CreditsV1TextPricingForPublicCatalog exposes the same immutable KIE snapshot
// used by settlement. Public catalog pricing must not maintain a second table.
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
	return false
}
