package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/thanhpk/randstr"
)

type creditsCheckoutRequest struct {
	PackageId     string `json:"package_id"`
	PaymentMethod string `json:"payment_method"`
}

type creditsCheckoutResponse struct {
	OrderId       string `json:"order_id"`
	CheckoutURL   string `json:"checkout_url"`
	PackageId     string `json:"package_id"`
	Credits       string `json:"credits"`
	BaseCredits   string `json:"base_credits"`
	BonusCredits  string `json:"bonus_credits"`
	Quota         int    `json:"quota"`
	Currency      string `json:"currency"`
	PriceMinor    int64  `json:"price_minor"`
	PaymentMethod string `json:"payment_method"`
}

func RequestCreditsCheckout(c *gin.Context) {
	if !common.CreditsV1Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "credits checkout is disabled"})
		return
	}
	var req creditsCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request"})
		return
	}
	pkg, ok := model.FindCreditPackage(req.PackageId)
	if !ok || !pkg.SupportsPaymentMethod(req.PaymentMethod) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unsupported credit package or payment method"})
		return
	}
	userID := c.GetInt("id")
	if err := model.CheckCreditTopUpCapacity(userID, pkg.Quota); err != nil {
		if errors.Is(err, model.ErrCreditBalanceLimit) {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"code":    "credit_balance_limit_exceeded",
				"message": "this package would exceed the current Credits balance limit",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to validate balance capacity"})
		return
	}

	switch req.PaymentMethod {
	case model.PaymentMethodWaffoPancake:
		requestCreditsWaffoPancakeCheckout(c, pkg)
	case "alipay":
		requestCreditsEpayCheckout(c, pkg)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unsupported payment method"})
	}
}

func creditsCheckoutPayload(pkg model.CreditPackage, tradeNo, checkoutURL, paymentMethod string) creditsCheckoutResponse {
	return creditsCheckoutResponse{
		OrderId:       tradeNo,
		CheckoutURL:   checkoutURL,
		PackageId:     pkg.PackageId,
		Credits:       pkg.CreditsDisplay,
		BaseCredits:   pkg.BaseCreditsDisplay,
		BonusCredits:  pkg.BonusCreditsDisplay,
		Quota:         pkg.Quota,
		Currency:      pkg.Currency,
		PriceMinor:    pkg.PriceMinor,
		PaymentMethod: paymentMethod,
	}
}

func newCreditsTopUp(userID int, pkg model.CreditPackage, tradeNo, paymentMethod, provider string) *model.TopUp {
	return &model.TopUp{
		UserId:          userID,
		Amount:          pkg.Credits,
		Money:           float64(pkg.PriceMinor) / 100,
		TradeNo:         tradeNo,
		PaymentMethod:   paymentMethod,
		PaymentProvider: provider,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
		PackageId:       pkg.PackageId,
		Credits:         pkg.Credits,
		QuotaAmount:     pkg.Quota,
		Currency:        pkg.Currency,
		PriceMinor:      pkg.PriceMinor,
		BaseCredits:     pkg.BaseCredits,
		BonusCredits:    pkg.BonusCredits,
	}
}

func requestCreditsWaffoPancakeCheckout(c *gin.Context, pkg model.CreditPackage) {
	if pkg.Currency != "USD" || !isWaffoPancakeTopUpEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "payment method unavailable"})
		return
	}
	userID := c.GetInt("id")
	user, err := model.GetUserById(userID, false)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "user not found"})
		return
	}
	tradeNo := fmt.Sprintf("CREDITS-WAFFO-%d-%d-%s", userID, time.Now().UnixMilli(), randstr.String(6))
	topUp := newCreditsTopUp(userID, pkg, tradeNo, model.PaymentMethodWaffoPancake, model.PaymentProviderWaffoPancake)
	topUp.PaymentStoreId = setting.WaffoPancakeStoreID
	topUp.PaymentEnvironment = waffoPancakeEnvironment()
	if err := model.InsertCreditTopUpWithCapacity(topUp); err != nil {
		if errors.Is(err, model.ErrCreditBalanceLimit) {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"code":    "credit_balance_limit_exceeded",
				"message": "active pending orders and this package would exceed the Credits balance limit",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to create order"})
		return
	}

	expiresInSeconds := int(model.CreditsCheckoutReservationTTLSeconds)
	session, err := service.CreateWaffoPancakeCheckoutSession(c.Request.Context(), &service.WaffoPancakeCreateSessionParams{
		ProductID:     setting.WaffoPancakeProductID,
		Currency:      pkg.Currency,
		BuyerIdentity: service.WaffoPancakeBuyerIdentityFromUserID(user.Id),
		PriceSnapshot: &service.WaffoPancakePriceSnapshot{
			Amount:      formatPriceMinor(pkg.PriceMinor),
			TaxCategory: "saas",
		},
		BuyerEmail:              getWaffoPancakeBuyerEmail(user),
		ExpiresInSeconds:        &expiresInSeconds,
		OrderMerchantExternalID: tradeNo,
	})
	if err != nil {
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "failed to create checkout"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": creditsCheckoutPayload(pkg, tradeNo, session.CheckoutURL, model.PaymentMethodWaffoPancake)})
}

func requestCreditsEpayCheckout(c *gin.Context, pkg model.CreditPackage) {
	if pkg.Currency != "CNY" || !isEpayMethodEnabled("alipay") {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "payment method unavailable"})
		return
	}
	userID := c.GetInt("id")
	client := GetEpayClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "payment method unavailable"})
		return
	}
	tradeNo := fmt.Sprintf("CREDITS-EPAY-%d-%d-%s", userID, time.Now().UnixMilli(), randstr.String(6))
	topUp := newCreditsTopUp(userID, pkg, tradeNo, "alipay", model.PaymentProviderEpay)
	topUp.PaymentStoreId = operation_setting.EpayId
	if err := model.InsertCreditTopUpWithCapacity(topUp); err != nil {
		if errors.Is(err, model.ErrCreditBalanceLimit) {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"code":    "credit_balance_limit_exceeded",
				"message": "active pending orders and this package would exceed the Credits balance limit",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to create order"})
		return
	}
	callbackAddress := service.GetCallbackAddress()
	returnURL, _ := url.Parse(paymentReturnPath("/wallet?show_history=true"))
	notifyURL, _ := url.Parse(callbackAddress + "/api/user/epay/notify")
	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           "alipay",
		ServiceTradeNo: tradeNo,
		Name:           fmt.Sprintf("%s Credits", pkg.CreditsDisplay),
		Money:          formatPriceMinor(pkg.PriceMinor),
		Device:         epay.PC,
		NotifyUrl:      notifyURL,
		ReturnUrl:      returnURL,
	})
	if err != nil {
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "failed to create checkout"})
		return
	}
	checkoutURL, err := url.Parse(uri)
	if err != nil {
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "invalid checkout url"})
		return
	}
	query := checkoutURL.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	checkoutURL.RawQuery = query.Encode()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": creditsCheckoutPayload(pkg, tradeNo, checkoutURL.String(), "alipay")})
}

func formatPriceMinor(priceMinor int64) string {
	return strconv.FormatInt(priceMinor/100, 10) + "." + fmt.Sprintf("%02d", priceMinor%100)
}

func parsePriceMinor(amount string) (int64, error) {
	parts := strings.Split(strings.TrimSpace(amount), ".")
	if len(parts) > 2 || len(parts) == 0 {
		return 0, fmt.Errorf("invalid amount")
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, fmt.Errorf("invalid amount")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		return 0, fmt.Errorf("invalid amount")
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	minor, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount")
	}
	return whole*100 + minor, nil
}
