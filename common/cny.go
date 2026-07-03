package common

import "github.com/shopspring/decimal"

const (
	CNYQuotaUnitInt64 = int64(100000)
	CNYQuotaUnit      = float64(CNYQuotaUnitInt64)
)

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
