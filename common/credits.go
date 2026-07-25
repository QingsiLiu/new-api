package common

import (
	"strings"

	"github.com/shopspring/decimal"
)

const (
	CreditsQuotaUnit      = 3600
	SignupCreditAmount    = int64(20)
	SignupCreditQuota     = int(SignupCreditAmount) * CreditsQuotaUnit
	CreditsFeatureFlagEnv = "GEILI_CREDITS_V1"
)

// CreditsV1Enabled is intentionally evaluated at request time so tests and
// rolling deployments can toggle the additive contract without process state.
func CreditsV1Enabled() bool {
	return GetEnvOrDefaultBool(CreditsFeatureFlagEnv, false)
}

// QuotaToCreditsString projects the integer ledger into a fixed-point public
// value. Quota remains the settlement truth; callers must keep it alongside
// this display projection when exact reconciliation matters.
func QuotaToCreditsString(quota int) string {
	value := decimal.NewFromInt(int64(quota)).
		Div(decimal.NewFromInt(CreditsQuotaUnit)).
		StringFixed(6)
	value = strings.TrimRight(value, "0")
	value = strings.TrimRight(value, ".")
	if value == "" || value == "-0" {
		return "0"
	}
	return value
}

func CreditsToQuota(credits int64) (int, bool) {
	if credits < 0 {
		return 0, false
	}
	maxInt := int64(^uint(0) >> 1)
	if credits > maxInt/int64(CreditsQuotaUnit) {
		return 0, false
	}
	return int(credits * int64(CreditsQuotaUnit)), true
}
