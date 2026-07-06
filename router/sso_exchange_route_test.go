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
