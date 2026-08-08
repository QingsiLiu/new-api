package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/textpricing"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"gorm.io/gorm"
)

const TextPricingModeOption = "TextPricingMode"

const (
	TextPricingModeLegacy = "legacy"
	TextPricingModeShadow = "shadow"
	TextPricingModeActive = "active"
)

type TextPricingProfileView = textpricing.PublicProfile

type EffectiveTextPricingView struct {
	Category                    string                         `json:"category"`
	CategoryMultiplier          float64                        `json:"category_multiplier"`
	CatalogVersion              string                         `json:"catalog_version"`
	OfficialPriceKey            string                         `json:"official_price_key"`
	PricingSource               string                         `json:"pricing_source"`
	InputQuotaPerMillion        int64                          `json:"input_quota_per_million"`
	OutputQuotaPerMillion       int64                          `json:"output_quota_per_million"`
	CachedInputQuotaPerMillion  int64                          `json:"cached_input_quota_per_million,omitempty"`
	CacheWriteQuotaPerMillion   int64                          `json:"cache_write_quota_per_million,omitempty"`
	CacheWrite5mQuotaPerMillion int64                          `json:"cache_write_5m_quota_per_million,omitempty"`
	CacheWrite1hQuotaPerMillion int64                          `json:"cache_write_1h_quota_per_million,omitempty"`
	Tiers                       []types.CreditsTextPricingTier `json:"tiers,omitempty"`
}

type TextPricingResolution struct {
	Model      Model
	Profile    textpricing.Profile
	Multiplier float64
	Pricing    *types.CreditsTextPricing
}

func ResolveLegacyTextPricingSnapshot(modelName string) (*types.CreditsTextPricing, bool) {
	if !common.CreditsV1Enabled() {
		return nil, false
	}
	return resolveCreditsV1TextPricingSnapshot(modelName)
}

func resolveCreditsV1TextPricingSnapshot(modelName string) (*types.CreditsTextPricing, bool) {
	profile, ok := textpricing.MatchModel(modelName)
	if !ok {
		return nil, false
	}
	multiplier, ok := textpricing.DefaultMultiplier(profile.Category)
	if !ok {
		return nil, false
	}
	pricing, err := textpricing.BuildPricing(profile, multiplier, false, "geili")
	if err != nil {
		return nil, false
	}
	pricing.Fallback = true
	return pricing, true
}

func ResolveEffectiveTextPricing(modelName string) (*types.CreditsTextPricing, bool, error) {
	return ResolveEffectiveTextPricingForMode(modelName, GetTextPricingMode())
}

func ResolveEffectiveTextPricingForMode(modelName string, mode string) (*types.CreditsTextPricing, bool, error) {
	mode, err := NormalizeTextPricingMode(mode)
	if err != nil {
		return nil, false, err
	}
	if mode != TextPricingModeActive {
		pricing, ok := ResolveLegacyTextPricingSnapshot(modelName)
		return pricing, ok, nil
	}
	if !IsModelPricingConfigTrusted() {
		if pricing, ok := resolveCreditsV1TextPricingSnapshot(modelName); ok {
			return pricing, true, nil
		}
		return nil, false, errors.New("model pricing parity is not trusted and no Credits V1 snapshot is available")
	}
	entry, ok, err := GetTextPricingModel(modelName)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, fmt.Errorf("text model %s has no model metadata", modelName)
	}
	if strings.TrimSpace(entry.Modal) != ModelModalText {
		return nil, false, nil
	}
	resolution, err := ResolveTextModelPricingFromMetadata(entry, nil)
	if err != nil {
		return nil, false, err
	}
	return resolution.Pricing, true, nil
}

func ResolveShadowTextPricing(modelName string) (*types.CreditsTextPricing, bool, error) {
	return ResolveShadowTextPricingForMode(modelName, GetTextPricingMode())
}

func ResolveShadowTextPricingForMode(modelName string, mode string) (*types.CreditsTextPricing, bool, error) {
	mode, err := NormalizeTextPricingMode(mode)
	if err != nil {
		return nil, false, err
	}
	if mode != TextPricingModeShadow {
		return nil, false, nil
	}
	entry, ok, err := GetTextPricingModel(modelName)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, fmt.Errorf("text model %s has no model metadata", modelName)
	}
	if strings.TrimSpace(entry.Modal) != ModelModalText {
		return nil, false, nil
	}
	resolution, err := ResolveTextModelPricingFromMetadata(entry, nil)
	if err != nil {
		return nil, false, err
	}
	return resolution.Pricing, true, nil
}

type TextPricingCategoryConfig struct {
	Category          string   `json:"category"`
	Multiplier        *float64 `json:"multiplier,omitempty"`
	ModelCount        int      `json:"model_count"`
	PricingReadyCount int      `json:"pricing_ready_count"`
	UpdatedTime       int64    `json:"updated_time,omitempty"`
}

type TextPricingConfigView struct {
	Mode           string                      `json:"mode"`
	CatalogVersion string                      `json:"catalog_version"`
	Categories     []TextPricingCategoryConfig `json:"categories"`
	Profiles       []textpricing.PublicProfile `json:"profiles"`
}

type TextPricingImpact struct {
	ID                    int    `json:"id"`
	ModelName             string `json:"model_name"`
	OfficialPriceKey      string `json:"official_price_key"`
	InputQuotaPerMillion  int64  `json:"input_quota_per_million"`
	OutputQuotaPerMillion int64  `json:"output_quota_per_million"`
	PricingReady          bool   `json:"pricing_ready"`
	PricingError          string `json:"pricing_error,omitempty"`
}

type TextPricingPreviewSummary struct {
	Category   string              `json:"category"`
	Multiplier *float64            `json:"multiplier,omitempty"`
	Models     []TextPricingImpact `json:"models"`
}

type TextPricingPreview struct {
	AffectedCount int                       `json:"affected_count"`
	Before        TextPricingPreviewSummary `json:"before"`
	After         TextPricingPreviewSummary `json:"after"`
}

func NormalizeTextPricingMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case TextPricingModeLegacy, TextPricingModeShadow, TextPricingModeActive:
		return mode, nil
	default:
		return "", fmt.Errorf("text pricing mode must be legacy, shadow, or active")
	}
}

func GetTextPricingMode() string {
	common.OptionMapRWMutex.RLock()
	mode := common.OptionMap[TextPricingModeOption]
	common.OptionMapRWMutex.RUnlock()
	mode, err := NormalizeTextPricingMode(mode)
	if err != nil {
		return TextPricingModeLegacy
	}
	return mode
}

func SetTextPricingMode(mode string) error {
	normalized, err := NormalizeTextPricingMode(mode)
	if err != nil {
		return err
	}
	if normalized == TextPricingModeActive {
		if !IsModelPricingConfigTrusted() {
			return errors.New("model pricing parity is not trusted; active text pricing is blocked")
		}
		if err := ValidateEnabledTextPricingModels(); err != nil {
			return err
		}
	}
	return UpdateOption(TextPricingModeOption, normalized)
}

func SeedTextPricingCenter() error {
	if DB == nil || !DB.Migrator().HasTable(&Option{}) || !DB.Migrator().HasTable(&Model{}) || !DB.Migrator().HasTable(&TextCategoryPricing{}) {
		return nil
	}
	if err := seedTextPricingMode(); err != nil {
		return err
	}
	if err := seedTextCategoryMultipliers(); err != nil {
		return err
	}
	return seedTextModelCatalogMetadata()
}

func seedTextPricingMode() error {
	var option Option
	err := DB.Where("key = ?", TextPricingModeOption).First(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DB.Create(&Option{Key: TextPricingModeOption, Value: TextPricingModeLegacy}).Error
	}
	if err != nil {
		return err
	}
	if _, err := NormalizeTextPricingMode(option.Value); err != nil {
		return DB.Model(&option).Update("value", TextPricingModeLegacy).Error
	}
	return nil
}

func seedTextCategoryMultipliers() error {
	for _, category := range textpricing.Categories() {
		multiplier, ok := textpricing.DefaultMultiplier(category)
		if !ok {
			continue
		}
		var count int64
		if err := DB.Model(&TextCategoryPricing{}).Where("category = ?", category).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := DB.Create(&TextCategoryPricing{
			Category:    category,
			Multiplier:  multiplier,
			UpdatedTime: common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedTextModelCatalogMetadata() error {
	var models []Model
	if err := DB.Find(&models).Error; err != nil {
		return err
	}
	for i := range models {
		profile, ok := textpricing.MatchModel(models[i].ModelName)
		if !ok {
			continue
		}
		updates := map[string]any{}
		if strings.TrimSpace(models[i].Modal) == "" {
			updates["modal"] = ModelModalText
		}
		if strings.TrimSpace(models[i].TextCategory) == "" {
			updates["text_category"] = profile.Category
		}
		if strings.TrimSpace(models[i].OfficialPriceKey) == "" {
			updates["official_price_key"] = profile.Key
		}
		if len(updates) == 0 {
			continue
		}
		updates["updated_time"] = common.GetTimestamp()
		if err := DB.Model(&Model{}).Where("id = ?", models[i].Id).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func GetTextCategoryPricingRows() (map[string]TextCategoryPricing, error) {
	var rows []TextCategoryPricing
	if err := DB.Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]TextCategoryPricing, len(rows))
	for _, row := range rows {
		row.Category = strings.ToLower(strings.TrimSpace(row.Category))
		result[row.Category] = row
	}
	return result, nil
}

func GetTextCategoryMultiplier(category string) (float64, bool, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" || category == textpricing.CategoryUnclassified {
		return 0, false, nil
	}
	var row TextCategoryPricing
	err := DB.Where("category = ?", category).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if err := textpricing.ValidateMultiplier(row.Multiplier); err != nil {
		return 0, false, err
	}
	return row.Multiplier, true, nil
}

func GetTextPricingModel(modelName string) (Model, bool, error) {
	if DB == nil || !DB.Migrator().HasTable(&Model{}) {
		return Model{}, false, nil
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return Model{}, false, nil
	}
	names := []string{modelName}
	formatted := ratio_setting.FormatMatchingModelName(modelName)
	if formatted != "" && formatted != modelName {
		names = append(names, formatted)
	}
	for _, name := range names {
		var entry Model
		err := DB.Where("model_name = ?", name).First(&entry).Error
		if err == nil {
			return entry, true, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return Model{}, false, err
		}
	}

	var rules []Model
	if err := DB.Where("name_rule <> ?", NameRuleExact).Order("id asc").Find(&rules).Error; err != nil {
		return Model{}, false, err
	}
	for _, rule := range rules {
		switch rule.NameRule {
		case NameRulePrefix:
			if strings.HasPrefix(modelName, rule.ModelName) {
				return rule, true, nil
			}
		case NameRuleContains:
			if strings.Contains(modelName, rule.ModelName) {
				return rule, true, nil
			}
		case NameRuleSuffix:
			if strings.HasSuffix(modelName, rule.ModelName) {
				return rule, true, nil
			}
		}
	}
	return Model{}, false, nil
}

func ResolveTextModelPricing(modelName string) (TextPricingResolution, error) {
	entry, ok, err := GetTextPricingModel(modelName)
	if err != nil {
		return TextPricingResolution{}, err
	}
	if !ok {
		return TextPricingResolution{}, fmt.Errorf("text model %s has no model metadata", modelName)
	}
	return ResolveTextModelPricingFromMetadata(entry, nil)
}

func ResolveTextModelPricingFromMetadata(entry Model, multiplierOverride *float64) (TextPricingResolution, error) {
	if strings.TrimSpace(entry.Modal) != ModelModalText {
		return TextPricingResolution{}, fmt.Errorf("model %s is not classified as text", entry.ModelName)
	}
	category := strings.ToLower(strings.TrimSpace(entry.TextCategory))
	if category == "" || category == textpricing.CategoryUnclassified || !textpricing.IsCategory(category) {
		return TextPricingResolution{}, fmt.Errorf("model %s has no valid text category", entry.ModelName)
	}
	key := strings.TrimSpace(entry.OfficialPriceKey)
	if key == "" {
		return TextPricingResolution{}, fmt.Errorf("model %s has no official price profile", entry.ModelName)
	}
	profile, ok := textpricing.Get(key)
	if !ok {
		return TextPricingResolution{}, fmt.Errorf("model %s references unknown official price profile %s", entry.ModelName, key)
	}
	if profile.Category != category {
		return TextPricingResolution{}, fmt.Errorf("model %s category %s does not match official price profile category %s", entry.ModelName, category, profile.Category)
	}

	multiplier := 0.0
	if multiplierOverride != nil {
		multiplier = *multiplierOverride
	} else {
		var found bool
		var err error
		multiplier, found, err = GetTextCategoryMultiplier(category)
		if err != nil {
			return TextPricingResolution{}, err
		}
		if !found {
			return TextPricingResolution{}, fmt.Errorf("text category %s has no configured multiplier", category)
		}
	}
	pricing, err := textpricing.BuildPricing(profile, multiplier, true, "official_catalog")
	if err != nil {
		return TextPricingResolution{}, err
	}
	return TextPricingResolution{
		Model:      entry,
		Profile:    profile,
		Multiplier: multiplier,
		Pricing:    pricing,
	}, nil
}

func ValidateModelTextPricing(entry *Model) error {
	if entry == nil || strings.TrimSpace(entry.Modal) != ModelModalText {
		return nil
	}
	_, err := ResolveTextModelPricingFromMetadata(*entry, nil)
	return err
}

func PrepareModelTextPricingMetadata(entry *Model) {
	if entry == nil {
		return
	}
	entry.Modal = strings.ToLower(strings.TrimSpace(entry.Modal))
	entry.TextCategory = strings.ToLower(strings.TrimSpace(entry.TextCategory))
	entry.OfficialPriceKey = strings.TrimSpace(entry.OfficialPriceKey)
	if entry.Modal != ModelModalText {
		entry.TextCategory = ""
		entry.OfficialPriceKey = ""
		return
	}
	if entry.TextCategory == "" {
		entry.TextCategory = textpricing.CategoryUnclassified
	}
}

func ValidateEnabledTextPricingModels() error {
	var models []Model
	if err := DB.Where("modal = ? AND status = ?", ModelModalText, 1).Order("model_name asc").Find(&models).Error; err != nil {
		return err
	}
	invalid := make([]string, 0)
	for i := range models {
		if err := ValidateModelTextPricing(&models[i]); err != nil {
			invalid = append(invalid, err.Error())
		}
	}
	if len(invalid) > 0 {
		limit := len(invalid)
		if limit > 8 {
			limit = 8
		}
		return fmt.Errorf("%d enabled text models are not pricing-ready: %s", len(invalid), strings.Join(invalid[:limit], "; "))
	}
	return nil
}

func EnrichTextPricingModels(models []*Model) {
	for _, entry := range models {
		if entry == nil || strings.TrimSpace(entry.Modal) != ModelModalText {
			continue
		}
		resolution, err := ResolveTextModelPricingFromMetadata(*entry, nil)
		if err != nil {
			entry.PricingReady = false
			entry.PricingError = err.Error()
			continue
		}
		profile := textpricing.ToPublicProfile(resolution.Profile)
		entry.PricingReady = true
		entry.PricingError = ""
		entry.OfficialPriceProfile = &profile
		entry.EffectiveTextPricing = effectiveTextPricingView(resolution.Pricing)
	}
}

func effectiveTextPricingView(pricing *types.CreditsTextPricing) *EffectiveTextPricingView {
	if pricing == nil {
		return nil
	}
	return &EffectiveTextPricingView{
		Category:                    pricing.TextCategory,
		CategoryMultiplier:          pricing.CategoryMultiplier,
		CatalogVersion:              pricing.CatalogVersion,
		OfficialPriceKey:            pricing.OfficialPriceKey,
		PricingSource:               pricing.PricingSource,
		InputQuotaPerMillion:        pricing.InputQuotaPerMillion,
		OutputQuotaPerMillion:       pricing.OutputQuotaPerMillion,
		CachedInputQuotaPerMillion:  pricing.CachedInputQuotaPerMillion,
		CacheWriteQuotaPerMillion:   pricing.CacheWriteQuotaPerMillion,
		CacheWrite5mQuotaPerMillion: pricing.CacheWrite5mQuotaPerMillion,
		CacheWrite1hQuotaPerMillion: pricing.CacheWrite1hQuotaPerMillion,
		Tiers:                       pricing.Tiers,
	}
}

func GetTextPricingConfigView() (TextPricingConfigView, error) {
	rows, err := GetTextCategoryPricingRows()
	if err != nil {
		return TextPricingConfigView{}, err
	}
	var models []Model
	if err := DB.Where("modal = ?", ModelModalText).Find(&models).Error; err != nil {
		return TextPricingConfigView{}, err
	}

	counts := make(map[string]int)
	ready := make(map[string]int)
	for _, entry := range models {
		category := strings.ToLower(strings.TrimSpace(entry.TextCategory))
		if category == "" {
			category = textpricing.CategoryUnclassified
		}
		counts[category]++
		if _, err := ResolveTextModelPricingFromMetadata(entry, nil); err == nil {
			ready[category]++
		}
	}

	categories := make([]TextPricingCategoryConfig, 0, len(textpricing.Categories()))
	for _, category := range textpricing.Categories() {
		config := TextPricingCategoryConfig{
			Category:          category,
			ModelCount:        counts[category],
			PricingReadyCount: ready[category],
		}
		if row, ok := rows[category]; ok {
			multiplier := row.Multiplier
			config.Multiplier = &multiplier
			config.UpdatedTime = row.UpdatedTime
		}
		categories = append(categories, config)
	}
	return TextPricingConfigView{
		Mode:           GetTextPricingMode(),
		CatalogVersion: textpricing.CatalogVersion,
		Categories:     categories,
		Profiles:       textpricing.List(),
	}, nil
}

func PreviewTextCategoryMultiplier(category string, multiplier float64) (TextPricingPreview, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == textpricing.CategoryUnclassified || !textpricing.IsCategory(category) {
		return TextPricingPreview{}, fmt.Errorf("invalid text pricing category %s", category)
	}
	if err := textpricing.ValidateMultiplier(multiplier); err != nil {
		return TextPricingPreview{}, err
	}

	var models []Model
	if err := DB.Where("modal = ? AND text_category = ?", ModelModalText, category).Order("model_name asc").Find(&models).Error; err != nil {
		return TextPricingPreview{}, err
	}
	current, found, err := GetTextCategoryMultiplier(category)
	if err != nil {
		return TextPricingPreview{}, err
	}
	var currentPtr *float64
	if found {
		currentPtr = &current
	}
	preview := TextPricingPreview{
		AffectedCount: len(models),
		Before: TextPricingPreviewSummary{
			Category:   category,
			Multiplier: currentPtr,
		},
		After: TextPricingPreviewSummary{
			Category:   category,
			Multiplier: &multiplier,
		},
	}
	for _, entry := range models {
		preview.Before.Models = append(preview.Before.Models, pricingImpact(entry, currentPtr))
		preview.After.Models = append(preview.After.Models, pricingImpact(entry, &multiplier))
	}
	return preview, nil
}

func pricingImpact(entry Model, multiplier *float64) TextPricingImpact {
	impact := TextPricingImpact{
		ID:               entry.Id,
		ModelName:        entry.ModelName,
		OfficialPriceKey: entry.OfficialPriceKey,
	}
	if multiplier == nil {
		impact.PricingError = "category multiplier is not configured"
		return impact
	}
	resolution, err := ResolveTextModelPricingFromMetadata(entry, multiplier)
	if err != nil {
		impact.PricingError = err.Error()
		return impact
	}
	impact.PricingReady = true
	impact.InputQuotaPerMillion = resolution.Pricing.InputQuotaPerMillion
	impact.OutputQuotaPerMillion = resolution.Pricing.OutputQuotaPerMillion
	return impact
}

func UpdateTextCategoryMultiplier(category string, multiplier float64) (TextCategoryPricing, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == textpricing.CategoryUnclassified || !textpricing.IsCategory(category) {
		return TextCategoryPricing{}, fmt.Errorf("invalid text pricing category %s", category)
	}
	if err := textpricing.ValidateMultiplier(multiplier); err != nil {
		return TextCategoryPricing{}, err
	}
	entry := TextCategoryPricing{
		Category:    category,
		Multiplier:  multiplier,
		UpdatedTime: common.GetTimestamp(),
	}
	tx := DB.Begin()
	if tx.Error != nil {
		return TextCategoryPricing{}, tx.Error
	}
	var existing TextCategoryPricing
	err := tx.Where("category = ?", category).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if err := tx.Create(&entry).Error; err != nil {
			tx.Rollback()
			return TextCategoryPricing{}, err
		}
	case err != nil:
		tx.Rollback()
		return TextCategoryPricing{}, err
	default:
		if err := tx.Model(&existing).Updates(map[string]any{
			"multiplier":   entry.Multiplier,
			"updated_time": entry.UpdatedTime,
		}).Error; err != nil {
			tx.Rollback()
			return TextCategoryPricing{}, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return TextCategoryPricing{}, err
	}
	InvalidatePricingCache()
	return entry, nil
}

func SortedTextPricingModelsByCategory(category string) ([]Model, error) {
	var models []Model
	if err := DB.Where("modal = ? AND text_category = ?", ModelModalText, strings.ToLower(strings.TrimSpace(category))).Find(&models).Error; err != nil {
		return nil, err
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ModelName < models[j].ModelName })
	return models, nil
}
