package controller

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type textPricingCategoryRequest struct {
	Category   string  `json:"category"`
	Multiplier float64 `json:"multiplier"`
}

type textPricingModeRequest struct {
	Mode string `json:"mode"`
}

type textPricingModelRequest struct {
	ModelID    int             `json:"model_id"`
	Multiplier json.RawMessage `json:"multiplier"`
}

func bindTextPricingModelRequest(c *gin.Context) (int, *float64, error) {
	var request textPricingModelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return 0, nil, err
	}
	if len(request.Multiplier) == 0 {
		return 0, nil, errors.New("multiplier field is required")
	}
	if common.GetJsonType(request.Multiplier) == "null" {
		return request.ModelID, nil, nil
	}
	var multiplier float64
	if err := common.Unmarshal(request.Multiplier, &multiplier); err != nil {
		return 0, nil, err
	}
	return request.ModelID, &multiplier, nil
}

func GetTextPricingConfig(c *gin.Context) {
	config, err := model.GetTextPricingConfigView()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, config)
}

func PreviewTextPricingCategory(c *gin.Context) {
	var request textPricingCategoryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	preview, err := model.PreviewTextCategoryMultiplier(request.Category, request.Multiplier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, preview)
}

func UpdateTextPricingCategory(c *gin.Context) {
	var request textPricingCategoryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	preview, err := model.PreviewTextCategoryMultiplier(request.Category, request.Multiplier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	entry, err := model.UpdateTextCategoryMultiplier(request.Category, request.Multiplier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	invalidateGeiliPublicModelCache()
	recordManageAudit(c, "model.text_pricing_category_update", map[string]interface{}{
		"category":            entry.Category,
		"previous_multiplier": preview.Before.Multiplier,
		"multiplier":          entry.Multiplier,
		"affected_count":      preview.AffectedCount,
		"override_count":      preview.OverrideCount,
		"multiplier_source":   model.TextMultiplierSourceCategory,
	})
	config, err := model.GetTextPricingConfigView()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for _, category := range config.Categories {
		if category.Category == entry.Category {
			common.ApiSuccess(c, category)
			return
		}
	}
	common.ApiSuccess(c, entry)
}

func PreviewTextPricingModel(c *gin.Context) {
	modelID, multiplier, err := bindTextPricingModelRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	preview, err := model.PreviewTextModelMultiplier(modelID, multiplier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, preview)
}

func UpdateTextPricingModel(c *gin.Context) {
	modelID, multiplier, err := bindTextPricingModelRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	preview, err := model.UpdateTextModelMultiplier(modelID, multiplier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	invalidateGeiliPublicModelCache()
	recordManageAudit(c, "model.text_pricing_model_update", map[string]interface{}{
		"model_id":                      preview.ModelID,
		"model":                         preview.ModelName,
		"category_multiplier":           preview.After.CategoryMultiplier,
		"previous_model_multiplier":     preview.Before.ModelMultiplierOverride,
		"model_multiplier_override":     preview.After.ModelMultiplierOverride,
		"previous_effective_multiplier": preview.Before.EffectiveMultiplier,
		"effective_multiplier":          preview.After.EffectiveMultiplier,
		"multiplier_source":             preview.After.MultiplierSource,
	})
	common.ApiSuccess(c, preview)
}

func UpdateTextPricingMode(c *gin.Context) {
	var request textPricingModeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	request.Mode = strings.ToLower(strings.TrimSpace(request.Mode))
	if err := model.SetTextPricingMode(request.Mode); err != nil {
		common.ApiError(c, err)
		return
	}
	invalidateGeiliPublicModelCache()
	recordManageAudit(c, "model.text_pricing_mode_update", map[string]interface{}{
		"mode": request.Mode,
	})
	common.ApiSuccess(c, gin.H{"mode": model.GetTextPricingMode()})
}
