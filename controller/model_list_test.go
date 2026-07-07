package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var testDBSerial uint64

type listModelsResponse struct {
	Success bool               `json:"success"`
	Data    []dto.OpenAIModels `json:"data"`
	Object  string             `json:"object"`
}

type adminModelsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Items []model.Model `json:"items"`
		Total int           `json:"total"`
	} `json:"data"`
}

type pricingParityResponse struct {
	Success bool                      `json:"success"`
	Data    model.PricingParityStatus `json:"data"`
}

func setupModelListControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	initModelListColumnNames(t)

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), atomic.AddUint64(&testDBSerial, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))
	require.NoError(t, db.AutoMigrate(&model.Option{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func initModelListColumnNames(t *testing.T) {
	t.Helper()

	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	defer func() {
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	}()

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s_init?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	require.NoError(t, os.Setenv("SQL_DSN", "local"))

	require.NoError(t, model.InitDB())
	if model.DB != nil {
		sqlDB, err := model.DB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
}

func withTieredBillingConfig(t *testing.T, modes map[string]string, exprs map[string]string) {
	t.Helper()

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		if strings.HasPrefix(key, "billing_setting.") {
			saved[key] = value
		}
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		model.InvalidatePricingCache()
	})

	modeBytes, err := common.Marshal(modes)
	require.NoError(t, err)
	exprBytes, err := common.Marshal(exprs)
	require.NoError(t, err)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": string(modeBytes),
		"billing_setting.billing_expr": string(exprBytes),
	}))
	model.InvalidatePricingCache()
}

func withSelfUseModeDisabled(t *testing.T) {
	t.Helper()

	original := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = false
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = original
	})
}

func decodeListModelsResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]struct{} {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload listModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "list", payload.Object)

	ids := make(map[string]struct{}, len(payload.Data))
	for _, item := range payload.Data {
		ids[item.Id] = struct{}{}
	}
	return ids
}

func pricingByModelName(pricings []model.Pricing) map[string]model.Pricing {
	byName := make(map[string]model.Pricing, len(pricings))
	for _, pricing := range pricings {
		byName[pricing.ModelName] = pricing
	}
	return byName
}

func TestListModelsIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-tiered-visible-model":      "tiered_expr",
		"zz-tiered-empty-expr-model":   "tiered_expr",
		"zz-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-tiered-empty-expr-model": "   ",
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1001,
		Username: "model-list-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-tiered-visible-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-empty-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-missing-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-unpriced-model", ChannelId: 1, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 1001)

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-tiered-visible-model")
	require.NotContains(t, ids, "zz-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-unpriced-model")

	pricingByName := pricingByModelName(model.GetPricing())
	visiblePricing, ok := pricingByName["zz-tiered-visible-model"]
	require.True(t, ok)
	require.Equal(t, "tiered_expr", visiblePricing.BillingMode)
	require.NotEmpty(t, visiblePricing.BillingExpr)

	emptyExprPricing, ok := pricingByName["zz-tiered-empty-expr-model"]
	require.True(t, ok)
	require.Empty(t, emptyExprPricing.BillingMode)
	require.Empty(t, emptyExprPricing.BillingExpr)

	missingExprPricing, ok := pricingByName["zz-tiered-missing-expr-model"]
	require.True(t, ok)
	require.Empty(t, missingExprPricing.BillingMode)
	require.Empty(t, missingExprPricing.BillingExpr)
}

func TestGetAllModelsMetaFiltersByModalAndPricingMode(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Model{
		{
			ModelName:     "chat-ratio-filter-model",
			Modal:         model.ModelModalText,
			PricingMode:   model.PricingModeRatio,
			PricingConfig: `{"mode":"ratio","base_ratio":1}`,
			Status:        1,
			SyncOfficial:  1,
		},
		{
			ModelName:     "image-spec-filter-model",
			Modal:         model.ModelModalImage,
			PricingMode:   model.PricingModeImageSpec,
			PricingConfig: `{"mode":"image_spec","resolutions":{"2k":{"cny_per_image":0.18}}}`,
			Status:        1,
			SyncOfficial:  1,
		},
		{
			ModelName:     "video-matrix-filter-model",
			Modal:         model.ModelModalVideo,
			PricingMode:   model.PricingModeVideoMatrix,
			PricingConfig: `{"mode":"video_matrix","prices":{}}`,
			Status:        1,
			SyncOfficial:  1,
		},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/models/?modal=image&pricing_mode=image_spec&p=1&page_size=10", nil)

	GetAllModelsMeta(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload adminModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, 1, payload.Data.Total)
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, "image-spec-filter-model", payload.Data.Items[0].ModelName)
}

func TestSearchModelsMetaReturnsAlias(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.Model{
		ModelName:    "gemini-2.5-flash-image",
		Alias:        "Nano Banana",
		Status:       1,
		SyncOfficial: 1,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/models/search?keyword=Nano&p=1&page_size=10", nil)

	SearchModelsMeta(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload adminModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, 1, payload.Data.Total)
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, "gemini-2.5-flash-image", payload.Data.Items[0].ModelName)
	require.Equal(t, "Nano Banana", payload.Data.Items[0].Alias)
}

func TestPricingIncludesAliasFromExactAndRuleMetadata(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.Channel{
		Id:     1,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Name:   "alias-channel",
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "exact-alias-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "prefix-alias-model-v2", ChannelId: 1, Enabled: true},
	}).Error)
	require.NoError(t, db.Create(&[]model.Model{
		{
			ModelName:    "exact-alias-model",
			Alias:        "Exact Display",
			Status:       1,
			SyncOfficial: 1,
			NameRule:     model.NameRuleExact,
		},
		{
			ModelName:    "prefix-alias-model",
			Alias:        "Prefix Display",
			Status:       1,
			SyncOfficial: 1,
			NameRule:     model.NameRulePrefix,
		},
	}).Error)
	model.InvalidatePricingCache()

	pricingByName := pricingByModelName(model.GetPricing())
	require.Equal(t, "Exact Display", pricingByName["exact-alias-model"].Alias)
	require.Equal(t, "Prefix Display", pricingByName["prefix-alias-model-v2"].Alias)
}

func TestPricingIncludesImageSpecPricingConfig(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.Channel{
		Id:     1,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Name:   "image-spec-channel",
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "image-spec-pricing-model",
		ChannelId: 1,
		Enabled:   true,
	}).Error)
	pricingConfig := `{"mode":"image_spec","unit":"per_image","resolutions":{"1k":{"cny_per_image":0.12},"2k":{"cny_per_image":0.18},"4k":{"cny_per_image":0.29}},"default_cny_per_image":0.12}`
	require.NoError(t, db.Create(&model.Model{
		ModelName:     "image-spec-pricing-model",
		Modal:         model.ModelModalImage,
		PricingMode:   model.PricingModeImageSpec,
		PricingConfig: pricingConfig,
		Status:        1,
		SyncOfficial:  1,
	}).Error)
	model.InvalidatePricingCache()

	pricingByName := pricingByModelName(model.GetPricing())
	pricing, ok := pricingByName["image-spec-pricing-model"]
	require.True(t, ok)
	require.Equal(t, model.PricingModeImageSpec, pricing.PricingMode)
	require.JSONEq(t, pricingConfig, pricing.PricingConfig)
}

func TestSeedInitialModelAliasesOnlyFillsEmptyExistingAliases(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Model{
		{ModelName: "gemini-2.5-flash-image", Status: 1},
		{ModelName: "gemini-3.1-flash-image-preview", Alias: "Custom Banana", Status: 1},
	}).Error)

	require.NoError(t, model.SeedInitialModelAliases())

	var seeded model.Model
	require.NoError(t, db.First(&seeded, "model_name = ?", "gemini-2.5-flash-image").Error)
	require.Equal(t, "Nano Banana", seeded.Alias)

	var custom model.Model
	require.NoError(t, db.First(&custom, "model_name = ?", "gemini-3.1-flash-image-preview").Error)
	require.Equal(t, "Custom Banana", custom.Alias)

	var count int64
	require.NoError(t, db.Model(&model.Model{}).Where("model_name = ?", "gemini-3-pro-image-preview").Count(&count).Error)
	require.Zero(t, count)
}

func TestSyncUpstreamModelsPreservesLocalAlias(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.Model{
		ModelName:    "sync-alias-model",
		Alias:        "Local Display",
		Description:  "local description",
		Status:       1,
		SyncOfficial: 1,
	}).Error)

	cacheMutex.Lock()
	etagCache = make(map[string]string)
	bodyCache = make(map[string][]byte)
	cacheMutex.Unlock()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload any
		switch r.URL.Path {
		case "/api/newapi/models.json":
			payload = gin.H{
				"success": true,
				"data": []gin.H{{
					"model_name":  "sync-alias-model",
					"description": "upstream description",
					"icon":        "Gemini",
					"tags":        "image,preview",
					"vendor_name": "Gemini",
					"name_rule":   model.NameRuleExact,
					"status":      1,
				}},
			}
		case "/api/newapi/vendors.json":
			payload = gin.H{
				"success": true,
				"data": []gin.H{{
					"name":        "Gemini",
					"description": "Gemini vendor",
					"icon":        "Gemini",
					"status":      1,
				}},
			}
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		data, err := common.Marshal(payload)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}))
	defer server.Close()
	t.Setenv("SYNC_UPSTREAM_BASE", server.URL)
	t.Setenv("SYNC_HTTP_RETRY", "1")

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/models/sync", strings.NewReader(`{"overwrite":[{"model_name":"sync-alias-model","fields":["description","icon","tags","vendor","name_rule","status"]}]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	SyncUpstreamModels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			UpdatedModels int `json:"updated_models"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, 1, payload.Data.UpdatedModels)

	var updated model.Model
	require.NoError(t, db.First(&updated, "model_name = ?", "sync-alias-model").Error)
	require.Equal(t, "Local Display", updated.Alias)
	require.Equal(t, "upstream description", updated.Description)
	require.Equal(t, "Gemini", updated.Icon)
}

func TestGetModelPricingParityReturnsSavedReport(t *testing.T) {
	report := model.PricingCompareReport{
		CheckedTextModels:        3,
		CheckedImageCombinations: 4,
		CheckedVideoCombinations: 5,
		Mismatches: []model.PricingCompareMismatch{
			{Kind: "image", Model: "gpt-image-2", SpecKey: "2k", LegacyQuota: 100, NewQuota: 101},
		},
	}
	restore := model.SetModelPricingParityForTest(report, false, "")
	t.Cleanup(restore)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/model/pricing-parity", nil)

	GetModelPricingParity(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response pricingParityResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.False(t, response.Data.Trusted)
	require.Equal(t, 3, response.Data.CheckedText)
	require.Equal(t, 4, response.Data.CheckedImage)
	require.Equal(t, 5, response.Data.CheckedVideo)
	require.Equal(t, 1, response.Data.MismatchCount)
	require.Len(t, response.Data.Mismatches, 1)
	require.Equal(t, "gpt-image-2", response.Data.Mismatches[0].Model)
}

func TestListModelsTokenLimitIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-token-tiered-visible-model":      "tiered_expr",
		"zz-token-tiered-empty-expr-model":   "tiered_expr",
		"zz-token-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-token-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-token-tiered-empty-expr-model": "",
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{
		"zz-token-tiered-visible-model":      true,
		"zz-token-tiered-empty-expr-model":   true,
		"zz-token-tiered-missing-expr-model": true,
		"zz-token-unpriced-model":            true,
	})

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-token-tiered-visible-model")
	require.NotContains(t, ids, "zz-token-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-token-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-token-unpriced-model")
}
