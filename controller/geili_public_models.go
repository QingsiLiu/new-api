package controller

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// Geili 自有：模型注册表公开只读 API（SLP/market/pricing 页数据源）。
// 红线：响应只从注册表白名单字段 + 规格价格表构建，绝不携带渠道/上游/成本信息
// （守卫测试见 geili_public_models_test.go）。

type publicModelSummary struct {
	Slug           string            `json:"slug"`
	Model          string            `json:"model"`
	Modality       string            `json:"modality"`
	Vendor         string            `json:"vendor"`
	DisplayName    map[string]string `json:"display_name"`
	VendorDisplay  map[string]string `json:"vendor_display"`
	Aliases        []string          `json:"aliases,omitempty"`
	CapabilityTags []string          `json:"capability_tags,omitempty"`
	PriceUnit      string            `json:"price_unit,omitempty"`      // per_image | per_second
	PriceFromCNY   float64           `json:"price_from_cny,omitempty"`  // "¥x 起"
	OfficialPrice  json.RawMessage   `json:"official_price,omitempty"`  // 官方价（对比列）
}

type publicModelDetail struct {
	publicModelSummary
	ParamsSchema  json.RawMessage            `json:"params_schema,omitempty"`
	ExampleParams json.RawMessage            `json:"example_params,omitempty"`
	Faq           map[string]json.RawMessage `json:"faq,omitempty"`
	Seo           map[string]string          `json:"seo,omitempty"`
	SpecPricing   json.RawMessage            `json:"spec_pricing,omitempty"` // 我方完整规格价（CNY）
}

// ---- 公开端点进程内响应缓存 ----
// 注册表与规格价均为低频变更数据，60s 内允许陈旧；管理端写操作主动失效。
// 与 GlobalAPIRateLimit 互补：限流挡刷量，缓存把命中期内的 DB 查询降为零。
// （CF 侧 all.geiliapi.com 无 API 缓存规则且 CF 配置属用户人工项，故缓存放进程内。）

const geiliPublicModelCacheTTL = 60 * time.Second

type geiliCachedBody struct {
	body    []byte
	expires time.Time
}

var (
	geiliPublicModelCacheMu     sync.RWMutex
	geiliPublicModelListCache   geiliCachedBody
	geiliPublicModelDetailCache = map[string]geiliCachedBody{}
)

func invalidateGeiliPublicModelCache() {
	geiliPublicModelCacheMu.Lock()
	geiliPublicModelListCache = geiliCachedBody{}
	geiliPublicModelDetailCache = map[string]geiliCachedBody{}
	geiliPublicModelCacheMu.Unlock()
}

func writeGeiliPublicJSON(c *gin.Context, body []byte) {
	c.Header("Cache-Control", "public, max-age=60")
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

func rawJSONOrNil(s string) json.RawMessage {
	s = strings.TrimSpace(s)
	if s == "" || !json.Valid([]byte(s)) {
		return nil
	}
	return json.RawMessage(s)
}

func stringListFromJSON(s string) []string {
	var out []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &out); err != nil {
		return nil
	}
	return out
}

// currentAsyncSpecPricing 经导出的 JSON 访问器取当前规格价（避免向 setting 包新增接口）。
func currentAsyncSpecPricing() operation_setting.AsyncSpecPricing {
	var pricing operation_setting.AsyncSpecPricing
	_ = json.Unmarshal([]byte(operation_setting.AsyncSpecPricing2JSONString()), &pricing)
	return pricing
}

// specPriceSummary 返回该模型的计价单位与最低价（"¥x 起"），以及模型的完整规格价 JSON。
func specPriceSummary(pricing operation_setting.AsyncSpecPricing, modality, modelName string) (unit string, fromCNY float64, raw json.RawMessage) {
	minPositive := func(min float64, v *float64) float64 {
		if v == nil || *v <= 0 {
			return min
		}
		if min == 0 || *v < min {
			return *v
		}
		return min
	}
	switch modality {
	case "image":
		spec, ok := pricing.Image[modelName]
		if !ok {
			return "", 0, nil
		}
		unit = spec.Unit
		if unit == "" {
			unit = "per_image"
		}
		fromCNY = 0
		for _, p := range spec.Resolutions {
			fromCNY = minPositive(fromCNY, p.CNYPerImage)
		}
		for _, p := range spec.Qualities {
			fromCNY = minPositive(fromCNY, p.CNYPerImage)
		}
		fromCNY = minPositive(fromCNY, spec.DefaultCNYPerImage)
		bytes, err := json.Marshal(spec)
		if err == nil {
			raw = bytes
		}
		return unit, fromCNY, raw
	case "video":
		spec, ok := pricing.Video[modelName]
		if !ok {
			return "", 0, nil
		}
		unit = spec.Unit
		if unit == "" {
			unit = "per_second"
		}
		fromCNY = 0
		for _, p := range spec.Resolutions {
			fromCNY = minPositive(fromCNY, p.CNYPerSecond)
		}
		for _, ratios := range spec.Prices {
			for _, modes := range ratios {
				for _, price := range modes {
					if price.Unsupported {
						continue
					}
					fromCNY = minPositive(fromCNY, price.CNYPerSecond)
				}
			}
		}
		fromCNY = minPositive(fromCNY, spec.DefaultCNYPerSecond)
		bytes, err := json.Marshal(spec)
		if err == nil {
			raw = bytes
		}
		return unit, fromCNY, raw
	}
	return "", 0, nil
}

func buildPublicModelSummary(entry model.ModelRegistry, pricing operation_setting.AsyncSpecPricing) publicModelSummary {
	unit, fromCNY, _ := specPriceSummary(pricing, entry.Modality, entry.ModelName)
	return publicModelSummary{
		Slug:     entry.Slug,
		Model:    entry.ModelName,
		Modality: entry.Modality,
		Vendor:   entry.Vendor,
		DisplayName: map[string]string{
			"zh": entry.DisplayNameZh,
			"en": entry.DisplayNameEn,
		},
		VendorDisplay: map[string]string{
			"zh": entry.VendorDisplayZh,
			"en": entry.VendorDisplayEn,
		},
		Aliases:        stringListFromJSON(entry.Aliases),
		CapabilityTags: stringListFromJSON(entry.CapabilityTags),
		PriceUnit:      unit,
		PriceFromCNY:   fromCNY,
		OfficialPrice:  rawJSONOrNil(entry.OfficialPrice),
	}
}

// GetPublicModels GET /v1/public/models
func GetPublicModels(c *gin.Context) {
	geiliPublicModelCacheMu.RLock()
	cached := geiliPublicModelListCache
	geiliPublicModelCacheMu.RUnlock()
	if cached.body != nil && time.Now().Before(cached.expires) {
		writeGeiliPublicJSON(c, cached.body)
		return
	}

	entries, err := model.GetEnabledModelRegistries()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pricing := currentAsyncSpecPricing()
	list := make([]publicModelSummary, 0, len(entries))
	for _, entry := range entries {
		list = append(list, buildPublicModelSummary(entry, pricing))
	}
	body, err := json.Marshal(gin.H{"success": true, "data": list})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	geiliPublicModelCacheMu.Lock()
	geiliPublicModelListCache = geiliCachedBody{body: body, expires: time.Now().Add(geiliPublicModelCacheTTL)}
	geiliPublicModelCacheMu.Unlock()
	writeGeiliPublicJSON(c, body)
}

// GetPublicModelBySlug GET /v1/public/models/:slug
func GetPublicModelBySlug(c *gin.Context) {
	slug := c.Param("slug")
	geiliPublicModelCacheMu.RLock()
	cached, ok := geiliPublicModelDetailCache[slug]
	geiliPublicModelCacheMu.RUnlock()
	if ok && cached.body != nil && time.Now().Before(cached.expires) {
		writeGeiliPublicJSON(c, cached.body)
		return
	}

	entry, err := model.GetModelRegistryBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "model not found"})
		return
	}
	pricing := currentAsyncSpecPricing()
	_, _, specRaw := specPriceSummary(pricing, entry.Modality, entry.ModelName)
	detail := publicModelDetail{
		publicModelSummary: buildPublicModelSummary(*entry, pricing),
		ParamsSchema:       rawJSONOrNil(entry.ParamsSchema),
		ExampleParams:      rawJSONOrNil(entry.ExampleParams),
		Faq: map[string]json.RawMessage{
			"zh": rawJSONOrNil(entry.FaqZh),
			"en": rawJSONOrNil(entry.FaqEn),
		},
		Seo: map[string]string{
			"zh": entry.SeoZh,
			"en": entry.SeoEn,
		},
		SpecPricing: specRaw,
	}
	body, err := json.Marshal(gin.H{"success": true, "data": detail})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	geiliPublicModelCacheMu.Lock()
	geiliPublicModelDetailCache[slug] = geiliCachedBody{body: body, expires: time.Now().Add(geiliPublicModelCacheTTL)}
	geiliPublicModelCacheMu.Unlock()
	writeGeiliPublicJSON(c, body)
}

// ---- 管理端（AdminAuth 挂在路由层） ----

type modelRegistryUpsertRequest struct {
	Model         string            `json:"model"`
	Slug          string            `json:"slug"`
	DisplayName   map[string]string `json:"display_name"`
	Aliases       []string          `json:"aliases"`
	Vendor        string            `json:"vendor"`
	VendorDisplay map[string]string `json:"vendor_display"`
	Modality      string            `json:"modality"`
	CapabilityTags []string         `json:"capability_tags"`
	OfficialPrice json.RawMessage   `json:"official_price"`
	ParamsSchema  json.RawMessage   `json:"params_schema"`
	ExampleParams json.RawMessage   `json:"example_params"`
	Faq           map[string]json.RawMessage `json:"faq"`
	SeoZh         string            `json:"seo_zh"`
	SeoEn         string            `json:"seo_en"`
	Enabled       *bool             `json:"enabled"`
}

func marshalOrEmpty(v any) string {
	if v == nil {
		return ""
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	out := string(bytes)
	if out == "null" {
		return ""
	}
	return out
}

// AdminUpsertModelRegistry POST /api/geili/model-registry
func AdminUpsertModelRegistry(c *gin.Context) {
	var req modelRegistryUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Modality != "image" && req.Modality != "video" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "modality 必须是 image 或 video"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	now := common.GetTimestamp()
	entry := model.ModelRegistry{
		ModelName:       req.Model,
		Slug:            req.Slug,
		DisplayNameZh:   req.DisplayName["zh"],
		DisplayNameEn:   req.DisplayName["en"],
		Aliases:         marshalOrEmpty(req.Aliases),
		Vendor:          req.Vendor,
		VendorDisplayZh: req.VendorDisplay["zh"],
		VendorDisplayEn: req.VendorDisplay["en"],
		Modality:        req.Modality,
		CapabilityTags:  marshalOrEmpty(req.CapabilityTags),
		OfficialPrice:   marshalOrEmpty(req.OfficialPrice),
		ParamsSchema:    marshalOrEmpty(req.ParamsSchema),
		ExampleParams:   marshalOrEmpty(req.ExampleParams),
		FaqZh:           marshalOrEmpty(req.Faq["zh"]),
		FaqEn:           marshalOrEmpty(req.Faq["en"]),
		SeoZh:           req.SeoZh,
		SeoEn:           req.SeoEn,
		Enabled:         enabled,
		CreatedTime:     now,
		UpdatedTime:     now,
	}
	if err := model.UpsertModelRegistry(&entry); err != nil {
		common.ApiError(c, err)
		return
	}
	invalidateGeiliPublicModelCache()
	common.ApiSuccess(c, gin.H{"model": entry.ModelName, "slug": entry.Slug})
}

// AdminListModelRegistry GET /api/geili/model-registry
func AdminListModelRegistry(c *gin.Context) {
	entries, err := model.GetAllModelRegistries()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, entries)
}

// AdminDeleteModelRegistry DELETE /api/geili/model-registry/:model
func AdminDeleteModelRegistry(c *gin.Context) {
	if err := model.DeleteModelRegistryByModelName(c.Param("model")); err != nil {
		common.ApiError(c, err)
		return
	}
	invalidateGeiliPublicModelCache()
	common.ApiSuccess(c, nil)
}
