package router

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasswordResetRoutesKeepSecurityMiddlewareAndCompatibility(t *testing.T) {
	source, err := os.ReadFile("api-router.go")
	require.NoError(t, err)
	text := string(source)
	require.Contains(t, text, `apiRouter.POST("/reset_password", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.SendPasswordResetEmailPost)`)
	require.Contains(t, text, `apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)`)
	require.Contains(t, text, `apiRouter.POST("/user/reset/v2", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.ResetPasswordV2)`)
	require.Contains(t, text, "Deprecated compatibility route")
	require.False(t, strings.Contains(text, `apiRouter.POST("/reset_password", controller.`))
}
