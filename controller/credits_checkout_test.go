package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCreditsPriceMinorFormattingAndParsing(t *testing.T) {
	require.Equal(t, "5.00", formatPriceMinor(500))
	require.Equal(t, "1250.00", formatPriceMinor(125000))

	for input, expected := range map[string]int64{
		"5":       500,
		"5.0":     500,
		"5.00":    500,
		"1250.00": 125000,
	} {
		actual, err := parsePriceMinor(input)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	for _, input := range []string{"", "-1", "5.001", "five", "92233720368547758.08"} {
		_, err := parsePriceMinor(input)
		require.Error(t, err)
	}
}

func TestStatusExposesCreditsContract(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	GetStatus(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, true, response.Data["credits_v1_enabled"])
	require.EqualValues(t, common.CreditsQuotaUnit, response.Data["quota_per_credit"])
}
