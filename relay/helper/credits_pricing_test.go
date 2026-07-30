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
		{"gpt-5.4", 7000, 56000, 0, 0, 0, 0},
		{"gpt-5.5", 14000, 84000, 1400, 0, 0, 0},
		{"gpt-5.6-sol", 28000, 168000, 2800, 35000, 0, 0},
		{"gpt-5.6-terra", 14000, 84000, 1400, 17500, 0, 0},
		{"gpt-5.6-luna", 5600, 33600, 560, 7000, 0, 0},
		{"gemini-2.5-flash", 900, 7500, 0, 0, 0, 0},
		{"gemini-3.1-pro-preview", 10000, 70000, 0, 0, 0, 0},
		{"claude-opus-4-6-high", 28500, 143000, 2850, 0, 35625, 57000},
		{"claude-opus-4-8", 40000, 200000, 4000, 0, 50000, 80000},
		{"claude-opus-5", 40000, 200000, 4000, 0, 50000, 80000},
		{"claude-sonnet-4-6-thinking", 17000, 85500, 1700, 0, 21250, 34000},
		{"claude-sonnet-5", 17000, 85500, 1700, 0, 21250, 34000},
		{"claude-fable-5", 80000, 400000, 8000, 0, 100000, 160000},
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
			require.Equal(t, "kie", pricing.PricingSource)
		})
	}

	for _, modelName := range []string{"gpt-5.4-mini", "gemini-3.5-flash", "seedream-5.0"} {
		_, ok := creditsV1TextPricing(modelName)
		require.False(t, ok, modelName)
	}
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
	require.Equal(t, 140*common.CreditsQuotaUnit, priceData.QuotaToPreConsume)
	require.Equal(t, "kie", priceData.PricingSource)

	withOutput, err := ModelPriceHelper(
		newPriceHelperTestContext(),
		newPriceHelperRelayInfo("gpt-5.5"),
		1_000_000,
		&types.TokenCountMeta{MaxTokens: 1_000_000},
	)
	require.NoError(t, err)
	require.Equal(t, (140+840)*common.CreditsQuotaUnit, withOutput.QuotaToPreConsume)
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
	require.Equal(t, 140*common.CreditsQuotaUnit, priceData.QuotaToPreConsume)
}

func TestCreditsV1TextPricingMarksGeiliFallback(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	withModelRatioSettingForHelperTest(t, `{"gpt-5.4-mini":0.1}`)
	priceData, err := ModelPriceHelper(
		newPriceHelperTestContext(),
		newPriceHelperRelayInfo("gpt-5.4-mini"),
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
