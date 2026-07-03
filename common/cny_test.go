package common

import "testing"

func TestCNYQuotaConversionsUse100KUnit(t *testing.T) {
	if got := CNYToQuota(0.11); got != 11000 {
		t.Fatalf("CNYToQuota(0.11) = %d, want 11000", got)
	}
	if got := CNYToQuota(0.2426 * 5); got != 121300 {
		t.Fatalf("CNYToQuota(0.2426*5) = %d, want 121300", got)
	}
	if got := QuotaToCNY(12499336807); got != 124993.36807 {
		t.Fatalf("QuotaToCNY(12499336807) = %f, want 124993.36807", got)
	}
	if got := QuotaToPublicCNY(12499336807); got != 124993.3681 {
		t.Fatalf("QuotaToPublicCNY(12499336807) = %f, want 124993.3681", got)
	}
}

func TestLegacyQuotaToCNY100KQuotaFloorsAtPointTwo(t *testing.T) {
	if got := LegacyQuotaToCNY100KQuota(500000); got != 100000 {
		t.Fatalf("LegacyQuotaToCNY100KQuota(500000) = %d, want 100000", got)
	}
	if got := LegacyQuotaToCNY100KQuota(62496684039); got != 12499336807 {
		t.Fatalf("LegacyQuotaToCNY100KQuota(62496684039) = %d, want 12499336807", got)
	}
}
