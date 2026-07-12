package model

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	NameRuleExact = iota
	NameRulePrefix
	NameRuleContains
	NameRuleSuffix
)

type BoundChannel struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Type int    `json:"type"`
}

type ModelListFilters struct {
	Keyword      string
	Vendor       string
	Status       string
	SyncOfficial string
	Modal        string
	PricingMode  string
}

type Model struct {
	Id                 int            `json:"id"`
	ModelName          string         `json:"model_name" gorm:"size:128;not null;uniqueIndex:uk_model_name_delete_at,priority:1"`
	Description        string         `json:"description,omitempty" gorm:"type:text"`
	Alias              string         `json:"alias,omitempty" gorm:"type:varchar(128);default:''"`
	Icon               string         `json:"icon,omitempty" gorm:"type:varchar(128)"`
	Tags               string         `json:"tags,omitempty" gorm:"type:varchar(255)"`
	VendorID           int            `json:"vendor_id,omitempty" gorm:"index"`
	Endpoints          string         `json:"endpoints,omitempty" gorm:"type:text"`
	Status             int            `json:"status" gorm:"default:1"`
	SyncOfficial       int            `json:"sync_official" gorm:"default:1"`
	Modal              string         `json:"modal,omitempty" gorm:"type:varchar(20);default:''"`
	PricingMode        string         `json:"pricing_mode,omitempty" gorm:"type:varchar(20);default:''"`
	PricingConfig      string         `json:"pricing_config,omitempty" gorm:"type:text"`
	PricingUpdatedTime int64          `json:"pricing_updated_time,omitempty" gorm:"bigint;default:0"`
	CreatedTime        int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime        int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_model_name_delete_at,priority:2"`

	BoundChannels []BoundChannel `json:"bound_channels,omitempty" gorm:"-"`
	EnableGroups  []string       `json:"enable_groups,omitempty" gorm:"-"`
	QuotaTypes    []int          `json:"quota_types,omitempty" gorm:"-"`
	NameRule      int            `json:"name_rule" gorm:"default:0"`

	MatchedModels []string `json:"matched_models,omitempty" gorm:"-"`
	MatchedCount  int      `json:"matched_count,omitempty" gorm:"-"`
}

func (mi *Model) Insert() error {
	now := common.GetTimestamp()
	mi.CreatedTime = now
	mi.UpdatedTime = now

	// 保存原始值（因为 Create 后可能被 GORM 的 default 标签覆盖为 1）
	originalStatus := mi.Status
	originalSyncOfficial := mi.SyncOfficial

	// 先创建记录（GORM 会对零值字段应用默认值）
	if err := DB.Create(mi).Error; err != nil {
		return err
	}

	// 使用保存的原始值进行更新，确保零值能正确保存
	return DB.Model(&Model{}).Where("id = ?", mi.Id).Updates(map[string]interface{}{
		"status":        originalStatus,
		"sync_official": originalSyncOfficial,
	}).Error
}

func IsModelNameDuplicated(id int, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&Model{}).Where("model_name = ? AND id <> ?", name, id).Count(&cnt).Error
	return cnt > 0, err
}

func (mi *Model) Update() error {
	mi.UpdatedTime = common.GetTimestamp()
	var existing Model
	if err := DB.Select("model_name").Where("id = ?", mi.Id).First(&existing).Error; err != nil {
		return err
	}

	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 使用 Select 强制更新所有字段，包括零值
	if err := tx.Model(&Model{}).Where("id = ?", mi.Id).
		Select("model_name", "description", "alias", "icon", "tags", "vendor_id", "endpoints", "status", "sync_official", "name_rule", "modal", "pricing_mode", "pricing_config", "pricing_updated_time", "updated_time").
		Updates(mi).Error; err != nil {
		tx.Rollback()
		return err
	}
	if _, err := renameModelChannelAccessTx(tx, existing.ModelName, mi.ModelName); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (mi *Model) Delete() error {
	return DB.Delete(mi).Error
}

func GetVendorModelCounts() (map[int64]int64, error) {
	var stats []struct {
		VendorID int64
		Count    int64
	}
	if err := DB.Model(&Model{}).
		Select("vendor_id as vendor_id, count(*) as count").
		Group("vendor_id").
		Scan(&stats).Error; err != nil {
		return nil, err
	}
	m := make(map[int64]int64, len(stats))
	for _, s := range stats {
		m[s.VendorID] = s.Count
	}
	return m, nil
}

func GetAllModels(offset int, limit int) ([]*Model, error) {
	models, _, err := SearchModels("", "", offset, limit)
	return models, err
}

func GetModelsByFilters(filters ModelListFilters, offset int, limit int) ([]*Model, int64, error) {
	var models []*Model
	db := applyModelListFilters(DB.Model(&Model{}), filters)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("models.id DESC").Offset(offset).Limit(limit).Find(&models).Error; err != nil {
		return nil, 0, err
	}
	return models, total, nil
}

func GetBoundChannelsByModelsMap(modelNames []string) (map[string][]BoundChannel, error) {
	result := make(map[string][]BoundChannel)
	if len(modelNames) == 0 {
		return result, nil
	}
	type row struct {
		Model string
		Id    int
		Name  string
		Type  int
	}
	var rows []row
	err := DB.Table("channels").
		Select("abilities.model as model, channels.id as id, channels.name as name, channels.type as type").
		Joins("JOIN abilities ON abilities.channel_id = channels.id").
		Where("abilities.model IN ? AND abilities.enabled = ?", modelNames, true).
		Distinct().
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.Model] = append(result[r.Model], BoundChannel{Id: r.Id, Name: r.Name, Type: r.Type})
	}
	return result, nil
}

func normalizeLookupValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func GetPreferredModelOwnerChannelTypes(modelNames []string, groups []string) (map[string]int, error) {
	result := make(map[string]int)
	modelNames = normalizeLookupValues(modelNames)
	if len(modelNames) == 0 {
		return result, nil
	}

	type row struct {
		Model       string
		ChannelType int
	}
	var rows []row

	query := DB.Table("abilities").
		Select("abilities.model as model, channels.type as channel_type").
		Joins("JOIN channels ON abilities.channel_id = channels.id").
		Where("abilities.model IN ? AND abilities.enabled = ? AND channels.status = ?", modelNames, true, common.ChannelStatusEnabled).
		Order("COALESCE(abilities.priority, 0) DESC").
		Order("abilities.weight DESC").
		Order("abilities.channel_id ASC")

	groups = normalizeLookupValues(groups)
	if len(groups) > 0 {
		query = query.Where("abilities."+commonGroupCol+" IN ?", groups)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, r := range rows {
		if _, ok := result[r.Model]; ok {
			continue
		}
		result[r.Model] = r.ChannelType
	}
	return result, nil
}

func SearchModels(keyword string, vendor string, offset int, limit int) ([]*Model, int64, error) {
	return GetModelsByFilters(ModelListFilters{Keyword: keyword, Vendor: vendor}, offset, limit)
}

func applyModelListFilters(db *gorm.DB, filters ModelListFilters) *gorm.DB {
	keyword := strings.TrimSpace(filters.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("model_name LIKE ? OR alias LIKE ? OR description LIKE ? OR tags LIKE ?", like, like, like, like)
	}
	vendor := strings.TrimSpace(filters.Vendor)
	if vendor != "" {
		if vid, err := strconv.Atoi(vendor); err == nil {
			db = db.Where("models.vendor_id = ?", vid)
		} else {
			db = db.Joins("JOIN vendors ON vendors.id = models.vendor_id").Where("vendors.name LIKE ?", "%"+vendor+"%")
		}
	}
	switch strings.ToLower(strings.TrimSpace(filters.Status)) {
	case "enabled", "1", "true":
		db = db.Where("models.status = ?", 1)
	case "disabled", "0", "false":
		db = db.Where("models.status <> ?", 1)
	}
	switch strings.ToLower(strings.TrimSpace(filters.SyncOfficial)) {
	case "yes", "official", "1", "true":
		db = db.Where("models.sync_official = ?", 1)
	case "no", "0", "false":
		db = db.Where("models.sync_official <> ?", 1)
	}
	if modal := strings.TrimSpace(filters.Modal); modal != "" && !strings.EqualFold(modal, "all") {
		db = db.Where("models.modal = ?", modal)
	}
	if pricingMode := strings.TrimSpace(filters.PricingMode); pricingMode != "" && !strings.EqualFold(pricingMode, "all") {
		db = db.Where("models.pricing_mode = ?", pricingMode)
	}
	return db
}

var initialModelAliases = map[string]string{
	"gemini-2.5-flash-image":         "Nano Banana",
	"gemini-3.1-flash-image-preview": "Nano Banana 2",
	"gemini-3-pro-image-preview":     "Nano Banana Pro",
}

func SeedInitialModelAliases() error {
	if DB == nil || !DB.Migrator().HasTable(&Model{}) {
		return nil
	}
	for modelName, alias := range initialModelAliases {
		if err := DB.Model(&Model{}).
			Where("model_name = ? AND (alias = '' OR alias IS NULL)", modelName).
			Update("alias", alias).Error; err != nil {
			return err
		}
	}
	return nil
}

// parseModelStatusFilter maps UI/API status values to the models.status column.
// Returns ok=false when no status filter should be applied.
func parseModelStatusFilter(status string) (value int, ok bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "all":
		return 0, false
	case "enabled", "1":
		return 1, true
	case "disabled", "0":
		return 0, true
	default:
		n, err := strconv.Atoi(status)
		if err != nil {
			return 0, false
		}
		return n, true
	}
}

// parseModelSyncFilter maps UI/API sync values to the models.sync_official column.
// Returns ok=false when no sync filter should be applied.
func parseModelSyncFilter(syncOfficial string) (value int, ok bool) {
	switch strings.ToLower(strings.TrimSpace(syncOfficial)) {
	case "", "all":
		return 0, false
	case "yes", "1":
		return 1, true
	case "no", "0":
		return 0, true
	default:
		n, err := strconv.Atoi(syncOfficial)
		if err != nil {
			return 0, false
		}
		return n, true
	}
}
