package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStatusExposesPublicProviderURLsForMigrationVerification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oidc := system_setting.GetOIDCSettings()
	oldOIDCRedirect := oidc.RedirectUri
	oldWaffoReturn := setting.WaffoPancakeReturnURL
	t.Cleanup(func() {
		oidc.RedirectUri = oldOIDCRedirect
		setting.WaffoPancakeReturnURL = oldWaffoReturn
	})
	oidc.RedirectUri = "https://auapi.ai/oauth/oidc"
	setting.WaffoPancakeReturnURL = "https://auapi.ai/wallet?show_history=true"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	GetStatus(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "https://auapi.ai/oauth/oidc", response.Data["oidc_redirect_uri"])
	require.Equal(t, "https://auapi.ai/wallet?show_history=true", response.Data["waffo_pancake_return_url"])
}
