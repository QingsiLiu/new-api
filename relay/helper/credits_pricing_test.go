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
		model  string
		input  int64
		output int64
		cache  int64
	}{
		{"gpt-5.4", 70, 560, 0},
		{"gpt-5.5", 140, 840, 14},
		{"gemini-2.5-flash", 9, 75, 0},
		{"gemini-3.1-pro-preview", 100, 700, 0},
		{"claude-opus-4-6-high", 285, 1430, 0},
		{"claude-opus-4-8", 400, 2000, 0},
		{"claude-sonnet-4-6-thinking", 170, 855, 0},
		{"claude-fable-5", 800, 4000, 0},
	}
	for _, test := range tests {
		t.Run(test.model, func(t *testing.T) {
			pricing, ok := creditsV1TextPricing(test.model)
			require.True(t, ok)
			require.Equal(t, test.input*int64(common.CreditsQuotaUnit), pricing.InputQuotaPerMillion)
			require.Equal(t, test.output*int64(common.CreditsQuotaUnit), pricing.OutputQuotaPerMillion)
			require.Equal(t, test.cache*int64(common.CreditsQuotaUnit), pricing.CachedInputQuotaPerMillion)
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
