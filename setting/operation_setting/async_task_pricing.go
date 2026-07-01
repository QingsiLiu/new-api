package operation_setting

import (
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	defaultUSDCNY = 7.2
)

var (
	AsyncTaskSpecPricingEnabled      = false
	AsyncTaskProductRoutesEnabled    = false
	AsyncTaskServiceUserProxyEnabled = false
	QuotaPerCNY                      = common.QuotaPerUnit / defaultUSDCNY
	asyncSpecPricing                 = defaultAsyncSpecPricing()
)

type AsyncSpecPricing struct {
	Currency string                         `json:"currency,omitempty"`
	Video    map[string]AsyncVideoSpecPrice `json:"video,omitempty"`
	Image    map[string]AsyncImageSpecPrice `json:"image,omitempty"`
}

type AsyncVideoSpecPrice struct {
	Unit                string                               `json:"unit,omitempty"`
	Resolutions         map[string]AsyncVideoResolutionPrice `json:"resolutions,omitempty"`
	DefaultCNYPerSecond *float64                             `json:"default_cny_per_second,omitempty"`
	MinCNY              float64                              `json:"min_cny,omitempty"`
	MaxCNY              float64                              `json:"max_cny,omitempty"`
}

type AsyncVideoResolutionPrice struct {
	CNYPerSecond *float64 `json:"cny_per_second,omitempty"`
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
	Kind        string
	Model       string
	SpecKey     string
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

func defaultAsyncSpecPricing() AsyncSpecPricing {
	return AsyncSpecPricing{
		Currency: "CNY",
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
	modelName = strings.TrimSpace(modelName)
	if seconds < 0 {
		seconds = 0
	}
	spec, ok := asyncSpecPricing.Video[modelName]
	if !ok {
		return AsyncSpecQuotaResult{Kind: "video", Model: modelName, QuotaPerCNY: QuotaPerCNY}
	}
	specKey := normalizeResolution(resolution)
	unitCNY, matchedSpecKey, matched := resolveVideoUnitCNY(spec, specKey)
	if !matched {
		return AsyncSpecQuotaResult{Kind: "video", Model: modelName, QuotaPerCNY: QuotaPerCNY}
	}
	totalCNY := unitCNY * float64(seconds)
	totalCNY = applyCNYBounds(totalCNY, spec.MinCNY, spec.MaxCNY)
	return AsyncSpecQuotaResult{
		Quota:       roundCNYToQuota(totalCNY),
		Matched:     true,
		Kind:        "video",
		Model:       modelName,
		SpecKey:     matchedSpecKey,
		UnitCNY:     unitCNY,
		TotalCNY:    totalCNY,
		QuotaPerCNY: QuotaPerCNY,
	}
}

func ResolveImageSpecQuota(modelName string, size string, resolution string, quality string, n int) AsyncSpecQuotaResult {
	modelName = strings.TrimSpace(modelName)
	if n <= 0 {
		n = 1
	}
	spec, ok := asyncSpecPricing.Image[modelName]
	if !ok {
		return AsyncSpecQuotaResult{Kind: "image", Model: modelName, QuotaPerCNY: QuotaPerCNY}
	}
	resolutionCandidates := []string{
		normalizeImageResolution(size),
		normalizeImageResolution(resolution),
	}
	qualityKey := normalizeQuality(quality)
	unitCNY, matchedSpecKey, matched := resolveImageUnitCNY(spec, resolutionCandidates, qualityKey)
	if !matched {
		return AsyncSpecQuotaResult{Kind: "image", Model: modelName, QuotaPerCNY: QuotaPerCNY}
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
		QuotaPerCNY: QuotaPerCNY,
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
	if cny <= 0 {
		return 0
	}
	return int(math.Round(cny * effectiveQuotaPerCNY()))
}

func effectiveQuotaPerCNY() float64 {
	if QuotaPerCNY > 0 {
		return QuotaPerCNY
	}
	return common.QuotaPerUnit / defaultUSDCNY
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
