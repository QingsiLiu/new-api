package helper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestModelPriceHelperTieredUsesPreloadedRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"tiered-test-model":"param(\"stream\") == true ? tier(\"stream\", p * 3) : tier(\"base\", p * 2)"}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &billingexpr.RequestInput{
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"stream":true}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{
		BillingRatios: map[string]float64{"n": 3},
	})
	require.NoError(t, err)
	require.Equal(t, int(3000.0/1_000_000*common.QuotaPerUnit), priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "stream", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, billing_setting.BillingModeTieredExpr, info.TieredBillingSnapshot.BillingMode)
	require.Equal(t, common.QuotaPerUnit, info.TieredBillingSnapshot.QuotaPerUnit)
}

func TestModelPriceHelperPerCallPrefersModelPricingConfigFixedPrice(t *testing.T) {
	setupModelPricingHelperTestDB(t)
	withModelPriceSettingForHelperTest(t, `{"unified-call-model":0.99}`)
	ctx := newPriceHelperTestContext()
	createPricingConfigModelForHelperTest(t, "unified-call-model", model.ModelPricingConfig{
		Mode:       model.PricingModeRatio,
		UsePrice:   true,
		ModelPrice: 0.12,
	})

	priceData, err := ModelPriceHelperPerCall(ctx, newPriceHelperRelayInfo("unified-call-model"))

	require.NoError(t, err)
	require.True(t, priceData.UsePrice)
	require.Equal(t, 0.12, priceData.ModelPrice)
	require.Equal(t, int(0.12*common.QuotaPerUnit), priceData.Quota)
}

func TestModelPriceHelperPerCallPrefersModelPricingConfigRatio(t *testing.T) {
	setupModelPricingHelperTestDB(t)
	withModelPriceSettingForHelperTest(t, `{}`)
	withModelRatioSettingForHelperTest(t, `{"unified-ratio-model":10}`)
	ctx := newPriceHelperTestContext()
	createPricingConfigModelForHelperTest(t, "unified-ratio-model", model.ModelPricingConfig{
		Mode:      model.PricingModeRatio,
		BaseRatio: 4,
	})

	priceData, err := ModelPriceHelperPerCall(ctx, newPriceHelperRelayInfo("unified-ratio-model"))

	require.NoError(t, err)
	require.False(t, priceData.UsePrice)
	require.Equal(t, 4.0, priceData.ModelRatio)
	require.Equal(t, int(4.0/2*common.QuotaPerUnit), priceData.Quota)
}

func TestModelPriceHelperPerCallFallsBackForEmptyAndInheritPricingConfig(t *testing.T) {
	setupModelPricingHelperTestDB(t)
	withModelPriceSettingForHelperTest(t, `{"empty-pricing-model":0.07,"inherit-pricing-model":0.08}`)
	ctx := newPriceHelperTestContext()
	require.NoError(t, model.DB.Create(&model.Model{
		ModelName: "empty-pricing-model",
		Status:    1,
	}).Error)
	createPricingConfigModelForHelperTest(t, "inherit-pricing-model", model.ModelPricingConfig{
		Mode: model.PricingModeInherit,
	})

	emptyPriceData, err := ModelPriceHelperPerCall(ctx, newPriceHelperRelayInfo("empty-pricing-model"))
	require.NoError(t, err)
	require.True(t, emptyPriceData.UsePrice)
	require.Equal(t, int(0.07*common.QuotaPerUnit), emptyPriceData.Quota)

	inheritPriceData, err := ModelPriceHelperPerCall(ctx, newPriceHelperRelayInfo("inherit-pricing-model"))
	require.NoError(t, err)
	require.True(t, inheritPriceData.UsePrice)
	require.Equal(t, int(0.08*common.QuotaPerUnit), inheritPriceData.Quota)
}

func TestModelPriceHelperUsesModelPricingConfigRatioForTokenPreconsume(t *testing.T) {
	setupModelPricingHelperTestDB(t)
	withModelRatioSettingForHelperTest(t, `{"chat-ratio-model":9}`)
	ctx := newPriceHelperTestContext()
	info := newPriceHelperRelayInfo("chat-ratio-model")
	createPricingConfigModelForHelperTest(t, "chat-ratio-model", model.ModelPricingConfig{
		Mode:             model.PricingModeRatio,
		BaseRatio:        2,
		CompletionRatio:  3,
		CacheRatio:       0.5,
		CreateCacheRatio: 1.75,
		ImageRatio:       4,
	})

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{MaxTokens: 100})

	require.NoError(t, err)
	require.Equal(t, 2.0, priceData.ModelRatio)
	require.Equal(t, 3.0, priceData.CompletionRatio)
	require.Equal(t, 0.5, priceData.CacheRatio)
	require.Equal(t, 1.75, priceData.CacheCreationRatio)
	require.Equal(t, 4.0, priceData.ImageRatio)
	require.Equal(t, 2200, priceData.QuotaToPreConsume)
	require.Equal(t, priceData, info.PriceData)
}

func TestModelPriceHelperUsesEmbeddedRatioFromSpecPricingConfigForTokenPreconsume(t *testing.T) {
	setupModelPricingHelperTestDB(t)
	withModelRatioSettingForHelperTest(t, `{"chat-spec-ratio-model":9}`)
	ctx := newPriceHelperTestContext()
	info := newPriceHelperRelayInfo("chat-spec-ratio-model")
	createPricingConfigModelForHelperTest(t, "chat-spec-ratio-model", model.ModelPricingConfig{
		Mode:             model.PricingModeImageSpec,
		UseRatio:         true,
		BaseRatio:        2,
		CompletionRatio:  3,
		CacheRatio:       0.5,
		CreateCacheRatio: 1.75,
		ImageRatio:       4,
		Resolutions: map[string]model.ModelSpecResolutionPrice{
			"2k": {CNYPerImage: common.GetPointer(0.18)},
		},
	})

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{MaxTokens: 100})

	require.NoError(t, err)
	require.Equal(t, 2.0, priceData.ModelRatio)
	require.Equal(t, 3.0, priceData.CompletionRatio)
	require.Equal(t, 0.5, priceData.CacheRatio)
	require.Equal(t, 1.75, priceData.CacheCreationRatio)
	require.Equal(t, 4.0, priceData.ImageRatio)
	require.Equal(t, 2200, priceData.QuotaToPreConsume)
	require.Equal(t, priceData, info.PriceData)
}

func TestModelPriceHelperPerCallDoesNotReturnSpecPlaceholderForGenericCallers(t *testing.T) {
	setupModelPricingHelperTestDB(t)
	withModelPriceSettingForHelperTest(t, `{"spec-generic-model":0.05}`)
	previousEnabled := operation_setting.AsyncTaskSpecPricingEnabled
	operation_setting.AsyncTaskSpecPricingEnabled = true
	t.Cleanup(func() {
		operation_setting.AsyncTaskSpecPricingEnabled = previousEnabled
	})
	ctx := newPriceHelperTestContext()
	createPricingConfigModelForHelperTest(t, "spec-generic-model", model.ModelPricingConfig{
		Mode: model.PricingModeImageSpec,
		Resolutions: map[string]model.ModelSpecResolutionPrice{
			"2k": {CNYPerImage: common.GetPointer(0.18)},
		},
	})

	priceData, err := ModelPriceHelperPerCall(ctx, newPriceHelperRelayInfo("spec-generic-model"))

	require.NoError(t, err)
	require.True(t, priceData.UsePrice)
	require.Equal(t, int(0.05*common.QuotaPerUnit), priceData.Quota)
	require.Nil(t, priceData.SpecPricing)
}

func TestModelPriceHelperPerCallUsesEmbeddedRatioFromSpecPricingConfigForGenericCallers(t *testing.T) {
	setupModelPricingHelperTestDB(t)
	withModelRatioSettingForHelperTest(t, `{"spec-generic-ratio-model":10}`)
	ctx := newPriceHelperTestContext()
	createPricingConfigModelForHelperTest(t, "spec-generic-ratio-model", model.ModelPricingConfig{
		Mode:      model.PricingModeImageSpec,
		UseRatio:  true,
		BaseRatio: 4,
		Resolutions: map[string]model.ModelSpecResolutionPrice{
			"2k": {CNYPerImage: common.GetPointer(0.18)},
		},
	})

	priceData, err := ModelPriceHelperPerCall(ctx, newPriceHelperRelayInfo("spec-generic-ratio-model"))

	require.NoError(t, err)
	require.False(t, priceData.UsePrice)
	require.Equal(t, 4.0, priceData.ModelRatio)
	require.Equal(t, int(4.0/2*common.QuotaPerUnit), priceData.Quota)
	require.Nil(t, priceData.SpecPricing)
}

func setupModelPricingHelperTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Model{}))
	restoreTrust := model.SetModelPricingConfigTrustedForTest(true)
	t.Cleanup(func() {
		restoreTrust()
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		model.DB = previousDB
	})
	return db
}

func TestModelPriceHelperPerCallSkipsModelPricingConfigWhenUntrusted(t *testing.T) {
	setupModelPricingHelperTestDB(t)
	restoreTrust := model.SetModelPricingConfigTrustedForTest(false)
	t.Cleanup(restoreTrust)
	withModelPriceSettingForHelperTest(t, `{"untrusted-call-model":0.99}`)
	ctx := newPriceHelperTestContext()
	createPricingConfigModelForHelperTest(t, "untrusted-call-model", model.ModelPricingConfig{
		Mode:       model.PricingModeRatio,
		UsePrice:   true,
		ModelPrice: 0.12,
	})

	priceData, err := ModelPriceHelperPerCall(ctx, newPriceHelperRelayInfo("untrusted-call-model"))

	require.NoError(t, err)
	require.True(t, priceData.UsePrice)
	require.Equal(t, 0.99, priceData.ModelPrice)
	require.Equal(t, int(0.99*common.QuotaPerUnit), priceData.Quota)
}

func createPricingConfigModelForHelperTest(t *testing.T, modelName string, cfg model.ModelPricingConfig) {
	t.Helper()

	configJSON, err := cfg.JSONString()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.Model{
		ModelName:          modelName,
		Modal:              model.ModelModalText,
		PricingMode:        cfg.Mode,
		PricingConfig:      configJSON,
		PricingUpdatedTime: common.GetTimestamp(),
		Status:             1,
		SyncOfficial:       0,
	}).Error)
}

func newPriceHelperTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	return ctx
}

func newPriceHelperRelayInfo(modelName string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: modelName,
		UserGroup:       "default",
		UsingGroup:      "default",
	}
}

func withModelPriceSettingForHelperTest(t *testing.T, jsonValue string) {
	t.Helper()

	previous := ratio_setting.ModelPrice2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(jsonValue))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(previous))
	})
}

func withModelRatioSettingForHelperTest(t *testing.T, jsonValue string) {
	t.Helper()

	previous := ratio_setting.ModelRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(jsonValue))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previous))
	})
}

func TestModelPriceHelperTieredPreConsumeMaxTokensFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":    `{"tiered-fallback-model":"tiered_expr"}`,
		"billing_setting.billing_expr":    `{"tiered-fallback-model":"tier(\"base\", p * 3 + c * 15)"}`,
		"group_ratio_setting.group_ratio": `{"default":1,"free":0}`,
	}))

	const promptTokens = 1000

	cases := []struct {
		name      string
		group     string
		maxTokens int
		expected  int
	}{
		{
			// max_tokens omitted in a paid group -> fall back to 8192 completion tokens.
			// p*3 + c*15 = 1000*3 + 8192*15 = 125880 -> /1e6 * QuotaPerUnit(CNY=1e5) = 12588
			name:      "non-free group falls back to 8192 completion tokens",
			group:     "default",
			maxTokens: 0,
			expected:  12588,
		},
		{
			// explicit max_tokens is used verbatim, no fallback.
			// 1000*3 + 100*15 = 4500 -> /1e6 * QuotaPerUnit(CNY=1e5) = 450
			name:      "explicit max_tokens is used verbatim",
			group:     "default",
			maxTokens: 100,
			expected:  450,
		},
		{
			// free group (ratio 0) stays zero; fallback is gated on non-zero group ratio.
			name:      "free group stays zero without fallback",
			group:     "free",
			maxTokens: 0,
			expected:  0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			req.Header.Set("Content-Type", "application/json")
			ctx.Request = req
			ctx.Set("group", tc.group)

			info := &relaycommon.RelayInfo{
				OriginModelName: "tiered-fallback-model",
				UserGroup:       tc.group,
				UsingGroup:      tc.group,
				RequestHeaders:  map[string]string{"Content-Type": "application/json"},
				BillingRequestInput: &billingexpr.RequestInput{
					Headers: map[string]string{"Content-Type": "application/json"},
					Body:    []byte(`{}`),
				},
			}

			priceData, err := ModelPriceHelper(ctx, info, promptTokens, &types.TokenCountMeta{MaxTokens: tc.maxTokens})
			require.NoError(t, err)
			require.Equal(t, tc.expected, priceData.QuotaToPreConsume)
		})
	}
}

func TestModelPriceHelperTieredRejectsPreConsumeOverflow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":    `{"tiered-overflow-model":"tiered_expr"}`,
		"billing_setting.billing_expr":    `{"tiered-overflow-model":"tier(\"overflow\", p * 1000000000000000)"}`,
		"group_ratio_setting.group_ratio": `{"default":1}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("group", "default")
	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-overflow-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		BillingRequestInput: &billingexpr.RequestInput{
			Body: []byte(`{}`),
		},
	}

	_, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})

	var clamp *common.QuotaClamp
	require.ErrorAs(t, err, &clamp)
	require.Equal(t, "QuotaRound", clamp.Op)
	require.Equal(t, common.QuotaClampOverflow, clamp.Kind)
}

func TestModelPriceHelperRequestBillingRatiosOnlyApplyToFixedPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	savedModelPrices := ratio_setting.ModelPrice2JSONString()
	savedModelRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrices))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedModelRatios))
	})

	modelPrices, err := common.Marshal(map[string]float64{
		"fixed-image-price":      0.04,
		"fractional-image-price": 0.0000012,
		"overflow-image-price":   float64(common.MaxQuota) / common.QuotaPerUnit / 2,
	})
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(modelPrices)))
	modelRatios, err := common.Marshal(map[string]float64{"ratio-image-price": 15})
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(modelRatios)))

	tests := []struct {
		name           string
		model          string
		wantQuota      int
		wantUsePrice   bool
		wantImageCount bool
	}{
		{
			name:           "fixed price applies image count",
			model:          "fixed-image-price",
			wantQuota:      int(0.04 * common.QuotaPerUnit * 3 * 3), // price×unit(CNY=1e5)×imageRatio×n
			wantUsePrice:   true,
			wantImageCount: true,
		},
		{
			name:         "ratio price ignores request billing ratios",
			model:        "ratio-image-price",
			wantQuota:    15000,
			wantUsePrice: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Set("group", "default")
			info := &relaycommon.RelayInfo{
				OriginModelName: tt.model,
				UserGroup:       "default",
				UsingGroup:      "default",
			}
			meta := &types.TokenCountMeta{
				ImagePriceRatio: 3,
				BillingRatios:   map[string]float64{"n": 3},
			}

			priceData, err := ModelPriceHelper(ctx, info, 1000, meta)

			require.NoError(t, err)
			require.Equal(t, tt.wantQuota, priceData.QuotaToPreConsume)
			require.Equal(t, tt.wantUsePrice, priceData.UsePrice)
			require.Equal(t, tt.wantImageCount, priceData.HasOtherRatio("n"))
			require.Equal(t, priceData.OtherRatios(), info.PriceData.OtherRatios())
		})
	}

	newInfo := func(model string) (*gin.Context, *relaycommon.RelayInfo) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Set("group", "default")
		return ctx, &relaycommon.RelayInfo{
			OriginModelName: model,
			UserGroup:       "default",
			UsingGroup:      "default",
		}
	}
	meta := &types.TokenCountMeta{BillingRatios: map[string]float64{"n": 3}}

	ctx, info := newInfo("fractional-image-price")
	priceData, err := ModelPriceHelper(ctx, info, 0, meta)
	require.NoError(t, err)
	// 0.0000012 * QuotaPerUnit(CNY=1e5) * 3 = 0.36, truncate once to 0.
	require.Equal(t, int(0.0000012*common.QuotaPerUnit*3), priceData.QuotaToPreConsume)

	ctx, info = newInfo("overflow-image-price")
	_, err = ModelPriceHelper(ctx, info, 0, meta)
	var clamp *common.QuotaClamp
	require.ErrorAs(t, err, &clamp)
	require.Equal(t, "QuotaFromFloat", clamp.Op)
	require.Equal(t, common.QuotaClampOverflow, clamp.Kind)
	require.Nil(t, info.Billing)
}
