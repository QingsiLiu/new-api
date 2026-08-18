package common

import "testing"

func TestPublicURLDefaults(t *testing.T) {
	t.Setenv(portalURLEnv, "")
	t.Setenv(publicAPIURLEnv, "")
	t.Setenv(oauthRedirectBaseEnv, "")
	t.Setenv(paymentReturnBaseEnv, "")
	t.Setenv(paymentCallbackBaseEnv, "")
	if got := PortalURL(); got != "https://auapi.ai" {
		t.Fatalf("PortalURL() = %q", got)
	}
	if got := PublicAPIURL(); got != "https://api.auapi.ai" {
		t.Fatalf("PublicAPIURL() = %q", got)
	}
	if got := OAuthRedirectBaseURL(); got != "https://auapi.ai" {
		t.Fatalf("OAuthRedirectBaseURL() = %q", got)
	}
	if got := PaymentReturnBaseURL(); got != "https://auapi.ai" {
		t.Fatalf("PaymentReturnBaseURL() = %q", got)
	}
	if got := PaymentCallbackBaseURL(); got != "https://api.auapi.ai" {
		t.Fatalf("PaymentCallbackBaseURL() = %q", got)
	}
}

func TestPublicURLRejectsInvalidOverrides(t *testing.T) {
	t.Setenv(portalURLEnv, "javascript:alert(1)")
	t.Setenv(publicAPIURLEnv, "not a url")
	if got := PortalURL(); got != "https://auapi.ai" {
		t.Fatalf("invalid portal override = %q", got)
	}
	if got := PublicAPIURL(); got != "https://api.auapi.ai" {
		t.Fatalf("invalid API override = %q", got)
	}
}

func TestPublicURLRejectsNonOriginOverrides(t *testing.T) {
	for _, value := range []string{
		"https://auapi.ai/path",
		"https://auapi.ai/?next=/console",
		"https://user:pass@auapi.ai",
		"https://auapi.ai#fragment",
	} {
		t.Setenv(portalURLEnv, value)
		if got := PortalURL(); got != "https://auapi.ai" {
			t.Fatalf("PortalURL(%q) = %q", value, got)
		}
	}
	t.Setenv(portalURLEnv, "https://auapi.ai/")
	if got := PortalURL(); got != "https://auapi.ai" {
		t.Fatalf("trailing slash should normalize, got %q", got)
	}
}
