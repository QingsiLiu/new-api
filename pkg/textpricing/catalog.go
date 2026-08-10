package textpricing

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/shopspring/decimal"
)

const CatalogVersion = "2026-08-10"

const (
	CategoryGPT          = "gpt"
	CategoryClaude       = "claude"
	CategoryGemini       = "gemini"
	CategoryGrok         = "grok"
	CategoryUnclassified = "unclassified"
)

var billableCategories = []string{
	CategoryGPT,
	CategoryClaude,
	CategoryGemini,
	CategoryGrok,
}

var categories = []string{
	CategoryGPT,
	CategoryClaude,
	CategoryGemini,
	CategoryGrok,
	CategoryUnclassified,
}

var defaultMultipliers = map[string]float64{
	CategoryGPT:    0.05,
	CategoryClaude: 0.22,
	CategoryGemini: 0.06,
}

type Dimensions struct {
	InputMicroUSD        int64 `json:"input_micro_usd"`
	OutputMicroUSD       int64 `json:"output_micro_usd"`
	CachedInputMicroUSD  int64 `json:"cached_input_micro_usd,omitempty"`
	CacheWriteMicroUSD   int64 `json:"cache_write_micro_usd,omitempty"`
	CacheWrite5mMicroUSD int64 `json:"cache_write_5m_micro_usd,omitempty"`
	CacheWrite1hMicroUSD int64 `json:"cache_write_1h_micro_usd,omitempty"`
}

type Tier struct {
	Label           string     `json:"label"`
	MinPromptTokens int        `json:"min_prompt_tokens,omitempty"`
	MaxPromptTokens int        `json:"max_prompt_tokens,omitempty"`
	Dimensions      Dimensions `json:"dimensions"`
}

type Profile struct {
	Key         string     `json:"key"`
	Category    string     `json:"category"`
	DisplayName string     `json:"display_name"`
	SourceURL   string     `json:"source_url"`
	Dimensions  Dimensions `json:"dimensions,omitempty"`
	Tiers       []Tier     `json:"tiers,omitempty"`
}

type PublicDimensions struct {
	Input        float64 `json:"input"`
	Output       float64 `json:"output"`
	CachedInput  float64 `json:"cached_input,omitempty"`
	CacheWrite   float64 `json:"cache_write,omitempty"`
	CacheWrite5m float64 `json:"cache_write_5m,omitempty"`
	CacheWrite1h float64 `json:"cache_write_1h,omitempty"`
}

type PublicTier struct {
	Label           string           `json:"label"`
	MinPromptTokens int              `json:"min_prompt_tokens,omitempty"`
	MaxPromptTokens int              `json:"max_prompt_tokens,omitempty"`
	Dimensions      PublicDimensions `json:"dimensions"`
}

type PublicProfile struct {
	Key         string           `json:"key"`
	Version     string           `json:"version"`
	Category    string           `json:"category"`
	DisplayName string           `json:"display_name"`
	Currency    string           `json:"currency"`
	Unit        string           `json:"unit"`
	SourceURL   string           `json:"source_url"`
	Dimensions  PublicDimensions `json:"dimensions,omitempty"`
	Tiers       []PublicTier     `json:"tiers,omitempty"`
}

type catalogEntry struct {
	profile Profile
	match   func(string) bool
}

const (
	openAIPriceURL    = "https://openai.com/api/pricing/"
	googlePriceURL    = "https://ai.google.dev/gemini-api/docs/pricing"
	anthropicPriceURL = "https://docs.anthropic.com/en/docs/about-claude/pricing"
	xAIPriceURL       = "https://docs.x.ai/docs/models"
	longContextGPTMin = 272001
	longContextGemini = 200001
	longContextGrok   = 200001
)

func usd(value string) int64 {
	amount, err := decimal.NewFromString(value)
	if err != nil {
		panic(err)
	}
	return amount.Mul(decimal.NewFromInt(1_000_000)).IntPart()
}

func dimensions(input, cachedInput, cacheWrite, output string) Dimensions {
	return Dimensions{
		InputMicroUSD:       usd(input),
		OutputMicroUSD:      usd(output),
		CachedInputMicroUSD: usd(cachedInput),
		CacheWriteMicroUSD:  usd(cacheWrite),
	}
}

func claudeDimensions(input, cacheWrite5m, cacheWrite1h, cachedInput, output string) Dimensions {
	return Dimensions{
		InputMicroUSD:        usd(input),
		OutputMicroUSD:       usd(output),
		CachedInputMicroUSD:  usd(cachedInput),
		CacheWrite5mMicroUSD: usd(cacheWrite5m),
		CacheWrite1hMicroUSD: usd(cacheWrite1h),
	}
}

func exact(name string) func(string) bool {
	return func(modelName string) bool { return modelName == name }
}

func anyExact(names ...string) func(string) bool {
	return func(modelName string) bool {
		for _, name := range names {
			if modelName == name {
				return true
			}
		}
		return false
	}
}

func prefix(prefix string) func(string) bool {
	return func(modelName string) bool { return strings.HasPrefix(modelName, prefix) }
}

func exactOrDated(base string) func(string) bool {
	return func(modelName string) bool {
		return modelName == base || strings.HasPrefix(modelName, base+"-2026-")
	}
}

func claudeAlias(base string) func(string) bool {
	return func(modelName string) bool {
		if modelName == base {
			return true
		}
		for _, suffix := range []string{"-thinking", "-max", "-xhigh", "-high", "-medium", "-low"} {
			if modelName == base+suffix {
				return true
			}
		}
		prefix := base + "-"
		if !strings.HasPrefix(modelName, prefix) {
			return false
		}
		suffix := strings.TrimPrefix(modelName, prefix)
		if len(suffix) != 8 {
			return false
		}
		for _, ch := range suffix {
			if ch < '0' || ch > '9' {
				return false
			}
		}
		return true
	}
}

func profile(key, category, displayName, sourceURL string, dims Dimensions, match func(string) bool) catalogEntry {
	return catalogEntry{
		profile: Profile{
			Key:         key,
			Category:    category,
			DisplayName: displayName,
			SourceURL:   sourceURL,
			Dimensions:  dims,
		},
		match: match,
	}
}

func tieredProfile(key, category, displayName, sourceURL string, tiers []Tier, match func(string) bool) catalogEntry {
	return catalogEntry{
		profile: Profile{
			Key:         key,
			Category:    category,
			DisplayName: displayName,
			SourceURL:   sourceURL,
			Dimensions:  tiers[0].Dimensions,
			Tiers:       tiers,
		},
		match: match,
	}
}

func contextTiers(short, long Dimensions, minPromptTokens int) []Tier {
	return []Tier{
		{Label: "short", MaxPromptTokens: minPromptTokens - 1, Dimensions: short},
		{Label: "long", MinPromptTokens: minPromptTokens, Dimensions: long},
	}
}

var catalog = []catalogEntry{
	profile("openai.gpt-5.2-pro", CategoryGPT, "GPT-5.2 Pro", openAIPriceURL, dimensions("21", "0", "0", "168"), prefix("gpt-5.2-pro")),
	profile("openai.gpt-5.2", CategoryGPT, "GPT-5.2", openAIPriceURL, dimensions("1.75", "0.175", "0", "14"), func(name string) bool {
		return (name == "gpt-5.2" || strings.HasPrefix(name, "gpt-5.2-")) && !strings.Contains(name, "pro")
	}),
	profile("openai.gpt-5.3-codex", CategoryGPT, "GPT-5.3 Codex", openAIPriceURL, dimensions("1.75", "0.175", "0", "14"), prefix("gpt-5.3-codex")),
	tieredProfile("openai.gpt-5.4-pro", CategoryGPT, "GPT-5.4 Pro", openAIPriceURL, contextTiers(
		dimensions("30", "0", "0", "180"), dimensions("60", "0", "0", "270"), longContextGPTMin,
	), prefix("gpt-5.4-pro")),
	profile("openai.gpt-5.4-mini", CategoryGPT, "GPT-5.4 Mini", openAIPriceURL, dimensions("0.75", "0.075", "0", "4.5"), prefix("gpt-5.4-mini")),
	profile("openai.gpt-5.4-nano", CategoryGPT, "GPT-5.4 Nano", openAIPriceURL, dimensions("0.2", "0.02", "0", "1.25"), prefix("gpt-5.4-nano")),
	tieredProfile("openai.gpt-5.4", CategoryGPT, "GPT-5.4", openAIPriceURL, contextTiers(
		dimensions("2.5", "0.25", "0", "15"), dimensions("5", "0.5", "0", "22.5"), longContextGPTMin,
	), exactOrDated("gpt-5.4")),
	tieredProfile("openai.gpt-5.5-pro", CategoryGPT, "GPT-5.5 Pro", openAIPriceURL, contextTiers(
		dimensions("30", "0", "0", "180"), dimensions("60", "0", "0", "270"), longContextGPTMin,
	), prefix("gpt-5.5-pro")),
	tieredProfile("openai.gpt-5.5", CategoryGPT, "GPT-5.5", openAIPriceURL, contextTiers(
		dimensions("5", "0.5", "0", "30"), dimensions("10", "1", "0", "45"), longContextGPTMin,
	), exactOrDated("gpt-5.5")),
	tieredProfile("openai.gpt-5.6-sol", CategoryGPT, "GPT-5.6 Sol", openAIPriceURL, contextTiers(
		dimensions("5", "0.5", "6.25", "30"), dimensions("10", "1", "12.5", "45"), longContextGPTMin,
	), exact("gpt-5.6-sol")),
	tieredProfile("openai.gpt-5.6-terra", CategoryGPT, "GPT-5.6 Terra", openAIPriceURL, contextTiers(
		dimensions("2", "0.2", "2.5", "12"), dimensions("4", "0.4", "5", "18"), longContextGPTMin,
	), exact("gpt-5.6-terra")),
	tieredProfile("openai.gpt-5.6-luna", CategoryGPT, "GPT-5.6 Luna", openAIPriceURL, contextTiers(
		dimensions("0.2", "0.02", "0.25", "1.2"), dimensions("0.4", "0.04", "0.5", "1.8"), longContextGPTMin,
	), exact("gpt-5.6-luna")),
	profile("google.gemini-2.5-flash", CategoryGemini, "Gemini 2.5 Flash", googlePriceURL, dimensions("0.3", "0.03", "0", "2.5"), anyExact("gemini-2.5-flash", "gemini-2.5-flash-nothinking")),
	profile("google.gemini-2.5-flash-lite", CategoryGemini, "Gemini 2.5 Flash-Lite", googlePriceURL, dimensions("0.1", "0.01", "0", "0.4"), exact("gemini-2.5-flash-lite")),
	tieredProfile("google.gemini-2.5-pro", CategoryGemini, "Gemini 2.5 Pro", googlePriceURL, contextTiers(
		dimensions("1.25", "0.125", "0", "10"), dimensions("2.5", "0.25", "0", "15"), longContextGemini,
	), prefix("gemini-2.5-pro")),
	profile("google.gemini-3-flash", CategoryGemini, "Gemini 3 Flash", googlePriceURL, dimensions("0.5", "0.05", "0", "3"), anyExact("gemini-3-flash-preview", "gemini-3-flash")),
	profile("google.gemini-3.1-flash-lite", CategoryGemini, "Gemini 3.1 Flash-Lite", googlePriceURL, dimensions("0.25", "0.025", "0", "1.5"), anyExact("gemini-3.1-flash-lite", "gemini-3.1-flash-lite-preview")),
	tieredProfile("google.gemini-3.1-pro", CategoryGemini, "Gemini 3.1 Pro", googlePriceURL, contextTiers(
		dimensions("2", "0.2", "0", "12"), dimensions("4", "0.4", "0", "18"), longContextGemini,
	), anyExact("gemini-3.1-pro-preview", "gemini-3.1-pro", "gemini-3.1-pro-preview-low")),
	profile("google.gemini-3.5-flash", CategoryGemini, "Gemini 3.5 Flash", googlePriceURL, dimensions("1.5", "0.15", "0", "9"), exact("gemini-3.5-flash")),
	profile("google.gemini-3.5-flash-lite", CategoryGemini, "Gemini 3.5 Flash-Lite", googlePriceURL, dimensions("0.3", "0.03", "0", "2.5"), exact("gemini-3.5-flash-lite")),
	profile("google.gemini-3.6-flash", CategoryGemini, "Gemini 3.6 Flash", googlePriceURL, dimensions("1.5", "0.15", "0", "7.5"), exact("gemini-3.6-flash")),
	profile("anthropic.claude-opus-4-5", CategoryClaude, "Claude Opus 4.5", anthropicPriceURL, claudeDimensions("5", "6.25", "10", "0.5", "25"), claudeAlias("claude-opus-4-5")),
	profile("anthropic.claude-opus-4-6", CategoryClaude, "Claude Opus 4.6", anthropicPriceURL, claudeDimensions("5", "6.25", "10", "0.5", "25"), claudeAlias("claude-opus-4-6")),
	profile("anthropic.claude-opus-4-7", CategoryClaude, "Claude Opus 4.7", anthropicPriceURL, claudeDimensions("5", "6.25", "10", "0.5", "25"), claudeAlias("claude-opus-4-7")),
	profile("anthropic.claude-opus-4-8", CategoryClaude, "Claude Opus 4.8", anthropicPriceURL, claudeDimensions("5", "6.25", "10", "0.5", "25"), claudeAlias("claude-opus-4-8")),
	profile("anthropic.claude-opus-5", CategoryClaude, "Claude Opus 5", anthropicPriceURL, claudeDimensions("5", "6.25", "10", "0.5", "25"), claudeAlias("claude-opus-5")),
	profile("anthropic.claude-sonnet-4-5", CategoryClaude, "Claude Sonnet 4.5", anthropicPriceURL, claudeDimensions("3", "3.75", "6", "0.3", "15"), claudeAlias("claude-sonnet-4-5")),
	profile("anthropic.claude-sonnet-4-6", CategoryClaude, "Claude Sonnet 4.6", anthropicPriceURL, claudeDimensions("3", "3.75", "6", "0.3", "15"), claudeAlias("claude-sonnet-4-6")),
	profile("anthropic.claude-sonnet-5", CategoryClaude, "Claude Sonnet 5", anthropicPriceURL, claudeDimensions("2", "2.5", "4", "0.2", "10"), claudeAlias("claude-sonnet-5")),
	profile("anthropic.claude-haiku-4-5", CategoryClaude, "Claude Haiku 4.5", anthropicPriceURL, claudeDimensions("1", "1.25", "2", "0.1", "5"), claudeAlias("claude-haiku-4-5")),
	profile("anthropic.claude-fable-5", CategoryClaude, "Claude Fable 5", anthropicPriceURL, claudeDimensions("10", "12.5", "20", "1", "50"), claudeAlias("claude-fable-5")),
	tieredProfile("xai.grok-code-fast-1", CategoryGrok, "Grok Code Fast 1", xAIPriceURL, contextTiers(
		dimensions("1", "0.2", "0", "2"), dimensions("2", "0.4", "0", "4"), longContextGrok,
	), anyExact("grok-build-0.1", "grok-code-fast-1", "grok-code-fast", "grok-code-fast-1-0825")),
	tieredProfile("xai.grok-4.3", CategoryGrok, "Grok 4.3", xAIPriceURL, contextTiers(
		dimensions("1.25", "0.2", "0", "2.5"), dimensions("2.5", "0.4", "0", "5"), longContextGrok,
	), anyExact("grok-4.3", "grok-4.3-latest", "grok-latest")),
	tieredProfile("xai.grok-4.20", CategoryGrok, "Grok 4.20", xAIPriceURL, contextTiers(
		dimensions("1.25", "0.2", "0", "2.5"), dimensions("2.5", "0.4", "0", "5"), longContextGrok,
	), anyExact(
		"grok-4.20-0309-non-reasoning", "grok-4.20-non-reasoning", "grok-4.20-non-reasoning-latest",
		"grok-4.20-beta-non-reasoning", "grok-4.20-beta-latest-non-reasoning",
		"grok-4.20-experimental-beta-0304-non-reasoning", "grok-4.20-experimental-beta-non-reasoning-latest",
		"grok-4.20-beta-0309-non-reasoning", "grok-4.20-non-reasoning-gv2",
		"grok-4.20-multi-agent-0309", "grok-4.20-multi-agent", "grok-4.20-multi-agent-latest",
		"grok-4.20-multi-agent-beta-latest", "grok-4.20-multi-agent-experimental-beta-0304",
		"grok-4.20-multi-agent-experimental-beta-latest", "grok-4.20-multi-agent-beta-0309",
		"grok-4.20-0309-reasoning", "grok-4.20-reasoning-latest", "grok-4.20",
		"grok-4.20-reasoning", "grok-4.20-0309", "grok-4.20-beta-0309-reasoning",
		"grok-4.20-beta", "grok-4.20-beta-0309", "grok-4.20-beta-latest",
		"grok-4.20-beta-latest-reasoning", "grok-4.20-beta-reasoning",
		"grok-4.20-experimental-beta-0304-reasoning", "grok-4.20-experimental-beta-0304",
		"grok-4.20-experimental-beta-reasoning-latest", "grok-4.20-experimental-beta-latest",
		"grok-4.20-reasoning-gv2",
	)),
	tieredProfile("xai.grok-4.5", CategoryGrok, "Grok 4.5", xAIPriceURL, contextTiers(
		dimensions("2", "0.3", "0", "6"), dimensions("4", "0.6", "0", "12"), longContextGrok,
	), anyExact("grok-4.5", "grok-4.5-latest", "grok-build-latest")),
}

func Categories() []string {
	return append([]string(nil), categories...)
}

func BillableCategories() []string {
	return append([]string(nil), billableCategories...)
}

func IsCategory(category string) bool {
	category = strings.ToLower(strings.TrimSpace(category))
	for _, allowed := range categories {
		if category == allowed {
			return true
		}
	}
	return false
}

func IsBillableCategory(category string) bool {
	category = strings.ToLower(strings.TrimSpace(category))
	for _, allowed := range billableCategories {
		if category == allowed {
			return true
		}
	}
	return false
}

func DefaultMultiplier(category string) (float64, bool) {
	multiplier, ok := defaultMultipliers[strings.ToLower(strings.TrimSpace(category))]
	return multiplier, ok
}

func Get(key string) (Profile, bool) {
	key = strings.TrimSpace(key)
	for _, entry := range catalog {
		if entry.profile.Key == key {
			return entry.profile, true
		}
	}
	return Profile{}, false
}

func MatchModel(modelName string) (Profile, bool) {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	for _, entry := range catalog {
		if entry.match(normalized) {
			return entry.profile, true
		}
	}
	return Profile{}, false
}

func List() []PublicProfile {
	profiles := make([]PublicProfile, 0, len(catalog))
	for _, entry := range catalog {
		profiles = append(profiles, ToPublicProfile(entry.profile))
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Key < profiles[j].Key })
	return profiles
}

func microUSDToUSD(value int64) float64 {
	return decimal.NewFromInt(value).Div(decimal.NewFromInt(1_000_000)).InexactFloat64()
}

func toPublicDimensions(dims Dimensions) PublicDimensions {
	return PublicDimensions{
		Input:        microUSDToUSD(dims.InputMicroUSD),
		Output:       microUSDToUSD(dims.OutputMicroUSD),
		CachedInput:  microUSDToUSD(dims.CachedInputMicroUSD),
		CacheWrite:   microUSDToUSD(dims.CacheWriteMicroUSD),
		CacheWrite5m: microUSDToUSD(dims.CacheWrite5mMicroUSD),
		CacheWrite1h: microUSDToUSD(dims.CacheWrite1hMicroUSD),
	}
}

func ToPublicProfile(profile Profile) PublicProfile {
	public := PublicProfile{
		Key:         profile.Key,
		Version:     CatalogVersion,
		Category:    profile.Category,
		DisplayName: profile.DisplayName,
		Currency:    "USD",
		Unit:        "per_1M_tokens",
		SourceURL:   profile.SourceURL,
		Dimensions:  toPublicDimensions(profile.Dimensions),
	}
	for _, tier := range profile.Tiers {
		public.Tiers = append(public.Tiers, PublicTier{
			Label:           tier.Label,
			MinPromptTokens: tier.MinPromptTokens,
			MaxPromptTokens: tier.MaxPromptTokens,
			Dimensions:      toPublicDimensions(tier.Dimensions),
		})
	}
	return public
}

func ValidateMultiplier(multiplier float64) error {
	value := decimal.NewFromFloat(multiplier)
	if value.LessThanOrEqual(decimal.Zero) || value.GreaterThan(decimal.NewFromInt(1)) {
		return errors.New("multiplier must be greater than 0 and no greater than 1")
	}
	if !value.Equal(value.Round(4)) {
		return errors.New("multiplier supports at most four decimal places")
	}
	return nil
}

func quotaPerMillion(microUSD int64, multiplier decimal.Decimal) int64 {
	if microUSD <= 0 {
		return 0
	}
	return decimal.NewFromInt(microUSD).
		Mul(multiplier).
		Mul(decimal.NewFromInt(200)).
		Mul(decimal.NewFromInt(int64(common.CreditsQuotaUnit))).
		Div(decimal.NewFromInt(1_000_000)).
		Round(0).
		IntPart()
}

func pricingFromDimensions(dims Dimensions, multiplier decimal.Decimal) types.CreditsTextPricing {
	return types.CreditsTextPricing{
		InputQuotaPerMillion:        quotaPerMillion(dims.InputMicroUSD, multiplier),
		OutputQuotaPerMillion:       quotaPerMillion(dims.OutputMicroUSD, multiplier),
		CachedInputQuotaPerMillion:  quotaPerMillion(dims.CachedInputMicroUSD, multiplier),
		CacheWriteQuotaPerMillion:   quotaPerMillion(dims.CacheWriteMicroUSD, multiplier),
		CacheWrite5mQuotaPerMillion: quotaPerMillion(dims.CacheWrite5mMicroUSD, multiplier),
		CacheWrite1hQuotaPerMillion: quotaPerMillion(dims.CacheWrite1hMicroUSD, multiplier),
	}
}

func BuildPricing(profile Profile, multiplier float64, applyGroupRatio bool, source string) (*types.CreditsTextPricing, error) {
	if err := ValidateMultiplier(multiplier); err != nil {
		return nil, err
	}
	if source == "" {
		source = "official_catalog"
	}
	multiplierDecimal := decimal.NewFromFloat(multiplier)
	base := pricingFromDimensions(profile.Dimensions, multiplierDecimal)
	pricing := &types.CreditsTextPricing{
		InputQuotaPerMillion:        base.InputQuotaPerMillion,
		OutputQuotaPerMillion:       base.OutputQuotaPerMillion,
		CachedInputQuotaPerMillion:  base.CachedInputQuotaPerMillion,
		CacheWriteQuotaPerMillion:   base.CacheWriteQuotaPerMillion,
		CacheWrite5mQuotaPerMillion: base.CacheWrite5mQuotaPerMillion,
		CacheWrite1hQuotaPerMillion: base.CacheWrite1hQuotaPerMillion,
		PricingSource:               source,
		CatalogVersion:              CatalogVersion,
		OfficialPriceKey:            profile.Key,
		TextCategory:                profile.Category,
		CategoryMultiplier:          multiplier,
		ApplyGroupRatio:             applyGroupRatio,
	}
	for _, tier := range profile.Tiers {
		tierPricing := pricingFromDimensions(tier.Dimensions, multiplierDecimal)
		pricing.Tiers = append(pricing.Tiers, types.CreditsTextPricingTier{
			Label:                       tier.Label,
			MinPromptTokens:             tier.MinPromptTokens,
			MaxPromptTokens:             tier.MaxPromptTokens,
			InputQuotaPerMillion:        tierPricing.InputQuotaPerMillion,
			OutputQuotaPerMillion:       tierPricing.OutputQuotaPerMillion,
			CachedInputQuotaPerMillion:  tierPricing.CachedInputQuotaPerMillion,
			CacheWriteQuotaPerMillion:   tierPricing.CacheWriteQuotaPerMillion,
			CacheWrite5mQuotaPerMillion: tierPricing.CacheWrite5mQuotaPerMillion,
			CacheWrite1hQuotaPerMillion: tierPricing.CacheWrite1hQuotaPerMillion,
		})
	}
	return pricing, nil
}
