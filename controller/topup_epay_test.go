package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func signedEpayNotifyParams(params map[string]string, key string) url.Values {
	signed := epay.GenerateParams(params, key)
	values := url.Values{}
	for name, value := range signed {
		values.Set(name, value)
	}
	return values
}

func callEpayNotifyForTest(t *testing.T, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/epay/notify", strings.NewReader(values.Encode()))
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	EpayNotify(ctx)
	return recorder
}

func callRequestEpayForTest(t *testing.T, userID int, payload string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/self/pay", strings.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userID)
	RequestEpay(ctx)
	return recorder
}

func TestEpayNotifyCreditsQuotaAndIsIdempotent(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.Log{}))

	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	paymentSetting := operation_setting.GetPaymentSetting()
	originalComplianceConfirmed := paymentSetting.ComplianceConfirmed
	originalComplianceTermsVersion := paymentSetting.ComplianceTermsVersion
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
		paymentSetting.ComplianceConfirmed = originalComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = originalComplianceTermsVersion
	})

	operation_setting.PayAddress = "https://pay.example.test"
	operation_setting.EpayId = "epay-test-merchant"
	operation_setting.EpayKey = "epay-test-key"
	operation_setting.PayMethods = []map[string]string{{"name": "Alipay", "type": "alipay"}}
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	user := &model.User{
		Username: "epay-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Quota:    100,
		Group:    "default",
	}
	require.NoError(t, db.Create(user).Error)

	create := callRequestEpayForTest(t, user.Id, `{"amount":2,"payment_method":"alipay"}`)
	require.Equal(t, http.StatusOK, create.Code)
	var createPayload struct {
		Message string            `json:"message"`
		Data    map[string]string `json:"data"`
		URL     string            `json:"url"`
	}
	require.NoError(t, json.Unmarshal(create.Body.Bytes(), &createPayload))
	require.Equal(t, "success", createPayload.Message)
	require.Equal(t, "https://pay.example.test/submit.php", createPayload.URL)
	require.Equal(t, "alipay", createPayload.Data["type"])

	var topUps []model.TopUp
	require.NoError(t, db.Where("user_id = ?", user.Id).Find(&topUps).Error)
	require.Len(t, topUps, 1)
	require.Equal(t, common.TopUpStatusPending, topUps[0].Status)
	require.Equal(t, model.PaymentProviderEpay, topUps[0].PaymentProvider)
	require.Equal(t, int64(2), topUps[0].Amount)
	tradeNo := topUps[0].TradeNo

	values := signedEpayNotifyParams(map[string]string{
		"pid":          operation_setting.EpayId,
		"type":         "alipay",
		"out_trade_no": tradeNo,
		"trade_no":     "provider-trade-1",
		"name":         "TUC2",
		"money":        "2.00",
		"trade_status": epay.StatusTradeSuccess,
	}, operation_setting.EpayKey)

	first := callEpayNotifyForTest(t, values)
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, "success", first.Body.String())
	second := callEpayNotifyForTest(t, values)
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, "success", second.Body.String())

	var reloaded model.User
	require.NoError(t, db.First(&reloaded, user.Id).Error)
	require.Equal(t, 100+common.CNYToQuota(2), reloaded.Quota)

	reloadedTopUp := model.GetTopUpByTradeNo(tradeNo)
	require.NotNil(t, reloadedTopUp)
	require.Equal(t, common.TopUpStatusSuccess, reloadedTopUp.Status)
	require.Equal(t, "alipay", reloadedTopUp.PaymentMethod)

	var topupLogs int64
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeTopup).Count(&topupLogs).Error)
	require.EqualValues(t, 1, topupLogs)
}
