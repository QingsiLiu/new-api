package operation_setting

import (
	"math"
	"testing"
)

func TestAsyncImageSpecPricingMultipliersCoverPlaceholderTiers(t *testing.T) {
	t.Parallel()

	qualityCases := []struct {
		name    string
		quality string
		want    float64
	}{
		{name: "empty defaults to standard", quality: "", want: AsyncImageQualityStandardMultiplier},
		{name: "standard", quality: "standard", want: AsyncImageQualityStandardMultiplier},
		{name: "medium", quality: "medium", want: AsyncImageQualityMediumMultiplier},
		{name: "hd aliases medium", quality: "hd", want: AsyncImageQualityMediumMultiplier},
		{name: "high", quality: "high", want: AsyncImageQualityHighMultiplier},
	}
	for _, tc := range qualityCases {
		t.Run("quality/"+tc.name, func(t *testing.T) {
			assertFloatEqual(t, tc.want, GetAsyncImageQualityMultiplier(tc.quality))
		})
	}

	sizeCases := []struct {
		name string
		size string
		want float64
	}{
		{name: "empty defaults to square", size: "", want: AsyncImageSize1024x1024Multiplier},
		{name: "1024x1024", size: "1024x1024", want: AsyncImageSize1024x1024Multiplier},
		{name: "1536x1024", size: "1536x1024", want: AsyncImageSize1536x1024Multiplier},
		{name: "1024x1536", size: "1024x1536", want: AsyncImageSize1024x1536Multiplier},
		{name: "1792x1024", size: "1792x1024", want: AsyncImageSize1792x1024Multiplier},
		{name: "1024x1792", size: "1024x1792", want: AsyncImageSize1024x1792Multiplier},
	}
	for _, tc := range sizeCases {
		t.Run("size/"+tc.name, func(t *testing.T) {
			assertFloatEqual(t, tc.want, GetAsyncImageSizeMultiplier(tc.size))
		})
	}

	countCases := []struct {
		name  string
		count int
		want  float64
	}{
		{name: "negative clamps to one", count: -1, want: 1},
		{name: "zero defaults to one", count: 0, want: 1},
		{name: "one", count: 1, want: 1},
		{name: "multiple images", count: 3, want: 3},
	}
	for _, tc := range countCases {
		t.Run("count/"+tc.name, func(t *testing.T) {
			assertFloatEqual(t, tc.want, GetAsyncImageCountMultiplier(tc.count))
		})
	}

	got := GetAsyncImageSpecMultiplier("high", "1792x1024", 2)
	want := AsyncImageQualityHighMultiplier * AsyncImageSize1792x1024Multiplier * 2
	assertFloatEqual(t, want, got)
}

func TestAsyncVideoSpecPricingMultipliersCoverPlaceholderTiers(t *testing.T) {
	t.Parallel()

	resolutionCases := []struct {
		name       string
		resolution string
		want       float64
	}{
		{name: "empty defaults to 480p", resolution: "", want: AsyncVideoResolution480PMultiplier},
		{name: "480p", resolution: "480p", want: AsyncVideoResolution480PMultiplier},
		{name: "720p", resolution: "720p", want: AsyncVideoResolution720PMultiplier},
		{name: "1080p", resolution: "1080p", want: AsyncVideoResolution1080PMultiplier},
	}
	for _, tc := range resolutionCases {
		t.Run("resolution/"+tc.name, func(t *testing.T) {
			assertFloatEqual(t, tc.want, GetAsyncVideoResolutionMultiplier(tc.resolution))
		})
	}

	durationCases := []struct {
		name    string
		seconds int
		want    float64
	}{
		{name: "negative defaults to base", seconds: -1, want: 1},
		{name: "zero defaults to base", seconds: 0, want: 1},
		{name: "below base clamps to one", seconds: AsyncVideoBaseSeconds - 1, want: 1},
		{name: "base", seconds: AsyncVideoBaseSeconds, want: 1},
		{name: "double base", seconds: AsyncVideoBaseSeconds * 2, want: 2},
	}
	for _, tc := range durationCases {
		t.Run("duration/"+tc.name, func(t *testing.T) {
			assertFloatEqual(t, tc.want, GetAsyncVideoDurationMultiplier(tc.seconds))
		})
	}

	got := GetAsyncVideoSpecMultiplier("1080p", AsyncVideoBaseSeconds*2)
	want := AsyncVideoResolution1080PMultiplier * 2
	assertFloatEqual(t, want, got)
}

func assertFloatEqual(t *testing.T, want, got float64) {
	t.Helper()
	if math.Abs(want-got) > 0.000001 {
		t.Fatalf("want %.6f, got %.6f", want, got)
	}
}
