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

func TestSettledUserUsageExcludesFullyRefundedTaskReservations(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	db := setupUserControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.Task{}, &model.Token{}))

	user := &model.User{
		Username: "settled-usage-user",
		Password: "hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(user).Error)
	token := &model.Token{UserId: user.Id, Name: "production", Key: "test-token"}
	require.NoError(t, db.Create(token).Error)

	require.NoError(t, db.Create(&model.Log{
		UserId:    user.Id,
		Username:  user.Username,
		CreatedAt: 100,
		Type:      model.LogTypeConsume,
		ModelName: "gpt-5.5",
		TokenName: token.Name,
		Quota:     3_600,
		RequestId: "req_sync",
		Other:     "{}",
	}).Error)

	success := &model.Task{
		TaskID:     "task_success",
		UserId:     user.Id,
		Group:      "default",
		Quota:      10_800,
		Status:     model.TaskStatusSuccess,
		SubmitTime: 200,
		CreatedAt:  200,
		Properties: model.Properties{OriginModelName: "gemini-2.5-flash-image"},
		PrivateData: model.TaskPrivateData{
			TokenId: token.Id,
		},
	}
	failed := &model.Task{
		TaskID:     "task_failed",
		UserId:     user.Id,
		Group:      "default",
		Quota:      10_800,
		Status:     model.TaskStatusFailure,
		SubmitTime: 300,
		CreatedAt:  300,
		Properties: model.Properties{OriginModelName: "gpt-image-2"},
		PrivateData: model.TaskPrivateData{
			TokenId: token.Id,
		},
	}
	failedWithoutRefund := &model.Task{
		TaskID:     "task_failed_without_refund",
		UserId:     user.Id,
		Group:      "default",
		Quota:      7_200,
		Status:     model.TaskStatusFailure,
		SubmitTime: 400,
		CreatedAt:  400,
		Properties: model.Properties{OriginModelName: "gpt-image-2"},
		PrivateData: model.TaskPrivateData{
			TokenId: token.Id,
		},
	}
	pending := &model.Task{
		TaskID:     "task_pending",
		UserId:     user.Id,
		Group:      "default",
		Quota:      3_600,
		Status:     model.TaskStatusInProgress,
		SubmitTime: 500,
		CreatedAt:  500,
		Properties: model.Properties{OriginModelName: "gpt-image-2"},
		PrivateData: model.TaskPrivateData{
			TokenId: token.Id,
		},
	}
	require.NoError(t, db.Create(success).Error)
	require.NoError(t, db.Create(failed).Error)
	require.NoError(t, db.Create(failedWithoutRefund).Error)
	require.NoError(t, db.Create(pending).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:    user.Id,
		Username:  user.Username,
		CreatedAt: 200,
		Type:      model.LogTypeConsume,
		ModelName: "gemini-2.5-flash-image",
		TokenName: token.Name,
		Quota:     10_800,
		RequestId: "req_success_reservation",
		Other:     `{"is_task":true,"task_id":"task_success"}`,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:    user.Id,
		Username:  user.Username,
		CreatedAt: 300,
		Type:      model.LogTypeConsume,
		ModelName: "gpt-image-2",
		TokenName: token.Name,
		Quota:     10_800,
		RequestId: "req_failed_reservation",
		Other:     `{"is_task":true,"task_id":"task_failed"}`,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:    user.Id,
		Username:  user.Username,
		CreatedAt: 301,
		Type:      model.LogTypeRefund,
		ModelName: "gpt-image-2",
		TokenName: token.Name,
		Quota:     10_800,
		RequestId: "req_failed_refund",
		Other:     `{"task_id":"task_failed"}`,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:    user.Id,
		Username:  user.Username,
		CreatedAt: 400,
		Type:      model.LogTypeConsume,
		ModelName: "gpt-image-2",
		TokenName: token.Name,
		Quota:     7_200,
		RequestId: "req_failed_without_refund",
		Other:     `{"is_task":true,"task_id":"task_failed_without_refund"}`,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:    user.Id,
		Username:  user.Username,
		CreatedAt: 500,
		Type:      model.LogTypeConsume,
		ModelName: "gpt-image-2",
		TokenName: token.Name,
		Quota:     3_600,
		RequestId: "req_pending",
		Other:     `{"is_task":true,"task_id":"task_pending"}`,
	}).Error)

	logRecorder := httptest.NewRecorder()
	logContext, _ := gin.CreateTestContext(logRecorder)
	logContext.Request = httptest.NewRequest(http.MethodGet, "/api/log/self?p=1&page_size=10&type=2&settled=true&start_timestamp=1&end_timestamp=999", nil)
	logContext.Set("id", user.Id)
	GetUserLogs(logContext)

	require.Equal(t, http.StatusOK, logRecorder.Code, logRecorder.Body.String())
	require.Contains(t, logRecorder.Body.String(), `"total":3`)
	require.Contains(t, logRecorder.Body.String(), `"request_id":"task_success"`)
	require.Contains(t, logRecorder.Body.String(), `"request_id":"task_failed_without_refund"`)
	require.Contains(t, logRecorder.Body.String(), `"request_id":"req_sync"`)
	require.NotContains(t, logRecorder.Body.String(), `"request_id":"task_failed"`)
	require.NotContains(t, logRecorder.Body.String(), "req_failed_reservation")
	require.NotContains(t, logRecorder.Body.String(), "req_failed_refund")
	require.NotContains(t, logRecorder.Body.String(), "task_pending")
	require.NotContains(t, logRecorder.Body.String(), "req_pending")

	statRecorder := httptest.NewRecorder()
	statContext, _ := gin.CreateTestContext(statRecorder)
	statContext.Request = httptest.NewRequest(http.MethodGet, "/api/log/self/stat?type=2&settled=true&start_timestamp=1&end_timestamp=999", nil)
	statContext.Set("id", user.Id)
	statContext.Set("username", user.Username)
	GetLogsSelfStat(statContext)

	require.Equal(t, http.StatusOK, statRecorder.Code, statRecorder.Body.String())
	require.Contains(t, statRecorder.Body.String(), `"quota":21600`)
	require.Contains(t, statRecorder.Body.String(), `"credits":"6"`)
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
