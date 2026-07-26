package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func publicCreditPackagesResponse(t *testing.T) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/credits/packages", nil)
	GetPublicCreditPackages(ctx)
	var response struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return recorder, response.Data
}

func TestPublicCreditPackagesFollowFeatureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(common.CreditsFeatureFlagEnv, "false")
	recorder, disabled := publicCreditPackagesResponse(t)
	require.Equal(t, "public, max-age=300", recorder.Header().Get("Cache-Control"))
	require.Equal(t, false, disabled["credits_enabled"])
	require.EqualValues(t, common.CreditsQuotaUnit, disabled["quota_per_credit"])
	require.Empty(t, disabled["packages"])

	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	_, enabled := publicCreditPackagesResponse(t)
	require.Equal(t, true, enabled["credits_enabled"])
	packages := enabled["packages"].([]any)
	require.Len(t, packages, 10)
	first := packages[0].(map[string]any)
	require.Equal(t, "credits-usd-1000", first["package_id"])
	require.Equal(t, "1000", first["credits"])
	require.Equal(t, "1000", first["base_credits"])
	require.Equal(t, "0", first["bonus_credits"])
	require.EqualValues(t, 3_600_000, first["quota"])
	require.Equal(t, "USD", first["currency"])
	require.EqualValues(t, 500, first["price_minor"])
}

func TestPublicCreditPackagesExposeOnlyPriceSnapshotFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	_, data := publicCreditPackagesResponse(t)
	packages := data["packages"].([]any)
	allowed := map[string]bool{
		"package_id": true, "credits": true, "base_credits": true,
		"bonus_credits": true, "quota": true, "currency": true, "price_minor": true,
	}
	for _, value := range packages {
		for key := range value.(map[string]any) {
			require.True(t, allowed[key], "public package leaked field %q", key)
		}
	}
	serialized, err := common.Marshal(data)
	require.NoError(t, err)
	lower := strings.ToLower(string(serialized))
	for _, forbidden := range []string{
		"payment_method", "provider", "merchant", "waffo", "alipay",
		"checkout_url", "secret", "token", "environment",
	} {
		require.NotContains(t, lower, forbidden)
	}
}
