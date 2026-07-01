package operation_setting

import "testing"

func resetAsyncSpecPricingForTest(t *testing.T) {
	t.Helper()
	previousPricing := AsyncSpecPricing2JSONString()
	previousQuotaPerCNY := QuotaPerCNY
	t.Cleanup(func() {
		if err := UpdateAsyncSpecPricingByJSONString(previousPricing); err != nil {
			t.Fatalf("restore async spec pricing: %v", err)
		}
		QuotaPerCNY = previousQuotaPerCNY
	})
}

func TestResolveVideoSpecQuotaUsesResolutionSecondsAndCNYConversion(t *testing.T) {
	resetAsyncSpecPricingForTest(t)
	QuotaPerCNY = 1000
	if err := UpdateAsyncSpecPricingByJSONString(`{
		"currency":"CNY",
		"video":{
			"seedance-2.0-fast":{
				"unit":"per_second",
				"resolutions":{
					"720p":{"cny_per_second":0.2},
					"1080p":{"cny_per_second":0.35}
				},
				"default_cny_per_second":0.1
			}
		}
	}`); err != nil {
		t.Fatalf("update async spec pricing: %v", err)
	}

	result := ResolveVideoSpecQuota("seedance-2.0-fast", "1920x1080", 8)

	if !result.Matched {
		t.Fatalf("expected spec pricing match")
	}
	if result.Quota != 2800 {
		t.Fatalf("want quota 2800, got %d", result.Quota)
	}
	if result.SpecKey != "1080p" {
		t.Fatalf("want spec key 1080p, got %q", result.SpecKey)
	}
	if result.TotalCNY != 2.8 {
		t.Fatalf("want total CNY 2.8, got %f", result.TotalCNY)
	}
}

func TestResolveVideoSpecQuotaAppliesDefaultMinMaxAndAllowsZeroPrice(t *testing.T) {
	resetAsyncSpecPricingForTest(t)
	QuotaPerCNY = 100
	if err := UpdateAsyncSpecPricingByJSONString(`{
		"video":{
			"video-free":{"resolutions":{"720p":{"cny_per_second":0}}},
			"video-bounded":{
				"default_cny_per_second":0.5,
				"min_cny":2,
				"max_cny":3
			}
		}
	}`); err != nil {
		t.Fatalf("update async spec pricing: %v", err)
	}

	free := ResolveVideoSpecQuota("video-free", "1280x720", 12)
	if !free.Matched || free.Quota != 0 || free.SpecKey != "720p" {
		t.Fatalf("free spec mismatch: %+v", free)
	}

	minBound := ResolveVideoSpecQuota("video-bounded", "unlisted", 1)
	if !minBound.Matched || minBound.Quota != 200 || minBound.SpecKey != "default" {
		t.Fatalf("min bound mismatch: %+v", minBound)
	}

	maxBound := ResolveVideoSpecQuota("video-bounded", "unlisted", 20)
	if !maxBound.Matched || maxBound.Quota != 300 || maxBound.SpecKey != "default" {
		t.Fatalf("max bound mismatch: %+v", maxBound)
	}
}

func TestResolveImageSpecQuotaUsesResolutionCountAndAliases(t *testing.T) {
	resetAsyncSpecPricingForTest(t)
	QuotaPerCNY = 1000
	if err := UpdateAsyncSpecPricingByJSONString(`{
		"image":{
			"gpt-image-2":{
				"unit":"per_image",
				"resolutions":{
					"1k":{"cny_per_image":0.11},
					"2k":{"cny_per_image":0.18},
					"4k":{"cny_per_image":0.29}
				},
				"default_cny_per_image":0.11
			}
		}
	}`); err != nil {
		t.Fatalf("update async spec pricing: %v", err)
	}

	result := ResolveImageSpecQuota("gpt-image-2", "2048x2048", "", "hd", 2)

	if !result.Matched {
		t.Fatalf("expected spec pricing match")
	}
	if result.Quota != 360 {
		t.Fatalf("want quota 360, got %d", result.Quota)
	}
	if result.SpecKey != "2k" {
		t.Fatalf("want spec key 2k, got %q", result.SpecKey)
	}
}

func TestResolveImageSpecQuotaNormalizesResolutionCandidates(t *testing.T) {
	resetAsyncSpecPricingForTest(t)
	QuotaPerCNY = 1000
	if err := UpdateAsyncSpecPricingByJSONString(`{
		"image":{
			"gemini-3-pro-image-preview":{
				"resolutions":{
					"1K":{"cny_per_image":0.32},
					"2K":{"cny_per_image":0.32},
					"4K":{"cny_per_image":0.49}
				},
				"default_cny_per_image":0.32
			}
		}
	}`); err != nil {
		t.Fatalf("update async spec pricing: %v", err)
	}

	tests := []struct {
		name       string
		size       string
		resolution string
		wantKey    string
		wantQuota  int
	}{
		{name: "explicit-resolution", resolution: "4K", wantKey: "4k", wantQuota: 490},
		{name: "numeric-resolution", resolution: "2048", wantKey: "2k", wantQuota: 320},
		{name: "size-max-dimension-1k", size: "768x1024", wantKey: "1k", wantQuota: 320},
		{name: "size-max-dimension-2k", size: "1024x2048", wantKey: "2k", wantQuota: 320},
		{name: "size-max-dimension-4k", size: "4096x2048", wantKey: "4k", wantQuota: 490},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveImageSpecQuota("gemini-3-pro-image-preview", tt.size, tt.resolution, "", 1)
			if !result.Matched || result.SpecKey != tt.wantKey || result.Quota != tt.wantQuota {
				t.Fatalf("resolution mismatch: got %+v, want key %s quota %d", result, tt.wantKey, tt.wantQuota)
			}
		})
	}
}

func TestResolveImageSpecQuotaFallsBackFromResolutionToQuality(t *testing.T) {
	resetAsyncSpecPricingForTest(t)
	QuotaPerCNY = 1000
	if err := UpdateAsyncSpecPricingByJSONString(`{
		"image":{
			"legacy-image-model":{
				"qualities":{
					"high":{"cny_per_image":0.3}
				},
				"default_cny_per_image":0.2
			}
		}
	}`); err != nil {
		t.Fatalf("update async spec pricing: %v", err)
	}

	result := ResolveImageSpecQuota("legacy-image-model", "bad-size", "", "hd", 2)

	if !result.Matched || result.SpecKey != "high" || result.Quota != 600 {
		t.Fatalf("quality fallback mismatch: %+v", result)
	}
}

func TestResolveImageSpecQuotaDefaultsCountAndFallsBackWhenUnconfigured(t *testing.T) {
	resetAsyncSpecPricingForTest(t)
	QuotaPerCNY = 100
	if err := UpdateAsyncSpecPricingByJSONString(`{
		"image":{
			"image-model":{
				"default_cny_per_image":0.25
			}
		}
	}`); err != nil {
		t.Fatalf("update async spec pricing: %v", err)
	}

	result := ResolveImageSpecQuota("image-model", "unknown", "", "", 0)
	if !result.Matched || result.Quota != 25 || result.SpecKey != "default" {
		t.Fatalf("default image mismatch: %+v", result)
	}

	unconfigured := ResolveImageSpecQuota("missing-model", "1024x1024", "", "high", 1)
	if unconfigured.Matched || unconfigured.Quota != 0 {
		t.Fatalf("expected per-model fallback for unconfigured model, got %+v", unconfigured)
	}
}

func TestAsyncSpecPricingBadJSONClearsToSafeFallback(t *testing.T) {
	resetAsyncSpecPricingForTest(t)
	if err := UpdateAsyncSpecPricingByJSONString(`{"video":{"seedance-2.0":{"default_cny_per_second":1}}}`); err != nil {
		t.Fatalf("update async spec pricing: %v", err)
	}
	if result := ResolveVideoSpecQuota("seedance-2.0", "720p", 1); !result.Matched {
		t.Fatalf("expected initial spec match")
	}

	if err := UpdateAsyncSpecPricingByJSONString(`{bad-json`); err == nil {
		t.Fatalf("expected bad JSON error")
	}
	if result := ResolveVideoSpecQuota("seedance-2.0", "720p", 1); result.Matched {
		t.Fatalf("bad JSON should fall back safely, got %+v", result)
	}
}

func TestResolveSpecQuotaUsesDefaultQuotaPerCNYWhenConfiguredRateInvalid(t *testing.T) {
	resetAsyncSpecPricingForTest(t)
	QuotaPerCNY = 0
	if err := UpdateAsyncSpecPricingByJSONString(`{
		"image":{
			"gpt-image-2":{
				"qualities":{"high":{"cny_per_image":0.72}}
			}
		}
	}`); err != nil {
		t.Fatalf("update async spec pricing: %v", err)
	}

	result := ResolveImageSpecQuota("gpt-image-2", "", "", "high", 1)

	if !result.Matched {
		t.Fatalf("expected spec pricing match")
	}
	if result.Quota != 50000 {
		t.Fatalf("want quota 50000 from default QuotaPerCNY fallback, got %d", result.Quota)
	}
}

func TestAsyncSpecPricingCoexistsWithGPTImage1NativePriceTable(t *testing.T) {
	resetAsyncSpecPricingForTest(t)
	QuotaPerCNY = 1000
	nativePrice := GetGPTImage1PriceOnceCall("high", "1024x1024")
	if nativePrice <= 0 {
		t.Fatalf("expected native gpt-image-1 price table to remain available")
	}
	if err := UpdateAsyncSpecPricingByJSONString(`{
		"image":{
			"gpt-image-1":{
				"qualities":{"high":{"cny_per_image":0.5}}
			}
		}
	}`); err != nil {
		t.Fatalf("update async spec pricing: %v", err)
	}

	spec := ResolveImageSpecQuota("gpt-image-1", "", "", "high", 2)
	unchangedNativePrice := GetGPTImage1PriceOnceCall("high", "1024x1024")

	if !spec.Matched || spec.Quota != 1000 {
		t.Fatalf("expected configured async spec price to match, got %+v", spec)
	}
	if unchangedNativePrice != nativePrice {
		t.Fatalf("native gpt-image-1 price changed from %f to %f", nativePrice, unchangedNativePrice)
	}
	if fallback := ResolveImageSpecQuota("gpt-image-unconfigured", "", "", "high", 1); fallback.Matched {
		t.Fatalf("unconfigured models should keep falling back to native/per-model paths, got %+v", fallback)
	}
}
