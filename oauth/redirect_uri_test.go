package oauth

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCallbackURLForRequestPreservesLegacyAndAUAPIHosts(t *testing.T) {
	for _, tc := range []struct {
		host string
		want string
	}{
		{host: "geiliapi.com", want: "https://geiliapi.com/oauth/oidc"},
		{host: "auapi.ai", want: "https://auapi.ai/oauth/oidc"},
		{host: "admin.auapi.ai", want: "https://admin.auapi.ai/oauth/oidc"},
		{host: "studio.geiliapi.com", want: "https://studio.geiliapi.com/oauth/oidc"},
	} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest("GET", "https://"+tc.host+"/oauth/oidc", nil)
		if got := CallbackURLForRequest(ctx, "/oauth/oidc", "https://fallback.example"); got != tc.want {
			t.Fatalf("host=%s got=%q want=%q", tc.host, got, tc.want)
		}
	}
}

func TestCallbackURLForRequestRejectsUntrustedHost(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "https://evil.example/oauth/oidc", nil)
	if got := CallbackURLForRequest(ctx, "/oauth/oidc", "https://auapi.ai"); got != "https://auapi.ai/oauth/oidc" {
		t.Fatalf("untrusted host selected callback %q", got)
	}
}
