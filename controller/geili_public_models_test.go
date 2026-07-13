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
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.ModelRegistry{}))

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
	"official_price": true, "params_schema": true, "example_params": true,
	"faq": true, "seo": true, "spec_pricing": true,
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
