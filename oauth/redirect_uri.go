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
	host := strings.ToLower(strings.TrimSpace(c.Request.Host))
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
		if host == "localhost" && c != nil && c.Request != nil && c.Request.TLS == nil {
			scheme = "http"
		}
		return fmt.Sprintf("%s://%s%s", scheme, host, path)
	}
	if strings.TrimSpace(configured) != "" {
		return strings.TrimRight(strings.TrimSpace(configured), "/") + path
	}
	return strings.TrimRight(common.OAuthRedirectBaseURL(), "/") + path
}
