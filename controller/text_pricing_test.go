package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type textPricingModelResponse struct {
	Success bool                          `json:"success"`
	Message string                        `json:"message"`
	Data    model.TextPricingModelPreview `json:"data"`
}

func textPricingRequestContext(method string, path string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("id", 1)
	context.Set("username", "pricing-admin")
	context.Set("role", 10)
	return context, recorder
}

func decodeTextPricingModelResponse(t *testing.T, recorder *httptest.ResponseRecorder) textPricingModelResponse {
	t.Helper()
	var response textPricingModelResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestTextPricingModelEndpointsPreviewUpdateInvalidateAndAudit(t *testing.T) {
	setupModelRegistryTestDB(t)
	previousLogDB := model.LOG_DB
	model.LOG_DB = model.DB
	t.Cleanup(func() { model.LOG_DB = previousLogDB })
	require.NoError(t, model.DB.AutoMigrate(&model.Log{}, &model.User{}))
	require.NoError(t, model.DB.Create(&model.TextCategoryPricing{Category: "gpt", Multiplier: 0.1}).Error)
	entry := model.Model{
		ModelName:        "gpt-5.5-model-api",
		Modal:            model.ModelModalText,
		TextCategory:     "gpt",
		OfficialPriceKey: "openai.gpt-5.5",
		Status:           1,
	}
	require.NoError(t, model.DB.Create(&entry).Error)

	previewContext, previewRecorder := textPricingRequestContext(
		http.MethodPost,
		"/api/models/text-pricing/model/preview",
		fmt.Sprintf(`{"model_id":%d,"multiplier":0.25}`, entry.Id),
	)
	PreviewTextPricingModel(previewContext)
	previewResponse := decodeTextPricingModelResponse(t, previewRecorder)
	require.True(t, previewResponse.Success, previewResponse.Message)
	require.Equal(t, 0.1, previewResponse.Data.Before.EffectiveMultiplier)
	require.Equal(t, 0.25, previewResponse.Data.After.EffectiveMultiplier)
	require.Equal(t, model.TextMultiplierSourceModelOverride, previewResponse.Data.After.MultiplierSource)

	geiliPublicModelCacheMu.Lock()
	geiliPublicModelListCache = geiliCachedBody{body: []byte(`{"cached":true}`), expires: time.Now().Add(time.Minute)}
	geiliPublicModelDetailCache["gpt-5-5-model-api"] = geiliCachedBody{body: []byte(`{"cached":true}`), expires: time.Now().Add(time.Minute)}
	geiliPublicModelCacheMu.Unlock()

	updateContext, updateRecorder := textPricingRequestContext(
		http.MethodPut,
		"/api/models/text-pricing/model",
		fmt.Sprintf(`{"model_id":%d,"multiplier":0.25}`, entry.Id),
	)
	UpdateTextPricingModel(updateContext)
	updateResponse := decodeTextPricingModelResponse(t, updateRecorder)
	require.True(t, updateResponse.Success, updateResponse.Message)
	require.Equal(t, 0.25, updateResponse.Data.After.EffectiveMultiplier)

	var persisted model.Model
	require.NoError(t, model.DB.First(&persisted, entry.Id).Error)
	require.NotNil(t, persisted.TextMultiplierOverride)
	require.Equal(t, 0.25, *persisted.TextMultiplierOverride)

	geiliPublicModelCacheMu.RLock()
	require.Nil(t, geiliPublicModelListCache.body)
	require.Empty(t, geiliPublicModelDetailCache)
	geiliPublicModelCacheMu.RUnlock()

	var auditLog model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeManage).Order("id desc").First(&auditLog).Error)
	var auditOther struct {
		Operation struct {
			Action string                 `json:"action"`
			Params map[string]interface{} `json:"params"`
		} `json:"op"`
	}
	require.NoError(t, common.UnmarshalJsonStr(auditLog.Other, &auditOther))
	require.Equal(t, "model.text_pricing_model_update", auditOther.Operation.Action)
	require.Equal(t, model.TextMultiplierSourceModelOverride, auditOther.Operation.Params["multiplier_source"])
	require.EqualValues(t, 0.25, auditOther.Operation.Params["effective_multiplier"])

	invalidContext, invalidRecorder := textPricingRequestContext(
		http.MethodPut,
		"/api/models/text-pricing/model",
		fmt.Sprintf(`{"model_id":%d,"multiplier":1.1}`, entry.Id),
	)
	UpdateTextPricingModel(invalidContext)
	invalidResponse := decodeTextPricingModelResponse(t, invalidRecorder)
	require.False(t, invalidResponse.Success)
	require.NoError(t, model.DB.First(&persisted, entry.Id).Error)
	require.NotNil(t, persisted.TextMultiplierOverride)
	require.Equal(t, 0.25, *persisted.TextMultiplierOverride)

	clearContext, clearRecorder := textPricingRequestContext(
		http.MethodPut,
		"/api/models/text-pricing/model",
		fmt.Sprintf(`{"model_id":%d,"multiplier":null}`, entry.Id),
	)
	UpdateTextPricingModel(clearContext)
	clearResponse := decodeTextPricingModelResponse(t, clearRecorder)
	require.True(t, clearResponse.Success, clearResponse.Message)
	require.Equal(t, model.TextMultiplierSourceCategory, clearResponse.Data.After.MultiplierSource)
	require.NoError(t, model.DB.First(&persisted, entry.Id).Error)
	require.Nil(t, persisted.TextMultiplierOverride)
}

func TestTextPricingModelEndpointsRequireExplicitMultiplierField(t *testing.T) {
	setupModelRegistryTestDB(t)
	require.NoError(t, model.DB.Create(&model.TextCategoryPricing{Category: "gpt", Multiplier: 0.1}).Error)
	entry := model.Model{
		ModelName:        "gpt-5.5-required-multiplier",
		Modal:            model.ModelModalText,
		TextCategory:     "gpt",
		OfficialPriceKey: "openai.gpt-5.5",
		Status:           1,
	}
	require.NoError(t, model.DB.Create(&entry).Error)

	for _, endpoint := range []struct {
		method  string
		path    string
		handler func(*gin.Context)
	}{
		{method: http.MethodPost, path: "/api/models/text-pricing/model/preview", handler: PreviewTextPricingModel},
		{method: http.MethodPut, path: "/api/models/text-pricing/model", handler: UpdateTextPricingModel},
	} {
		context, recorder := textPricingRequestContext(
			endpoint.method,
			endpoint.path,
			fmt.Sprintf(`{"model_id":%d}`, entry.Id),
		)
		endpoint.handler(context)
		response := decodeTextPricingModelResponse(t, recorder)
		require.False(t, response.Success)
		require.Contains(t, response.Message, "multiplier field is required")
	}

	var persisted model.Model
	require.NoError(t, model.DB.First(&persisted, entry.Id).Error)
	require.Nil(t, persisted.TextMultiplierOverride)
}
