package helper

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

type creditsTextRate struct {
	inputCredits       int64
	outputCredits      int64
	cachedInputCredits int64
}

func creditsV1TextPricing(modelName string) (*types.CreditsTextPricing, bool) {
	if !common.CreditsV1Enabled() {
		return nil, false
	}
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	var rate creditsTextRate
	switch {
	case normalized == "gpt-5.4" || strings.HasPrefix(normalized, "gpt-5.4-2026-"):
		rate = creditsTextRate{inputCredits: 70, outputCredits: 560}
	case normalized == "gpt-5.5" || strings.HasPrefix(normalized, "gpt-5.5-2026-"):
		rate = creditsTextRate{cachedInputCredits: 14, inputCredits: 140, outputCredits: 840}
	case normalized == "gemini-2.5-flash":
		rate = creditsTextRate{inputCredits: 9, outputCredits: 75}
	case normalized == "gemini-3.1-pro-preview":
		rate = creditsTextRate{inputCredits: 100, outputCredits: 700}
	case isCreditsClaudeAlias(normalized, "claude-opus-4-6"):
		rate = creditsTextRate{inputCredits: 285, outputCredits: 1430}
	case isCreditsClaudeAlias(normalized, "claude-opus-4-8"):
		rate = creditsTextRate{inputCredits: 400, outputCredits: 2000}
	case isCreditsClaudeAlias(normalized, "claude-sonnet-4-6"):
		rate = creditsTextRate{inputCredits: 170, outputCredits: 855}
	case isCreditsClaudeAlias(normalized, "claude-fable-5"):
		rate = creditsTextRate{inputCredits: 800, outputCredits: 4000}
	default:
		return nil, false
	}
	return &types.CreditsTextPricing{
		InputQuotaPerMillion:       rate.inputCredits * int64(common.CreditsQuotaUnit),
		OutputQuotaPerMillion:      rate.outputCredits * int64(common.CreditsQuotaUnit),
		CachedInputQuotaPerMillion: rate.cachedInputCredits * int64(common.CreditsQuotaUnit),
		PricingSource:              "kie",
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
