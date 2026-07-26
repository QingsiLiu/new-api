package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicCreditPackagesAreAnonymousButCheckoutRemainsAuthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	previousRateLimit := common.GlobalApiRateLimitEnable
	common.GlobalApiRateLimitEnable = false
	t.Cleanup(func() {
		common.GlobalApiRateLimitEnable = previousRateLimit
	})

	engine := gin.New()
	SetGeiliPublicModelRouter(engine)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/public/credits/packages", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	source, err := os.ReadFile("api-router.go")
	require.NoError(t, err)
	require.Contains(t, string(source), `selfRoute.POST("/checkout", middleware.CriticalRateLimit(), controller.RequestCreditsCheckout)`)
}

func TestAsyncTaskProductRoutesDisabledByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	operation_setting.AsyncTaskProductRoutesEnabled = false
	t.Cleanup(func() {
		operation_setting.AsyncTaskProductRoutesEnabled = false
	})

	engine := gin.New()
	SetAsyncTaskProductRouter(engine)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/images/tasks", strings.NewReader(`{}`))
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAsyncTaskProductRoutesEnabledRequiresTokenAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	operation_setting.AsyncTaskProductRoutesEnabled = true
	t.Cleanup(func() {
		operation_setting.AsyncTaskProductRoutesEnabled = false
	})

	engine := gin.New()
	SetAsyncTaskProductRouter(engine)

	for _, target := range []string{
		"/v1/images/tasks",
		"/v1/videos/tasks",
		"/v1/tasks/task-1",
		"/v1/tasks/task-1/content",
		"/v1/tasks/task-1/cancel",
		"/v1/pricing/estimate",
		"/v1/billing/balance",
		"/v1/billing/usage",
	} {
		t.Run(target, func(t *testing.T) {
			method := http.MethodGet
			if strings.HasSuffix(target, "/tasks") || strings.HasSuffix(target, "/cancel") || strings.HasSuffix(target, "/pricing/estimate") {
				method = http.MethodPost
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(method, target, strings.NewReader(`{}`))
			engine.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
		})
	}
}
