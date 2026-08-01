package controller

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relayhelper "github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/shopspring/decimal"
)

const publicCatalogCreditsPerUSD = 200

type publicCreditsPriceSpec struct {
	Key             string `json:"key"`
	Credits         string `json:"credits"`
	Quota           int    `json:"quota"`
	PricingSource   string `json:"pricing_source"`
	Resolution      string `json:"resolution,omitempty"`
	Quality         string `json:"quality,omitempty"`
	Ratio           string `json:"ratio,omitempty"`
	Mode            string `json:"mode,omitempty"`
	Dimension       string `json:"dimension,omitempty"`
	Tier            string `json:"tier,omitempty"`
	MinPromptTokens *int   `json:"min_prompt_tokens,omitempty"`
	MaxPromptTokens *int   `json:"max_prompt_tokens,omitempty"`
}

type publicCreditsPricing struct {
	Unit             string                   `json:"unit"`
	PriceFromCredits string                   `json:"price_from_credits"`
	PriceFromQuota   int                      `json:"price_from_quota"`
	PricingSource    string                   `json:"pricing_source"`
	Specs            []publicCreditsPriceSpec `json:"specs"`
}

type quotaPriceSpec struct {
	publicCreditsPriceSpec
	quota int
}

func makeQuotaPriceSpec(key string, quota int, source string) quotaPriceSpec {
	return quotaPriceSpec{
		publicCreditsPriceSpec: publicCreditsPriceSpec{
			Key:           key,
			Credits:       common.QuotaToCreditsString(quota),
			Quota:         quota,
			PricingSource: source,
		},
		quota: quota,
	}
}

func finalizePublicCreditsPricing(unit string, specs []quotaPriceSpec) *publicCreditsPricing {
	if len(specs) == 0 {
		return nil
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Key < specs[j].Key
	})
	minQuota := 0
	source := ""
	publicSpecs := make([]publicCreditsPriceSpec, 0, len(specs))
	for _, spec := range specs {
		if spec.quota <= 0 {
			continue
		}
		if minQuota == 0 || spec.quota < minQuota {
			minQuota = spec.quota
			source = spec.PricingSource
		}
		publicSpecs = append(publicSpecs, spec.publicCreditsPriceSpec)
	}
	if minQuota == 0 || len(publicSpecs) == 0 {
		return nil
	}
	return &publicCreditsPricing{
		Unit:             unit,
		PriceFromCredits: common.QuotaToCreditsString(minQuota),
		PriceFromQuota:   minQuota,
		PricingSource:    source,
		Specs:            publicSpecs,
	}
}

func buildPublicCreditsPricing(
	entry model.ModelRegistry,
	pricing operation_setting.AsyncSpecPricing,
	categoryMultiplier *float64,
) *publicCreditsPricing {
	if !common.CreditsV1Enabled() {
		return nil
	}
	switch entry.Modality {
	case asyncTaskKindImage:
		return buildPublicImageCreditsPricing(entry.ModelName, pricing)
	case asyncTaskKindVideo:
		return buildPublicVideoCreditsPricing(entry.ModelName, pricing)
	case "text":
		return buildPublicTextCreditsPricing(entry, categoryMultiplier)
	default:
		return nil
	}
}

func buildPublicImageCreditsPricing(modelName string, pricing operation_setting.AsyncSpecPricing) *publicCreditsPricing {
	exactSpecs := make([]quotaPriceSpec, 0, 4)
	seen := map[string]bool{}
	for _, resolution := range []string{"", "1k", "2k", "4k"} {
		request := asyncTaskRequest{
			Kind:       asyncTaskKindImage,
			Model:      modelName,
			Parameters: map[string]interface{}{"resolution": resolution, "n": 1},
		}
		exact, ok, err := resolveCreditsV1ImageSpec(request, modelName)
		if err != nil || !ok || exact.UnitQuota <= 0 || seen[exact.SpecKey] {
			continue
		}
		seen[exact.SpecKey] = true
		spec := makeQuotaPriceSpec("resolution:"+exact.SpecKey, exact.UnitQuota, "kie")
		spec.Resolution = exact.SpecKey
		exactSpecs = append(exactSpecs, spec)
	}
	if result := finalizePublicCreditsPricing("per_image", exactSpecs); result != nil {
		return result
	}

	imageSpec, ok := pricing.Image[modelName]
	if !ok {
		return nil
	}
	specs := make([]quotaPriceSpec, 0, len(imageSpec.Resolutions)+len(imageSpec.Qualities)+1)
	for resolution := range imageSpec.Resolutions {
		result := operation_setting.ResolveImageSpecQuotaFromPricing(pricing, modelName, resolution, "", "", 1)
		if !result.Matched || result.Quota <= 0 {
			continue
		}
		spec := makeQuotaPriceSpec("resolution:"+resolution, result.Quota, "geili")
		spec.Resolution = resolution
		specs = append(specs, spec)
	}
	for quality := range imageSpec.Qualities {
		result := operation_setting.ResolveImageSpecQuotaFromPricing(pricing, modelName, "", "", quality, 1)
		if !result.Matched || result.Quota <= 0 {
			continue
		}
		spec := makeQuotaPriceSpec("quality:"+quality, result.Quota, "geili")
		spec.Quality = quality
		specs = append(specs, spec)
	}
	if imageSpec.DefaultCNYPerImage != nil {
		result := operation_setting.ResolveImageSpecQuotaFromPricing(pricing, modelName, "", "", "", 1)
		if result.Matched && result.Quota > 0 {
			specs = append(specs, makeQuotaPriceSpec("default", result.Quota, "geili"))
		}
	}
	unit := imageSpec.Unit
	if unit == "" {
		unit = "per_image"
	}
	return finalizePublicCreditsPricing(unit, specs)
}

func buildPublicVideoCreditsPricing(modelName string, pricing operation_setting.AsyncSpecPricing) *publicCreditsPricing {
	videoSpec, hasSpec := pricing.Video[modelName]
	specsByKey := make(map[string]quotaPriceSpec)
	exactCoverage := make(map[string]bool)

	for _, resolution := range []string{"480p", "720p"} {
		for _, mode := range []string{"no_video_input", "with_video_input"} {
			parameters := map[string]interface{}{
				"resolution": resolution,
				"duration":   1,
			}
			if mode == "with_video_input" {
				parameters["video_url"] = "https://catalog.invalid/input.mp4"
			}
			request := asyncTaskRequest{
				Kind:       asyncTaskKindVideo,
				Model:      modelName,
				Parameters: parameters,
			}
			exact, ok, err := resolveCreditsV1VideoSpec(request, modelName)
			if err != nil || !ok || exact.UnitQuota <= 0 || exact.Mode != mode {
				continue
			}
			coverageKey := strings.Join([]string{resolution, mode}, ":")
			spec := makeQuotaPriceSpec(coverageKey, exact.UnitQuota, "kie")
			spec.Resolution = resolution
			spec.Mode = mode
			specsByKey[coverageKey] = spec
			exactCoverage[coverageKey] = true
		}
	}

	if hasSpec {
		for resolution, ratios := range videoSpec.Prices {
			for ratio, modes := range ratios {
				for mode, price := range modes {
					coverageKey := strings.Join([]string{resolution, mode}, ":")
					if price.Unsupported || exactCoverage[coverageKey] {
						continue
					}
					result := operation_setting.ResolveVideoSpecQuotaByContextFromPricing(
						pricing, modelName, resolution, ratio, mode, 1,
					)
					if !result.Matched || result.Quota <= 0 {
						continue
					}
					key := strings.Join([]string{resolution, ratio, mode}, ":")
					spec := makeQuotaPriceSpec(key, result.Quota, "geili")
					spec.Resolution = resolution
					spec.Ratio = ratio
					spec.Mode = mode
					if _, exists := specsByKey[key]; !exists {
						specsByKey[key] = spec
					}
				}
			}
		}
		for resolution := range videoSpec.Resolutions {
			if exactCoverage[strings.Join([]string{resolution, "no_video_input"}, ":")] {
				continue
			}
			result := operation_setting.ResolveVideoSpecQuotaByContextFromPricing(
				pricing, modelName, resolution, "", "", 1,
			)
			if !result.Matched || result.Quota <= 0 {
				continue
			}
			key := "resolution:" + resolution
			spec := makeQuotaPriceSpec(key, result.Quota, "geili")
			spec.Resolution = resolution
			if _, exists := specsByKey[key]; !exists {
				specsByKey[key] = spec
			}
		}
		if videoSpec.DefaultCNYPerSecond != nil && len(exactCoverage) == 0 {
			result := operation_setting.ResolveVideoSpecQuotaByContextFromPricing(pricing, modelName, "", "", "", 1)
			if result.Matched && result.Quota > 0 {
				specsByKey["default"] = makeQuotaPriceSpec("default", result.Quota, "geili")
			}
		}
	}

	specs := make([]quotaPriceSpec, 0, len(specsByKey))
	for _, spec := range specsByKey {
		specs = append(specs, spec)
	}
	unit := videoSpec.Unit
	if unit == "" {
		unit = "per_second"
	}
	return finalizePublicCreditsPricing(unit, specs)
}

func buildPublicTextCreditsPricing(entry model.ModelRegistry, categoryMultiplier *float64) *publicCreditsPricing {
	if exact, ok := relayhelper.CreditsV1TextPricingForPublicCatalog(entry.ModelName); ok {
		specs := make([]quotaPriceSpec, 0, 6*max(1, len(exact.Tiers)))
		appendTier := func(
			input, output, cachedInput, cacheWrite, cacheWrite5m, cacheWrite1h int64,
			tier string, minPromptTokens, maxPromptTokens int,
		) {
			cachedInputQuota := cachedInput
			if cachedInputQuota <= 0 {
				cachedInputQuota = input
			}
			appendSpec := func(dimension string, quota int64) {
				spec := textQuotaPriceSpec(dimension, quota, "geili", tier)
				if spec.quota <= 0 {
					return
				}
				if minPromptTokens > 0 {
					spec.MinPromptTokens = &minPromptTokens
				}
				if maxPromptTokens > 0 {
					spec.MaxPromptTokens = &maxPromptTokens
				}
				specs = append(specs, spec)
			}
			appendSpec("input", input)
			appendSpec("cached_input", cachedInputQuota)
			appendSpec("cache_write", cacheWrite)
			appendSpec("cache_write_5m", cacheWrite5m)
			appendSpec("cache_write_1h", cacheWrite1h)
			appendSpec("output", output)
		}
		if len(exact.Tiers) > 0 {
			for _, tier := range exact.Tiers {
				appendTier(
					tier.InputQuotaPerMillion,
					tier.OutputQuotaPerMillion,
					tier.CachedInputQuotaPerMillion,
					tier.CacheWriteQuotaPerMillion,
					tier.CacheWrite5mQuotaPerMillion,
					tier.CacheWrite1hQuotaPerMillion,
					tier.Label,
					tier.MinPromptTokens,
					tier.MaxPromptTokens,
				)
			}
		} else {
			appendTier(
				exact.InputQuotaPerMillion,
				exact.OutputQuotaPerMillion,
				exact.CachedInputQuotaPerMillion,
				exact.CacheWriteQuotaPerMillion,
				exact.CacheWrite5mQuotaPerMillion,
				exact.CacheWrite1hQuotaPerMillion,
				"",
				0,
				0,
			)
		}
		return finalizePublicCreditsPricing("per_1M_tokens", specs)
	}
	if categoryMultiplier == nil || *categoryMultiplier <= 0 || *categoryMultiplier > 1 {
		return nil
	}
	var official officialTokenPrice
	if err := common.UnmarshalJsonStr(entry.OfficialPrice, &official); err != nil {
		return nil
	}
	specs := make([]quotaPriceSpec, 0)
	appendDimensions := func(dimensions map[string]float64, tier string, minTokens, maxTokens *int) {
		keys := make([]string, 0, len(dimensions))
		for dimension := range dimensions {
			keys = append(keys, dimension)
		}
		sort.Strings(keys)
		for _, dimension := range keys {
			usd := dimensions[dimension]
			quotaDecimal := decimal.NewFromFloat(usd).
				Mul(decimal.NewFromFloat(*categoryMultiplier)).
				Mul(decimal.NewFromInt(publicCatalogCreditsPerUSD)).
				Mul(decimal.NewFromInt(int64(common.CreditsQuotaUnit)))
			quota64 := quotaDecimal.Round(0).IntPart()
			if quota64 <= 0 || quota64 > int64(^uint(0)>>1) {
				continue
			}
			quota := int(quota64)
			key := dimension
			if tier != "" {
				key = tier + ":" + dimension
			}
			spec := makeQuotaPriceSpec(key, quota, "geili")
			spec.Dimension = dimension
			spec.Tier = tier
			spec.MinPromptTokens = minTokens
			spec.MaxPromptTokens = maxTokens
			specs = append(specs, spec)
		}
	}
	if len(official.Dimensions) > 0 {
		appendDimensions(official.Dimensions, "", nil, nil)
	}
	for _, tier := range official.Tiers {
		appendDimensions(tier.Dimensions, tier.Label, tier.MinPromptTokens, tier.MaxPromptTokens)
	}
	return finalizePublicCreditsPricing("per_1M_tokens", specs)
}

func textQuotaPriceSpec(dimension string, quota int64, source, tier string) quotaPriceSpec {
	if quota <= 0 || quota > int64(^uint(0)>>1) {
		return quotaPriceSpec{}
	}
	key := dimension
	if tier != "" {
		key = tier + ":" + dimension
	}
	spec := makeQuotaPriceSpec(key, int(quota), source)
	spec.Dimension = dimension
	spec.Tier = tier
	return spec
}
