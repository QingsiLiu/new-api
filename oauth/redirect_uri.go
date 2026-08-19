package oauth

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

var callbackHosts = map[string]struct{}{
	"auapi.ai":            {},
	"geiliapi.com":        {},
	"admin.auapi.ai":      {},
	"admin.geiliapi.com":  {},
	"studio.auapi.ai":     {},
	"studio.geiliapi.com": {},
	"localhost":           {},
}

func requestHost(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if comma := strings.IndexByte(host, ','); comma >= 0 {
		host = strings.TrimSpace(host[:comma])
	}
	host = strings.ToLower(host)
	if parsed, err := url.Parse("https://" + host); err == nil {
		host = parsed.Hostname()
	}
	if _, ok := callbackHosts[host]; !ok {
		return ""
	}
	return host
}

// CallbackURLForRequest binds the token-exchange redirect_uri to the host that
// received the provider callback. This keeps legacy and AUAPI callbacks valid
// during migration while rejecting arbitrary Host values. The configured
// origin remains the fallback for internal jobs and local test contexts.
func CallbackURLForRequest(c *gin.Context, path string, configured string) string {
	host := requestHost(c)
	if host != "" {
		scheme := "https"
		if c != nil && c.Request != nil {
			forwardedProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
			if comma := strings.IndexByte(forwardedProto, ','); comma >= 0 {
				forwardedProto = strings.TrimSpace(forwardedProto[:comma])
			}
			if forwardedProto == "http" || forwardedProto == "https" {
				scheme = forwardedProto
			} else if host == "localhost" && c.Request.TLS == nil {
				scheme = "http"
			}
		}
		return fmt.Sprintf("%s://%s%s", scheme, host, path)
	}
	if configured = strings.TrimSpace(configured); configured != "" {
		// The OIDC setting UI documents this field as an exact redirect URI,
		// while older deployments stored only an origin. Preserve both forms:
		// a configured path is already complete; an origin still receives the
		// provider callback path. This avoids producing /oauth/oidc/oauth/oidc
		// from the legacy production value during the AUAPI migration.
		if parsed, err := url.Parse(configured); err == nil && parsed.Host != "" && parsed.Path != "" && parsed.Path != "/" {
			return configured
		}
		return strings.TrimRight(configured, "/") + path
	}
	return strings.TrimRight(common.OAuthRedirectBaseURL(), "/") + path
}
