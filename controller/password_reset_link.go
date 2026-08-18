package controller

import (
	"net/url"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const geiliPasswordResetURLEnv = "GEILI_PASSWORD_RESET_URL"

func validPasswordResetURL(raw string) (*url.URL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, false
	}
	return parsed, true
}

func passwordResetLink(email string, token string) string {
	base, ok := validPasswordResetURL(os.Getenv(geiliPasswordResetURLEnv))
	if !ok {
		fallback := common.PortalURL() + "/user/reset"
		base, ok = validPasswordResetURL(fallback)
	}
	if !ok {
		base = &url.URL{Path: "/user/reset"}
	}

	query := base.Query()
	if passwordResetV2Enabled() {
		query.Del("email")
		query.Del("token")
		query.Del("ticket")
		base.RawQuery = query.Encode()
		base.Fragment = "ticket=" + token
		base.RawFragment = ""
	} else {
		query.Set("email", email)
		query.Set("token", token)
		base.RawQuery = query.Encode()
	}
	return base.String()
}
