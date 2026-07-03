package common

import "math"

const (
	CNYQuotaUnit                  = 100000.0
	LegacyUSDQuotaUnit            = 500000.0
	QuotaMigrationInProgressKey   = "QuotaMigrationInProgress"
	QuotaMigrationVersionKey      = "QuotaMigrationVersion"
	QuotaMigrationCNY100KVersion  = "cny100k_20260703"
	QuotaMigrationProgressEnabled = "true"
)

func CNYToQuota(cny float64) int {
	if cny <= 0 {
		return 0
	}
	return int(math.Round(cny * CNYQuotaUnit))
}

func QuotaToCNY(quota int) float64 {
	return float64(quota) / CNYQuotaUnit
}

func LegacyQuotaToCNY100KQuota(quota int64) int64 {
	if quota <= 0 {
		return 0
	}
	return int64(math.Floor(float64(quota) * CNYQuotaUnit / LegacyUSDQuotaUnit))
}
