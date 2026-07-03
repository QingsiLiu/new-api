package common

import "github.com/shopspring/decimal"

const (
	CNYQuotaUnitInt64             = int64(100000)
	LegacyUSDQuotaUnitInt64       = int64(500000)
	CNYQuotaUnit                  = float64(CNYQuotaUnitInt64)
	LegacyUSDQuotaUnit            = float64(LegacyUSDQuotaUnitInt64)
	QuotaMigrationInProgressKey   = "QuotaMigrationInProgress"
	QuotaMigrationVersionKey      = "QuotaMigrationVersion"
	QuotaMigrationCNY100KVersion  = "cny100k_20260703"
	QuotaMigrationProgressEnabled = "true"
)

func CNYToQuota(cny float64) int {
	return CNYDecimalToQuota(decimal.NewFromFloat(cny))
}

func CNYDecimalToQuota(cny decimal.Decimal) int {
	if cny.LessThanOrEqual(decimal.Zero) {
		return 0
	}
	return int(cny.Mul(decimal.NewFromInt(CNYQuotaUnitInt64)).Round(0).IntPart())
}

func QuotaToCNY(quota int) float64 {
	return decimal.NewFromInt(int64(quota)).
		Div(decimal.NewFromInt(CNYQuotaUnitInt64)).
		InexactFloat64()
}

func QuotaToPublicCNY(quota int) float64 {
	return RoundPublicCNY(QuotaToCNY(quota))
}

func RoundPublicCNY(cny float64) float64 {
	return decimal.NewFromFloat(cny).Round(4).InexactFloat64()
}

func LegacyQuotaToCNY100KQuota(quota int64) int64 {
	if quota <= 0 {
		return 0
	}
	return quota / (LegacyUSDQuotaUnitInt64 / CNYQuotaUnitInt64)
}
