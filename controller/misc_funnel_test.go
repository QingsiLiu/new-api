package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetStatusFunnelConfigFailsClosedOnlyWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestStatus := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
		GetStatus(ctx)
		return recorder
	}

	t.Setenv("GEILI_FUNNEL_ENABLED", "false")
	t.Setenv("GEILI_FUNNEL_SECRET", "")
	require.Equal(t, http.StatusOK, requestStatus().Code)

	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", "short")
	invalid := requestStatus()
	require.Equal(t, http.StatusServiceUnavailable, invalid.Code)
	require.JSONEq(t, `{"success":false,"message":"service configuration unavailable"}`, invalid.Body.String())
	require.NotContains(t, invalid.Body.String(), "short")

	t.Setenv("GEILI_FUNNEL_SECRET", strings.Repeat("s", 32))
	require.Equal(t, http.StatusOK, requestStatus().Code)
}
