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

	TextMultiplierSourceCategory      = "category"
	TextMultiplierSourceModelOverride = "model_override"
)

type TextPricingProfileView = textpricing.PublicProfile

type EffectiveTextPricingView struct {
	Category                    string                         `json:"category"`
	CategoryMultiplier          *float64                       `json:"category_multiplier,omitempty"`
	ModelMultiplierOverride     *float64                       `json:"model_multiplier_override,omitempty"`
	EffectiveMultiplier         float64                        `json:"effective_multiplier"`
	MultiplierSource            string                         `json:"multiplier_source"`
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
	Model                   Model
	Profile                 textpricing.Profile
	CategoryMultiplier      *float64
	ModelMultiplierOverride *float64
	EffectiveMultiplier     float64
	MultiplierSource        string
	Multiplier              float64
	Pricing                 *types.CreditsTextPricing
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
	pricing.EffectiveMultiplier = multiplier
	pricing.MultiplierSource = TextMultiplierSourceCategory
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
		return nil, false, errors.New("model pricing parity is not trusted; active text pricing is blocked")
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
	if resolution.CategoryMultiplier == nil {
		return nil, false, fmt.Errorf("text category %s has no configured multiplier", resolution.Profile.Category)
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
	Category            string   `json:"category"`
	Multiplier          *float64 `json:"multiplier,omitempty"`
	ModelCount          int      `json:"model_count"`
	PricingReadyCount   int      `json:"pricing_ready_count"`
	PricingBlockedCount int      `json:"pricing_blocked_count"`
	OverrideCount       int      `json:"override_count"`
	InheritedCount      int      `json:"inherited_count"`
	CatalogProfileCount int      `json:"catalog_profile_count"`
	ActivationReady     bool     `json:"activation_ready"`
	ActivationError     string   `json:"activation_error,omitempty"`
	UpdatedTime         int64    `json:"updated_time,omitempty"`
}

type TextPricingConfigView struct {
	Mode                        string                      `json:"mode"`
	CatalogVersion              string                      `json:"catalog_version"`
	Categories                  []TextPricingCategoryConfig `json:"categories"`
	Profiles                    []textpricing.PublicProfile `json:"profiles"`
	PendingCount                int                         `json:"pending_count"`
	UnclassifiedCount           int                         `json:"unclassified_count"`
	MissingOfficialProfileCount int                         `json:"missing_official_profile_count"`
	ActivationReady             bool                        `json:"activation_ready"`
	ActivationBlockers          []string                    `json:"activation_blockers"`
}

type TextPricingImpact struct {
	ID                      int      `json:"id"`
	ModelName               string   `json:"model_name"`
	OfficialPriceKey        string   `json:"official_price_key"`
	CategoryMultiplier      *float64 `json:"category_multiplier,omitempty"`
	ModelMultiplierOverride *float64 `json:"model_multiplier_override,omitempty"`
	EffectiveMultiplier     float64  `json:"effective_multiplier,omitempty"`
	MultiplierSource        string   `json:"multiplier_source,omitempty"`
	InputQuotaPerMillion    int64    `json:"input_quota_per_million"`
	OutputQuotaPerMillion   int64    `json:"output_quota_per_million"`
	PricingReady            bool     `json:"pricing_ready"`
	Affected                bool     `json:"affected"`
	PricingError            string   `json:"pricing_error,omitempty"`
}

type TextPricingPreviewSummary struct {
	Category   string              `json:"category"`
	Multiplier *float64            `json:"multiplier,omitempty"`
	Models     []TextPricingImpact `json:"models"`
}

type TextPricingPreview struct {
	AffectedCount int                       `json:"affected_count"`
	OverrideCount int                       `json:"override_count"`
	Before        TextPricingPreviewSummary `json:"before"`
	After         TextPricingPreviewSummary `json:"after"`
}

type TextPricingModelPreview struct {
	ModelID   int               `json:"model_id"`
	ModelName string            `json:"model_name"`
	Before    TextPricingImpact `json:"before"`
	After     TextPricingImpact `json:"after"`
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
		if err := ValidateTextPricingActivationReadiness(); err != nil {
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
	return getTextCategoryMultiplier(DB, category, false)
}

func getTextCategoryMultiplier(tx *gorm.DB, category string, forUpdate bool) (float64, bool, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" || category == textpricing.CategoryUnclassified {
		return 0, false, nil
	}
	var row TextCategoryPricing
	query := tx
	if forUpdate {
		query = lockForUpdate(query)
	}
	err := query.Where("category = ?", category).First(&row).Error
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
	return resolveTextModelPricingFromMetadata(entry, textPricingResolutionOptions{
		categoryMultiplierOverride: multiplierOverride,
	})
}

func resolveTextModelPricingFromRows(entry Model, rows map[string]TextCategoryPricing) (TextPricingResolution, error) {
	category := strings.ToLower(strings.TrimSpace(entry.TextCategory))
	var categoryMultiplier *float64
	if row, ok := rows[category]; ok {
		if err := textpricing.ValidateMultiplier(row.Multiplier); err != nil {
			return TextPricingResolution{}, err
		}
		value := row.Multiplier
		categoryMultiplier = &value
	}
	return resolveTextModelPricingFromMetadata(entry, textPricingResolutionOptions{
		categoryMultiplierOverride: categoryMultiplier,
		replaceCategoryMultiplier:  true,
	})
}

type textPricingResolutionOptions struct {
	categoryMultiplierOverride *float64
	replaceCategoryMultiplier  bool
	modelMultiplierOverride    *float64
	replaceModelOverride       bool
}

func ResolveTextModelPricingWithModelOverride(entry Model, multiplier *float64) (TextPricingResolution, error) {
	return resolveTextModelPricingFromMetadata(entry, textPricingResolutionOptions{
		modelMultiplierOverride: multiplier,
		replaceModelOverride:    true,
	})
}

func resolveTextModelPricingFromMetadata(entry Model, options textPricingResolutionOptions) (TextPricingResolution, error) {
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

	var categoryMultiplier *float64
	if options.replaceCategoryMultiplier || options.categoryMultiplierOverride != nil {
		if options.categoryMultiplierOverride != nil {
			value := *options.categoryMultiplierOverride
			if err := textpricing.ValidateMultiplier(value); err != nil {
				return TextPricingResolution{}, err
			}
			categoryMultiplier = &value
		}
	} else {
		value, found, err := GetTextCategoryMultiplier(category)
		if err != nil {
			return TextPricingResolution{}, err
		}
		if found {
			categoryMultiplier = &value
		}
	}

	modelMultiplierOverride := entry.TextMultiplierOverride
	if options.replaceModelOverride {
		modelMultiplierOverride = options.modelMultiplierOverride
	}
	if modelMultiplierOverride != nil {
		value := *modelMultiplierOverride
		if err := textpricing.ValidateMultiplier(value); err != nil {
			return TextPricingResolution{}, fmt.Errorf("model %s has invalid multiplier override: %w", entry.ModelName, err)
		}
		modelMultiplierOverride = &value
	}

	effectiveMultiplier := 0.0
	multiplierSource := TextMultiplierSourceCategory
	if modelMultiplierOverride != nil {
		effectiveMultiplier = *modelMultiplierOverride
		multiplierSource = TextMultiplierSourceModelOverride
	} else {
		if categoryMultiplier == nil {
			return TextPricingResolution{}, fmt.Errorf("text category %s has no configured multiplier", category)
		}
		effectiveMultiplier = *categoryMultiplier
	}
	pricing, err := textpricing.BuildPricing(profile, effectiveMultiplier, true, "official_catalog")
	if err != nil {
		return TextPricingResolution{}, err
	}
	pricing.CategoryMultiplier = 0
	if categoryMultiplier != nil {
		pricing.CategoryMultiplier = *categoryMultiplier
	}
	pricing.ModelMultiplierOverride = modelMultiplierOverride
	pricing.EffectiveMultiplier = effectiveMultiplier
	pricing.MultiplierSource = multiplierSource
	return TextPricingResolution{
		Model:                   entry,
		Profile:                 profile,
		CategoryMultiplier:      categoryMultiplier,
		ModelMultiplierOverride: modelMultiplierOverride,
		EffectiveMultiplier:     effectiveMultiplier,
		MultiplierSource:        multiplierSource,
		Multiplier:              effectiveMultiplier,
		Pricing:                 pricing,
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

func ValidateTextPricingActivationReadiness() error {
	if !IsModelPricingConfigTrusted() {
		return errors.New("model pricing parity is not trusted; active text pricing is blocked")
	}
	for _, category := range textpricing.BillableCategories() {
		if _, found, err := GetTextCategoryMultiplier(category); err != nil {
			return err
		} else if !found {
			return fmt.Errorf("text category %s has no configured multiplier", category)
		}
	}
	return ValidateEnabledTextPricingModels()
}

func EnrichTextPricingModels(models []*Model) {
	rows, rowsErr := GetTextCategoryPricingRows()
	for _, entry := range models {
		if entry == nil || strings.TrimSpace(entry.Modal) != ModelModalText {
			continue
		}
		if rowsErr != nil {
			entry.PricingReady = false
			entry.PricingError = rowsErr.Error()
			continue
		}
		resolution, err := resolveTextModelPricingFromRows(*entry, rows)
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
		CategoryMultiplier:          optionalMultiplier(pricing.CategoryMultiplier),
		ModelMultiplierOverride:     pricing.ModelMultiplierOverride,
		EffectiveMultiplier:         pricing.EffectiveMultiplier,
		MultiplierSource:            pricing.MultiplierSource,
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

func optionalMultiplier(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func GetTextPricingConfigView() (TextPricingConfigView, error) {
	rows, err := GetTextCategoryPricingRows()
	if err != nil {
		return TextPricingConfigView{}, err
	}
	var models []Model
	if err := DB.Select("id", "model_name", "status", "modal", "text_category", "official_price_key", "text_multiplier_override").
		Where("modal = ?", ModelModalText).
		Find(&models).Error; err != nil {
		return TextPricingConfigView{}, err
	}

	counts := make(map[string]int)
	ready := make(map[string]int)
	blocked := make(map[string]int)
	activationBlocked := make(map[string]int)
	overrides := make(map[string]int)
	inherited := make(map[string]int)
	profileCounts := make(map[string]int)
	for _, profile := range textpricing.List() {
		profileCounts[profile.Category]++
	}
	pendingCount := 0
	unclassifiedCount := 0
	missingProfileCount := 0
	activationBlockers := make([]string, 0)
	if !IsModelPricingConfigTrusted() {
		activationBlockers = append(activationBlockers, "model pricing parity is not trusted")
	}
	for _, entry := range models {
		category := strings.ToLower(strings.TrimSpace(entry.TextCategory))
		issueKind, metadataErr := textPricingMetadataIssue(entry)
		if metadataErr != nil {
			pendingCount++
			if issueKind == textpricing.CategoryUnclassified {
				unclassifiedCount++
			} else {
				missingProfileCount++
			}
			if entry.Status == 1 && len(activationBlockers) < 12 {
				activationBlockers = append(activationBlockers, metadataErr.Error())
			}
		}
		if !textpricing.IsBillableCategory(category) {
			continue
		}
		counts[category]++
		if entry.TextMultiplierOverride != nil {
			overrides[category]++
		} else {
			inherited[category]++
		}
		if _, err := resolveTextModelPricingFromRows(entry, rows); err == nil {
			ready[category]++
		} else {
			blocked[category]++
			if entry.Status == 1 {
				activationBlocked[category]++
				if metadataErr == nil && len(activationBlockers) < 12 {
					activationBlockers = append(activationBlockers, err.Error())
				}
			}
		}
	}

	categories := make([]TextPricingCategoryConfig, 0, len(textpricing.BillableCategories()))
	for _, category := range textpricing.BillableCategories() {
		config := TextPricingCategoryConfig{
			Category:            category,
			ModelCount:          counts[category],
			PricingReadyCount:   ready[category],
			PricingBlockedCount: blocked[category],
			OverrideCount:       overrides[category],
			InheritedCount:      inherited[category],
			CatalogProfileCount: profileCounts[category],
		}
		if row, ok := rows[category]; ok {
			multiplier := row.Multiplier
			if err := textpricing.ValidateMultiplier(multiplier); err == nil {
				config.Multiplier = &multiplier
				config.UpdatedTime = row.UpdatedTime
			} else {
				config.ActivationError = err.Error()
			}
		}
		if config.Multiplier == nil && config.ActivationError == "" {
			config.ActivationError = fmt.Sprintf("text category %s has no configured multiplier", category)
		}
		if config.ActivationError != "" {
			activationBlockers = append(activationBlockers, config.ActivationError)
		}
		if activationBlocked[category] > 0 && config.ActivationError == "" {
			config.ActivationError = fmt.Sprintf("%d enabled models are not pricing-ready", activationBlocked[category])
		}
		config.ActivationReady = config.ActivationError == ""
		categories = append(categories, config)
	}
	return TextPricingConfigView{
		Mode:                        GetTextPricingMode(),
		CatalogVersion:              textpricing.CatalogVersion,
		Categories:                  categories,
		Profiles:                    textpricing.List(),
		PendingCount:                pendingCount,
		UnclassifiedCount:           unclassifiedCount,
		MissingOfficialProfileCount: missingProfileCount,
		ActivationReady:             len(activationBlockers) == 0,
		ActivationBlockers:          activationBlockers,
	}, nil
}

func textPricingMetadataIssue(entry Model) (string, error) {
	category := strings.ToLower(strings.TrimSpace(entry.TextCategory))
	if !textpricing.IsBillableCategory(category) {
		return textpricing.CategoryUnclassified, fmt.Errorf("model %s has no valid text category", entry.ModelName)
	}
	key := strings.TrimSpace(entry.OfficialPriceKey)
	if key == "" {
		return "missing_profile", fmt.Errorf("model %s has no official price profile", entry.ModelName)
	}
	profile, ok := textpricing.Get(key)
	if !ok {
		return "missing_profile", fmt.Errorf("model %s references unknown official price profile %s", entry.ModelName, key)
	}
	if profile.Category != category {
		return "missing_profile", fmt.Errorf("model %s category %s does not match official price profile category %s", entry.ModelName, category, profile.Category)
	}
	return "", nil
}

func GetPendingTextPricingModelIDs() ([]int, error) {
	var models []Model
	if err := DB.Select("id", "model_name", "modal", "text_category", "official_price_key").
		Where("modal = ?", ModelModalText).
		Find(&models).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0)
	for _, entry := range models {
		if _, err := textPricingMetadataIssue(entry); err != nil {
			ids = append(ids, entry.Id)
		}
	}
	return ids, nil
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
		affected := entry.TextMultiplierOverride == nil
		if affected {
			preview.AffectedCount++
		} else {
			preview.OverrideCount++
		}
		preview.Before.Models = append(preview.Before.Models, pricingImpact(entry, currentPtr, affected))
		preview.After.Models = append(preview.After.Models, pricingImpact(entry, &multiplier, affected))
	}
	return preview, nil
}

func pricingImpact(entry Model, categoryMultiplier *float64, affected bool) TextPricingImpact {
	impact := TextPricingImpact{
		ID:                      entry.Id,
		ModelName:               entry.ModelName,
		OfficialPriceKey:        entry.OfficialPriceKey,
		CategoryMultiplier:      categoryMultiplier,
		ModelMultiplierOverride: entry.TextMultiplierOverride,
		Affected:                affected,
	}
	resolution, err := ResolveTextModelPricingFromMetadata(entry, categoryMultiplier)
	if err != nil {
		impact.PricingError = err.Error()
		return impact
	}
	impact.CategoryMultiplier = resolution.CategoryMultiplier
	impact.ModelMultiplierOverride = resolution.ModelMultiplierOverride
	impact.EffectiveMultiplier = resolution.EffectiveMultiplier
	impact.MultiplierSource = resolution.MultiplierSource
	impact.PricingReady = true
	impact.InputQuotaPerMillion = resolution.Pricing.InputQuotaPerMillion
	impact.OutputQuotaPerMillion = resolution.Pricing.OutputQuotaPerMillion
	return impact
}

func pricingImpactWithResolvedMultipliers(
	entry Model,
	categoryMultiplier *float64,
	modelMultiplier *float64,
	affected bool,
	replaceCategoryMultiplier bool,
) TextPricingImpact {
	impact := TextPricingImpact{
		ID:                      entry.Id,
		ModelName:               entry.ModelName,
		OfficialPriceKey:        entry.OfficialPriceKey,
		CategoryMultiplier:      categoryMultiplier,
		ModelMultiplierOverride: modelMultiplier,
		Affected:                affected,
	}
	resolution, err := resolveTextModelPricingFromMetadata(entry, textPricingResolutionOptions{
		categoryMultiplierOverride: categoryMultiplier,
		replaceCategoryMultiplier:  replaceCategoryMultiplier,
		modelMultiplierOverride:    modelMultiplier,
		replaceModelOverride:       true,
	})
	if err != nil {
		impact.PricingError = err.Error()
		return impact
	}
	impact.CategoryMultiplier = resolution.CategoryMultiplier
	impact.ModelMultiplierOverride = resolution.ModelMultiplierOverride
	impact.EffectiveMultiplier = resolution.EffectiveMultiplier
	impact.MultiplierSource = resolution.MultiplierSource
	impact.PricingReady = true
	impact.InputQuotaPerMillion = resolution.Pricing.InputQuotaPerMillion
	impact.OutputQuotaPerMillion = resolution.Pricing.OutputQuotaPerMillion
	return impact
}

func PreviewTextModelMultiplier(modelID int, multiplier *float64) (TextPricingModelPreview, error) {
	var preview TextPricingModelPreview
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		preview, err = previewTextModelMultiplier(tx, modelID, multiplier, false)
		return err
	})
	return preview, err
}

func previewTextModelMultiplier(tx *gorm.DB, modelID int, multiplier *float64, forUpdate bool) (TextPricingModelPreview, error) {
	if modelID <= 0 {
		return TextPricingModelPreview{}, errors.New("model_id must be greater than 0")
	}
	if multiplier != nil {
		if err := textpricing.ValidateMultiplier(*multiplier); err != nil {
			return TextPricingModelPreview{}, err
		}
	}
	var entry Model
	query := tx
	if forUpdate {
		query = lockForUpdate(query)
	}
	if err := query.Where("id = ?", modelID).First(&entry).Error; err != nil {
		return TextPricingModelPreview{}, err
	}
	if strings.TrimSpace(entry.Modal) != ModelModalText {
		return TextPricingModelPreview{}, fmt.Errorf("model %s is not classified as text", entry.ModelName)
	}
	if _, err := textPricingMetadataIssue(entry); err != nil {
		return TextPricingModelPreview{}, err
	}
	categoryMultiplierValue, found, err := getTextCategoryMultiplier(tx, entry.TextCategory, forUpdate)
	if err != nil {
		return TextPricingModelPreview{}, err
	}
	var categoryMultiplier *float64
	if found {
		categoryMultiplier = &categoryMultiplierValue
	}
	return TextPricingModelPreview{
		ModelID:   entry.Id,
		ModelName: entry.ModelName,
		Before: pricingImpactWithResolvedMultipliers(
			entry,
			categoryMultiplier,
			entry.TextMultiplierOverride,
			false,
			true,
		),
		After: pricingImpactWithResolvedMultipliers(
			entry,
			categoryMultiplier,
			multiplier,
			true,
			true,
		),
	}, nil
}

func UpdateTextModelMultiplier(modelID int, multiplier *float64) (TextPricingModelPreview, error) {
	var preview TextPricingModelPreview
	now := common.GetTimestamp()
	if err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		preview, err = previewTextModelMultiplier(tx, modelID, multiplier, true)
		if err != nil {
			return err
		}
		return tx.Model(&Model{}).Where("id = ?", modelID).Updates(map[string]any{
			"text_multiplier_override": multiplier,
			"pricing_updated_time":     now,
			"updated_time":             now,
		}).Error
	}); err != nil {
		return TextPricingModelPreview{}, err
	}
	InvalidatePricingCache()
	return preview, nil
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
	err := lockForUpdate(tx).Where("category = ?", category).First(&existing).Error
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
