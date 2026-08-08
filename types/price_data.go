package types

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

type GroupRatioInfo struct {
	GroupRatio        float64
	GroupSpecialRatio float64
	HasSpecialRatio   bool
}

type PriceData struct {
	FreeModel              bool
	ModelPrice             float64
	ModelRatio             float64
	CompletionRatio        float64
	CacheRatio             float64
	CacheCreationRatio     float64
	CacheCreation5mRatio   float64
	CacheCreation1hRatio   float64
	ImageRatio             float64
	AudioRatio             float64
	AudioCompletionRatio   float64
	otherRatios            map[string]float64
	UsePrice               bool
	Quota                  int // 按次计费的最终额度（MJ / Task）
	QuotaToPreConsume      int // 按量计费的预消耗额度
	GroupRatioInfo         GroupRatioInfo
	SpecPricing            *SpecPricingInfo
	TextPricingMode        string
	CreditsTextPricing     *CreditsTextPricing
	ShadowTextPricing      *CreditsTextPricing
	ShadowTextGroupRatio   float64
	ShadowTextPricingError string
	PricingSource          string
}

type CreditsTextPricing struct {
	InputQuotaPerMillion        int64                    `json:"input_quota_per_million"`
	OutputQuotaPerMillion       int64                    `json:"output_quota_per_million"`
	CachedInputQuotaPerMillion  int64                    `json:"cached_input_quota_per_million,omitempty"`
	CacheWriteQuotaPerMillion   int64                    `json:"cache_write_quota_per_million,omitempty"`
	CacheWrite5mQuotaPerMillion int64                    `json:"cache_write_5m_quota_per_million,omitempty"`
	CacheWrite1hQuotaPerMillion int64                    `json:"cache_write_1h_quota_per_million,omitempty"`
	PricingSource               string                   `json:"pricing_source"`
	Tiers                       []CreditsTextPricingTier `json:"tiers,omitempty"`
	CatalogVersion              string                   `json:"catalog_version,omitempty"`
	OfficialPriceKey            string                   `json:"official_price_key,omitempty"`
	TextCategory                string                   `json:"text_category,omitempty"`
	CategoryMultiplier          float64                  `json:"category_multiplier,omitempty"`
	ApplyGroupRatio             bool                     `json:"apply_group_ratio,omitempty"`
	Fallback                    bool                     `json:"fallback,omitempty"`
}

// CreditsTextPricingTier contains a context-length-specific price. A zero
// boundary means that the corresponding side is unbounded. The settlement
// path selects a tier from prompt tokens, while the base fields above remain
// the short-context compatibility projection.
type CreditsTextPricingTier struct {
	Label                       string `json:"label"`
	MinPromptTokens             int    `json:"min_prompt_tokens,omitempty"`
	MaxPromptTokens             int    `json:"max_prompt_tokens,omitempty"`
	InputQuotaPerMillion        int64  `json:"input_quota_per_million"`
	OutputQuotaPerMillion       int64  `json:"output_quota_per_million"`
	CachedInputQuotaPerMillion  int64  `json:"cached_input_quota_per_million,omitempty"`
	CacheWriteQuotaPerMillion   int64  `json:"cache_write_quota_per_million,omitempty"`
	CacheWrite5mQuotaPerMillion int64  `json:"cache_write_5m_quota_per_million,omitempty"`
	CacheWrite1hQuotaPerMillion int64  `json:"cache_write_1h_quota_per_million,omitempty"`
}

// ForPromptTokens returns the applicable immutable pricing projection. It
// deliberately does not mutate the shared catalog object because the same
// pointer is used by pre-consumption, final settlement, and public pricing.
func (p *CreditsTextPricing) ForPromptTokens(promptTokens int) *CreditsTextPricing {
	if p == nil || len(p.Tiers) == 0 {
		return p
	}
	for _, tier := range p.Tiers {
		if tier.MinPromptTokens > 0 && promptTokens < tier.MinPromptTokens {
			continue
		}
		if tier.MaxPromptTokens > 0 && promptTokens > tier.MaxPromptTokens {
			continue
		}
		return &CreditsTextPricing{
			InputQuotaPerMillion:        tier.InputQuotaPerMillion,
			OutputQuotaPerMillion:       tier.OutputQuotaPerMillion,
			CachedInputQuotaPerMillion:  tier.CachedInputQuotaPerMillion,
			CacheWriteQuotaPerMillion:   tier.CacheWriteQuotaPerMillion,
			CacheWrite5mQuotaPerMillion: tier.CacheWrite5mQuotaPerMillion,
			CacheWrite1hQuotaPerMillion: tier.CacheWrite1hQuotaPerMillion,
			PricingSource:               p.PricingSource,
			CatalogVersion:              p.CatalogVersion,
			OfficialPriceKey:            p.OfficialPriceKey,
			TextCategory:                p.TextCategory,
			CategoryMultiplier:          p.CategoryMultiplier,
			ApplyGroupRatio:             p.ApplyGroupRatio,
			Fallback:                    p.Fallback,
		}
	}
	return p
}

type SpecPricingInfo struct {
	Priced        bool    `json:"spec_priced,omitempty"`
	Kind          string  `json:"spec_kind,omitempty"`
	Model         string  `json:"spec_model,omitempty"`
	SpecKey       string  `json:"spec_key,omitempty"`
	Resolution    string  `json:"spec_resolution,omitempty"`
	Ratio         string  `json:"spec_ratio,omitempty"`
	Mode          string  `json:"spec_mode,omitempty"`
	UnitCNY       float64 `json:"spec_unit_cny,omitempty"`
	TotalCNY      float64 `json:"spec_total_cny,omitempty"`
	Quota         int     `json:"spec_quota,omitempty"`
	QuotaPerCNY   float64 `json:"quota_per_cny,omitempty"`
	UnitCredits   string  `json:"spec_unit_credits,omitempty"`
	TotalCredits  string  `json:"spec_total_credits,omitempty"`
	PricingSource string  `json:"pricing_source,omitempty"`
}

func (p *PriceData) AddOtherRatio(key string, ratio float64) {
	if !isValidOtherRatio(ratio) {
		return
	}
	if p.otherRatios == nil {
		p.otherRatios = make(map[string]float64)
	}
	p.otherRatios[key] = ratio
}

func (p *PriceData) ReplaceOtherRatios(ratios map[string]float64) bool {
	p.otherRatios = nil
	for key, ratio := range ratios {
		p.AddOtherRatio(key, ratio)
	}
	return len(p.otherRatios) > 0
}

func (p *PriceData) HasOtherRatio(key string) bool {
	ratio, ok := p.otherRatios[key]
	return ok && isValidOtherRatio(ratio)
}

func (p *PriceData) OtherRatios() map[string]float64 {
	if len(p.otherRatios) == 0 {
		return nil
	}
	ratios := make(map[string]float64, len(p.otherRatios))
	for key, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) {
			ratios[key] = ratio
		}
	}
	if len(ratios) == 0 {
		return nil
	}
	return ratios
}

func (p *PriceData) OtherRatioMultiplier() float64 {
	multiplier := 1.0
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			multiplier *= ratio
		}
	}
	return multiplier
}

func (p *PriceData) ApplyOtherRatiosToFloat(value float64) float64 {
	return value * p.OtherRatioMultiplier()
}

func (p *PriceData) ApplyOtherRatiosToDecimal(value decimal.Decimal) decimal.Decimal {
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			value = value.Mul(decimal.NewFromFloat(ratio))
		}
	}
	return value
}

func (p *PriceData) RemoveOtherRatiosFromFloat(value float64) float64 {
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			value /= ratio
		}
	}
	return value
}

func isValidOtherRatio(ratio float64) bool {
	// NaN/Inf would poison every downstream quota multiplication
	// (int(NaN * quota) wraps to a negative charge).
	return ratio > 0 && !math.IsInf(ratio, 1)
}

func (p *PriceData) ToSetting() string {
	return fmt.Sprintf("ModelPrice: %f, ModelRatio: %f, CompletionRatio: %f, CacheRatio: %f, GroupRatio: %f, UsePrice: %t, CacheCreationRatio: %f, CacheCreation5mRatio: %f, CacheCreation1hRatio: %f, QuotaToPreConsume: %d, ImageRatio: %f, AudioRatio: %f, AudioCompletionRatio: %f", p.ModelPrice, p.ModelRatio, p.CompletionRatio, p.CacheRatio, p.GroupRatioInfo.GroupRatio, p.UsePrice, p.CacheCreationRatio, p.CacheCreation5mRatio, p.CacheCreation1hRatio, p.QuotaToPreConsume, p.ImageRatio, p.AudioRatio, p.AudioCompletionRatio)
}
