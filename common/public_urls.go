package common

import (
	"net/url"
	"os"
	"strings"
)

const (
	defaultPortalURL       = "https://auapi.ai"
	defaultPublicAPIURL    = "https://api.auapi.ai"
	portalURLEnv           = "GEILI_PORTAL_URL"
	publicAPIURLEnv        = "GEILI_PUBLIC_API_URL"
	oauthRedirectBaseEnv   = "GEILI_OAUTH_REDIRECT_BASE_URL"
	paymentReturnBaseEnv   = "GEILI_PAYMENT_RETURN_BASE_URL"
	paymentCallbackBaseEnv = "GEILI_PAYMENT_CALLBACK_URL"
)

func configuredPublicURL(envName string, fallback string) string {
	raw := strings.TrimRight(strings.TrimSpace(os.Getenv(envName)), "/")
	if raw == "" {
		raw = fallback
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return fallback
	}
	return raw
}

// PortalURL is the customer-facing AUAPI web origin.
func PortalURL() string {
	return configuredPublicURL(portalURLEnv, defaultPortalURL)
}

// PublicAPIURL is the canonical machine API origin.
func PublicAPIURL() string {
	return configuredPublicURL(publicAPIURLEnv, defaultPublicAPIURL)
}

// OAuthRedirectBaseURL keeps browser OAuth callbacks on the portal origin even
// when ServerAddress is the machine API origin used for task/result URLs.
func OAuthRedirectBaseURL() string {
	return configuredPublicURL(oauthRedirectBaseEnv, PortalURL())
}

func PaymentReturnBaseURL() string {
	return configuredPublicURL(paymentReturnBaseEnv, PortalURL())
}

func PaymentCallbackBaseURL() string {
	return configuredPublicURL(paymentCallbackBaseEnv, PublicAPIURL())
}
