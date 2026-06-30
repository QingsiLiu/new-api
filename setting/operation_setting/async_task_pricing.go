package operation_setting

import "strings"

var (
	AsyncTaskSpecPricingEnabled      = false
	AsyncTaskProductRoutesEnabled    = false
	AsyncTaskServiceUserProxyEnabled = false
)

// TODO(D1): replace these placeholder async multipliers with the approved quota table.
const (
	AsyncImageQualityStandardMultiplier = 1.0
	AsyncImageQualityMediumMultiplier   = 1.4
	AsyncImageQualityHighMultiplier     = 2.0

	AsyncImageSize1024x1024Multiplier = 1.0
	AsyncImageSize1536x1024Multiplier = 1.15
	AsyncImageSize1024x1536Multiplier = 1.15
	AsyncImageSize1792x1024Multiplier = 1.25
	AsyncImageSize1024x1792Multiplier = 1.25

	AsyncVideoResolution480PMultiplier  = 1.0
	AsyncVideoResolution720PMultiplier  = 1.5
	AsyncVideoResolution1080PMultiplier = 2.0

	AsyncVideoBaseSeconds = 4
)

func GetAsyncImageQualityMultiplier(quality string) float64 {
	switch strings.ToLower(strings.TrimSpace(quality)) {
	case "medium", "hd":
		return AsyncImageQualityMediumMultiplier
	case "high":
		return AsyncImageQualityHighMultiplier
	default:
		return AsyncImageQualityStandardMultiplier
	}
}

func GetAsyncImageSizeMultiplier(size string) float64 {
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "1024x1024":
		return AsyncImageSize1024x1024Multiplier
	case "1536x1024", "1024x1536":
		return AsyncImageSize1536x1024Multiplier
	case "1792x1024", "1024x1792":
		return AsyncImageSize1792x1024Multiplier
	default:
		return AsyncImageSize1024x1024Multiplier
	}
}

func GetAsyncImageCountMultiplier(count int) float64 {
	if count <= 0 {
		return 1
	}
	return float64(count)
}

func GetAsyncImageSpecMultiplier(quality, size string, count int) float64 {
	return GetAsyncImageQualityMultiplier(quality) *
		GetAsyncImageSizeMultiplier(size) *
		GetAsyncImageCountMultiplier(count)
}

func GetAsyncVideoResolutionMultiplier(resolution string) float64 {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "720p":
		return AsyncVideoResolution720PMultiplier
	case "1080p":
		return AsyncVideoResolution1080PMultiplier
	default:
		return AsyncVideoResolution480PMultiplier
	}
}

func GetAsyncVideoDurationMultiplier(seconds int) float64 {
	if seconds <= 0 {
		seconds = AsyncVideoBaseSeconds
	}
	multiplier := float64(seconds) / float64(AsyncVideoBaseSeconds)
	if multiplier < 1 {
		multiplier = 1
	}
	return multiplier
}

func GetAsyncVideoSpecMultiplier(resolution string, seconds int) float64 {
	return GetAsyncVideoResolutionMultiplier(resolution) *
		GetAsyncVideoDurationMultiplier(seconds)
}
