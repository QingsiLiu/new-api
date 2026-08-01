package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestCreditsV1TextPricingSnapshotAndAliases(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	tests := []struct {
		model      string
		input      int64
		output     int64
		cache      int64
		cacheWrite int64
		cache5m    int64
		cache1h    int64
	}{
		{"gpt-5.2", 1750, 14000, 175, 0, 0, 0},
		{"gpt-5.2-pro", 21000, 168000, 0, 0, 0, 0},
		{"gpt-5.3-codex", 1750, 14000, 175, 0, 0, 0},
		{"gpt-5.4", 2500, 15000, 250, 0, 0, 0},
		{"gpt-5.4-mini", 750, 4500, 75, 0, 0, 0},
		{"gpt-5.4-nano", 200, 1250, 20, 0, 0, 0},
		{"gpt-5.4-pro-2026-03-05", 30000, 180000, 0, 0, 0, 0},
		{"gpt-5.5", 5000, 30000, 500, 0, 0, 0},
		{"gpt-5.6-sol", 5000, 30000, 500, 6250, 0, 0},
		{"gpt-5.6-terra", 2000, 12000, 200, 2500, 0, 0},
		{"gpt-5.6-luna", 200, 1200, 20, 250, 0, 0},
		{"gemini-2.5-flash", 360, 3000, 36, 0, 0, 0},
		{"gemini-2.5-flash-lite", 120, 480, 12, 0, 0, 0},
		{"gemini-2.5-pro", 1500, 12000, 150, 0, 0, 0},
		{"gemini-3.1-pro-preview", 2400, 14400, 240, 0, 0, 0},
		{"gemini-3.1-flash-lite", 300, 1800, 30, 0, 0, 0},
		{"gemini-3-flash-preview", 600, 3600, 60, 0, 0, 0},
		{"gemini-3.5-flash", 1800, 10800, 180, 0, 0, 0},
		{"gemini-3.5-flash-lite", 360, 3000, 36, 0, 0, 0},
		{"gemini-3.6-flash", 1800, 9000, 180, 0, 0, 0},
		{"claude-opus-4-6-high", 22000, 110000, 2200, 0, 27500, 44000},
		{"claude-opus-4-7", 22000, 110000, 2200, 0, 27500, 44000},
		{"claude-opus-4-8", 22000, 110000, 2200, 0, 27500, 44000},
		{"claude-opus-5", 22000, 110000, 2200, 0, 27500, 44000},
		{"claude-sonnet-4-6-thinking", 13200, 66000, 1320, 0, 16500, 26400},
		{"claude-sonnet-5", 8800, 44000, 880, 0, 11000, 17600},
		{"claude-haiku-4-5-20251001", 4400, 22000, 440, 0, 5500, 8800},
		{"claude-fable-5", 44000, 220000, 4400, 0, 55000, 88000},
	}
	for _, test := range tests {
		t.Run(test.model, func(t *testing.T) {
			pricing, ok := creditsV1TextPricing(test.model)
			require.True(t, ok)
			require.Equal(t, quotaPerMillionFromCentiCredits(test.input), pricing.InputQuotaPerMillion)
			require.Equal(t, quotaPerMillionFromCentiCredits(test.output), pricing.OutputQuotaPerMillion)
			require.Equal(t, quotaPerMillionFromCentiCredits(test.cache), pricing.CachedInputQuotaPerMillion)
			require.Equal(t, quotaPerMillionFromCentiCredits(test.cacheWrite), pricing.CacheWriteQuotaPerMillion)
			require.Equal(t, quotaPerMillionFromCentiCredits(test.cache5m), pricing.CacheWrite5mQuotaPerMillion)
			require.Equal(t, quotaPerMillionFromCentiCredits(test.cache1h), pricing.CacheWrite1hQuotaPerMillion)
			require.Equal(t, "geili", pricing.PricingSource)
		})
	}

	for _, modelName := range []string{"seedream-5.0", "gpt-5.5-cyber"} {
		_, ok := creditsV1TextPricing(modelName)
		require.False(t, ok, modelName)
	}
}

func TestCreditsV1TextPricingContextTierBoundaries(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")

	gpt, ok := creditsV1TextPricing("gpt-5.5")
	require.True(t, ok)
	require.Equal(t, 50*int64(common.CreditsQuotaUnit), gpt.ForPromptTokens(272_000).InputQuotaPerMillion)
	require.Equal(t, 100*int64(common.CreditsQuotaUnit), gpt.ForPromptTokens(272_001).InputQuotaPerMillion)

	gemini, ok := creditsV1TextPricing("gemini-2.5-pro")
	require.True(t, ok)
	require.Equal(t, 15*int64(common.CreditsQuotaUnit), gemini.ForPromptTokens(200_000).InputQuotaPerMillion)
	require.Equal(t, 30*int64(common.CreditsQuotaUnit), gemini.ForPromptTokens(200_001).InputQuotaPerMillion)
}

func TestCreditsV1TextPricingOverridesProductionGroupRatio(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	setupModelPricingHelperTestDB(t)
	createPricingConfigModelForHelperTest(t, "gpt-5.5", model.ModelPricingConfig{
		Mode:       model.PricingModeRatio,
		UsePrice:   true,
		ModelPrice: 999,
		BaseRatio:  999,
	})
	previousGroups := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":6}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroups))
	})

	info := newPriceHelperRelayInfo("gpt-5.5")
	priceData, err := ModelPriceHelper(
		newPriceHelperTestContext(),
		info,
		1_000_000,
		&types.TokenCountMeta{},
	)
	require.NoError(t, err)
	require.NotNil(t, priceData.CreditsTextPricing)
	require.Equal(t, 1.0, priceData.GroupRatioInfo.GroupRatio)
	require.Equal(t, 100*common.CreditsQuotaUnit, priceData.QuotaToPreConsume)
	require.Equal(t, "geili", priceData.PricingSource)

	withOutput, err := ModelPriceHelper(
		newPriceHelperTestContext(),
		newPriceHelperRelayInfo("gpt-5.5"),
		1_000_000,
		&types.TokenCountMeta{MaxTokens: 1_000_000},
	)
	require.NoError(t, err)
	require.Equal(t, (100+450)*common.CreditsQuotaUnit, withOutput.QuotaToPreConsume)
}

func TestCreditsV1TextPricingTakesPrecedenceOverTieredExpression(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":    `{"gpt-5.5":"tiered_expr"}`,
		"billing_setting.billing_expr":    `{"gpt-5.5":"tier(\"wrong\", p * 999)"}`,
		"group_ratio_setting.group_ratio": `{"default":5}`,
	}))

	info := newPriceHelperRelayInfo("gpt-5.5")
	priceData, err := ModelPriceHelper(
		newPriceHelperTestContext(),
		info,
		1_000_000,
		&types.TokenCountMeta{},
	)
	require.NoError(t, err)
	require.NotNil(t, priceData.CreditsTextPricing)
	require.Nil(t, info.TieredBillingSnapshot)
	require.Equal(t, 100*common.CreditsQuotaUnit, priceData.QuotaToPreConsume)
}

func TestCreditsV1TextPricingMarksGeiliFallback(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	withModelRatioSettingForHelperTest(t, `{"seedream-5.0":0.1}`)
	priceData, err := ModelPriceHelper(
		newPriceHelperTestContext(),
		newPriceHelperRelayInfo("seedream-5.0"),
		1_000,
		&types.TokenCountMeta{},
	)
	require.NoError(t, err)
	require.Nil(t, priceData.CreditsTextPricing)
	require.Equal(t, "geili", priceData.PricingSource)
}

func TestCreditsV1TextPricingDisabledUsesLegacyPath(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "false")
	pricing, ok := creditsV1TextPricing("gpt-5.5")
	require.False(t, ok)
	require.Nil(t, pricing)
}
