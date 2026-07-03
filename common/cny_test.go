package common

import "testing"

func TestCNYQuotaConversionsUse100KUnit(t *testing.T) {
	if got := QuotaToCNY(12499336807); got != 124993.36807 {
		t.Fatalf("QuotaToCNY(12499336807) = %f, want 124993.36807", got)
	}
	if got := QuotaToPublicCNY(12499336807); got != 124993.3681 {
		t.Fatalf("QuotaToPublicCNY(12499336807) = %f, want 124993.3681", got)
	}
}
