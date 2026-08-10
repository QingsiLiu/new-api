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

func TestGrokCatalogUsesVersionedOfficialProfiles(t *testing.T) {
	profile, ok := MatchModel("grok-code-fast-1")
	require.True(t, ok)
	require.Equal(t, CategoryGrok, profile.Category)
	require.Equal(t, "xai.grok-code-fast-1", profile.Key)
	require.Len(t, profile.Tiers, 2)
	require.Equal(t, int64(1_000_000), profile.Dimensions.InputMicroUSD)
	require.Equal(t, int64(2_000_000), profile.Dimensions.OutputMicroUSD)

	longProfile, ok := MatchModel("grok-4.5-latest")
	require.True(t, ok)
	require.Equal(t, "xai.grok-4.5", longProfile.Key)
	require.Equal(t, int64(4_000_000), longProfile.Tiers[1].Dimensions.InputMicroUSD)
	require.Equal(t, int64(12_000_000), longProfile.Tiers[1].Dimensions.OutputMicroUSD)
	_, ok = MatchModel("grok-4-1-fast-reasoning")
	require.False(t, ok, "unmatched names must remain fail-closed instead of being estimated")
}
