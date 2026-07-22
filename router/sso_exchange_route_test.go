package router

import (
	"os"
	"strings"
	"testing"
)

func TestSSOExchangeRouteUsesAdminAuthNotAPIKeyTokenAuth(t *testing.T) {
	source, err := os.ReadFile("api-router.go")
	if err != nil {
		t.Fatalf("failed to read api-router.go: %v", err)
	}
	text := string(source)
	if strings.Contains(text, `userRoute.POST("/sso/exchange", middleware.TokenAuth(), controller.SSOExchange)`) {
		t.Fatal("SSO exchange must use admin service auth, not API-key TokenAuth")
	}
	if !strings.Contains(text, `userRoute.POST("/sso/exchange", middleware.AdminAuth(), controller.SSOExchange)`) {
		t.Fatal("SSO exchange route must be wired through AdminAuth")
	}
}

func TestSSOV2RoutesKeepOptionalUserIssueAndAdminExchangeBoundaries(t *testing.T) {
	source, err := os.ReadFile("api-router.go")
	if err != nil {
		t.Fatalf("failed to read api-router.go: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, `userRoute.GET("/sso/v2", middleware.TryUserAuth(), controller.SSORedirectV2)`) {
		t.Fatal("SSO v2 issue route must use the browser session boundary")
	}
	if !strings.Contains(text, `userRoute.POST("/sso/v2/exchange", middleware.AdminAuth(), controller.SSOExchangeV2)`) {
		t.Fatal("SSO v2 exchange route must use AdminAuth")
	}
	if strings.Contains(text, `userRoute.POST("/sso/v2/exchange", middleware.TokenAuth()`) {
		t.Fatal("SSO v2 exchange must not accept user API keys")
	}
}
