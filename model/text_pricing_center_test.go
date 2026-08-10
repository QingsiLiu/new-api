package model

import (
	"database/sql"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/textpricing"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

func TestResolveEffectiveTextPricingFailsClosedWhenParityIsUntrusted(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "false")
	setupTextPricingCenterTest(t)
	restoreTrust := SetModelPricingConfigTrustedForTest(false)
	t.Cleanup(restoreTrust)

	pricing, ok, err := ResolveEffectiveTextPricingForMode("gpt-5.5", TextPricingModeActive)
	require.ErrorContains(t, err, "parity is not trusted")
	require.False(t, ok)
	require.Nil(t, pricing)
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

func TestGrokKeepsLegacyDatabasePricingAndFailsClosedWithoutMultiplier(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	setupTextPricingCenterTest(t)
	restoreTrust := SetModelPricingConfigTrustedForTest(true)
	t.Cleanup(restoreTrust)
	require.NoError(t, DB.Create(&Model{
		ModelName:        "grok-4.5",
		Modal:            ModelModalText,
		TextCategory:     textpricing.CategoryGrok,
		OfficialPriceKey: "xai.grok-4.5",
		Status:           1,
	}).Error)

	legacy, ok, err := ResolveEffectiveTextPricingForMode("grok-4.5", TextPricingModeLegacy)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, legacy)

	_, ok, err = ResolveEffectiveTextPricingForMode("grok-4.5", TextPricingModeActive)
	require.ErrorContains(t, err, "no configured multiplier")
	require.False(t, ok)
}

func TestTextModelMultiplierOverrideReplacesCategoryMultiplier(t *testing.T) {
	setupTextPricingCenterTest(t)
	require.NoError(t, DB.Create(&TextCategoryPricing{Category: textpricing.CategoryGPT, Multiplier: 0.1}).Error)
	modelOverride := 0.2
	models := []Model{
		{
			ModelName:        "gpt-5.5-inherited",
			Modal:            ModelModalText,
			TextCategory:     textpricing.CategoryGPT,
			OfficialPriceKey: "openai.gpt-5.5",
			Status:           1,
		},
		{
			ModelName:              "gpt-5.5-overridden",
			Modal:                  ModelModalText,
			TextCategory:           textpricing.CategoryGPT,
			OfficialPriceKey:       "openai.gpt-5.5",
			TextMultiplierOverride: &modelOverride,
			Status:                 1,
		},
	}
	require.NoError(t, DB.Create(&models).Error)

	inherited, err := ResolveTextModelPricing("gpt-5.5-inherited")
	require.NoError(t, err)
	require.Equal(t, 0.1, inherited.EffectiveMultiplier)
	require.Equal(t, TextMultiplierSourceCategory, inherited.MultiplierSource)
	require.Nil(t, inherited.ModelMultiplierOverride)
	require.Equal(t, int64(360_000), inherited.Pricing.InputQuotaPerMillion)

	overridden, err := ResolveTextModelPricing("gpt-5.5-overridden")
	require.NoError(t, err)
	require.NotNil(t, overridden.CategoryMultiplier)
	require.Equal(t, 0.1, *overridden.CategoryMultiplier)
	require.NotNil(t, overridden.ModelMultiplierOverride)
	require.Equal(t, 0.2, *overridden.ModelMultiplierOverride)
	require.Equal(t, 0.2, overridden.EffectiveMultiplier)
	require.Equal(t, TextMultiplierSourceModelOverride, overridden.MultiplierSource)
	require.Equal(t, int64(720_000), overridden.Pricing.InputQuotaPerMillion)

	preview, err := PreviewTextCategoryMultiplier(textpricing.CategoryGPT, 0.05)
	require.NoError(t, err)
	require.Equal(t, 1, preview.AffectedCount)
	require.Equal(t, 1, preview.OverrideCount)
	require.True(t, preview.After.Models[0].Affected)
	require.False(t, preview.After.Models[1].Affected)
	require.Equal(t, int64(180_000), preview.After.Models[0].InputQuotaPerMillion)
	require.Equal(t, preview.Before.Models[1].InputQuotaPerMillion, preview.After.Models[1].InputQuotaPerMillion)

	_, err = UpdateTextCategoryMultiplier(textpricing.CategoryGPT, 0.05)
	require.NoError(t, err)
	var persisted Model
	require.NoError(t, DB.Where("model_name = ?", "gpt-5.5-overridden").First(&persisted).Error)
	require.NotNil(t, persisted.TextMultiplierOverride)
	require.Equal(t, 0.2, *persisted.TextMultiplierOverride)
}

func TestTextModelMultiplierPreviewUpdateAndClear(t *testing.T) {
	setupTextPricingCenterTest(t)
	require.NoError(t, DB.Create(&TextCategoryPricing{Category: textpricing.CategoryGPT, Multiplier: 0.1}).Error)
	entry := Model{
		ModelName:        "gpt-5.5-model-edit",
		Modal:            ModelModalText,
		TextCategory:     textpricing.CategoryGPT,
		OfficialPriceKey: "openai.gpt-5.5",
		Status:           1,
	}
	require.NoError(t, DB.Create(&entry).Error)

	customMultiplier := 0.25
	preview, err := PreviewTextModelMultiplier(entry.Id, &customMultiplier)
	require.NoError(t, err)
	require.Equal(t, 0.1, preview.Before.EffectiveMultiplier)
	require.Equal(t, 0.25, preview.After.EffectiveMultiplier)
	require.Equal(t, TextMultiplierSourceModelOverride, preview.After.MultiplierSource)

	callbackName := "test:text-model-multiplier-transaction"
	queryTransactions := make([]bool, 0, 2)
	require.NoError(t, DB.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "models" && tx.Statement.Table != "text_category_pricing" {
			return
		}
		_, insideTransaction := tx.Statement.ConnPool.(*sql.Tx)
		queryTransactions = append(queryTransactions, insideTransaction)
	}))
	t.Cleanup(func() {
		_ = DB.Callback().Query().Remove(callbackName)
	})

	updated, err := UpdateTextModelMultiplier(entry.Id, &customMultiplier)
	require.NoError(t, err)
	require.NotEmpty(t, queryTransactions)
	for _, insideTransaction := range queryTransactions {
		require.True(t, insideTransaction, "model and category reads must share the update transaction")
	}
	require.Equal(t, preview.After.InputQuotaPerMillion, updated.After.InputQuotaPerMillion)
	var persisted Model
	require.NoError(t, DB.First(&persisted, entry.Id).Error)
	require.NotNil(t, persisted.TextMultiplierOverride)
	require.Equal(t, 0.25, *persisted.TextMultiplierOverride)

	clearPreview, err := PreviewTextModelMultiplier(entry.Id, nil)
	require.NoError(t, err)
	require.Equal(t, TextMultiplierSourceCategory, clearPreview.After.MultiplierSource)
	require.Equal(t, 0.1, clearPreview.After.EffectiveMultiplier)
	require.Nil(t, clearPreview.After.ModelMultiplierOverride)
	_, err = UpdateTextModelMultiplier(entry.Id, nil)
	require.NoError(t, err)
	require.NoError(t, DB.First(&persisted, entry.Id).Error)
	require.Nil(t, persisted.TextMultiplierOverride)

	invalidMultiplier := 0.12345
	_, err = UpdateTextModelMultiplier(entry.Id, &invalidMultiplier)
	require.ErrorContains(t, err, "four decimal")
}

func TestPendingTextPricingFilterReturnsOnlyUnresolvedTextModels(t *testing.T) {
	setupTextPricingCenterTest(t)
	require.NoError(t, DB.Create(&TextCategoryPricing{Category: textpricing.CategoryGPT, Multiplier: 0.1}).Error)
	models := []Model{
		{
			ModelName:        "ready-text-model",
			Modal:            ModelModalText,
			TextCategory:     textpricing.CategoryGPT,
			OfficialPriceKey: "openai.gpt-5.5",
		},
		{
			ModelName:    "unclassified-text-model",
			Modal:        ModelModalText,
			TextCategory: textpricing.CategoryUnclassified,
		},
		{
			ModelName:        "missing-profile-text-model",
			Modal:            ModelModalText,
			TextCategory:     textpricing.CategoryGPT,
			OfficialPriceKey: "missing.profile",
		},
		{
			ModelName:    "unresolved-image-model",
			Modal:        ModelModalImage,
			TextCategory: textpricing.CategoryUnclassified,
		},
	}
	require.NoError(t, DB.Create(&models).Error)

	entries, total, err := GetModelsByFilters(ModelListFilters{
		Modal:             ModelModalText,
		TextPricingStatus: "pending",
	}, 0, 20)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, entries, 2)
	require.ElementsMatch(t, []string{
		"unclassified-text-model",
		"missing-profile-text-model",
	}, []string{entries[0].ModelName, entries[1].ModelName})
}

func TestTextPricingModesKeepLegacyAndApplyOverrideOnlyToCandidate(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	setupTextPricingCenterTest(t)
	restoreTrust := SetModelPricingConfigTrustedForTest(true)
	t.Cleanup(restoreTrust)
	require.NoError(t, DB.Create(&TextCategoryPricing{Category: textpricing.CategoryGPT, Multiplier: 0.1}).Error)
	modelOverride := 0.2
	require.NoError(t, DB.Create(&Model{
		ModelName:              "gpt-5.5",
		Modal:                  ModelModalText,
		TextCategory:           textpricing.CategoryGPT,
		OfficialPriceKey:       "openai.gpt-5.5",
		TextMultiplierOverride: &modelOverride,
		Status:                 1,
	}).Error)

	legacy, ok, err := ResolveEffectiveTextPricingForMode("gpt-5.5", TextPricingModeLegacy)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 0.05, legacy.EffectiveMultiplier)
	require.Nil(t, legacy.ModelMultiplierOverride)

	shadow, ok, err := ResolveShadowTextPricingForMode("gpt-5.5", TextPricingModeShadow)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 0.2, shadow.EffectiveMultiplier)
	require.Equal(t, TextMultiplierSourceModelOverride, shadow.MultiplierSource)

	active, ok, err := ResolveEffectiveTextPricingForMode("gpt-5.5", TextPricingModeActive)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 0.2, active.EffectiveMultiplier)
	require.True(t, active.ApplyGroupRatio)
}

func TestTextPricingConfigHasFourGroupsAndBlocksActiveUntilGrokMultiplier(t *testing.T) {
	setupTextPricingCenterTest(t)
	restoreTrust := SetModelPricingConfigTrustedForTest(true)
	t.Cleanup(restoreTrust)
	require.NoError(t, DB.Create(&[]TextCategoryPricing{
		{Category: textpricing.CategoryGPT, Multiplier: 0.05},
		{Category: textpricing.CategoryClaude, Multiplier: 0.22},
		{Category: textpricing.CategoryGemini, Multiplier: 0.06},
	}).Error)
	pendingModel := Model{
		ModelName:    "pending-text-model",
		Modal:        ModelModalText,
		TextCategory: textpricing.CategoryUnclassified,
	}
	require.NoError(t, DB.Create(&pendingModel).Error)
	require.NoError(t, DB.Model(&pendingModel).Update("status", 0).Error)

	config, err := GetTextPricingConfigView()
	require.NoError(t, err)
	require.Len(t, config.Categories, 4)
	require.Equal(t, 1, config.PendingCount)
	require.Equal(t, 1, config.UnclassifiedCount)
	require.False(t, config.ActivationReady)
	require.ErrorContains(t, SetTextPricingMode(TextPricingModeActive), "grok")

	_, err = UpdateTextCategoryMultiplier(textpricing.CategoryGrok, 0.1)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&pendingModel).Update("status", 1).Error)
	config, err = GetTextPricingConfigView()
	require.NoError(t, err)
	require.False(t, config.ActivationReady)
	require.Contains(t, config.ActivationBlockers, "model pending-text-model has no valid text category")
	require.ErrorContains(t, SetTextPricingMode(TextPricingModeActive), "pending-text-model")

	require.NoError(t, DB.Model(&pendingModel).Update("status", 0).Error)
	require.NoError(t, SetTextPricingMode(TextPricingModeActive))
	require.Equal(t, TextPricingModeActive, GetTextPricingMode())
}
