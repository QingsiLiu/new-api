package controller

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

type creditsV1AsyncSpec struct {
	Kind       string
	Model      string
	SpecKey    string
	Resolution string
	Ratio      string
	Mode       string
	UnitQuota  int
	TotalQuota int
}

type creditsV1VideoTier struct {
	withVideoTenths int
	noVideoTenths   int
}

var creditsV1SeedanceVideoPrices = map[string]map[string]creditsV1VideoTier{
	"seedance-2.0-mini": {
		"480p": {withVideoTenths: 60, noVideoTenths: 95},
		"720p": {withVideoTenths: 125, noVideoTenths: 205},
	},
	"seedance-2.0-fast": {
		"480p": {withVideoTenths: 90, noVideoTenths: 155},
		"720p": {withVideoTenths: 200, noVideoTenths: 330},
	},
	"seedance-2.0": {
		"480p":  {withVideoTenths: 115, noVideoTenths: 190},
		"720p":  {withVideoTenths: 250, noVideoTenths: 410},
		"1080p": {withVideoTenths: 620, noVideoTenths: 1020},
		"4k":    {withVideoTenths: 1280, noVideoTenths: 2080},
	},
}

func resolveCreditsV1AsyncSpec(request asyncTaskRequest, modelName string) (creditsV1AsyncSpec, bool, error) {
	switch request.Kind {
	case asyncTaskKindVideo:
		return resolveCreditsV1VideoSpec(request, modelName)
	default:
		return resolveCreditsV1ImageSpec(request, modelName)
	}
}

func resolveCreditsV1ImageSpec(request asyncTaskRequest, modelName string) (creditsV1AsyncSpec, bool, error) {
	normalized := normalizeCreditsModelName(modelName)
	specKey := operation_setting.ResolveImageSpecKey(
		asyncParamString(request.Parameters, "size"),
		asyncParamString(request.Parameters, "resolution"),
	)
	count := asyncImageSpecCount(request.Parameters)
	if count <= 0 {
		count = 1
	}

	var unitQuota int
	switch {
	case strings.Contains(normalized, "gpt-image-2"):
		if specKey == "" {
			specKey = "1k"
		}
		unitQuota = imageTierQuota(specKey, 3, 5, 8)
	case strings.Contains(normalized, "gemini-2-5-flash-image"),
		normalized == "google-nano-banana",
		normalized == "google-nano-banana-edit",
		normalized == "nano-banana",
		normalized == "nano-banana-edit":
		specKey = "default"
		unitQuota = creditsQuota(3)
	case strings.Contains(normalized, "gemini-3-1-flash-image-preview"),
		normalized == "nano-banana-2":
		if specKey == "" {
			specKey = "1k"
		}
		unitQuota = imageTierQuota(specKey, 5, 8, 12)
	case strings.Contains(normalized, "gemini-3-pro-image-preview"),
		normalized == "nano-banana-pro":
		if specKey == "" {
			specKey = "1k"
		}
		unitQuota = imageTierQuota(specKey, 8, 8, 14)
	case strings.Contains(normalized, "seedream-4-5"):
		specKey = "default"
		unitQuota = creditsHalfQuota(6, 1)
	default:
		return creditsV1AsyncSpec{}, false, nil
	}
	if unitQuota <= 0 {
		return creditsV1AsyncSpec{}, false, nil
	}
	totalQuota, err := checkedCreditsQuotaProduct(unitQuota, count)
	if err != nil {
		return creditsV1AsyncSpec{}, true, err
	}
	return creditsV1AsyncSpec{
		Kind:       asyncTaskKindImage,
		Model:      modelName,
		SpecKey:    specKey,
		Resolution: specKey,
		UnitQuota:  unitQuota,
		TotalQuota: totalQuota,
	}, true, nil
}

func resolveCreditsV1VideoSpec(request asyncTaskRequest, modelName string) (creditsV1AsyncSpec, bool, error) {
	normalized := normalizeCreditsModelName(modelName)
	resolution := normalizeCreditsVideoResolution(asyncVideoSpecResolution(request.Parameters))
	mode := asyncVideoSpecMode(request)
	seconds := asyncParamIntValue(request.Parameters, "duration", 0)
	if seconds <= 0 {
		seconds = asyncParamIntValue(request.Parameters, "seconds", 0)
	}
	if seconds <= 0 {
		return creditsV1AsyncSpec{}, false, nil
	}

	var priceModel string
	switch {
	case strings.Contains(normalized, "seedance-2-0-mini"):
		priceModel = "seedance-2.0-mini"
	case strings.Contains(normalized, "seedance-2-0-fast") || normalized == "bytedance-seedance-2-fast":
		priceModel = "seedance-2.0-fast"
	case (strings.Contains(normalized, "seedance-2-0") &&
		!strings.Contains(normalized, "mini") &&
		!strings.Contains(normalized, "fast")) ||
		normalized == "bytedance-seedance-2":
		priceModel = "seedance-2.0"
	default:
		return creditsV1AsyncSpec{}, false, nil
	}
	unitQuota := seedanceVideoCreditsUnitQuota(resolution, mode, creditsV1SeedanceVideoPrices[priceModel])
	if unitQuota <= 0 {
		return creditsV1AsyncSpec{}, false, nil
	}
	totalQuota, err := checkedCreditsQuotaProduct(unitQuota, seconds)
	if err != nil {
		return creditsV1AsyncSpec{}, true, err
	}
	return creditsV1AsyncSpec{
		Kind:       asyncTaskKindVideo,
		Model:      modelName,
		SpecKey:    resolution + ":" + mode,
		Resolution: resolution,
		Ratio:      asyncVideoSpecRatio(request.Parameters),
		Mode:       mode,
		UnitQuota:  unitQuota,
		TotalQuota: totalQuota,
	}, true, nil
}

func applyCreditsV1AsyncSpecPricing(relayInfo *relaycommon.RelayInfo, exact creditsV1AsyncSpec) error {
	relayInfo.PriceData.Quota = exact.TotalQuota
	relayInfo.PriceData.QuotaToPreConsume = exact.TotalQuota
	relayInfo.PriceData.SpecPricing = &types.SpecPricingInfo{
		Priced:        true,
		Kind:          exact.Kind,
		Model:         exact.Model,
		SpecKey:       exact.SpecKey,
		Resolution:    exact.Resolution,
		Ratio:         exact.Ratio,
		Mode:          exact.Mode,
		UnitCNY:       common2QuotaToCNY(exact.UnitQuota),
		TotalCNY:      common2QuotaToCNY(exact.TotalQuota),
		Quota:         exact.TotalQuota,
		QuotaPerCNY:   float64(common.QuotaPerUnit),
		UnitCredits:   common.QuotaToCreditsString(exact.UnitQuota),
		TotalCredits:  common.QuotaToCreditsString(exact.TotalQuota),
		PricingSource: "kie",
	}
	relayInfo.PriceData.ReplaceOtherRatios(map[string]float64{"spec_priced": 1})
	return nil
}

func normalizeCreditsModelName(modelName string) string {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	replacer := strings.NewReplacer(".", "-", "_", "-", "/", "-", " ", "")
	for strings.Contains(normalized, "--") {
		normalized = strings.ReplaceAll(normalized, "--", "-")
	}
	return replacer.Replace(normalized)
}

func normalizeCreditsVideoResolution(resolution string) string {
	normalized := strings.ToLower(strings.TrimSpace(resolution))
	switch {
	case strings.Contains(normalized, "1080"):
		return "1080p"
	case strings.Contains(normalized, "720"):
		return "720p"
	case strings.Contains(normalized, "480"):
		return "480p"
	default:
		return normalized
	}
}

func imageTierQuota(specKey string, oneK, twoK, fourK int) int {
	switch strings.ToLower(strings.TrimSpace(specKey)) {
	case "1k":
		return creditsQuota(oneK)
	case "2k":
		return creditsQuota(twoK)
	case "4k":
		return creditsQuota(fourK)
	default:
		return 0
	}
}

func seedanceVideoCreditsUnitQuota(resolution, mode string, prices map[string]creditsV1VideoTier) int {
	tier, ok := prices[resolution]
	if !ok {
		return 0
	}
	switch mode {
	case "with_video_input":
		return creditsTenthQuota(tier.withVideoTenths)
	case "no_video_input":
		return creditsTenthQuota(tier.noVideoTenths)
	default:
		return 0
	}
}

func creditsQuota(credits int) int {
	return credits * common.CreditsQuotaUnit
}

func creditsHalfQuota(whole, halves int) int {
	return whole*common.CreditsQuotaUnit + halves*(common.CreditsQuotaUnit/2)
}

func creditsTenthQuota(tenths int) int {
	return tenths * (common.CreditsQuotaUnit / 10)
}

func checkedCreditsQuotaProduct(unitQuota, units int) (int, error) {
	if unitQuota <= 0 || units <= 0 || unitQuota > common.MaxQuota/units {
		return 0, fmt.Errorf("credits price exceeds the supported quota limit")
	}
	return unitQuota * units, nil
}

func common2QuotaToCNY(quota int) float64 {
	return common.QuotaToPublicCNY(quota)
}
