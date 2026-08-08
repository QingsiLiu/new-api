package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/textpricing"
	"github.com/stretchr/testify/require"
)

func setupTextPricingCenterTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TextCategoryPricing{}))
	require.NoError(t, DB.Exec("DELETE FROM text_category_pricing").Error)
	require.NoError(t, DB.Exec("DELETE FROM models").Error)
	require.NoError(t, DB.Exec("DELETE FROM options").Error)

	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{TextPricingModeOption: TextPricingModeLegacy}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		_ = DB.Exec("DELETE FROM text_category_pricing").Error
		_ = DB.Exec("DELETE FROM models").Error
		_ = DB.Exec("DELETE FROM options").Error
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})
}

func TestSeedTextPricingCenterSeedsDefaultsAndCatalogMetadata(t *testing.T) {
	setupTextPricingCenterTest(t)
	require.NoError(t, DB.Create(&Model{ModelName: "gpt-5.5", Status: 0}).Error)

	require.NoError(t, SeedTextPricingCenter())

	var option Option
	require.NoError(t, DB.Where("key = ?", TextPricingModeOption).First(&option).Error)
	require.Equal(t, TextPricingModeLegacy, option.Value)

	multipliers, err := GetTextCategoryMultipliers()
	require.NoError(t, err)
	require.Equal(t, 0.05, multipliers[textpricing.CategoryGPT])
	require.Equal(t, 0.22, multipliers[textpricing.CategoryClaude])
	require.Equal(t, 0.06, multipliers[textpricing.CategoryGemini])
	_, hasGrokDefault := multipliers[textpricing.CategoryGrok]
	require.False(t, hasGrokDefault)

	var entry Model
	require.NoError(t, DB.Where("model_name = ?", "gpt-5.5").First(&entry).Error)
	require.Equal(t, ModelModalText, entry.Modal)
	require.Equal(t, textpricing.CategoryGPT, entry.TextCategory)
	require.Equal(t, "openai.gpt-5.5", entry.OfficialPriceKey)

	_, err = UpdateTextCategoryMultiplier(textpricing.CategoryGPT, 0.125)
	require.NoError(t, err)
	require.NoError(t, SeedTextPricingCenter())
	multiplier, found, err := GetTextCategoryMultiplier(textpricing.CategoryGPT)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 0.125, multiplier, "seeding must not overwrite an administrator value")
}

func TestResolveTextModelPricingUsesOfficialCatalogAndCategoryMultiplier(t *testing.T) {
	setupTextPricingCenterTest(t)
	require.NoError(t, DB.Create(&TextCategoryPricing{
		Category:    textpricing.CategoryGPT,
		Multiplier:  0.1,
		UpdatedTime: common.GetTimestamp(),
	}).Error)
	require.NoError(t, DB.Create(&Model{
		ModelName:        "gpt-5.5",
		Modal:            ModelModalText,
		TextCategory:     textpricing.CategoryGPT,
		OfficialPriceKey: "openai.gpt-5.5",
		Status:           1,
	}).Error)

	resolution, err := ResolveTextModelPricing("gpt-5.5")
	require.NoError(t, err)
	require.Equal(t, 0.1, resolution.Multiplier)
	require.Equal(t, int64(360_000), resolution.Pricing.InputQuotaPerMillion)
	require.Equal(t, int64(2_160_000), resolution.Pricing.OutputQuotaPerMillion)
	require.True(t, resolution.Pricing.ApplyGroupRatio)
	require.False(t, resolution.Pricing.Fallback)
	require.Equal(t, textpricing.CatalogVersion, resolution.Pricing.CatalogVersion)
}

func TestResolveEffectiveTextPricingModesAndFailClosed(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	setupTextPricingCenterTest(t)
	restoreTrust := SetModelPricingConfigTrustedForTest(true)
	t.Cleanup(restoreTrust)

	legacy, ok, err := ResolveEffectiveTextPricingForMode("gpt-5.5", TextPricingModeLegacy)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, legacy.Fallback)
	require.False(t, legacy.ApplyGroupRatio)

	_, ok, err = ResolveEffectiveTextPricingForMode("missing-model", TextPricingModeActive)
	require.Error(t, err)
	require.False(t, ok)

	require.NoError(t, DB.Create(&Model{
		ModelName:        "bad-gpt",
		Modal:            ModelModalText,
		TextCategory:     textpricing.CategoryClaude,
		OfficialPriceKey: "openai.gpt-5.5",
		Status:           1,
	}).Error)
	_, ok, err = ResolveEffectiveTextPricingForMode("bad-gpt", TextPricingModeActive)
	require.ErrorContains(t, err, "does not match")
	require.False(t, ok)
}

func TestResolveEffectiveTextPricingFallsBackToCreditsV1WhenParityIsUntrusted(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "false")
	setupTextPricingCenterTest(t)
	restoreTrust := SetModelPricingConfigTrustedForTest(false)
	t.Cleanup(restoreTrust)

	pricing, ok, err := ResolveEffectiveTextPricingForMode("gpt-5.5", TextPricingModeActive)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, pricing.Fallback)
	require.False(t, pricing.ApplyGroupRatio)
	require.Equal(t, "geili", pricing.PricingSource)
}

func TestPreviewTextCategoryMultiplierShowsAffectedPriceDelta(t *testing.T) {
	setupTextPricingCenterTest(t)
	require.NoError(t, DB.Create(&TextCategoryPricing{Category: textpricing.CategoryGPT, Multiplier: 0.05}).Error)
	require.NoError(t, DB.Create(&Model{
		ModelName:        "gpt-5.5",
		Modal:            ModelModalText,
		TextCategory:     textpricing.CategoryGPT,
		OfficialPriceKey: "openai.gpt-5.5",
		Status:           1,
	}).Error)

	preview, err := PreviewTextCategoryMultiplier(textpricing.CategoryGPT, 0.1)
	require.NoError(t, err)
	require.Equal(t, 1, preview.AffectedCount)
	require.Len(t, preview.Before.Models, 1)
	require.Len(t, preview.After.Models, 1)
	require.Equal(t, int64(180_000), preview.Before.Models[0].InputQuotaPerMillion)
	require.Equal(t, int64(360_000), preview.After.Models[0].InputQuotaPerMillion)
}

func TestGrokKeepsLegacyDatabasePricingAndFailsClosedInActiveMode(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	setupTextPricingCenterTest(t)
	restoreTrust := SetModelPricingConfigTrustedForTest(true)
	t.Cleanup(restoreTrust)
	require.NoError(t, DB.Create(&Model{
		ModelName:    "grok-4.1",
		Modal:        ModelModalText,
		TextCategory: textpricing.CategoryGrok,
		Status:       1,
	}).Error)

	legacy, ok, err := ResolveEffectiveTextPricingForMode("grok-4.1", TextPricingModeLegacy)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, legacy)

	_, ok, err = ResolveEffectiveTextPricingForMode("grok-4.1", TextPricingModeActive)
	require.ErrorContains(t, err, "no official price profile")
	require.False(t, ok)
}
