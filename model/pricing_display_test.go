package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestGetModelPricingConfigForDisplayIgnoresTrustGate(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Where("1 = 1").Delete(&Model{}).Error)
	restore := SetModelPricingConfigTrustedForTest(false)
	t.Cleanup(restore)

	cfg := ModelPricingConfig{
		Mode:               PricingModeImageSpec,
		Resolutions:        map[string]ModelSpecResolutionPrice{"1k": {CNYPerImage: testFloat64Ptr(0.18)}},
		DefaultCNYPerImage: testFloat64Ptr(0.18),
	}
	configJSON, err := cfg.JSONString()
	require.NoError(t, err)
	require.NoError(t, DB.Create(&Model{
		ModelName:     "display-image-model",
		Status:        1,
		PricingMode:   PricingModeImageSpec,
		PricingConfig: configJSON,
	}).Error)

	_, gated, err := GetModelPricingConfig("display-image-model")
	require.NoError(t, err)
	require.False(t, gated)

	loaded, ok, err := GetModelPricingConfigForDisplay("display-image-model")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, PricingModeImageSpec, loaded.Mode)
	require.Equal(t, 0.18, *loaded.DefaultCNYPerImage)
}

func TestUpdatePricingDisplaysImageSpecConfig(t *testing.T) {
	truncateTables(t)
	resetPricingDisplayTestState(t)
	restore := SetModelPricingConfigTrustedForTest(false)
	t.Cleanup(restore)

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"display-image-model":0.15}`))
	configJSON, err := (ModelPricingConfig{
		Mode: PricingModeImageSpec,
		Resolutions: map[string]ModelSpecResolutionPrice{
			"1k": {CNYPerImage: testFloat64Ptr(0.18)},
			"2k": {CNYPerImage: testFloat64Ptr(0.28)},
			"4k": {CNYPerImage: testFloat64Ptr(0.42)},
		},
		DefaultCNYPerImage: testFloat64Ptr(0.18),
	}).JSONString()
	require.NoError(t, err)
	require.NoError(t, DB.Create(&Model{
		ModelName:     "display-image-model",
		Status:        1,
		Modal:         ModelModalImage,
		PricingMode:   PricingModeImageSpec,
		PricingConfig: configJSON,
	}).Error)
	channel := Channel{
		Type:   1,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "image-channel",
		Models: "display-image-model",
		Group:  "default",
	}
	require.NoError(t, DB.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))

	pricing := pricingForModel(t, "display-image-model")

	require.Equal(t, PricingModeImageSpec, pricing.PricingMode)
	require.InDelta(t, 0.18, *pricing.AmountCNY, 0.000001)
	require.Equal(t, 1, pricing.QuotaType)
	resolutions, ok := pricing.SpecPricing.(map[string]ModelSpecResolutionPrice)
	require.True(t, ok)
	require.InDelta(t, 0.28, *resolutions["2k"].CNYPerImage, 0.000001)
	require.InDelta(t, 0.42, *resolutions["4k"].CNYPerImage, 0.000001)
}

func TestUpdatePricingDisplaysVideoMatrixConfigWithStartPrice(t *testing.T) {
	truncateTables(t)
	resetPricingDisplayTestState(t)
	restore := SetModelPricingConfigTrustedForTest(false)
	t.Cleanup(restore)

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"display-video-model":83}`))
	configJSON, err := (ModelPricingConfig{
		Mode: PricingModeVideoMatrix,
		Prices: map[string]operation_setting.AsyncVideoRatioPrices{
			"720p": {
				"16:9": {
					"no_video_input":   {CNYPerSecond: testFloat64Ptr(0.6)},
					"with_video_input": {Unsupported: true},
				},
			},
			"1080p": {
				"16:9": {
					"no_video_input": {CNYPerSecond: testFloat64Ptr(0.9)},
				},
			},
		},
		MinCNY: 0.6,
		MaxCNY: 0.9,
	}).JSONString()
	require.NoError(t, err)
	require.NoError(t, DB.Create(&Model{
		ModelName:     "display-video-model",
		Status:        1,
		Modal:         ModelModalVideo,
		PricingMode:   PricingModeVideoMatrix,
		PricingConfig: configJSON,
	}).Error)
	channel := Channel{
		Type:   1,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "video-channel",
		Models: "display-video-model",
		Group:  "default",
	}
	require.NoError(t, DB.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))

	pricing := pricingForModel(t, "display-video-model")

	require.Equal(t, PricingModeVideoMatrix, pricing.PricingMode)
	require.InDelta(t, 0.6, *pricing.AmountCNY, 0.000001)
	require.Equal(t, 1, pricing.QuotaType)
	matrix, ok := pricing.SpecPricing.(map[string]operation_setting.AsyncVideoRatioPrices)
	require.True(t, ok)
	require.True(t, matrix["720p"]["16:9"]["with_video_input"].Unsupported)
	require.InDelta(t, 0.9, *matrix["1080p"]["16:9"]["no_video_input"].CNYPerSecond, 0.000001)
}

func TestAddAbilitiesSkipsBlankModelNames(t *testing.T) {
	truncateTables(t)

	channel := Channel{
		Type:   1,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "blank-model-channel",
		Models: " ,real-model,, ",
		Group:  "default,vip",
	}
	require.NoError(t, DB.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))

	var abilities []Ability
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
	require.Len(t, abilities, 2)
	for _, ability := range abilities {
		require.NotEmpty(t, ability.Model)
		require.Equal(t, "real-model", ability.Model)
	}
}

func TestUpdateAbilitiesSkipsBlankModelNames(t *testing.T) {
	truncateTables(t)

	channel := Channel{
		Type:   1,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "blank-model-channel",
		Models: " ,real-model,, ",
		Group:  "default,vip",
	}
	require.NoError(t, DB.Create(&channel).Error)
	require.NoError(t, channel.UpdateAbilities(nil))

	var abilities []Ability
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
	require.Len(t, abilities, 2)
	for _, ability := range abilities {
		require.NotEmpty(t, ability.Model)
		require.Equal(t, "real-model", ability.Model)
	}
}

func TestGetPricingSkipsBlankModelNames(t *testing.T) {
	truncateTables(t)
	resetPricingDisplayTestState(t)

	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"real-model":1}`))
	require.NoError(t, DB.Create(&Channel{
		Id:     10,
		Name:   "blank-model-channel",
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Models: "",
		Group:  "default",
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "",
		ChannelId: 10,
		Enabled:   true,
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "real-model",
		ChannelId: 10,
		Enabled:   true,
	}).Error)
	InvalidatePricingCache()

	pricings := GetPricing()
	require.Len(t, pricings, 1)
	require.Equal(t, "real-model", pricings[0].ModelName)
}

func resetPricingDisplayTestState(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Where("1 = 1").Delete(&Model{}).Error)
	InvalidatePricingCache()
	previousModelPrice := ratio_setting.ModelPrice2JSONString()
	previousModelRatio := ratio_setting.ModelRatio2JSONString()
	previousCompletionRatio := ratio_setting.CompletionRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(previousModelPrice))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatio))
		require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(previousCompletionRatio))
		InvalidatePricingCache()
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{}`))
}

func pricingForModel(t *testing.T, modelName string) Pricing {
	t.Helper()
	for _, pricing := range GetPricing() {
		if pricing.ModelName == modelName {
			return pricing
		}
	}
	require.Failf(t, "pricing model not found", "model %q not present in pricing map", modelName)
	return Pricing{}
}

func testFloat64Ptr(value float64) *float64 {
	return &value
}
