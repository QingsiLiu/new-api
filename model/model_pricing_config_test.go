package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func withPricingOptionsForTest(t *testing.T) {
	t.Helper()
	previousModelRatio := ratio_setting.ModelRatio2JSONString()
	previousCompletionRatio := ratio_setting.CompletionRatio2JSONString()
	previousCacheRatio := ratio_setting.CacheRatio2JSONString()
	previousModelPrice := ratio_setting.ModelPrice2JSONString()
	previousImageRatio := ratio_setting.ImageRatio2JSONString()
	previousAsyncPricing := operation_setting.AsyncSpecPricing2JSONString()
	previousQuotaPerCNY := operation_setting.QuotaPerCNY
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatio))
		require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(previousCompletionRatio))
		require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(previousCacheRatio))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(previousModelPrice))
		require.NoError(t, ratio_setting.UpdateImageRatioByJSONString(previousImageRatio))
		require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(previousAsyncPricing))
		operation_setting.QuotaPerCNY = previousQuotaPerCNY
	})

	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"text-priced":2.5,"image-token-model":3.5}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"text-priced":4.5}`))
	require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(`{"text-priced":0.25}`))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"per-call-model":0.42}`))
	require.NoError(t, ratio_setting.UpdateImageRatioByJSONString(`{"image-token-model":1.75}`))
	require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(`{
		"currency":"CNY",
		"image":{
			"spec-image-model":{
				"unit":"per_image",
				"resolutions":{"2k":{"cny_per_image":0.18}},
				"default_cny_per_image":0.11
			}
		},
		"video":{
			"spec-video-model":{
				"unit":"per_second",
				"prices":{
					"720p":{"16:9":{"no_video_input":{"cny_per_second":0.5},"with_video_input":{"unsupported":true}}}
				},
				"min_cny":0.2,
				"max_cny":2.5
			}
		}
	}`))
	operation_setting.QuotaPerCNY = 1000
}

func TestMigrateModelPricingConfigsCopiesEveryPricedSourceWithoutDeletingOptions(t *testing.T) {
	truncateTables(t)
	withPricingOptionsForTest(t)
	require.NoError(t, DB.Create(&Model{ModelName: "existing-model", Status: 1}).Error)
	oldModelRatio := ratio_setting.ModelRatio2JSONString()
	oldImageRatio := ratio_setting.ImageRatio2JSONString()
	oldAsyncPricing := operation_setting.AsyncSpecPricing2JSONString()

	stats, err := MigrateModelPricingConfigs()

	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.CreatedModels, 5)
	require.GreaterOrEqual(t, stats.PricedModels, 5)
	require.Equal(t, oldModelRatio, ratio_setting.ModelRatio2JSONString())
	require.Equal(t, oldImageRatio, ratio_setting.ImageRatio2JSONString())
	require.Equal(t, oldAsyncPricing, operation_setting.AsyncSpecPricing2JSONString())

	var textModel Model
	require.NoError(t, DB.Where("model_name = ?", "text-priced").First(&textModel).Error)
	require.Equal(t, "text", textModel.Modal)
	require.Equal(t, PricingModeRatio, textModel.PricingMode)
	textConfig, err := textModel.ParsePricingConfig()
	require.NoError(t, err)
	require.Equal(t, 2.5, textConfig.BaseRatio)
	require.Equal(t, 4.5, textConfig.CompletionRatio)
	require.Equal(t, 0.25, textConfig.CacheRatio)

	var imageModel Model
	require.NoError(t, DB.Where("model_name = ?", "spec-image-model").First(&imageModel).Error)
	require.Equal(t, "image", imageModel.Modal)
	require.Equal(t, PricingModeImageSpec, imageModel.PricingMode)
	require.Contains(t, imageModel.PricingConfig, `"2k"`)

	var videoModel Model
	require.NoError(t, DB.Where("model_name = ?", "spec-video-model").First(&videoModel).Error)
	require.Equal(t, "video", videoModel.Modal)
	require.Equal(t, PricingModeVideoMatrix, videoModel.PricingMode)
	require.Contains(t, videoModel.PricingConfig, `"unsupported":true`)

	secondStats, err := MigrateModelPricingConfigs()
	require.NoError(t, err)
	require.Equal(t, 0, secondStats.CreatedModels)
}

func TestCompareMigratedPricingConfigsMatchesLegacyQuotaForEveryConfiguredCombination(t *testing.T) {
	truncateTables(t)
	withPricingOptionsForTest(t)
	require.NoError(t, MigrateModelPricingConfigsOnly())

	report, err := CompareMigratedPricingConfigs()

	require.NoError(t, err)
	require.Empty(t, report.Mismatches)
	require.GreaterOrEqual(t, report.CheckedTextModels, 2)
	require.GreaterOrEqual(t, report.CheckedImageCombinations, 2)
	require.GreaterOrEqual(t, report.CheckedVideoCombinations, 2)
}

func TestRunModelPricingParityCheckSetsTrustedOnMatch(t *testing.T) {
	truncateTables(t)
	withPricingOptionsForTest(t)
	require.NoError(t, MigrateModelPricingConfigsOnly())

	status := RunModelPricingParityCheck()

	require.True(t, status.Trusted)
	require.True(t, IsModelPricingConfigTrusted())
	require.Zero(t, status.MismatchCount)
	require.GreaterOrEqual(t, status.CheckedText, 2)
	require.GreaterOrEqual(t, status.CheckedImage, 2)
	require.GreaterOrEqual(t, status.CheckedVideo, 2)
}

func TestRunModelPricingParityCheckDisablesTrustOnMismatch(t *testing.T) {
	truncateTables(t)
	withPricingOptionsForTest(t)
	require.NoError(t, MigrateModelPricingConfigsOnly())
	require.NoError(t, DB.Model(&Model{}).Where("model_name = ?", "text-priced").Update("pricing_config", `{"mode":"ratio","base_ratio":999}`).Error)

	status := RunModelPricingParityCheck()

	require.False(t, status.Trusted)
	require.False(t, IsModelPricingConfigTrusted())
	require.NotZero(t, status.MismatchCount)
	require.NotEmpty(t, status.Mismatches)
}

func TestGetModelPricingConfigRequiresTrustedParity(t *testing.T) {
	truncateTables(t)
	restore := SetModelPricingConfigTrustedForTest(false)
	t.Cleanup(restore)
	cfg := ModelPricingConfig{Mode: PricingModeRatio, BaseRatio: 2}
	configJSON, err := cfg.JSONString()
	require.NoError(t, err)
	require.NoError(t, DB.Create(&Model{ModelName: "gated-model", PricingMode: PricingModeRatio, PricingConfig: configJSON, Status: 1}).Error)

	_, ok, err := GetModelPricingConfig("gated-model")
	require.NoError(t, err)
	require.False(t, ok)

	restoreTrusted := SetModelPricingConfigTrustedForTest(true)
	t.Cleanup(restoreTrusted)
	loaded, ok, err := GetModelPricingConfig("gated-model")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 2.0, loaded.BaseRatio)
}
