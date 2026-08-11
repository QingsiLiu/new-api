package controller

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetAllModelsMeta 获取模型列表（分页）
func GetAllModelsMeta(c *gin.Context) {

	pageInfo := common.GetPageQuery(c)
	filters := model.ModelListFilters{
		Vendor:            c.Query("vendor"),
		Status:            c.Query("status"),
		SyncOfficial:      c.Query("sync_official"),
		Modal:             c.Query("modal"),
		TextCategory:      c.Query("text_category"),
		PricingMode:       c.Query("pricing_mode"),
		TextPricingStatus: c.Query("text_pricing_status"),
	}
	modelsMeta, total, err := model.GetModelsByFilters(filters, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 批量填充附加字段，提升列表接口性能
	enrichModels(modelsMeta)

	// 统计供应商计数（全部数据，不受分页影响）
	vendorCounts, _ := model.GetVendorModelCounts()

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(modelsMeta)
	common.ApiSuccess(c, gin.H{
		"items":         modelsMeta,
		"total":         total,
		"page":          pageInfo.GetPage(),
		"page_size":     pageInfo.GetPageSize(),
		"vendor_counts": vendorCounts,
	})
}

// SearchModelsMeta 搜索模型列表
func SearchModelsMeta(c *gin.Context) {

	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)

	filters := model.ModelListFilters{
		Keyword:           keyword,
		Vendor:            c.Query("vendor"),
		Status:            c.Query("status"),
		SyncOfficial:      c.Query("sync_official"),
		Modal:             c.Query("modal"),
		TextCategory:      c.Query("text_category"),
		PricingMode:       c.Query("pricing_mode"),
		TextPricingStatus: c.Query("text_pricing_status"),
	}
	modelsMeta, total, err := model.GetModelsByFilters(filters, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 批量填充附加字段，提升列表接口性能
	enrichModels(modelsMeta)
	vendorCounts, _ := model.GetVendorModelCounts()
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(modelsMeta)
	common.ApiSuccess(c, gin.H{
		"items":         modelsMeta,
		"total":         total,
		"page":          pageInfo.GetPage(),
		"page_size":     pageInfo.GetPageSize(),
		"vendor_counts": vendorCounts,
	})
}

// GetModelMeta 根据 ID 获取单条模型信息
func GetModelMeta(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var m model.Model
	if err := model.DB.First(&m, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	enrichModels([]*model.Model{&m})
	common.ApiSuccess(c, &m)
}

// CreateModelMeta 新建模型
func CreateModelMeta(c *gin.Context) {
	var m model.Model
	if err := c.ShouldBindJSON(&m); err != nil {
		common.ApiError(c, err)
		return
	}
	if m.ModelName == "" {
		common.ApiErrorMsg(c, "模型名称不能为空")
		return
	}
	model.PrepareModelTextPricingMetadata(&m)
	if m.Status == 1 {
		if err := model.ValidateModelTextPricing(&m); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	// 名称冲突检查
	if dup, err := model.IsModelNameDuplicated(0, m.ModelName); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "模型名称已存在")
		return
	}

	if err := m.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.RefreshPricing()
	model.EnrichTextPricingModels([]*model.Model{&m})
	recordManageAudit(c, "model.create", map[string]interface{}{
		"id":       m.Id,
		"model":    m.ModelName,
		"modality": m.Modal,
	})
	common.ApiSuccess(c, &m)
}

// UpdateModelMeta 更新模型
func UpdateModelMeta(c *gin.Context) {
	statusOnly := c.Query("status_only") == "true"

	var m model.Model
	if err := c.ShouldBindJSON(&m); err != nil {
		common.ApiError(c, err)
		return
	}
	if m.Id == 0 {
		common.ApiErrorMsg(c, "缺少模型 ID")
		return
	}

	if statusOnly {
		requestedStatus := m.Status
		var existing model.Model
		if err := model.DB.First(&existing, m.Id).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		if requestedStatus == 1 {
			if err := model.ValidateModelTextPricing(&existing); err != nil {
				common.ApiError(c, err)
				return
			}
		}
		if err := model.DB.Model(&existing).Update("status", requestedStatus).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		m = existing
		m.Status = requestedStatus
		recordManageAudit(c, "model.status_update", map[string]interface{}{
			"id":     existing.Id,
			"model":  existing.ModelName,
			"status": m.Status,
		})
	} else {
		model.PrepareModelTextPricingMetadata(&m)
		if m.Status == 1 {
			if err := model.ValidateModelTextPricing(&m); err != nil {
				common.ApiError(c, err)
				return
			}
		}
		// 名称冲突检查
		if dup, err := model.IsModelNameDuplicated(m.Id, m.ModelName); err != nil {
			common.ApiError(c, err)
			return
		} else if dup {
			common.ApiErrorMsg(c, "模型名称已存在")
			return
		}

		if err := m.Update(); err != nil {
			common.ApiError(c, err)
			return
		}
		recordManageAudit(c, "model.update", map[string]interface{}{
			"id":       m.Id,
			"model":    m.ModelName,
			"modality": m.Modal,
		})
	}
	model.RefreshPricing()
	model.EnrichTextPricingModels([]*model.Model{&m})
	common.ApiSuccess(c, &m)
}

type updateModelAccessRequest struct {
	ChannelIDs []int `json:"channel_ids"`
}

func UpdateModelAccess(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var m model.Model
	if err := model.DB.First(&m, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	var req updateModelAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	updatedChannels, err := model.UpdateModelChannelAccess(m.ModelName, req.ChannelIDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	model.InitChannelCache()
	model.RefreshPricing()
	enrichModels([]*model.Model{&m})
	recordManageAudit(c, "model.access_update", map[string]interface{}{
		"id":               m.Id,
		"model":            m.ModelName,
		"channel_ids":      req.ChannelIDs,
		"updated_channels": updatedChannels,
	})
	common.ApiSuccess(c, gin.H{
		"model":            &m,
		"updated_channels": updatedChannels,
	})
}

func GetModelPricingParity(c *gin.Context) {
	common.ApiSuccess(c, model.GetModelPricingParityStatus())
}

// DeleteModelMeta 删除模型
func DeleteModelMeta(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := model.DeleteModelAndReferences(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	model.RefreshPricing()
	recordManageAudit(c, "model.delete", map[string]interface{}{
		"id":                result.Model.Id,
		"model":             result.Model.ModelName,
		"updated_channels":  result.UpdatedChannels,
		"deleted_abilities": result.DeletedAbilities,
	})
	common.ApiSuccess(c, gin.H{
		"updated_channels":  result.UpdatedChannels,
		"deleted_abilities": result.DeletedAbilities,
	})
}

// enrichModels 批量填充附加信息：端点、渠道、分组、计费类型，避免 N+1 查询
func enrichModels(models []*model.Model) {
	if len(models) == 0 {
		return
	}
	model.EnrichTextPricingModels(models)

	// 1) 拆分精确与规则匹配
	exactNames := make([]string, 0)
	exactIdx := make(map[string][]int) // modelName -> indices in models
	ruleIndices := make([]int, 0)
	for i, m := range models {
		if m == nil {
			continue
		}
		if m.NameRule == model.NameRuleExact {
			exactNames = append(exactNames, m.ModelName)
			exactIdx[m.ModelName] = append(exactIdx[m.ModelName], i)
		} else {
			ruleIndices = append(ruleIndices, i)
		}
	}

	// 2) 批量查询精确模型的绑定渠道
	channelsByModel, _ := model.GetBoundChannelsByModelsMap(exactNames)

	// 3) 精确模型：端点从缓存、渠道批量映射、分组/计费类型从缓存
	for name, indices := range exactIdx {
		chs := channelsByModel[name]
		for _, idx := range indices {
			mm := models[idx]
			if mm.Endpoints == "" {
				eps := model.GetModelSupportEndpointTypes(mm.ModelName)
				if b, err := json.Marshal(eps); err == nil {
					mm.Endpoints = string(b)
				}
			}
			mm.BoundChannels = chs
			mm.EnableGroups = model.GetModelEnableGroups(mm.ModelName)
			mm.QuotaTypes = model.GetModelQuotaTypes(mm.ModelName)
		}
	}

	if len(ruleIndices) == 0 {
		return
	}

	// 4) 一次性读取定价缓存，内存匹配所有规则模型
	pricings := model.GetPricing()

	// 为全部规则模型收集匹配名集合、端点并集、分组并集、配额集合
	matchedNamesByIdx := make(map[int][]string)
	endpointSetByIdx := make(map[int]map[constant.EndpointType]struct{})
	groupSetByIdx := make(map[int]map[string]struct{})
	quotaSetByIdx := make(map[int]map[int]struct{})

	for _, p := range pricings {
		for _, idx := range ruleIndices {
			mm := models[idx]
			var matched bool
			switch mm.NameRule {
			case model.NameRulePrefix:
				matched = strings.HasPrefix(p.ModelName, mm.ModelName)
			case model.NameRuleSuffix:
				matched = strings.HasSuffix(p.ModelName, mm.ModelName)
			case model.NameRuleContains:
				matched = strings.Contains(p.ModelName, mm.ModelName)
			}
			if !matched {
				continue
			}
			matchedNamesByIdx[idx] = append(matchedNamesByIdx[idx], p.ModelName)

			es := endpointSetByIdx[idx]
			if es == nil {
				es = make(map[constant.EndpointType]struct{})
				endpointSetByIdx[idx] = es
			}
			for _, et := range p.SupportedEndpointTypes {
				es[et] = struct{}{}
			}

			gs := groupSetByIdx[idx]
			if gs == nil {
				gs = make(map[string]struct{})
				groupSetByIdx[idx] = gs
			}
			for _, g := range p.EnableGroup {
				gs[g] = struct{}{}
			}

			qs := quotaSetByIdx[idx]
			if qs == nil {
				qs = make(map[int]struct{})
				quotaSetByIdx[idx] = qs
			}
			qs[p.QuotaType] = struct{}{}
		}
	}

	// 5) 汇总所有匹配到的模型名称，批量查询一次渠道
	allMatchedSet := make(map[string]struct{})
	for _, names := range matchedNamesByIdx {
		for _, n := range names {
			allMatchedSet[n] = struct{}{}
		}
	}
	allMatched := make([]string, 0, len(allMatchedSet))
	for n := range allMatchedSet {
		allMatched = append(allMatched, n)
	}
	matchedChannelsByModel, _ := model.GetBoundChannelsByModelsMap(allMatched)

	// 6) 回填每个规则模型的并集信息
	for _, idx := range ruleIndices {
		mm := models[idx]

		// 端点并集 -> 序列化
		if es, ok := endpointSetByIdx[idx]; ok && mm.Endpoints == "" {
			eps := make([]constant.EndpointType, 0, len(es))
			for et := range es {
				eps = append(eps, et)
			}
			if b, err := json.Marshal(eps); err == nil {
				mm.Endpoints = string(b)
			}
		}

		// 分组并集
		if gs, ok := groupSetByIdx[idx]; ok {
			groups := make([]string, 0, len(gs))
			for g := range gs {
				groups = append(groups, g)
			}
			mm.EnableGroups = groups
		}

		// 配额类型集合（保持去重并排序）
		if qs, ok := quotaSetByIdx[idx]; ok {
			arr := make([]int, 0, len(qs))
			for k := range qs {
				arr = append(arr, k)
			}
			sort.Ints(arr)
			mm.QuotaTypes = arr
		}

		// 渠道并集
		names := matchedNamesByIdx[idx]
		channelSet := make(map[string]model.BoundChannel)
		for _, n := range names {
			for _, ch := range matchedChannelsByModel[n] {
				key := ch.Name + "_" + strconv.Itoa(ch.Type)
				channelSet[key] = ch
			}
		}
		if len(channelSet) > 0 {
			chs := make([]model.BoundChannel, 0, len(channelSet))
			for _, ch := range channelSet {
				chs = append(chs, ch)
			}
			mm.BoundChannels = chs
		}

		// 匹配信息
		mm.MatchedModels = names
		mm.MatchedCount = len(names)
	}
}
