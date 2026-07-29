package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserLogEndpointsReturnCreditsProjectionWhenEnabled(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	db := setupUserControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	require.NoError(t, db.Create(&model.Log{
		UserId:       42,
		Username:     "credits-log-user",
		Type:         model.LogTypeConsume,
		ModelName:    "gpt-5.5",
		TokenName:    "production",
		Quota:        11000,
		PromptTokens: 10,
		CreatedAt:    100,
	}).Error)

	logRecorder := httptest.NewRecorder()
	logContext, _ := gin.CreateTestContext(logRecorder)
	logContext.Request = httptest.NewRequest(http.MethodGet, "/api/log/self?p=1&page_size=5&type=2", nil)
	logContext.Set("id", 42)
	GetUserLogs(logContext)

	require.Equal(t, http.StatusOK, logRecorder.Code, logRecorder.Body.String())
	require.Contains(t, logRecorder.Body.String(), `"credits":"3.055556"`)
	require.Contains(t, logRecorder.Body.String(), `"quota":11000`)

	statRecorder := httptest.NewRecorder()
	statContext, _ := gin.CreateTestContext(statRecorder)
	statContext.Request = httptest.NewRequest(http.MethodGet, "/api/log/self/stat?type=2", nil)
	statContext.Set("username", "credits-log-user")
	GetLogsSelfStat(statContext)

	require.Equal(t, http.StatusOK, statRecorder.Code, statRecorder.Body.String())
	require.Contains(t, statRecorder.Body.String(), `"credits":"3.055556"`)
	require.Contains(t, statRecorder.Body.String(), `"quota":11000`)
}

func TestTokenCreditsProjectionFollowsFeatureFlag(t *testing.T) {
	token := &model.Token{
		Id:          7,
		Key:         "sensitive-token-value",
		Name:        "production",
		RemainQuota: 23400,
		UsedQuota:   11000,
	}

	t.Run("disabled", func(t *testing.T) {
		t.Setenv(common.CreditsFeatureFlagEnv, "false")
		response := buildMaskedTokenResponse(token)
		require.NotNil(t, response)
		require.Nil(t, response.RemainCredits)
		require.Nil(t, response.UsedCredits)
		require.NotEqual(t, token.Key, response.Key)
		encoded, err := common.Marshal(response)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), `"remain_credits"`)
		require.NotContains(t, string(encoded), `"used_credits"`)
	})

	t.Run("enabled", func(t *testing.T) {
		t.Setenv(common.CreditsFeatureFlagEnv, "true")
		response := buildMaskedTokenResponse(token)
		require.NotNil(t, response)
		require.NotNil(t, response.RemainCredits)
		require.NotNil(t, response.UsedCredits)
		require.Equal(t, "6.5", *response.RemainCredits)
		require.Equal(t, "3.055556", *response.UsedCredits)
	})
}

func TestProductBillingCreditsProjectionUsesCreditsCurrency(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")

	estimate := asyncTaskPricingEstimateResponse{
		Quota:    23400,
		Currency: "CNY",
		Unit:     "CNY",
	}
	applyCreditsV1PricingEstimateProjection(&estimate)
	require.Equal(t, "CREDITS", estimate.Currency)
	require.Equal(t, "CREDITS", estimate.Unit)
	require.NotNil(t, estimate.Credits)
	require.Equal(t, "6.5", *estimate.Credits)
	require.NotNil(t, estimate.PublicQuota)
	require.Equal(t, 23400, *estimate.PublicQuota)

	balance := asyncBillingBalanceResponse{Currency: "CNY"}
	applyCreditsV1BillingBalanceProjection(&balance, 45000, 11000)
	require.Equal(t, "CREDITS", balance.Currency)
	require.NotNil(t, balance.BalanceCredits)
	require.Equal(t, "12.5", *balance.BalanceCredits)
	require.NotNil(t, balance.UsedCredits)
	require.Equal(t, "3.055556", *balance.UsedCredits)
	require.NotNil(t, balance.QuotaPerCredit)
	require.Equal(t, common.CreditsQuotaUnit, *balance.QuotaPerCredit)
}
