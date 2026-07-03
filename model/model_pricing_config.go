package model

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

const (
	PricingModeRatio       = "ratio"
	PricingModeImageSpec   = "image_spec"
	PricingModeVideoMatrix = "video_matrix"
	PricingModeFree        = "free"
	PricingModeInherit     = "inherit"
)

const (
	ModelModalText  = "text"
	ModelModalImage = "image"
	ModelModalVideo = "video"
	ModelModalAudio = "audio"
)

type ModelPricingConfig struct {
	Mode string `json:"mode"`

	BaseRatio            float64 `json:"base_ratio"`
	CompletionRatio      float64 `json:"completion_ratio"`
	CacheRatio           float64 `json:"cache_ratio"`
	CreateCacheRatio     float64 `json:"create_cache_ratio"`
	ModelPrice           float64 `json:"model_price"`
	UsePrice             bool    `json:"use_price,omitempty"`
	UseRatio             bool    `json:"use_ratio,omitempty"`
	ImageRatio           float64 `json:"image_ratio"`
	AudioRatio           float64 `json:"audio_ratio"`
	AudioCompletionRatio float64 `json:"audio_completion_ratio"`

	Unit               string                                              `json:"unit,omitempty"`
	Resolutions        map[string]ModelSpecResolutionPrice                 `json:"resolutions,omitempty"`
	Qualities          map[string]operation_setting.AsyncImageQualityPrice `json:"qualities,omitempty"`
	DefaultCNYPerImage *float64                                            `json:"default_cny_per_image,omitempty"`

	Prices              map[string]operation_setting.AsyncVideoRatioPrices `json:"prices,omitempty"`
	DefaultCNYPerSecond *float64                                           `json:"default_cny_per_second,omitempty"`
	MinCNY              float64                                            `json:"min_cny,omitempty"`
	MaxCNY              float64                                            `json:"max_cny,omitempty"`
}

type ModelSpecResolutionPrice struct {
	CNYPerImage  *float64 `json:"cny_per_image,omitempty"`
	CNYPerSecond *float64 `json:"cny_per_second,omitempty"`
}

type ModelPricingMigrationStats struct {
	TotalCandidates        int `json:"total_candidates"`
	CreatedModels          int `json:"created_models"`
	UpdatedModels          int `json:"updated_models"`
	PricedModels           int `json:"priced_models"`
	SkippedExistingConfigs int `json:"skipped_existing_configs"`
}

type PricingCompareReport struct {
	CheckedTextModels        int                      `json:"checked_text_models"`
	CheckedImageCombinations int                      `json:"checked_image_combinations"`
	CheckedVideoCombinations int                      `json:"checked_video_combinations"`
	Mismatches               []PricingCompareMismatch `json:"mismatches"`
}

type PricingCompareMismatch struct {
	Kind              string `json:"kind"`
	Model             string `json:"model"`
	SpecKey           string `json:"spec_key,omitempty"`
	LegacyQuota       int    `json:"legacy_quota"`
	NewQuota          int    `json:"new_quota"`
	LegacyUnsupported bool   `json:"legacy_unsupported,omitempty"`
	NewUnsupported    bool   `json:"new_unsupported,omitempty"`
	Reason            string `json:"reason,omitempty"`
}

type PricingParityStatus struct {
	Trusted       bool                     `json:"trusted"`
	CheckedText   int                      `json:"checked_text"`
	CheckedImage  int                      `json:"checked_image"`
	CheckedVideo  int                      `json:"checked_video"`
	MismatchCount int                      `json:"mismatch_count"`
	Mismatches    []PricingCompareMismatch `json:"mismatches"`
	Error         string                   `json:"error,omitempty"`
}

var (
	modelPricingParityMu      sync.RWMutex
	ModelPricingConfigTrusted bool
	modelPricingParityReport  PricingCompareReport
	modelPricingParityError   string
)

func IsModelPricingConfigTrusted() bool {
	modelPricingParityMu.RLock()
	defer modelPricingParityMu.RUnlock()
	return ModelPricingConfigTrusted
}

func SetModelPricingConfigTrustedForTest(trusted bool) func() {
	modelPricingParityMu.Lock()
	previousTrusted := ModelPricingConfigTrusted
	previousReport := modelPricingParityReport
	previousError := modelPricingParityError
	ModelPricingConfigTrusted = trusted
	modelPricingParityReport = PricingCompareReport{}
	modelPricingParityError = ""
	modelPricingParityMu.Unlock()

	return func() {
		modelPricingParityMu.Lock()
		ModelPricingConfigTrusted = previousTrusted
		modelPricingParityReport = previousReport
		modelPricingParityError = previousError
		modelPricingParityMu.Unlock()
	}
}

func SetModelPricingParityForTest(report PricingCompareReport, trusted bool, errorMessage string) func() {
	modelPricingParityMu.Lock()
	previousTrusted := ModelPricingConfigTrusted
	previousReport := modelPricingParityReport
	previousError := modelPricingParityError
	ModelPricingConfigTrusted = trusted
	modelPricingParityReport = report
	modelPricingParityError = errorMessage
	modelPricingParityMu.Unlock()

	return func() {
		modelPricingParityMu.Lock()
		ModelPricingConfigTrusted = previousTrusted
		modelPricingParityReport = previousReport
		modelPricingParityError = previousError
		modelPricingParityMu.Unlock()
	}
}

func GetModelPricingParityStatus() PricingParityStatus {
	modelPricingParityMu.RLock()
	defer modelPricingParityMu.RUnlock()

	mismatches := modelPricingParityReport.Mismatches
	if len(mismatches) > 20 {
		mismatches = mismatches[:20]
	}
	copiedMismatches := append([]PricingCompareMismatch(nil), mismatches...)
	return PricingParityStatus{
		Trusted:       ModelPricingConfigTrusted,
		CheckedText:   modelPricingParityReport.CheckedTextModels,
		CheckedImage:  modelPricingParityReport.CheckedImageCombinations,
		CheckedVideo:  modelPricingParityReport.CheckedVideoCombinations,
		MismatchCount: len(modelPricingParityReport.Mismatches),
		Mismatches:    copiedMismatches,
		Error:         modelPricingParityError,
	}
}

func (m *Model) ParsePricingConfig() (ModelPricingConfig, error) {
	return ParseModelPricingConfig(m.PricingConfig)
}

func ParseModelPricingConfig(raw string) (ModelPricingConfig, error) {
	var cfg ModelPricingConfig
	if strings.TrimSpace(raw) == "" {
		return cfg, nil
	}
	if err := common.UnmarshalJsonStr(raw, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Mode == "" {
		cfg.Mode = PricingModeInherit
	}
	return cfg, nil
}

func (cfg ModelPricingConfig) JSONString() (string, error) {
	bytes, err := common.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func MigrateModelPricingConfigsOnly() error {
	_, err := MigrateModelPricingConfigs()
	return err
}

func MigrateModelPricingConfigs() (ModelPricingMigrationStats, error) {
	var stats ModelPricingMigrationStats
	if DB == nil || !DB.Migrator().HasTable(&Model{}) {
		return stats, nil
	}

	var existingModels []Model
	if err := DB.Unscoped().Find(&existingModels).Error; err != nil {
		return stats, err
	}
	existingByName := make(map[string]*Model, len(existingModels))
	candidates := map[string]string{}
	for i := range existingModels {
		name := strings.TrimSpace(existingModels[i].ModelName)
		if name == "" {
			continue
		}
		existingByName[name] = &existingModels[i]
		candidates[name] = ""
	}

	modelRatios := ratio_setting.GetModelRatioCopy()
	completionRatios := ratio_setting.GetCompletionRatioCopy()
	cacheRatios := ratio_setting.GetCacheRatioCopy()
	createCacheRatios := ratio_setting.GetCreateCacheRatioCopy()
	modelPrices := ratio_setting.GetModelPriceCopy()
	imageRatios := ratio_setting.GetImageRatioCopy()
	audioRatios := ratio_setting.GetAudioRatioCopy()
	audioCompletionRatios := ratio_setting.GetAudioCompletionRatioCopy()
	asyncPricing := operation_setting.GetAsyncSpecPricingCopy()

	addCandidates(candidates, modelRatios, ModelModalText)
	addCandidates(candidates, completionRatios, ModelModalText)
	addCandidates(candidates, cacheRatios, ModelModalText)
	addCandidates(candidates, createCacheRatios, ModelModalText)
	addCandidates(candidates, modelPrices, ModelModalText)
	addCandidates(candidates, imageRatios, ModelModalImage)
	addCandidates(candidates, audioRatios, ModelModalAudio)
	addCandidates(candidates, audioCompletionRatios, ModelModalAudio)
	for modelName := range asyncPricing.Image {
		candidates[strings.TrimSpace(modelName)] = ModelModalImage
	}
	for modelName := range asyncPricing.Video {
		candidates[strings.TrimSpace(modelName)] = ModelModalVideo
	}

	names := sortedCandidateNames(candidates)
	stats.TotalCandidates = len(names)
	now := common.GetTimestamp()

	for _, modelName := range names {
		if modelName == "" {
			continue
		}
		current := existingByName[modelName]
		if current == nil {
			current = &Model{
				ModelName:    modelName,
				Status:       1,
				SyncOfficial: 0,
				Modal:        inferModalFromPricingSource(modelName, candidates[modelName], nil),
			}
			if err := current.Insert(); err != nil {
				return stats, err
			}
			existingByName[modelName] = current
			stats.CreatedModels++
		}
		cfg, mode, modal := buildPricingConfigForModel(modelName, current, candidates[modelName], asyncPricing, modelPrices, modelRatios, completionRatios, cacheRatios, createCacheRatios, imageRatios, audioRatios, audioCompletionRatios)
		configJSON, err := cfg.JSONString()
		if err != nil {
			return stats, err
		}
		if modal == "" {
			modal = inferModalFromPricingSource(modelName, candidates[modelName], current)
		}
		if modal == "" {
			modal = ModelModalText
		}
		updates := map[string]interface{}{
			"modal":                modal,
			"pricing_mode":         mode,
			"pricing_config":       configJSON,
			"pricing_updated_time": now,
			"updated_time":         now,
		}
		if strings.TrimSpace(current.PricingConfig) != "" && current.PricingMode == mode && strings.TrimSpace(current.PricingConfig) == configJSON {
			stats.SkippedExistingConfigs++
			if isPricedMode(current.PricingMode) {
				stats.PricedModels++
			}
			continue
		}
		if err := DB.Model(&Model{}).Where("id = ?", current.Id).Updates(updates).Error; err != nil {
			return stats, err
		}
		current.Modal = modal
		current.PricingMode = mode
		current.PricingConfig = configJSON
		current.PricingUpdatedTime = now
		current.UpdatedTime = now
		stats.UpdatedModels++
		if isPricedMode(mode) {
			stats.PricedModels++
		}
	}
	return stats, nil
}

func addCandidates(target map[string]string, src map[string]float64, modal string) {
	for modelName := range src {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if target[modelName] == "" {
			target[modelName] = modal
		}
	}
}

func sortedCandidateNames(candidates map[string]string) []string {
	names := make([]string, 0, len(candidates))
	for name := range candidates {
		if strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func buildPricingConfigForModel(
	modelName string,
	current *Model,
	sourceModal string,
	asyncPricing operation_setting.AsyncSpecPricing,
	modelPrices map[string]float64,
	modelRatios map[string]float64,
	completionRatios map[string]float64,
	cacheRatios map[string]float64,
	createCacheRatios map[string]float64,
	imageRatios map[string]float64,
	audioRatios map[string]float64,
	audioCompletionRatios map[string]float64,
) (ModelPricingConfig, string, string) {
	ratioCfg, hasRatioPricing := buildLegacyRatioPricingConfig(modelName)
	if spec, ok := asyncPricing.Video[modelName]; ok {
		cfg := ModelPricingConfig{
			Mode:                PricingModeVideoMatrix,
			Unit:                spec.Unit,
			Resolutions:         videoResolutionsToModelConfig(spec.Resolutions),
			Prices:              spec.Prices,
			DefaultCNYPerSecond: spec.DefaultCNYPerSecond,
			MinCNY:              spec.MinCNY,
			MaxCNY:              spec.MaxCNY,
		}
		if hasRatioPricing {
			copyLegacyRatioPricingFields(&cfg, ratioCfg)
		}
		return cfg, PricingModeVideoMatrix, ModelModalVideo
	}
	if spec, ok := asyncPricing.Image[modelName]; ok {
		cfg := ModelPricingConfig{
			Mode:               PricingModeImageSpec,
			Unit:               spec.Unit,
			Resolutions:        imageResolutionsToModelConfig(spec.Resolutions),
			Qualities:          spec.Qualities,
			DefaultCNYPerImage: spec.DefaultCNYPerImage,
		}
		if hasRatioPricing {
			copyLegacyRatioPricingFields(&cfg, ratioCfg)
		}
		return cfg, PricingModeImageSpec, ModelModalImage
	}

	if hasRatioPricing {
		return ratioCfg, PricingModeRatio, inferModalFromPricingSource(modelName, sourceModal, current)
	}
	cfg := ModelPricingConfig{Mode: PricingModeInherit}
	return cfg, PricingModeInherit, inferModalFromPricingSource(modelName, sourceModal, current)
}

func buildLegacyRatioPricingConfig(modelName string) (ModelPricingConfig, bool) {
	cfg := ModelPricingConfig{
		Mode:                 PricingModeRatio,
		CompletionRatio:      ratio_setting.GetCompletionRatio(modelName),
		CacheRatio:           1,
		CreateCacheRatio:     1.25,
		ImageRatio:           1,
		AudioRatio:           1,
		AudioCompletionRatio: 1,
	}
	if cacheRatio, ok := ratio_setting.GetCacheRatio(modelName); ok {
		cfg.CacheRatio = cacheRatio
	}
	if createCacheRatio, ok := ratio_setting.GetCreateCacheRatio(modelName); ok {
		cfg.CreateCacheRatio = createCacheRatio
	}
	if imageRatio, ok := ratio_setting.GetImageRatio(modelName); ok {
		cfg.ImageRatio = imageRatio
	}
	cfg.AudioRatio = ratio_setting.GetAudioRatio(modelName)
	cfg.AudioCompletionRatio = ratio_setting.GetAudioCompletionRatio(modelName)

	if modelPrice, ok := ratio_setting.GetModelPrice(modelName, false); ok {
		cfg.ModelPrice = modelPrice
		cfg.UsePrice = true
		return cfg, true
	}
	if modelRatio, ok, _ := ratio_setting.GetModelRatio(modelName); ok {
		cfg.BaseRatio = modelRatio
		cfg.UseRatio = true
		return cfg, true
	}
	return cfg, false
}

func copyLegacyRatioPricingFields(dst *ModelPricingConfig, src ModelPricingConfig) {
	dst.BaseRatio = src.BaseRatio
	dst.CompletionRatio = src.CompletionRatio
	dst.CacheRatio = src.CacheRatio
	dst.CreateCacheRatio = src.CreateCacheRatio
	dst.ModelPrice = src.ModelPrice
	dst.UsePrice = src.UsePrice
	dst.UseRatio = src.UseRatio
	dst.ImageRatio = src.ImageRatio
	dst.AudioRatio = src.AudioRatio
	dst.AudioCompletionRatio = src.AudioCompletionRatio
}

func inferModalFromPricingSource(modelName string, sourceModal string, current *Model) string {
	if sourceModal != "" {
		return sourceModal
	}
	var haystack string
	if current != nil {
		if strings.TrimSpace(current.Modal) != "" {
			return strings.TrimSpace(current.Modal)
		}
		haystack = strings.ToLower(strings.Join([]string{current.Endpoints, current.Tags, current.ModelName}, " "))
	} else {
		haystack = strings.ToLower(modelName)
	}
	switch {
	case strings.Contains(haystack, string(constant.EndpointTypeOpenAIVideo)), strings.Contains(haystack, "video"), strings.Contains(haystack, "seedance"), strings.Contains(haystack, "veo"), strings.Contains(haystack, "sora"):
		return ModelModalVideo
	case strings.Contains(haystack, string(constant.EndpointTypeImageGeneration)), strings.Contains(haystack, "image"), strings.Contains(haystack, "dall-e"), strings.Contains(haystack, "imagen"), strings.Contains(haystack, "flux"):
		return ModelModalImage
	case strings.Contains(haystack, "audio"), strings.Contains(haystack, "tts"), strings.Contains(haystack, "whisper"):
		return ModelModalAudio
	default:
		return ModelModalText
	}
}

func isPricedMode(mode string) bool {
	switch mode {
	case PricingModeRatio, PricingModeImageSpec, PricingModeVideoMatrix:
		return true
	default:
		return false
	}
}

func buildPricingCompareReport() (PricingCompareReport, error) {
	var report PricingCompareReport
	if DB == nil || !DB.Migrator().HasTable(&Model{}) {
		return report, nil
	}
	modelConfigs, err := loadPricingConfigsByModel()
	if err != nil {
		return report, err
	}
	compareTextPricing(modelConfigs, &report)
	compareAsyncImagePricing(modelConfigs, &report)
	compareAsyncVideoPricing(modelConfigs, &report)
	return report, nil
}

func CompareMigratedPricingConfigs() (PricingCompareReport, error) {
	report, err := buildPricingCompareReport()
	if err != nil {
		return report, err
	}
	if len(report.Mismatches) > 0 {
		return report, fmt.Errorf("pricing compare found %d mismatches", len(report.Mismatches))
	}
	return report, nil
}

func RunModelPricingParityCheck() PricingParityStatus {
	report, err := CompareMigratedPricingConfigs()
	trusted := err == nil && len(report.Mismatches) == 0
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}

	modelPricingParityMu.Lock()
	ModelPricingConfigTrusted = trusted
	modelPricingParityReport = report
	modelPricingParityError = errorMessage
	modelPricingParityMu.Unlock()

	if trusted {
		common.SysLog(fmt.Sprintf("model pricing parity OK: text=%d image=%d video=%d, 0 mismatch", report.CheckedTextModels, report.CheckedImageCombinations, report.CheckedVideoCombinations))
	} else {
		details := formatPricingParityMismatchDetails(report.Mismatches, 8)
		if errorMessage != "" && details == "" {
			details = errorMessage
		}
		common.SysError(fmt.Sprintf("model pricing parity MISMATCH: %d 处, 详情[%s]; 自动回退旧价格", len(report.Mismatches), details))
	}
	return GetModelPricingParityStatus()
}

func formatPricingParityMismatchDetails(mismatches []PricingCompareMismatch, limit int) string {
	if len(mismatches) == 0 {
		return ""
	}
	if limit <= 0 || limit > len(mismatches) {
		limit = len(mismatches)
	}
	details := make([]string, 0, limit)
	for _, mismatch := range mismatches[:limit] {
		details = append(details, fmt.Sprintf(
			"kind=%s model=%s spec=%s legacy=%d new=%d legacy_unsupported=%t new_unsupported=%t reason=%s",
			mismatch.Kind,
			mismatch.Model,
			mismatch.SpecKey,
			mismatch.LegacyQuota,
			mismatch.NewQuota,
			mismatch.LegacyUnsupported,
			mismatch.NewUnsupported,
			mismatch.Reason,
		))
	}
	return strings.Join(details, "; ")
}

func loadPricingConfigsByModel() (map[string]ModelPricingConfig, error) {
	var models []Model
	if err := DB.Unscoped().Find(&models).Error; err != nil {
		return nil, err
	}
	configs := make(map[string]ModelPricingConfig, len(models))
	for _, m := range models {
		if strings.TrimSpace(m.PricingConfig) == "" {
			continue
		}
		cfg, err := m.ParsePricingConfig()
		if err != nil {
			return nil, fmt.Errorf("parse pricing_config for %s: %w", m.ModelName, err)
		}
		configs[m.ModelName] = cfg
	}
	return configs, nil
}

func compareTextPricing(configs map[string]ModelPricingConfig, report *PricingCompareReport) {
	models := map[string]struct{}{}
	for modelName := range ratio_setting.GetModelPriceCopy() {
		models[modelName] = struct{}{}
	}
	for modelName := range ratio_setting.GetModelRatioCopy() {
		models[modelName] = struct{}{}
	}
	for modelName := range ratio_setting.GetImageRatioCopy() {
		models[modelName] = struct{}{}
	}
	names := make([]string, 0, len(models))
	for modelName := range models {
		names = append(names, modelName)
	}
	sort.Strings(names)
	for _, modelName := range names {
		cfg, ok := configs[modelName]
		if !ok {
			report.Mismatches = append(report.Mismatches, PricingCompareMismatch{Kind: "text", Model: modelName, Reason: "missing migrated pricing_config"})
			continue
		}
		legacyQuota, legacyOK := legacyTextSampleQuota(modelName)
		newQuota, newOK := configTextSampleQuota(cfg)
		if legacyOK != newOK || legacyQuota != newQuota {
			report.Mismatches = append(report.Mismatches, PricingCompareMismatch{Kind: "text", Model: modelName, LegacyQuota: legacyQuota, NewQuota: newQuota, Reason: fmt.Sprintf("legacy_ok=%t new_ok=%t", legacyOK, newOK)})
		}
		report.CheckedTextModels++
	}
}

func legacyTextSampleQuota(modelName string) (int, bool) {
	if modelPrice, ok := ratio_setting.GetModelPrice(modelName, false); ok {
		return int(modelPrice * common.QuotaPerUnit), true
	}
	modelRatio, ok, _ := ratio_setting.GetModelRatio(modelName)
	if !ok {
		return 0, false
	}
	cacheRatio, _ := ratio_setting.GetCacheRatio(modelName)
	imageRatio, _ := ratio_setting.GetImageRatio(modelName)
	quota := textSampleQuota(modelRatio, ratio_setting.GetCompletionRatio(modelName), cacheRatio, imageRatio)
	return quota, true
}

func configTextSampleQuota(cfg ModelPricingConfig) (int, bool) {
	if cfg.Mode != PricingModeRatio && !cfg.UsePrice && !cfg.UseRatio {
		return 0, false
	}
	if cfg.UsePrice {
		return int(cfg.ModelPrice * common.QuotaPerUnit), true
	}
	return textSampleQuota(cfg.BaseRatio, cfg.CompletionRatio, fallbackPositive(cfg.CacheRatio, 1), fallbackPositive(cfg.ImageRatio, 1)), true
}

func textSampleQuota(modelRatio float64, completionRatio float64, cacheRatio float64, imageRatio float64) int {
	const (
		promptTokens     = 1000
		completionTokens = 500
		cacheTokens      = 100
		imageTokens      = 50
	)
	baseTokens := promptTokens - cacheTokens - imageTokens
	quota := (float64(baseTokens) + float64(cacheTokens)*cacheRatio + float64(imageTokens)*imageRatio + float64(completionTokens)*completionRatio) * modelRatio
	return int(math.Round(quota))
}

func fallbackPositive(value float64, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}

func compareAsyncImagePricing(configs map[string]ModelPricingConfig, report *PricingCompareReport) {
	legacy := operation_setting.GetAsyncSpecPricingCopy()
	models := sortedImageSpecModels(legacy.Image)
	for _, modelName := range models {
		cfg, ok := configs[modelName]
		if !ok {
			report.Mismatches = append(report.Mismatches, PricingCompareMismatch{Kind: "image", Model: modelName, Reason: "missing migrated pricing_config"})
			continue
		}
		newPricing := operation_setting.AsyncSpecPricing{Currency: legacy.Currency, Image: map[string]operation_setting.AsyncImageSpecPrice{
			modelName: pricingConfigToImageSpec(cfg),
		}}
		spec := legacy.Image[modelName]
		keys := imageSpecCompareKeys(spec)
		for _, key := range keys {
			legacyResult := operation_setting.ResolveImageSpecQuotaFromPricing(legacy, modelName, key.size, key.resolution, key.quality, 1)
			newResult := operation_setting.ResolveImageSpecQuotaFromPricing(newPricing, modelName, key.size, key.resolution, key.quality, 1)
			if legacyResult.Matched != newResult.Matched || legacyResult.Quota != newResult.Quota {
				report.Mismatches = append(report.Mismatches, PricingCompareMismatch{Kind: "image", Model: modelName, SpecKey: key.label, LegacyQuota: legacyResult.Quota, NewQuota: newResult.Quota, Reason: fmt.Sprintf("legacy_matched=%t new_matched=%t", legacyResult.Matched, newResult.Matched)})
			}
			report.CheckedImageCombinations++
		}
	}
}

type imageCompareKey struct {
	label      string
	size       string
	resolution string
	quality    string
}

func sortedImageSpecModels(src map[string]operation_setting.AsyncImageSpecPrice) []string {
	models := make([]string, 0, len(src))
	for modelName := range src {
		models = append(models, modelName)
	}
	sort.Strings(models)
	return models
}

func imageSpecCompareKeys(spec operation_setting.AsyncImageSpecPrice) []imageCompareKey {
	keys := make([]imageCompareKey, 0)
	for resolution := range spec.Resolutions {
		keys = append(keys, imageCompareKey{label: resolution, size: resolution, resolution: resolution})
	}
	for quality := range spec.Qualities {
		keys = append(keys, imageCompareKey{label: quality, quality: quality})
	}
	if spec.DefaultCNYPerImage != nil {
		keys = append(keys, imageCompareKey{label: "default"})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].label < keys[j].label })
	return keys
}

func pricingConfigToImageSpec(cfg ModelPricingConfig) operation_setting.AsyncImageSpecPrice {
	return operation_setting.AsyncImageSpecPrice{
		Unit:               cfg.Unit,
		Resolutions:        modelConfigToImageResolutions(cfg.Resolutions),
		Qualities:          cfg.Qualities,
		DefaultCNYPerImage: cfg.DefaultCNYPerImage,
	}
}

func (cfg ModelPricingConfig) AsyncImageSpecPrice() operation_setting.AsyncImageSpecPrice {
	return pricingConfigToImageSpec(cfg)
}

func imageResolutionsToModelConfig(src map[string]operation_setting.AsyncImageResolutionPrice) map[string]ModelSpecResolutionPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]ModelSpecResolutionPrice, len(src))
	for resolution, price := range src {
		dst[resolution] = ModelSpecResolutionPrice{CNYPerImage: price.CNYPerImage}
	}
	return dst
}

func videoResolutionsToModelConfig(src map[string]operation_setting.AsyncVideoResolutionPrice) map[string]ModelSpecResolutionPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]ModelSpecResolutionPrice, len(src))
	for resolution, price := range src {
		dst[resolution] = ModelSpecResolutionPrice{CNYPerSecond: price.CNYPerSecond}
	}
	return dst
}

func modelConfigToImageResolutions(src map[string]ModelSpecResolutionPrice) map[string]operation_setting.AsyncImageResolutionPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]operation_setting.AsyncImageResolutionPrice, len(src))
	for resolution, price := range src {
		if price.CNYPerImage != nil {
			dst[resolution] = operation_setting.AsyncImageResolutionPrice{CNYPerImage: price.CNYPerImage}
		}
	}
	if len(dst) == 0 {
		return nil
	}
	return dst
}

func modelConfigToVideoResolutions(src map[string]ModelSpecResolutionPrice) map[string]operation_setting.AsyncVideoResolutionPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]operation_setting.AsyncVideoResolutionPrice, len(src))
	for resolution, price := range src {
		if price.CNYPerSecond != nil {
			dst[resolution] = operation_setting.AsyncVideoResolutionPrice{CNYPerSecond: price.CNYPerSecond}
		}
	}
	if len(dst) == 0 {
		return nil
	}
	return dst
}

func compareAsyncVideoPricing(configs map[string]ModelPricingConfig, report *PricingCompareReport) {
	legacy := operation_setting.GetAsyncSpecPricingCopy()
	models := sortedVideoSpecModels(legacy.Video)
	for _, modelName := range models {
		cfg, ok := configs[modelName]
		if !ok {
			report.Mismatches = append(report.Mismatches, PricingCompareMismatch{Kind: "video", Model: modelName, Reason: "missing migrated pricing_config"})
			continue
		}
		newPricing := operation_setting.AsyncSpecPricing{Currency: legacy.Currency, Video: map[string]operation_setting.AsyncVideoSpecPrice{
			modelName: pricingConfigToVideoSpec(cfg),
		}}
		spec := legacy.Video[modelName]
		keys := videoSpecCompareKeys(spec)
		for _, key := range keys {
			legacyResult := operation_setting.ResolveVideoSpecQuotaByContextFromPricing(legacy, modelName, key.resolution, key.ratio, key.mode, 5)
			newResult := operation_setting.ResolveVideoSpecQuotaByContextFromPricing(newPricing, modelName, key.resolution, key.ratio, key.mode, 5)
			if legacyResult.Matched != newResult.Matched || legacyResult.Unsupported != newResult.Unsupported || legacyResult.Quota != newResult.Quota {
				report.Mismatches = append(report.Mismatches, PricingCompareMismatch{
					Kind:              "video",
					Model:             modelName,
					SpecKey:           key.label,
					LegacyQuota:       legacyResult.Quota,
					NewQuota:          newResult.Quota,
					LegacyUnsupported: legacyResult.Unsupported,
					NewUnsupported:    newResult.Unsupported,
					Reason:            fmt.Sprintf("legacy_matched=%t new_matched=%t", legacyResult.Matched, newResult.Matched),
				})
			}
			report.CheckedVideoCombinations++
		}
	}
}

type videoCompareKey struct {
	label      string
	resolution string
	ratio      string
	mode       string
}

func sortedVideoSpecModels(src map[string]operation_setting.AsyncVideoSpecPrice) []string {
	models := make([]string, 0, len(src))
	for modelName := range src {
		models = append(models, modelName)
	}
	sort.Strings(models)
	return models
}

func videoSpecCompareKeys(spec operation_setting.AsyncVideoSpecPrice) []videoCompareKey {
	keys := make([]videoCompareKey, 0)
	for resolution, ratios := range spec.Prices {
		for ratio, modes := range ratios {
			for mode := range modes {
				label := strings.Join([]string{resolution, ratio, mode}, ":")
				keys = append(keys, videoCompareKey{label: label, resolution: resolution, ratio: ratio, mode: mode})
			}
		}
	}
	for resolution := range spec.Resolutions {
		keys = append(keys, videoCompareKey{label: resolution, resolution: resolution})
	}
	if spec.DefaultCNYPerSecond != nil {
		keys = append(keys, videoCompareKey{label: "default"})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].label < keys[j].label })
	return keys
}

func pricingConfigToVideoSpec(cfg ModelPricingConfig) operation_setting.AsyncVideoSpecPrice {
	return operation_setting.AsyncVideoSpecPrice{
		Unit:                cfg.Unit,
		Resolutions:         modelConfigToVideoResolutions(cfg.Resolutions),
		Prices:              cfg.Prices,
		DefaultCNYPerSecond: cfg.DefaultCNYPerSecond,
		MinCNY:              cfg.MinCNY,
		MaxCNY:              cfg.MaxCNY,
	}
}

func (cfg ModelPricingConfig) AsyncVideoSpecPrice() operation_setting.AsyncVideoSpecPrice {
	return pricingConfigToVideoSpec(cfg)
}

func GetModelPricingConfig(modelName string) (ModelPricingConfig, bool, error) {
	if !IsModelPricingConfigTrusted() {
		return ModelPricingConfig{}, false, nil
	}
	return getModelPricingConfigFromDB(modelName)
}

func GetModelPricingConfigForDisplay(modelName string) (ModelPricingConfig, bool, error) {
	return getModelPricingConfigFromDB(modelName)
}

func getModelPricingConfigFromDB(modelName string) (ModelPricingConfig, bool, error) {
	if DB == nil || !DB.Migrator().HasTable(&Model{}) {
		return ModelPricingConfig{}, false, nil
	}
	modelName = strings.TrimSpace(modelName)
	names := []string{modelName}
	formatted := ratio_setting.FormatMatchingModelName(modelName)
	if formatted != "" && formatted != modelName {
		names = append(names, formatted)
	}
	for _, name := range names {
		var m Model
		err := DB.Where("model_name = ?", name).First(&m).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return ModelPricingConfig{}, false, err
		}
		if strings.TrimSpace(m.PricingConfig) == "" {
			return ModelPricingConfig{}, false, nil
		}
		cfg, err := m.ParsePricingConfig()
		if err != nil {
			return ModelPricingConfig{}, false, err
		}
		if cfg.Mode == "" || cfg.Mode == PricingModeInherit {
			return cfg, false, nil
		}
		return cfg, true, nil
	}
	return ModelPricingConfig{}, false, nil
}

func AutoMigrateModelPricingConfigsFromOptions() {
	if DB == nil || !DB.Migrator().HasTable(&Model{}) {
		return
	}
	stats, err := MigrateModelPricingConfigs()
	if err != nil {
		modelPricingParityMu.Lock()
		ModelPricingConfigTrusted = false
		modelPricingParityError = err.Error()
		modelPricingParityReport = PricingCompareReport{}
		modelPricingParityMu.Unlock()
		common.SysError("failed to migrate model pricing configs: " + err.Error() + "; 自动回退旧价格")
		return
	}
	if stats.CreatedModels > 0 || stats.UpdatedModels > 0 {
		common.SysLog(fmt.Sprintf("model pricing configs migrated: candidates=%d created=%d updated=%d priced=%d", stats.TotalCandidates, stats.CreatedModels, stats.UpdatedModels, stats.PricedModels))
	}
	RunModelPricingParityCheck()
}
