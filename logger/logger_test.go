package logger

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestQuotaFormattingIsCNYNativeRegardlessOfLegacyDisplaySettings(t *testing.T) {
	originalDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalExchangeRate := operation_setting.USDExchangeRate
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
		operation_setting.USDExchangeRate = originalExchangeRate
	})

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	operation_setting.USDExchangeRate = 7.2
	if got := FormatQuota(11000); got != "¥0.110000" {
		t.Fatalf("FormatQuota with legacy USD display = %q, want ¥0.110000", got)
	}

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	if got := LogQuota(242550); got != "¥2.425500" {
		t.Fatalf("LogQuota with legacy token display = %q, want ¥2.425500", got)
	}
}
