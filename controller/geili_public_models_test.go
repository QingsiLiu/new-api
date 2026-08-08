package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelRegistryTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv(common.CreditsFeatureFlagEnv, "false")
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMapRWMutex.Unlock()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.ModelRegistry{}, &model.TextCategoryPricing{}, &model.Model{}, &model.Option{}))

	// 公开端点缓存为包级状态：逐测试清空，避免跨用例串味
	invalidateGeiliPublicModelCache()
	t.Cleanup(invalidateGeiliPublicModelCache)

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func seedModelRegistryFixtures(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.ModelRegistry{
		ModelName:       "seedance-2.0",
		Slug:            "seedance-2-0",
		DisplayNameZh:   "Seedance 2.0",
		DisplayNameEn:   "Seedance 2.0",
		Aliases:         `["Seedance V2"]`,
		Vendor:          "bytedance",
		VendorDisplayZh: "字节跳动",
		VendorDisplayEn: "ByteDance",
		Modality:        "video",
		CapabilityTags:  `["text-to-video","image-to-video"]`,
		OfficialPrice:   `{"unit":"per_second","items":[{"spec":"720p","usd":0.25}]}`,
		ExampleParams:   `{"resolution":"720p","duration":5}`,
		FaqZh:           `[{"q":"如何计费？","a":"按秒计费。"}]`,
		FaqEn:           `[{"q":"How is it billed?","a":"Per second."}]`,
		SeoZh:           "# Seedance 2.0 中文长文",
		SeoEn:           "# Seedance 2.0 English",
		Enabled:         true,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ModelRegistry{
		ModelName: "disabled-model",
		Slug:      "disabled-model",
		Modality:  "image",
		Enabled:   false,
	}).Error)

	originalPricing := operation_setting.AsyncSpecPricing2JSONString()
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(originalPricing))
	})
	require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(`{
		"currency":"CNY",
		"video":{"seedance-2.0":{"unit":"per_second","prices":{
			"720p":{"16:9":{"no_video_input":{"cny_per_second":1.0433},"with_video_input":{"cny_per_second":0.635}}},
			"480p":{"16:9":{"no_video_input":{"cny_per_second":0.4851}}}
		}}}
	}`))
}

// 白名单：公开响应只允许出现这些顶层字段（新增字段须先过防泄露评审再入名单）。
var publicModelAllowedKeys = map[string]bool{
	"slug": true, "model": true, "modality": true, "vendor": true,
	"display_name": true, "vendor_display": true, "aliases": true,
	"capability_tags": true, "price_unit": true, "price_from_cny": true,
	"price_from_credits": true, "price_from_quota": true, "pricing_source": true, "credits_pricing": true,
	"official_price": true, "params_schema": true, "example_params": true,
	"faq": true, "seo": true, "spec_pricing": true,
	"text_category": true, "category_multiplier": true,
}

// 黑名单：任何渠道/上游/密钥痕迹出现在公开响应即失败。
var publicModelForbiddenSubstrings = []string{
	`"channel`, `base_url`, `api_key`, `upstream`, `"secret`, `"token`, `"priority`, `"weight`,
}

func assertNoLeak(t *testing.T, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	for _, forbidden := range publicModelForbiddenSubstrings {
		require.NotContains(t, lower, forbidden, "公开模型 API 响应泄露了内部字段: %s", forbidden)
	}
}

func getPublicModelListForTest(t *testing.T) []map[string]interface{} {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models", nil)
	GetPublicModels(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	assertNoLeak(t, rec.Body.String())
	var response struct {
		Success bool                     `json:"success"`
		Data    []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response.Data
}

func getPublicModelDetailForTest(t *testing.T, slug string) map[string]interface{} {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models/"+slug, nil)
	ctx.Params = gin.Params{{Key: "slug", Value: slug}}
	GetPublicModelBySlug(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	assertNoLeak(t, rec.Body.String())
	var response struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response.Data
}

func requirePublicCreditsSpec(
	t *testing.T,
	detail map[string]interface{},
	key string,
	credits string,
	source string,
) map[string]interface{} {
	t.Helper()
	pricing, ok := detail["credits_pricing"].(map[string]interface{})
	require.True(t, ok, "credits_pricing must be an object")
	specs, ok := pricing["specs"].([]interface{})
	require.True(t, ok, "credits_pricing.specs must be an array")
	for _, item := range specs {
		spec := item.(map[string]interface{})
		if spec["key"] == key {
			require.Equal(t, credits, spec["credits"])
			require.Equal(t, source, spec["pricing_source"])
			quota := int(spec["quota"].(float64))
			require.Equal(t, credits, common.QuotaToCreditsString(quota))
			return spec
		}
	}
	require.FailNow(t, "missing public Credits specification", "key=%s specs=%v", key, specs)
	return nil
}

func TestGetPublicModelsListOnlyEnabledAndNoLeak(t *testing.T) {
	setupModelRegistryTestDB(t)
	seedModelRegistryFixtures(t)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models", nil)
	GetPublicModels(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	assertNoLeak(t, rec.Body.String())

	var resp struct {
		Success bool                     `json:"success"`
		Data    []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Len(t, resp.Data, 1, "disabled 条目不得出现在公开列表")

	item := resp.Data[0]
	require.Equal(t, "seedance-2-0", item["slug"])
	require.Equal(t, "per_second", item["price_unit"])
	require.InDelta(t, 0.4851, item["price_from_cny"].(float64), 1e-9, "price_from_cny 应为全规格最低价")
	for key := range item {
		require.True(t, publicModelAllowedKeys[key], "公开列表出现白名单外字段: %s", key)
	}
}

func TestGetPublicModelBySlugDetailAndNoLeak(t *testing.T) {
	setupModelRegistryTestDB(t)
	seedModelRegistryFixtures(t)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models/seedance-2-0", nil)
	ctx.Params = gin.Params{{Key: "slug", Value: "seedance-2-0"}}
	GetPublicModelBySlug(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	assertNoLeak(t, rec.Body.String())

	var resp struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	for key := range resp.Data {
		require.True(t, publicModelAllowedKeys[key], "公开详情出现白名单外字段: %s", key)
	}
	seo := resp.Data["seo"].(map[string]interface{})
	require.Contains(t, seo["zh"], "Seedance 2.0")
	require.NotNil(t, resp.Data["spec_pricing"], "详情应包含我方规格价")

	// 未启用/不存在 slug → 404
	rec404 := httptest.NewRecorder()
	ctx404, _ := gin.CreateTestContext(rec404)
	ctx404.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models/disabled-model", nil)
	ctx404.Params = gin.Params{{Key: "slug", Value: "disabled-model"}}
	GetPublicModelBySlug(ctx404)
	require.Equal(t, http.StatusNotFound, rec404.Code)
}

func TestPublicModelCreditsContractIsAdditiveAndCacheSeparatesFeatureFlag(t *testing.T) {
	setupModelRegistryTestDB(t)
	seedModelRegistryFixtures(t)

	legacy := getPublicModelListForTest(t)
	require.Len(t, legacy, 1)
	require.NotContains(t, legacy[0], "price_from_credits")
	require.NotContains(t, legacy[0], "pricing_source")
	legacyDetail := getPublicModelDetailForTest(t, "seedance-2-0")
	require.NotContains(t, legacyDetail, "credits_pricing")

	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	withCredits := getPublicModelListForTest(t)
	require.Len(t, withCredits, 1)
	require.Equal(t, "11.5", withCredits[0]["price_from_credits"])
	require.EqualValues(t, 41400, withCredits[0]["price_from_quota"])
	require.Equal(t, "kie", withCredits[0]["pricing_source"])
	detail := getPublicModelDetailForTest(t, "seedance-2-0")
	require.Equal(t, "11.5", detail["price_from_credits"])
	require.EqualValues(t, 41400, detail["price_from_quota"])
	requirePublicCreditsSpec(t, detail, "480p:no_video_input", "19", "kie")
	requirePublicCreditsSpec(t, detail, "480p:with_video_input", "11.5", "kie")
	requirePublicCreditsSpec(t, detail, "720p:no_video_input", "41", "kie")
	requirePublicCreditsSpec(t, detail, "720p:with_video_input", "25", "kie")
	requirePublicCreditsSpec(t, detail, "1080p:no_video_input", "102", "kie")
	requirePublicCreditsSpec(t, detail, "1080p:with_video_input", "62", "kie")
	requirePublicCreditsSpec(t, detail, "4k:no_video_input", "208", "kie")
	requirePublicCreditsSpec(t, detail, "4k:with_video_input", "128", "kie")
}

func TestPublicModelSeedanceExactCreditsDoNotDependOnLegacyCNYTable(t *testing.T) {
	setupModelRegistryTestDB(t)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	require.NoError(t, model.DB.Create(&model.ModelRegistry{
		ModelName: "seedance-2.0-mini", Slug: "seedance-2-0-mini", Modality: "video",
		DisplayNameEn: "Seedance 2.0 Mini", Enabled: true,
	}).Error)

	originalPricing := operation_setting.AsyncSpecPricing2JSONString()
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(originalPricing))
	})
	require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(`{"currency":"CNY"}`))

	detail := getPublicModelDetailForTest(t, "seedance-2-0-mini")
	require.Equal(t, "6", detail["price_from_credits"])
	require.Equal(t, "kie", detail["pricing_source"])
	requirePublicCreditsSpec(t, detail, "480p:no_video_input", "9.5", "kie")
	requirePublicCreditsSpec(t, detail, "480p:with_video_input", "6", "kie")
	requirePublicCreditsSpec(t, detail, "720p:no_video_input", "20.5", "kie")
	requirePublicCreditsSpec(t, detail, "720p:with_video_input", "12.5", "kie")
}

func TestPublicModelSeedanceCompactPricingOmitsAspectRatio(t *testing.T) {
	setupModelRegistryTestDB(t)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	require.NoError(t, model.DB.Create(&model.ModelRegistry{
		ModelName: "seedance-1.5-pro", Slug: "seedance-1-5-pro", Modality: "video",
		DisplayNameEn: "Seedance 1.5 Pro", Enabled: true,
	}).Error)

	originalPricing := operation_setting.AsyncSpecPricing2JSONString()
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(originalPricing))
	})
	require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(operation_setting.AsyncSpecPricingSeedJSONString()))

	detail := getPublicModelDetailForTest(t, "seedance-1-5-pro")
	require.InDelta(t, 0.0591, detail["price_from_cny"].(float64), 1e-9)
	require.Equal(t, "1.641667", detail["price_from_credits"])
	require.Equal(t, "geili", detail["pricing_source"])
	requirePublicCreditsSpec(t, detail, "480p:image_no_audio", "1.641667", "geili")
	requirePublicCreditsSpec(t, detail, "480p:text_audio", "4.686111", "geili")
	requirePublicCreditsSpec(t, detail, "720p:text_no_audio", "5.072222", "geili")
	requirePublicCreditsSpec(t, detail, "1080p:text_audio", "22.825", "geili")
	pricing := detail["credits_pricing"].(map[string]interface{})
	specs := pricing["specs"].([]interface{})
	require.Len(t, specs, 8)
	for _, raw := range specs {
		require.NotContains(t, raw.(map[string]interface{}), "ratio")
	}
}

func TestPublicSeedanceCatalogContainsExactlyTwentyFourCompactPrices(t *testing.T) {
	setupModelRegistryTestDB(t)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	entries := []model.ModelRegistry{
		{ModelName: "seedance-1.5-pro", Slug: "seedance-1-5-pro", Modality: "video", DisplayNameEn: "Seedance 1.5 Pro", Enabled: true},
		{ModelName: "seedance-2.0", Slug: "seedance-2-0", Modality: "video", DisplayNameEn: "Seedance 2.0", Enabled: true},
		{ModelName: "seedance-2.0-fast", Slug: "seedance-2-0-fast", Modality: "video", DisplayNameEn: "Seedance 2.0 Fast", Enabled: true},
		{ModelName: "seedance-2.0-mini", Slug: "seedance-2-0-mini", Modality: "video", DisplayNameEn: "Seedance 2.0 Mini", Enabled: true},
	}
	require.NoError(t, model.DB.Create(&entries).Error)

	originalPricing := operation_setting.AsyncSpecPricing2JSONString()
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(originalPricing))
	})
	require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(operation_setting.AsyncSpecPricingSeedJSONString()))

	expectedCounts := map[string]int{
		"seedance-1-5-pro":  8,
		"seedance-2-0":      8,
		"seedance-2-0-fast": 4,
		"seedance-2-0-mini": 4,
	}
	total := 0
	for slug, expected := range expectedCounts {
		detail := getPublicModelDetailForTest(t, slug)
		pricing := detail["credits_pricing"].(map[string]interface{})
		specs := pricing["specs"].([]interface{})
		require.Len(t, specs, expected, slug)
		total += len(specs)
		for _, raw := range specs {
			spec := raw.(map[string]interface{})
			require.NotContains(t, spec, "ratio", slug)
			require.NotContains(t, spec["key"].(string), "16:9", slug)
			require.NotContains(t, spec["key"].(string), "21:9", slug)
		}
	}
	require.Equal(t, 24, total)
}

func TestFinalizePublicCreditsPricingUsesMinimumSpecificationSource(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	kieMinimum := finalizePublicCreditsPricing("per_image", []quotaPriceSpec{
		makeQuotaPriceSpec("kie", 3600, "kie"),
		makeQuotaPriceSpec("geili", 7200, "geili"),
	})
	require.Equal(t, "1", kieMinimum.PriceFromCredits)
	require.Equal(t, "kie", kieMinimum.PricingSource)

	geiliMinimum := finalizePublicCreditsPricing("per_image", []quotaPriceSpec{
		makeQuotaPriceSpec("kie", 7200, "kie"),
		makeQuotaPriceSpec("geili", 1800, "geili"),
	})
	require.Equal(t, "0.5", geiliMinimum.PriceFromCredits)
	require.Equal(t, "geili", geiliMinimum.PricingSource)
}

func TestPublicModelCreditsUseExactOfficialTargetPrices(t *testing.T) {
	setupModelRegistryTestDB(t)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")

	entries := []model.ModelRegistry{
		{
			ModelName: "gpt-image-2", Slug: "gpt-image-2", Modality: "image",
			DisplayNameEn: "GPT Image 2", Enabled: true,
		},
		{
			ModelName: "gpt-5.5", Slug: "gpt-5-5", Modality: "text",
			DisplayNameEn: "GPT-5.5", TextCategory: "gpt",
			OfficialPrice: `{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":999,"cached_input":999,"output":999},"source_url":"https://example.com/pricing"}`,
			Enabled:       true,
		},
		{
			ModelName: "gpt-5.6-luna", Slug: "gpt-5-6-luna", Modality: "text",
			DisplayNameEn: "GPT-5.6 Luna", TextCategory: "gpt",
			OfficialPrice: `{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":999,"cached_input":999,"output":999},"source_url":"https://example.com/pricing"}`,
			Enabled:       true,
		},
		{
			ModelName: "claude-opus-5", Slug: "claude-opus-5", Modality: "text",
			DisplayNameEn: "Claude Opus 5", TextCategory: "claude",
			OfficialPrice: `{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":999,"cache_read":999,"cache_write_5m":999,"cache_write_1h":999,"output":999},"source_url":"https://example.com/pricing"}`,
			Enabled:       true,
		},
	}
	require.NoError(t, model.DB.Create(&entries).Error)

	image := getPublicModelDetailForTest(t, "gpt-image-2")
	require.Equal(t, "3", image["price_from_credits"])
	require.EqualValues(t, 10800, image["price_from_quota"])
	require.Equal(t, "kie", image["pricing_source"])
	requirePublicCreditsSpec(t, image, "resolution:1k", "3", "kie")
	requirePublicCreditsSpec(t, image, "resolution:2k", "5", "kie")
	requirePublicCreditsSpec(t, image, "resolution:4k", "8", "kie")

	text := getPublicModelDetailForTest(t, "gpt-5-5")
	require.Equal(t, "5", text["price_from_credits"])
	require.Equal(t, "geili", text["pricing_source"])
	requirePublicCreditsSpec(t, text, "short:input", "50", "geili")
	requirePublicCreditsSpec(t, text, "short:cached_input", "5", "geili")
	requirePublicCreditsSpec(t, text, "short:output", "300", "geili")
	longInput := requirePublicCreditsSpec(t, text, "long:input", "100", "geili")
	require.EqualValues(t, 272001, int(longInput["min_prompt_tokens"].(float64)))

	luna := getPublicModelDetailForTest(t, "gpt-5-6-luna")
	requirePublicCreditsSpec(t, luna, "short:cached_input", "0.2", "geili")
	requirePublicCreditsSpec(t, luna, "short:cache_write", "2.5", "geili")
	requirePublicCreditsSpec(t, luna, "short:input", "2", "geili")
	requirePublicCreditsSpec(t, luna, "short:output", "12", "geili")

	opus := getPublicModelDetailForTest(t, "claude-opus-5")
	requirePublicCreditsSpec(t, opus, "cached_input", "22", "geili")
	requirePublicCreditsSpec(t, opus, "cache_write_5m", "275", "geili")
	requirePublicCreditsSpec(t, opus, "cache_write_1h", "440", "geili")
	requirePublicCreditsSpec(t, opus, "input", "220", "geili")
	requirePublicCreditsSpec(t, opus, "output", "1100", "geili")

	list := getPublicModelListForTest(t)
	var listedLuna map[string]interface{}
	for _, item := range list {
		if item["slug"] == "gpt-5-6-luna" {
			listedLuna = item
			break
		}
	}
	require.NotNil(t, listedLuna)
	requirePublicCreditsSpec(t, listedLuna, "short:cached_input", "0.2", "geili")
}

func TestPublicModelExactTextCacheFallsBackToInputSettlementRate(t *testing.T) {
	setupModelRegistryTestDB(t)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	require.NoError(t, model.DB.Create(&model.ModelRegistry{
		ModelName: "gpt-5.4-pro", Slug: "gpt-5-4-pro", Modality: "text",
		DisplayNameEn: "GPT-5.4 Pro", TextCategory: "gpt",
		OfficialPrice: `{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":999,"output":999},"source_url":"https://example.com/pricing"}`,
		Enabled:       true,
	}).Error)

	detail := getPublicModelDetailForTest(t, "gpt-5-4-pro")
	input := requirePublicCreditsSpec(t, detail, "short:input", "300", "geili")
	cached := requirePublicCreditsSpec(t, detail, "short:cached_input", "300", "geili")
	require.Equal(t, input["quota"], cached["quota"], "settlement charges undiscounted cache tokens at the input rate")
	requirePublicCreditsSpec(t, detail, "short:output", "1800", "geili")
}

func TestPublicModelCreditsProjectGeiliFallbackFromAuthoritativeQuota(t *testing.T) {
	setupModelRegistryTestDB(t)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")

	originalPricing := operation_setting.AsyncSpecPricing2JSONString()
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(originalPricing))
	})
	require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(`{
		"currency":"CNY",
		"image":{"seedream-5.0":{"unit":"per_image","resolutions":{
			"2k":{"cny_per_image":0.72},"4k":{"cny_per_image":1.44}
		}}}
	}`))

	entries := []model.ModelRegistry{
		{
			ModelName: "seedream-5.0", Slug: "seedream-5-0", Modality: "image",
			DisplayNameEn: "Seedream 5.0", Enabled: true,
		},
		{
			ModelName: "gpt-5.4-mini", Slug: "gpt-5-4-mini", Modality: "text",
			DisplayNameEn: "GPT-5.4 mini", TextCategory: "gpt",
			OfficialPrice: `{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":0.25,"cached_input":0.025,"output":2},"source_url":"https://example.com/pricing"}`,
			Enabled:       true,
		},
	}
	require.NoError(t, model.DB.Create(&entries).Error)
	require.NoError(t, model.DB.Create(&model.TextCategoryPricing{
		Category: "gpt", Multiplier: 0.8,
	}).Error)

	image := getPublicModelDetailForTest(t, "seedream-5-0")
	require.Equal(t, "20", image["price_from_credits"])
	require.Equal(t, "geili", image["pricing_source"])
	requirePublicCreditsSpec(t, image, "resolution:2k", "20", "geili")
	requirePublicCreditsSpec(t, image, "resolution:4k", "40", "geili")

	text := getPublicModelDetailForTest(t, "gpt-5-4-mini")
	require.Equal(t, "0.75", text["price_from_credits"])
	require.Equal(t, "geili", text["pricing_source"])
	requirePublicCreditsSpec(t, text, "input", "7.5", "geili")
	requirePublicCreditsSpec(t, text, "cached_input", "0.75", "geili")
	requirePublicCreditsSpec(t, text, "output", "45", "geili")
}

func TestPublicModelCreditsListPreservesTwentyOneModelCatalog(t *testing.T) {
	setupModelRegistryTestDB(t)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")

	entries := make([]model.ModelRegistry, 0, 21)
	for index := 0; index < 21; index++ {
		entries = append(entries, model.ModelRegistry{
			ModelName: fmt.Sprintf("catalog-model-%02d", index),
			Slug:      fmt.Sprintf("catalog-model-%02d", index),
			Modality:  "image",
			Enabled:   true,
		})
	}
	entries[0].ModelName = "gpt-image-2"
	entries[0].Slug = "gpt-image-2"
	entries[1].ModelName = "gpt-5.4-mini"
	entries[1].Slug = "gpt-5-4-mini"
	entries[1].Modality = "text"
	entries[1].TextCategory = "gpt"
	entries[1].OfficialPrice = `{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":0.25,"output":2},"source_url":"https://example.com/pricing"}`
	require.NoError(t, model.DB.Create(&entries).Error)
	require.NoError(t, model.DB.Create(&model.TextCategoryPricing{
		Category: "gpt", Multiplier: 0.8,
	}).Error)

	list := getPublicModelListForTest(t)
	require.Len(t, list, 21)
	byModel := make(map[string]map[string]interface{}, len(list))
	for _, item := range list {
		byModel[item["model"].(string)] = item
		if _, hasCredits := item["price_from_credits"]; hasCredits {
			require.NotEmpty(t, item["price_unit"], "Credits-priced list entries must publish their billing unit")
		}
		for key := range item {
			require.True(t, publicModelAllowedKeys[key], "公开列表出现白名单外字段: %s", key)
		}
	}
	require.Equal(t, "3", byModel["gpt-image-2"]["price_from_credits"])
	require.Equal(t, "kie", byModel["gpt-image-2"]["pricing_source"])
	require.Equal(t, "0.75", byModel["gpt-5.4-mini"]["price_from_credits"])
	require.Equal(t, "geili", byModel["gpt-5.4-mini"]["pricing_source"])
	require.Equal(t, "per_1M_tokens", byModel["gpt-5.4-mini"]["price_unit"])
}

func TestAdminUpsertModelRegistryIdempotent(t *testing.T) {
	setupModelRegistryTestDB(t)

	payload := `{
		"model": "gpt-image-2",
		"slug": "gpt-image-2",
		"display_name": {"zh": "GPT Image 2", "en": "GPT Image 2"},
		"vendor": "openai",
		"vendor_display": {"zh": "OpenAI", "en": "OpenAI"},
		"modality": "image",
		"capability_tags": ["text-to-image", "image-editing"],
		"seo_zh": "第一版"
	}`
	doUpsert := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/geili/model-registry", strings.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		AdminUpsertModelRegistry(ctx)
		return rec
	}

	require.Equal(t, http.StatusOK, doUpsert(payload).Code)
	require.Equal(t, http.StatusOK, doUpsert(strings.Replace(payload, "第一版", "第二版", 1)).Code)

	entries, err := model.GetAllModelRegistries()
	require.NoError(t, err)
	require.Len(t, entries, 1, "重复 upsert 不得产生重复行")
	require.Equal(t, "第二版", entries[0].SeoZh)
	require.Equal(t, `["text-to-image","image-editing"]`, entries[0].CapabilityTags)

	// modality 校验
	bad := doUpsert(strings.Replace(payload, `"modality": "image"`, `"modality": "audio"`, 1))
	require.Contains(t, bad.Body.String(), "modality")
}

func TestAdminUpsertTextModelRegistryIgnoresManualPricingFields(t *testing.T) {
	setupModelRegistryTestDB(t)

	payload := `{
		"model":"gpt-5.5",
		"slug":"gpt-5-5",
		"display_name":{"zh":"GPT-5.5","en":"GPT-5.5"},
		"vendor":"openai",
		"vendor_display":{"zh":"OpenAI","en":"OpenAI"},
		"modality":"text",
		"text_category":"gpt",
		"capability_tags":["chat","reasoning"],
		"official_price":{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":5,"cached_input":0.5,"output":30},"source_url":"https://developers.openai.com/api/docs/pricing"},
		"enabled":true
	}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/geili/model-registry", strings.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	AdminUpsertModelRegistry(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"success":true`)
	entries, err := model.GetAllModelRegistries()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Empty(t, entries[0].TextCategory)
	require.Empty(t, entries[0].OfficialPrice)

	detailRec := httptest.NewRecorder()
	detailCtx, _ := gin.CreateTestContext(detailRec)
	detailCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models/gpt-5-5", nil)
	detailCtx.Params = gin.Params{{Key: "slug", Value: "gpt-5-5"}}
	GetPublicModelBySlug(detailCtx)

	require.Equal(t, http.StatusOK, detailRec.Code)
	assertNoLeak(t, detailRec.Body.String())
	var detail struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(detailRec.Body.Bytes(), &detail))
	require.Equal(t, "text", detail.Data["modality"])
	require.NotContains(t, detail.Data, "text_category")
	require.NotContains(t, detail.Data, "pricing", "公开响应不得持久化或手抄成品价")
	require.NotContains(t, detail.Data, "price_from_cny", "类目倍率未拍板时不得推导起价")
	require.NotContains(t, detail.Data, "category_multiplier", "未配置类目倍率时应保持空值")
}

func TestPublicModelActivePricingUsesUnifiedMetadataResolver(t *testing.T) {
	setupModelRegistryTestDB(t)
	restoreTrust := model.SetModelPricingConfigTrustedForTest(true)
	t.Cleanup(restoreTrust)
	require.NoError(t, model.DB.Create(&model.Model{
		ModelName:        "gpt-5.5",
		Modal:            model.ModelModalText,
		TextCategory:     "gpt",
		OfficialPriceKey: "openai.gpt-5.5",
		Status:           1,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TextCategoryPricing{
		Category:   "gpt",
		Multiplier: 0.1,
	}).Error)
	require.NoError(t, model.SetTextPricingMode(model.TextPricingModeActive))
	t.Cleanup(func() {
		_ = model.SetTextPricingMode(model.TextPricingModeLegacy)
	})
	require.NoError(t, model.DB.Create(&model.ModelRegistry{
		ModelName:     "gpt-5.5",
		Slug:          "gpt-5-5",
		Modality:      "text",
		TextCategory:  "grok",
		OfficialPrice: `{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":999,"output":999},"source_url":"https://example.com"}`,
		Enabled:       true,
	}).Error)

	detail := getPublicModelDetailForTest(t, "gpt-5-5")
	require.Equal(t, "gpt", detail["text_category"])
	require.InDelta(t, 0.1, detail["category_multiplier"].(float64), 1e-9)
	official := detail["official_price"].(map[string]interface{})
	require.Equal(t, "openai.gpt-5.5", official["key"])
	dimensions := official["dimensions"].(map[string]interface{})
	require.InDelta(t, 5, dimensions["input"].(float64), 1e-9)
	require.InDelta(t, 30, dimensions["output"].(float64), 1e-9)
}

func TestAdminUpsertTextCategoryPricingRejectsUnsetSentinelsAndOutOfRangeValues(t *testing.T) {
	setupModelRegistryTestDB(t)

	for name, payload := range map[string]string{
		"missing":  `{"category":"gpt"}`,
		"zero":     `{"category":"gpt","multiplier":0}`,
		"negative": `{"category":"gpt","multiplier":-0.1}`,
		"over one": `{"category":"gpt","multiplier":1.1}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodPut, "/api/geili/text-category-pricing", strings.NewReader(payload))
			ctx.Request.Header.Set("Content-Type", "application/json")
			AdminUpsertTextCategoryPricing(ctx)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Contains(t, rec.Body.String(), `"success":false`)
		})
	}
}

func TestTextDraftUpsertStaysInvisibleFromPublicEndpoints(t *testing.T) {
	setupModelRegistryTestDB(t)

	payload := `{
		"model":"draft-text-model",
		"slug":"draft-text-model",
		"modality":"text",
		"text_category":"gpt",
		"official_price":{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":1,"output":4},"source_url":"https://example.com"},
		"enabled":false
	}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/geili/model-registry", strings.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	AdminUpsertModelRegistry(ctx)
	require.Contains(t, rec.Body.String(), `"success":true`)

	listRec := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRec)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models", nil)
	GetPublicModels(listCtx)
	require.NotContains(t, listRec.Body.String(), "draft-text-model")

	detailRec := httptest.NewRecorder()
	detailCtx, _ := gin.CreateTestContext(detailRec)
	detailCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models/draft-text-model", nil)
	detailCtx.Params = gin.Params{{Key: "slug", Value: "draft-text-model"}}
	GetPublicModelBySlug(detailCtx)
	require.Equal(t, http.StatusNotFound, detailRec.Code)
}

func TestPublicModelsCacheTTLAndAdminInvalidation(t *testing.T) {
	setupModelRegistryTestDB(t)
	seedModelRegistryFixtures(t)

	doList := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models", nil)
		GetPublicModels(ctx)
		return rec
	}

	// 首次请求：填缓存 + 带 Cache-Control
	first := doList()
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, "public, max-age=60", first.Header().Get("Cache-Control"))

	// 绕过管理端直改 DB：TTL 内应命中缓存返回旧 body（DB 零查询语义的可观测面）
	require.NoError(t, model.DB.Model(&model.ModelRegistry{}).
		Where("slug = ?", "seedance-2-0").
		Update("display_name_zh", "缓存期内不应看到我").Error)
	stale := doList()
	require.Equal(t, first.Body.String(), stale.Body.String(), "TTL 内应返回缓存 body")
	require.NotContains(t, stale.Body.String(), "缓存期内不应看到我")

	// 管理端写操作 → 主动失效 → 下一次请求返回新数据
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/geili/model-registry/disabled-model", nil)
	ctx.Params = gin.Params{{Key: "model", Value: "disabled-model"}}
	AdminDeleteModelRegistry(ctx)
	require.Equal(t, http.StatusOK, rec.Code)

	fresh := doList()
	require.Contains(t, fresh.Body.String(), "缓存期内不应看到我", "管理端写后缓存应失效")
}

func TestPublicModelDetailCacheBysSlug(t *testing.T) {
	setupModelRegistryTestDB(t)
	seedModelRegistryFixtures(t)

	doDetail := func(slug string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models/"+slug, nil)
		ctx.Params = gin.Params{{Key: "slug", Value: slug}}
		GetPublicModelBySlug(ctx)
		return rec
	}

	first := doDetail("seedance-2-0")
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, "public, max-age=60", first.Header().Get("Cache-Control"))

	require.NoError(t, model.DB.Model(&model.ModelRegistry{}).
		Where("slug = ?", "seedance-2-0").
		Update("seo_zh", "详情缓存期内不应看到我").Error)
	stale := doDetail("seedance-2-0")
	require.Equal(t, first.Body.String(), stale.Body.String())

	// 404 不缓存：未知 slug 每次都查 DB（限流兜底），不留负缓存
	miss := doDetail("no-such-model")
	require.Equal(t, http.StatusNotFound, miss.Code)
}
