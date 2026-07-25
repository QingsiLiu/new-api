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

	var unitQuota int
	switch {
	case strings.Contains(normalized, "seedance-2-0-mini"):
		unitQuota = seedance2CreditsUnitQuota(resolution, mode, 60, 95, 125, 205)
	case (strings.Contains(normalized, "seedance-2-0") &&
		!strings.Contains(normalized, "mini") &&
		!strings.Contains(normalized, "fast")) ||
		normalized == "bytedance-seedance-2":
		unitQuota = seedance2CreditsUnitQuota(resolution, mode, 90, 155, 200, 330)
	default:
		return creditsV1AsyncSpec{}, false, nil
	}
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
		SpecKey:    resolution + "_" + mode,
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

func seedance2CreditsUnitQuota(resolution, mode string, video480, noVideo480, video720, noVideo720 int) int {
	withVideo := mode == "with_video_input"
	switch resolution {
	case "480p":
		if withVideo {
			return creditsTenthQuota(video480)
		}
		return creditsTenthQuota(noVideo480)
	case "720p":
		if withVideo {
			return creditsTenthQuota(video720)
		}
		return creditsTenthQuota(noVideo720)
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
