package textpricing

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestCatalogMatchesAliasesAndBuildsExactQuota(t *testing.T) {
	profile, ok := MatchModel("claude-sonnet-4-6-thinking")
	require.True(t, ok)
	require.Equal(t, "anthropic.claude-sonnet-4-6", profile.Key)

	pricing, err := BuildPricing(profile, 0.22, true, "official_catalog")
	require.NoError(t, err)
	require.Equal(t, int64(13200*common.CreditsQuotaUnit/100), pricing.InputQuotaPerMillion)
	require.Equal(t, int64(66000*common.CreditsQuotaUnit/100), pricing.OutputQuotaPerMillion)
	require.Equal(t, int64(16500*common.CreditsQuotaUnit/100), pricing.CacheWrite5mQuotaPerMillion)
	require.Equal(t, int64(26400*common.CreditsQuotaUnit/100), pricing.CacheWrite1hQuotaPerMillion)
	require.True(t, pricing.ApplyGroupRatio)
}

func TestValidateMultiplierRequiresBoundedFourDecimalValue(t *testing.T) {
	require.NoError(t, ValidateMultiplier(0.1234))
	require.Error(t, ValidateMultiplier(0))
	require.Error(t, ValidateMultiplier(1.0001))
	require.Error(t, ValidateMultiplier(0.12345))
}

func TestCatalogLongContextBoundary(t *testing.T) {
	profile, ok := MatchModel("gpt-5.5")
	require.True(t, ok)
	pricing, err := BuildPricing(profile, 0.05, false, "geili")
	require.NoError(t, err)
	require.Equal(t, int64(180_000), pricing.ForPromptTokens(272_000).InputQuotaPerMillion)
	require.Equal(t, int64(360_000), pricing.ForPromptTokens(272_001).InputQuotaPerMillion)
}
