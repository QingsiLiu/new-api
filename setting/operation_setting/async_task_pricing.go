package operation_setting

import (
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var (
	AsyncTaskSpecPricingEnabled      = false
	AsyncTaskProductRoutesEnabled    = false
	AsyncTaskServiceUserProxyEnabled = false
	QuotaPerCNY                      = common.CNYQuotaUnit
	asyncSpecPricing                 = emptyAsyncSpecPricing()
)

type AsyncSpecPricing struct {
	Currency string                         `json:"currency,omitempty"`
	Video    map[string]AsyncVideoSpecPrice `json:"video,omitempty"`
	Image    map[string]AsyncImageSpecPrice `json:"image,omitempty"`
}

type AsyncVideoSpecPrice struct {
	Unit                string                               `json:"unit,omitempty"`
	Resolutions         map[string]AsyncVideoResolutionPrice `json:"resolutions,omitempty"`
	Prices              map[string]AsyncVideoRatioPrices     `json:"prices,omitempty"`
	DefaultCNYPerSecond *float64                             `json:"default_cny_per_second,omitempty"`
	MinCNY              float64                              `json:"min_cny,omitempty"`
	MaxCNY              float64                              `json:"max_cny,omitempty"`
}

type AsyncVideoResolutionPrice struct {
	CNYPerSecond *float64 `json:"cny_per_second,omitempty"`
}

type AsyncVideoRatioPrices map[string]AsyncVideoModePrices

type AsyncVideoModePrices map[string]AsyncVideoModePrice

type AsyncVideoModePrice struct {
	CNYPerSecond *float64 `json:"cny_per_second,omitempty"`
	Unsupported  bool     `json:"unsupported,omitempty"`
}

type AsyncImageSpecPrice struct {
	Unit               string                               `json:"unit,omitempty"`
	Resolutions        map[string]AsyncImageResolutionPrice `json:"resolutions,omitempty"`
	Qualities          map[string]AsyncImageQualityPrice    `json:"qualities,omitempty"`
	DefaultCNYPerImage *float64                             `json:"default_cny_per_image,omitempty"`
}

type AsyncImageResolutionPrice struct {
	CNYPerImage *float64 `json:"cny_per_image,omitempty"`
}

type AsyncImageQualityPrice struct {
	CNYPerImage *float64 `json:"cny_per_image,omitempty"`
}

type AsyncSpecQuotaResult struct {
	Quota       int
	Matched     bool
	Unsupported bool
	Kind        string
	Model       string
	SpecKey     string
	Resolution  string
	Ratio       string
	Mode        string
	UnitCNY     float64
	TotalCNY    float64
	QuotaPerCNY float64
}

func AsyncSpecPricing2JSONString() string {
	bytes, err := common.Marshal(asyncSpecPricing)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func AsyncSpecPricingSeedJSONString() string {
	bytes, err := common.Marshal(seedAsyncSpecPricing())
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func emptyAsyncSpecPricing() AsyncSpecPricing {
	return AsyncSpecPricing{Currency: "CNY"}
}

func seedAsyncSpecPricing() AsyncSpecPricing {
	return AsyncSpecPricing{
		Currency: "CNY",
		Video:    seedSeedanceVideoSpecPricing(),
		Image: map[string]AsyncImageSpecPrice{
			"gemini-2.5-flash-image": {
				Unit:               "per_image",
				DefaultCNYPerImage: float64Ptr(0.12),
			},
			"gemini-3.1-flash-image-preview": {
				Unit: "per_image",
				Resolutions: map[string]AsyncImageResolutionPrice{
					"1k": {CNYPerImage: float64Ptr(0.18)},
					"2k": {CNYPerImage: float64Ptr(0.28)},
					"4k": {CNYPerImage: float64Ptr(0.42)},
				},
				DefaultCNYPerImage: float64Ptr(0.18),
			},
			"gemini-3-pro-image-preview": {
				Unit: "per_image",
				Resolutions: map[string]AsyncImageResolutionPrice{
					"1k": {CNYPerImage: float64Ptr(0.32)},
					"2k": {CNYPerImage: float64Ptr(0.32)},
					"4k": {CNYPerImage: float64Ptr(0.49)},
				},
				DefaultCNYPerImage: float64Ptr(0.32),
			},
			"gpt-image-2": {
				Unit: "per_image",
				Resolutions: map[string]AsyncImageResolutionPrice{
					"1k": {CNYPerImage: float64Ptr(0.11)},
					"2k": {CNYPerImage: float64Ptr(0.18)},
					"4k": {CNYPerImage: float64Ptr(0.29)},
				},
				DefaultCNYPerImage: float64Ptr(0.11),
			},
		},
	}
}

func float64Ptr(value float64) *float64 {
	return &value
}

type seedance2VideoPriceRow struct {
	resolution   string
	ratio        string
	noVideoInput float64
	videoInput   float64
}

type seedance15VideoPriceRow struct {
	resolution   string
	ratio        string
	textAudio    float64
	textNoAudio  float64
	imageAudio   *float64
	imageNoAudio *float64
}

func seedSeedanceVideoSpecPricing() map[string]AsyncVideoSpecPrice {
	return map[string]AsyncVideoSpecPrice{
		"seedance-2.0-mini": seedSeedance2VideoSpec([]seedance2VideoPriceRow{
			{"480p", "16:9", 0.2426, 0.1476},
			{"480p", "9:16", 0.2426, 0.1476},
			{"480p", "4:3", 0.2384, 0.1451},
			{"480p", "3:4", 0.2384, 0.1451},
			{"480p", "1:1", 0.2318, 0.1411},
			{"480p", "21:9", 0.2426, 0.1476},
			{"720p", "16:9", 0.5216, 0.3175},
			{"720p", "9:16", 0.5216, 0.3175},
			{"720p", "4:3", 0.5249, 0.3195},
			{"720p", "3:4", 0.5249, 0.3195},
			{"720p", "1:1", 0.5216, 0.3175},
			{"720p", "21:9", 0.5242, 0.3191},
		}),
		"seedance-2.0-fast": seedSeedance2VideoSpec([]seedance2VideoPriceRow{
			{"480p", "16:9", 0.3902, 0.232},
			{"480p", "9:16", 0.3902, 0.232},
			{"480p", "4:3", 0.3834, 0.228},
			{"480p", "3:4", 0.3834, 0.228},
			{"480p", "1:1", 0.373, 0.2218},
			{"480p", "21:9", 0.3902, 0.232},
			{"720p", "16:9", 0.8392, 0.499},
			{"720p", "9:16", 0.8392, 0.499},
			{"720p", "4:3", 0.6294, 0.3742},
			{"720p", "3:4", 0.6294, 0.3742},
			{"720p", "1:1", 0.8392, 0.499},
			{"720p", "21:9", 1.1014, 0.6549},
		}),
		"seedance-2.0": seedSeedance2VideoSpec([]seedance2VideoPriceRow{
			{"480p", "16:9", 0.4851, 0.2953},
			{"480p", "9:16", 0.4851, 0.2953},
			{"480p", "4:3", 0.4851, 0.2953},
			{"480p", "3:4", 0.4851, 0.2953},
			{"480p", "1:1", 0.4851, 0.2953},
			{"480p", "21:9", 0.4851, 0.2953},
			{"720p", "16:9", 1.0433, 0.635},
			{"720p", "9:16", 1.0433, 0.635},
			{"720p", "4:3", 1.0433, 0.635},
			{"720p", "3:4", 1.0433, 0.635},
			{"720p", "1:1", 1.0433, 0.635},
			{"720p", "21:9", 1.3693, 0.8335},
			{"1080p", "16:9", 2.6025, 1.5819},
			{"1080p", "9:16", 2.6025, 1.5819},
			{"1080p", "4:3", 2.6025, 1.5819},
			{"1080p", "3:4", 2.6025, 1.5819},
			{"1080p", "1:1", 2.6025, 1.5819},
			{"1080p", "21:9", 3.4159, 2.0763},
			{"4k", "16:9", 5.307, 3.2657},
			{"4k", "9:16", 5.307, 3.2657},
			{"4k", "4:3", 3.9802, 2.4493},
			{"4k", "3:4", 3.9802, 2.4493},
			{"4k", "1:1", 5.307, 3.2657},
			{"4k", "21:9", 6.9654, 4.2862},
		}),
		"seedance-1.5-pro": seedSeedance15VideoSpec([]seedance15VideoPriceRow{
			{"480p", "16:9", 0.1687, 0.0844, float64Ptr(0.1012), float64Ptr(0.0591)},
			{"480p", "9:16", 0.1687, 0.0844, float64Ptr(0.1012), float64Ptr(0.0591)},
			{"480p", "4:3", 0.1658, 0.0829, float64Ptr(0.0995), float64Ptr(0.058)},
			{"480p", "3:4", 0.1658, 0.0829, float64Ptr(0.0995), float64Ptr(0.058)},
			{"480p", "1:1", 0.1613, 0.0806, float64Ptr(0.0968), float64Ptr(0.0564)},
			{"480p", "21:9", 0.1687, 0.0844, float64Ptr(0.1012), float64Ptr(0.0591)},
			{"720p", "16:9", 0.3629, 0.1814, nil, nil},
			{"720p", "9:16", 0.3629, 0.1814, nil, nil},
			{"720p", "4:3", 0.3652, 0.1826, nil, nil},
			{"720p", "3:4", 0.3652, 0.1826, nil, nil},
			{"720p", "1:1", 0.3629, 0.1814, nil, nil},
			{"720p", "21:9", 0.3646, 0.1823, nil, nil},
			{"1080p", "16:9", 0.8165, 0.4082, nil, nil},
			{"1080p", "9:16", 0.8165, 0.4082, nil, nil},
			{"1080p", "4:3", 0.8177, 0.4088, nil, nil},
			{"1080p", "3:4", 0.8177, 0.4088, nil, nil},
			{"1080p", "1:1", 0.8165, 0.4082, nil, nil},
			{"1080p", "21:9", 0.8217, 0.4109, nil, nil},
		}),
	}
}

func seedSeedance2VideoSpec(rows []seedance2VideoPriceRow) AsyncVideoSpecPrice {
	spec := AsyncVideoSpecPrice{
		Unit:   "per_second",
		Prices: map[string]AsyncVideoRatioPrices{},
	}
	for _, row := range rows {
		putSeedVideoModePrice(spec.Prices, row.resolution, row.ratio, "no_video_input", AsyncVideoModePrice{CNYPerSecond: float64Ptr(row.noVideoInput)})
		putSeedVideoModePrice(spec.Prices, row.resolution, row.ratio, "with_video_input", AsyncVideoModePrice{CNYPerSecond: float64Ptr(row.videoInput)})
	}
	return spec
}

func seedSeedance15VideoSpec(rows []seedance15VideoPriceRow) AsyncVideoSpecPrice {
	spec := AsyncVideoSpecPrice{
		Unit:   "per_second",
		Prices: map[string]AsyncVideoRatioPrices{},
	}
	for _, row := range rows {
		putSeedVideoModePrice(spec.Prices, row.resolution, row.ratio, "text_audio", AsyncVideoModePrice{CNYPerSecond: float64Ptr(row.textAudio)})
		putSeedVideoModePrice(spec.Prices, row.resolution, row.ratio, "text_no_audio", AsyncVideoModePrice{CNYPerSecond: float64Ptr(row.textNoAudio)})
		putSeedVideoModePrice(spec.Prices, row.resolution, row.ratio, "image_audio", seedVideoModePrice(row.imageAudio))
		putSeedVideoModePrice(spec.Prices, row.resolution, row.ratio, "image_no_audio", seedVideoModePrice(row.imageNoAudio))
	}
	return spec
}

func seedVideoModePrice(value *float64) AsyncVideoModePrice {
	if value == nil {
		return AsyncVideoModePrice{Unsupported: true}
	}
	return AsyncVideoModePrice{CNYPerSecond: value}
}

func putSeedVideoModePrice(prices map[string]AsyncVideoRatioPrices, resolution string, ratio string, mode string, price AsyncVideoModePrice) {
	if _, ok := prices[resolution]; !ok {
		prices[resolution] = AsyncVideoRatioPrices{}
	}
	if _, ok := prices[resolution][ratio]; !ok {
		prices[resolution][ratio] = AsyncVideoModePrices{}
	}
	prices[resolution][ratio][mode] = price
}

func UpdateAsyncSpecPricingByJSONString(jsonStr string) error {
	next, err := parseAsyncSpecPricingJSONString(jsonStr)
	if err != nil {
		asyncSpecPricing = AsyncSpecPricing{Currency: "CNY"}
		return err
	}
	asyncSpecPricing = next
	return nil
}

func ValidateAsyncSpecPricingJSONString(jsonStr string) error {
	_, err := parseAsyncSpecPricingJSONString(jsonStr)
	return err
}

func parseAsyncSpecPricingJSONString(jsonStr string) (AsyncSpecPricing, error) {
	var next AsyncSpecPricing
	if strings.TrimSpace(jsonStr) == "" {
		next = AsyncSpecPricing{Currency: "CNY"}
	} else if err := common.Unmarshal([]byte(jsonStr), &next); err != nil {
		return AsyncSpecPricing{Currency: "CNY"}, err
	}
	if strings.TrimSpace(next.Currency) == "" {
		next.Currency = "CNY"
	}
	next.Video = normalizeVideoSpecPricing(next.Video)
	next.Image = normalizeImageSpecPricing(next.Image)
	return next, nil
}

func GetAsyncSpecPricingCopy() AsyncSpecPricing {
	copyPricing := AsyncSpecPricing{
		Currency: asyncSpecPricing.Currency,
		Video:    make(map[string]AsyncVideoSpecPrice, len(asyncSpecPricing.Video)),
		Image:    make(map[string]AsyncImageSpecPrice, len(asyncSpecPricing.Image)),
	}
	for model, spec := range asyncSpecPricing.Video {
		spec.Resolutions = copyVideoResolutionPrices(spec.Resolutions)
		spec.Prices = copyVideoMatrixPrices(spec.Prices)
		copyPricing.Video[model] = spec
	}
	for model, spec := range asyncSpecPricing.Image {
		spec.Resolutions = copyImageResolutionPrices(spec.Resolutions)
		spec.Qualities = copyImageQualityPrices(spec.Qualities)
		copyPricing.Image[model] = spec
	}
	return copyPricing
}

func ResolveVideoSpecQuota(modelName string, resolution string, seconds int) AsyncSpecQuotaResult {
	return resolveVideoSpecQuotaFromPricing(asyncSpecPricing, modelName, resolution, "", "", seconds, false)
}

func ResolveVideoSpecQuotaByContext(modelName string, resolution string, ratio string, mode string, seconds int) AsyncSpecQuotaResult {
	return resolveVideoSpecQuotaFromPricing(asyncSpecPricing, modelName, resolution, ratio, mode, seconds, true)
}

func ResolveVideoSpecQuotaByContextFromPricing(pricing AsyncSpecPricing, modelName string, resolution string, ratio string, mode string, seconds int) AsyncSpecQuotaResult {
	return resolveVideoSpecQuotaFromPricing(pricing, modelName, resolution, ratio, mode, seconds, true)
}

func resolveVideoSpecQuotaFromPricing(pricing AsyncSpecPricing, modelName string, resolution string, ratio string, mode string, seconds int, includeMatrix bool) AsyncSpecQuotaResult {
	modelName = strings.TrimSpace(modelName)
	if seconds < 0 {
		seconds = 0
	}
	spec, ok := pricing.Video[modelName]
	if !ok {
		return AsyncSpecQuotaResult{Kind: "video", Model: modelName, QuotaPerCNY: effectiveQuotaPerCNY()}
	}
	specKey := normalizeResolution(resolution)
	ratioKey := normalizeRatio(firstNonEmptyString(ratio, ratioFromSize(resolution)))
	modeKey := normalizeVideoMode(mode)
	if includeMatrix && len(spec.Prices) > 0 {
		unitCNY, matrixSpecKey, matched, unsupported := resolveVideoMatrixUnitCNY(spec, specKey, ratioKey, modeKey)
		if unsupported {
			return AsyncSpecQuotaResult{
				Unsupported: true,
				Kind:        "video",
				Model:       modelName,
				SpecKey:     matrixSpecKey,
				Resolution:  specKey,
				Ratio:       ratioKey,
				Mode:        modeKey,
				QuotaPerCNY: effectiveQuotaPerCNY(),
			}
		}
		if matched {
			totalCNY := unitCNY * float64(seconds)
			totalCNY = applyCNYBounds(totalCNY, spec.MinCNY, spec.MaxCNY)
			return AsyncSpecQuotaResult{
				Quota:       roundCNYToQuota(totalCNY),
				Matched:     true,
				Kind:        "video",
				Model:       modelName,
				SpecKey:     matrixSpecKey,
				Resolution:  specKey,
				Ratio:       ratioKey,
				Mode:        modeKey,
				UnitCNY:     unitCNY,
				TotalCNY:    totalCNY,
				QuotaPerCNY: effectiveQuotaPerCNY(),
			}
		}
		if specKey != "" || ratioKey != "" || modeKey != "" {
			return AsyncSpecQuotaResult{
				Unsupported: true,
				Kind:        "video",
				Model:       modelName,
				SpecKey:     videoMatrixSpecKey(specKey, ratioKey, modeKey),
				Resolution:  specKey,
				Ratio:       ratioKey,
				Mode:        modeKey,
				QuotaPerCNY: effectiveQuotaPerCNY(),
			}
		}
	}
	unitCNY, matchedSpecKey, matched := resolveVideoUnitCNY(spec, specKey)
	if !matched {
		return AsyncSpecQuotaResult{Kind: "video", Model: modelName, QuotaPerCNY: effectiveQuotaPerCNY()}
	}
	totalCNY := unitCNY * float64(seconds)
	totalCNY = applyCNYBounds(totalCNY, spec.MinCNY, spec.MaxCNY)
	return AsyncSpecQuotaResult{
		Quota:       roundCNYToQuota(totalCNY),
		Matched:     true,
		Kind:        "video",
		Model:       modelName,
		SpecKey:     matchedSpecKey,
		Resolution:  specKey,
		UnitCNY:     unitCNY,
		TotalCNY:    totalCNY,
		QuotaPerCNY: effectiveQuotaPerCNY(),
	}
}

func ResolveImageSpecQuota(modelName string, size string, resolution string, quality string, n int) AsyncSpecQuotaResult {
	return ResolveImageSpecQuotaFromPricing(asyncSpecPricing, modelName, size, resolution, quality, n)
}

func ResolveImageSpecQuotaFromPricing(pricing AsyncSpecPricing, modelName string, size string, resolution string, quality string, n int) AsyncSpecQuotaResult {
	modelName = strings.TrimSpace(modelName)
	if n <= 0 {
		n = 1
	}
	spec, ok := pricing.Image[modelName]
	if !ok {
		return AsyncSpecQuotaResult{Kind: "image", Model: modelName, QuotaPerCNY: effectiveQuotaPerCNY()}
	}
	resolutionCandidates := []string{
		normalizeImageResolution(size),
		normalizeImageResolution(resolution),
	}
	qualityKey := normalizeQuality(quality)
	unitCNY, matchedSpecKey, matched := resolveImageUnitCNY(spec, resolutionCandidates, qualityKey)
	if !matched {
		return AsyncSpecQuotaResult{Kind: "image", Model: modelName, QuotaPerCNY: effectiveQuotaPerCNY()}
	}
	totalCNY := unitCNY * float64(n)
	return AsyncSpecQuotaResult{
		Quota:       roundCNYToQuota(totalCNY),
		Matched:     true,
		Kind:        "image",
		Model:       modelName,
		SpecKey:     matchedSpecKey,
		UnitCNY:     unitCNY,
		TotalCNY:    totalCNY,
		QuotaPerCNY: effectiveQuotaPerCNY(),
	}
}

func GetAsyncImageSpecQuota(quality string, _ string, count int) int {
	result := ResolveImageSpecQuota("", "", "", quality, count)
	return result.Quota
}

func GetAsyncVideoSpecQuota(resolution string, seconds int) int {
	result := ResolveVideoSpecQuota("", resolution, seconds)
	return result.Quota
}

func resolveVideoUnitCNY(spec AsyncVideoSpecPrice, specKey string) (float64, string, bool) {
	if specKey != "" {
		if price, ok := spec.Resolutions[specKey]; ok && price.CNYPerSecond != nil {
			return *price.CNYPerSecond, specKey, true
		}
	}
	if spec.DefaultCNYPerSecond != nil {
		return *spec.DefaultCNYPerSecond, "default", true
	}
	return 0, "", false
}

func resolveVideoMatrixUnitCNY(spec AsyncVideoSpecPrice, resolution string, ratio string, mode string) (float64, string, bool, bool) {
	if resolution == "" || ratio == "" || mode == "" {
		return 0, videoMatrixSpecKey(resolution, ratio, mode), false, false
	}
	ratioPrices, ok := spec.Prices[resolution]
	if !ok {
		return 0, videoMatrixSpecKey(resolution, ratio, mode), false, false
	}
	modePrices, ok := ratioPrices[ratio]
	if !ok {
		return 0, videoMatrixSpecKey(resolution, ratio, mode), false, false
	}
	price, ok := modePrices[mode]
	if !ok {
		return 0, videoMatrixSpecKey(resolution, ratio, mode), false, false
	}
	specKey := videoMatrixSpecKey(resolution, ratio, mode)
	if price.Unsupported {
		return 0, specKey, false, true
	}
	if price.CNYPerSecond == nil {
		return 0, specKey, false, false
	}
	return *price.CNYPerSecond, specKey, true, false
}

func videoMatrixSpecKey(resolution string, ratio string, mode string) string {
	return strings.Join([]string{resolution, ratio, mode}, ":")
}

func resolveImageUnitCNY(spec AsyncImageSpecPrice, resolutionCandidates []string, qualityKey string) (float64, string, bool) {
	seenResolutionKeys := map[string]bool{}
	for _, specKey := range resolutionCandidates {
		if specKey == "" || seenResolutionKeys[specKey] {
			continue
		}
		seenResolutionKeys[specKey] = true
		if price, ok := spec.Resolutions[specKey]; ok && price.CNYPerImage != nil {
			return *price.CNYPerImage, specKey, true
		}
	}
	if qualityKey != "" {
		if price, ok := spec.Qualities[qualityKey]; ok && price.CNYPerImage != nil {
			return *price.CNYPerImage, qualityKey, true
		}
	}
	if spec.DefaultCNYPerImage != nil {
		return *spec.DefaultCNYPerImage, "default", true
	}
	return 0, "", false
}

func applyCNYBounds(value float64, minCNY float64, maxCNY float64) float64 {
	if minCNY > 0 && value < minCNY {
		value = minCNY
	}
	if maxCNY > 0 && value > maxCNY {
		value = maxCNY
	}
	return value
}

func roundCNYToQuota(cny float64) int {
	return common.CNYToQuota(cny)
}

func effectiveQuotaPerCNY() float64 {
	return common.CNYQuotaUnit
}

func normalizeVideoSpecPricing(src map[string]AsyncVideoSpecPrice) map[string]AsyncVideoSpecPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]AsyncVideoSpecPrice, len(src))
	for model, spec := range src {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		spec.Resolutions = normalizeVideoResolutionPrices(spec.Resolutions)
		spec.Prices = normalizeVideoMatrixPrices(spec.Prices)
		dst[model] = spec
	}
	return dst
}

func normalizeVideoResolutionPrices(src map[string]AsyncVideoResolutionPrice) map[string]AsyncVideoResolutionPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]AsyncVideoResolutionPrice, len(src))
	for resolution, price := range src {
		key := normalizeResolution(resolution)
		if key == "" {
			continue
		}
		dst[key] = price
	}
	return dst
}

func normalizeVideoMatrixPrices(src map[string]AsyncVideoRatioPrices) map[string]AsyncVideoRatioPrices {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]AsyncVideoRatioPrices, len(src))
	for resolution, ratioPrices := range src {
		resolutionKey := normalizeResolution(resolution)
		if resolutionKey == "" {
			continue
		}
		normalizedRatios := make(AsyncVideoRatioPrices, len(ratioPrices))
		for ratio, modePrices := range ratioPrices {
			ratioKey := normalizeRatio(ratio)
			if ratioKey == "" {
				continue
			}
			normalizedModes := make(AsyncVideoModePrices, len(modePrices))
			for mode, price := range modePrices {
				modeKey := normalizeVideoMode(mode)
				if modeKey == "" {
					continue
				}
				normalizedModes[modeKey] = price
			}
			if len(normalizedModes) > 0 {
				normalizedRatios[ratioKey] = normalizedModes
			}
		}
		if len(normalizedRatios) > 0 {
			dst[resolutionKey] = normalizedRatios
		}
	}
	return dst
}

func normalizeImageSpecPricing(src map[string]AsyncImageSpecPrice) map[string]AsyncImageSpecPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]AsyncImageSpecPrice, len(src))
	for model, spec := range src {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		spec.Resolutions = normalizeImageResolutionPrices(spec.Resolutions)
		spec.Qualities = normalizeImageQualityPrices(spec.Qualities)
		dst[model] = spec
	}
	return dst
}

func normalizeImageResolutionPrices(src map[string]AsyncImageResolutionPrice) map[string]AsyncImageResolutionPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]AsyncImageResolutionPrice, len(src))
	for resolution, price := range src {
		key := normalizeImageResolution(resolution)
		if key == "" {
			continue
		}
		dst[key] = price
	}
	return dst
}

func normalizeImageQualityPrices(src map[string]AsyncImageQualityPrice) map[string]AsyncImageQualityPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]AsyncImageQualityPrice, len(src))
	for quality, price := range src {
		key := normalizeQuality(quality)
		if key == "" {
			continue
		}
		dst[key] = price
	}
	return dst
}

func normalizeResolution(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, "*", "x")
	switch value {
	case "", "auto", "default":
		return ""
	case "sd", "480", "480p", "854x480", "480x854":
		return "480p"
	case "hd", "720", "720p", "1280x720", "720x1280":
		return "720p"
	case "fhd", "fullhd", "full-hd", "1080", "1080p", "1920x1080", "1080x1920":
		return "1080p"
	case "2k", "1440", "1440p", "2560x1440", "1440x2560":
		return "2k"
	case "uhd", "4k", "2160", "2160p", "3840x2160", "2160x3840":
		return "4k"
	default:
		if normalized := normalizeVideoSizeResolution(value); normalized != "" {
			return normalized
		}
		return value
	}
}

func normalizeVideoSizeResolution(value string) string {
	if !strings.Contains(value, "x") {
		return ""
	}
	parts := strings.Split(value, "x")
	if len(parts) != 2 {
		return ""
	}
	width, widthErr := strconv.Atoi(parts[0])
	height, heightErr := strconv.Atoi(parts[1])
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return ""
	}
	shortSide := width
	if height < shortSide {
		shortSide = height
	}
	switch {
	case shortSide <= 480:
		return "480p"
	case shortSide <= 720:
		return "720p"
	case shortSide <= 1080:
		return "1080p"
	case shortSide <= 1440:
		return "2k"
	default:
		return "4k"
	}
}

func normalizeRatio(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, "：", ":")
	value = strings.ReplaceAll(value, "*", "x")
	switch value {
	case "", "auto", "default":
		return ""
	case "1", "1:1", "square":
		return "1:1"
	case "16:9", "landscape", "horizontal":
		return "16:9"
	case "9:16", "portrait", "vertical":
		return "9:16"
	case "4:3":
		return "4:3"
	case "3:4":
		return "3:4"
	case "21:9", "ultrawide":
		return "21:9"
	}
	if strings.Contains(value, "x") {
		return ratioFromSize(value)
	}
	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		if len(parts) != 2 {
			return ""
		}
		left, leftErr := strconv.Atoi(parts[0])
		right, rightErr := strconv.Atoi(parts[1])
		if leftErr != nil || rightErr != nil || left <= 0 || right <= 0 {
			return ""
		}
		return normalizeKnownRatio(left, right)
	}
	return ""
}

func ratioFromSize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, "*", "x")
	if !strings.Contains(value, "x") {
		return ""
	}
	parts := strings.Split(value, "x")
	if len(parts) != 2 {
		return ""
	}
	width, widthErr := strconv.Atoi(parts[0])
	height, heightErr := strconv.Atoi(parts[1])
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return ""
	}
	return normalizeKnownRatio(width, height)
}

func normalizeKnownRatio(width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	ratio := float64(width) / float64(height)
	candidates := []struct {
		key   string
		value float64
	}{
		{"16:9", 16.0 / 9.0},
		{"9:16", 9.0 / 16.0},
		{"4:3", 4.0 / 3.0},
		{"3:4", 3.0 / 4.0},
		{"1:1", 1},
		{"21:9", 21.0 / 9.0},
	}
	bestKey := ""
	bestDelta := math.MaxFloat64
	for _, candidate := range candidates {
		delta := math.Abs(ratio - candidate.value)
		if delta < bestDelta {
			bestDelta = delta
			bestKey = candidate.key
		}
	}
	if bestDelta <= 0.08 {
		return bestKey
	}
	return ""
}

func normalizeVideoMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "", "auto", "default":
		return ""
	case "no_video", "without_video", "without_video_input", "no_video_input", "text_to_video", "text2video":
		return "no_video_input"
	case "with_video", "video_input", "with_video_input", "video_to_video", "video2video":
		return "with_video_input"
	case "text_audio", "no_image_audio", "without_image_audio":
		return "text_audio"
	case "text_no_audio", "no_image_no_audio", "without_image_no_audio":
		return "text_no_audio"
	case "image_audio", "with_image_audio", "image_to_video_audio":
		return "image_audio"
	case "image_no_audio", "with_image_no_audio", "image_to_video_no_audio":
		return "image_no_audio"
	default:
		return value
	}
}

func normalizeImageResolution(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, "*", "x")
	switch value {
	case "", "auto", "default":
		return ""
	case "1k", "1024", "1024p":
		return "1k"
	case "2k", "2048", "2048p":
		return "2k"
	case "4k", "4096", "4096p":
		return "4k"
	}

	if strings.Contains(value, "x") {
		parts := strings.Split(value, "x")
		if len(parts) != 2 {
			return ""
		}
		width, widthErr := strconv.Atoi(parts[0])
		height, heightErr := strconv.Atoi(parts[1])
		if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
			return ""
		}
		maxDimension := width
		if height > maxDimension {
			maxDimension = height
		}
		switch {
		case maxDimension <= 1024:
			return "1k"
		case maxDimension <= 2048:
			return "2k"
		default:
			return "4k"
		}
	}
	return ""
}

func normalizeQuality(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "-")
	switch value {
	case "", "auto", "standard", "default":
		return ""
	case "low", "l", "sd":
		return "low"
	case "medium", "med", "m":
		return "medium"
	case "high", "hd", "hq", "large":
		return "high"
	default:
		return value
	}
}

func copyVideoResolutionPrices(src map[string]AsyncVideoResolutionPrice) map[string]AsyncVideoResolutionPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]AsyncVideoResolutionPrice, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func copyVideoMatrixPrices(src map[string]AsyncVideoRatioPrices) map[string]AsyncVideoRatioPrices {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]AsyncVideoRatioPrices, len(src))
	for resolution, ratioPrices := range src {
		ratioCopy := make(AsyncVideoRatioPrices, len(ratioPrices))
		for ratio, modePrices := range ratioPrices {
			modeCopy := make(AsyncVideoModePrices, len(modePrices))
			for mode, price := range modePrices {
				modeCopy[mode] = price
			}
			ratioCopy[ratio] = modeCopy
		}
		dst[resolution] = ratioCopy
	}
	return dst
}

func copyImageQualityPrices(src map[string]AsyncImageQualityPrice) map[string]AsyncImageQualityPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]AsyncImageQualityPrice, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func copyImageResolutionPrices(src map[string]AsyncImageResolutionPrice) map[string]AsyncImageResolutionPrice {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]AsyncImageResolutionPrice, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
