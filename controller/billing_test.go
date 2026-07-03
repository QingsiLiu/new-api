package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAICompatibleBillingUsesCNYUnitsWithoutExchangeRate(t *testing.T) {
	db := setupUserControllerTestDB(t)
	user := &model.User{
		Username:  "billing-cny-user",
		Password:  "hash",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Quota:     110000,
		UsedQuota: 242550,
		Group:     "default",
	}
	require.NoError(t, db.Create(user).Error)

	originalRate := operation_setting.USDExchangeRate
	originalDisplayTokenStat := common.DisplayTokenStatEnabled
	t.Cleanup(func() {
		operation_setting.USDExchangeRate = originalRate
		common.DisplayTokenStatEnabled = originalDisplayTokenStat
	})
	operation_setting.USDExchangeRate = 7
	common.DisplayTokenStatEnabled = false

	subRecorder := httptest.NewRecorder()
	subCtx, _ := gin.CreateTestContext(subRecorder)
	subCtx.Request = httptest.NewRequest(http.MethodGet, "/dashboard/billing/subscription", nil)
	subCtx.Set("id", user.Id)
	GetSubscription(subCtx)

	require.Equal(t, http.StatusOK, subRecorder.Code, subRecorder.Body.String())
	var subscription OpenAISubscriptionResponse
	require.NoError(t, common.Unmarshal(subRecorder.Body.Bytes(), &subscription))
	require.InDelta(t, 3.5255, subscription.SoftLimitUSD, 0.000001)
	require.InDelta(t, 3.5255, subscription.HardLimitUSD, 0.000001)

	usageRecorder := httptest.NewRecorder()
	usageCtx, _ := gin.CreateTestContext(usageRecorder)
	usageCtx.Request = httptest.NewRequest(http.MethodGet, "/dashboard/billing/usage", nil)
	usageCtx.Set("id", user.Id)
	GetUsage(usageCtx)

	require.Equal(t, http.StatusOK, usageRecorder.Code, usageRecorder.Body.String())
	var usage OpenAIUsageResponse
	require.NoError(t, common.Unmarshal(usageRecorder.Body.Bytes(), &usage))
	require.InDelta(t, 242.55, usage.TotalUsage, 0.000001)
}
