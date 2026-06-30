package operation_setting

import "testing"

func TestAsyncImageSpecPricingUsesFreePlaceholderQuota(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		quality string
		size    string
		count   int
	}{
		{name: "standard square", quality: "standard", size: "1024x1024", count: 1},
		{name: "high large multiple", quality: "high", size: "1792x1024", count: 4},
		{name: "empty defaults", quality: "", size: "", count: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetAsyncImageSpecQuota(tc.quality, tc.size, tc.count); got != AsyncTaskSpecPricingPlaceholderQuota {
				t.Fatalf("want free placeholder quota %d, got %d", AsyncTaskSpecPricingPlaceholderQuota, got)
			}
		})
	}
}

func TestAsyncVideoSpecPricingUsesFreePlaceholderQuota(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		resolution string
		seconds    int
	}{
		{name: "base", resolution: "480p", seconds: 4},
		{name: "high long", resolution: "1080p", seconds: 12},
		{name: "empty defaults", resolution: "", seconds: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetAsyncVideoSpecQuota(tc.resolution, tc.seconds); got != AsyncTaskSpecPricingPlaceholderQuota {
				t.Fatalf("want free placeholder quota %d, got %d", AsyncTaskSpecPricingPlaceholderQuota, got)
			}
		})
	}
}
