package operation_setting

var (
	AsyncTaskSpecPricingEnabled      = false
	AsyncTaskProductRoutesEnabled    = false
	AsyncTaskServiceUserProxyEnabled = false
)

// TODO(D1): replace this free placeholder with the approved quota table.
// Keep image/video spec pricing funneled through this single constant and the
// two resolver functions below so D1 can be swapped in one place.
const AsyncTaskSpecPricingPlaceholderQuota = 0

func GetAsyncImageSpecQuota(_ string, _ string, _ int) int {
	return AsyncTaskSpecPricingPlaceholderQuota
}

func GetAsyncVideoSpecQuota(_ string, _ int) int {
	return AsyncTaskSpecPricingPlaceholderQuota
}
