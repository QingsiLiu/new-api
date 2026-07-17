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
	require.NoError(t, db.AutoMigrate(&model.ModelRegistry{}, &model.TextCategoryPricing{}))

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

func TestAdminUpsertTextModelRegistryProjectsOfficialPricingAndSharedCategoryMultiplier(t *testing.T) {
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
	require.Equal(t, "gpt", detail.Data["text_category"])
	require.NotContains(t, detail.Data, "pricing", "公开响应不得持久化或手抄成品价")
	require.NotContains(t, detail.Data, "price_from_cny", "类目倍率未拍板时不得推导起价")
	require.NotContains(t, detail.Data, "category_multiplier", "未配置类目倍率时应保持空值")
	official := detail.Data["official_price"].(map[string]interface{})
	dimensions := official["dimensions"].(map[string]interface{})
	require.InDelta(t, 5, dimensions["input"].(float64), 1e-9)
	require.InDelta(t, 0.5, dimensions["cached_input"].(float64), 1e-9)
	require.InDelta(t, 30, dimensions["output"].(float64), 1e-9)

	multiplierRec := httptest.NewRecorder()
	multiplierCtx, _ := gin.CreateTestContext(multiplierRec)
	multiplierCtx.Request = httptest.NewRequest(http.MethodPut, "/api/geili/text-category-pricing", strings.NewReader(`{"category":"gpt","multiplier":0.8}`))
	multiplierCtx.Request.Header.Set("Content-Type", "application/json")
	AdminUpsertTextCategoryPricing(multiplierCtx)
	require.Contains(t, multiplierRec.Body.String(), `"success":true`)

	detailAfterMultiplier := httptest.NewRecorder()
	detailAfterCtx, _ := gin.CreateTestContext(detailAfterMultiplier)
	detailAfterCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models/gpt-5-5", nil)
	detailAfterCtx.Params = gin.Params{{Key: "slug", Value: "gpt-5-5"}}
	GetPublicModelBySlug(detailAfterCtx)
	var after struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(detailAfterMultiplier.Body.Bytes(), &after))
	require.InDelta(t, 0.8, after.Data["category_multiplier"].(float64), 1e-9)
	require.NotContains(t, after.Data, "pricing", "引擎只投影真源，页面负责运行时推导")
}

func TestAdminUpsertTextModelRegistryRejectsInvalidOfficialPricing(t *testing.T) {
	setupModelRegistryTestDB(t)

	cases := map[string]string{
		"missing category":       `{"model":"bad-category","slug":"bad-category","modality":"text","official_price":{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":1,"output":2},"source_url":"https://example.com"}}`,
		"missing official price": `{"model":"bad-missing","slug":"bad-missing","modality":"text","text_category":"gpt"}`,
		"missing output":         `{"model":"bad-output","slug":"bad-output","modality":"text","text_category":"gpt","official_price":{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":1},"source_url":"https://example.com"}}`,
		"negative cache":         `{"model":"bad-negative","slug":"bad-negative","modality":"text","text_category":"gpt","official_price":{"currency":"USD","unit":"per_1M_tokens","dimensions":{"input":1,"cache_read":-1,"output":2},"source_url":"https://example.com"}}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/geili/model-registry", strings.NewReader(payload))
			ctx.Request.Header.Set("Content-Type", "application/json")
			AdminUpsertModelRegistry(ctx)

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
