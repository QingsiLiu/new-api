package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
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

	const callbackCount = 8
	responses := make(chan *httptest.ResponseRecorder, callbackCount)
	var waitGroup sync.WaitGroup
	waitGroup.Add(callbackCount)
	for range callbackCount {
		go func() {
			defer waitGroup.Done()
			responses <- callEpayNotifyForTest(t, values)
		}()
	}
	waitGroup.Wait()
	close(responses)
	for response := range responses {
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, "success", response.Body.String())
	}

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

func TestEpayNotifyCreditsRejectsMerchantAndMethodMismatch(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.PaymentEvent{}, &model.Log{}))

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
	operation_setting.EpayId = "credits-merchant"
	operation_setting.EpayKey = "credits-key"
	operation_setting.PayMethods = []map[string]string{{"name": "Alipay", "type": "alipay"}}
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	user := &model.User{Username: "credits-epay-guard", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	pkg, ok := model.FindCreditPackage("credits-cny-1000")
	require.True(t, ok)
	topUp := &model.TopUp{
		UserId:          user.Id,
		Amount:          pkg.Credits,
		Money:           float64(pkg.PriceMinor) / 100,
		TradeNo:         "credits-epay-callback-guard",
		PaymentMethod:   "alipay",
		PaymentProvider: model.PaymentProviderEpay,
		CreateTime:      common.GetTimestamp() - model.CreditsCheckoutReservationTTLSeconds - 1,
		Status:          common.TopUpStatusExpired,
		PackageId:       pkg.PackageId,
		Credits:         pkg.Credits,
		QuotaAmount:     pkg.Quota,
		Currency:        pkg.Currency,
		PriceMinor:      pkg.PriceMinor,
		BaseCredits:     pkg.BaseCredits,
		BonusCredits:    pkg.BonusCredits,
		PaymentStoreId:  operation_setting.EpayId,
	}
	require.NoError(t, db.Create(topUp).Error)

	callback := func(pid, method, eventID string) *httptest.ResponseRecorder {
		return callEpayNotifyForTest(t, signedEpayNotifyParams(map[string]string{
			"pid":          pid,
			"type":         method,
			"out_trade_no": topUp.TradeNo,
			"trade_no":     eventID,
			"name":         "1000 Credits",
			"money":        formatPriceMinor(pkg.PriceMinor),
			"trade_status": epay.StatusTradeSuccess,
		}, operation_setting.EpayKey))
	}

	require.Equal(t, "fail", callback("other-merchant", "alipay", "evt-wrong-merchant").Body.String())
	require.Equal(t, "fail", callback(operation_setting.EpayId, "wxpay", "evt-wrong-method").Body.String())
	require.Equal(t, common.TopUpStatusExpired, model.GetTopUpByTradeNo(topUp.TradeNo).Status)

	require.Equal(t, "success", callback(operation_setting.EpayId, "alipay", "evt-success").Body.String())
	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	require.Equal(t, pkg.Quota, updated.Quota)
}
