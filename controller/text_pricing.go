package controller

import (
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
	entry, err := model.UpdateTextCategoryMultiplier(request.Category, request.Multiplier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	invalidateGeiliPublicModelCache()
	recordManageAudit(c, "model.text_pricing_category_update", map[string]interface{}{
		"category":   entry.Category,
		"multiplier": entry.Multiplier,
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
