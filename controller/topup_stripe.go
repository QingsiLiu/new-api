package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
	"github.com/thanhpk/randstr"
)

var stripeAdaptor = &StripeAdaptor{}

// StripePayRequest represents a payment request for Stripe checkout.
type StripePayRequest struct {
	// Amount is the quantity of units to purchase.
	Amount int64 `json:"amount"`
	// PaymentMethod specifies the payment method (e.g., "stripe").
	PaymentMethod string `json:"payment_method"`
	// SuccessURL is the optional custom URL to redirect after successful payment.
	// If empty, defaults to the server's console log page.
	SuccessURL string `json:"success_url,omitempty"`
	// CancelURL is the optional custom URL to redirect when payment is canceled.
	// If empty, defaults to the server's console topup page.
	CancelURL string `json:"cancel_url,omitempty"`
}

type StripeAdaptor struct {
}

func (*StripeAdaptor) RequestAmount(c *gin.Context, req *StripePayRequest) {
	if req.Amount < getStripeMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getStripeMinTopup())})
		return
	}
	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getStripePayMoney(float64(req.Amount), group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": strconv.FormatFloat(payMoney, 'f', 2, 64)})
}

func (*StripeAdaptor) RequestPay(c *gin.Context, req *StripePayRequest) {
	if req.PaymentMethod != model.PaymentMethodStripe {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付渠道"})
		return
	}
	if req.Amount < getStripeMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("充值数量不能小于 %d", getStripeMinTopup()), "data": 10})
		return
	}
	if req.Amount > 10000 {
		c.JSON(http.StatusOK, gin.H{"message": "充值数量不能大于 10000", "data": 10})
		return
	}

	if req.SuccessURL != "" && common.ValidateRedirectURL(req.SuccessURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "支付成功重定向URL不在可信任域名列表中", "data": ""})
		return
	}

	if req.CancelURL != "" && common.ValidateRedirectURL(req.CancelURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "支付取消重定向URL不在可信任域名列表中", "data": ""})
		return
	}

	id := c.GetInt("id")
	user, _ := model.GetUserById(id, false)
	chargedMoney := GetChargedAmount(float64(req.Amount), *user)

	reference := fmt.Sprintf("new-api-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "ref_" + common.Sha1([]byte(reference))

	payLink, err := genStripeLink(referenceId, user.StripeCustomer, user.Email, req.Amount, req.SuccessURL, req.CancelURL)
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "stripe", "checkout_create_failed", paymentSecurityFields{
			UserID: id, OrderID: referenceId, Amount: req.Amount, Err: err,
		})
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	topUp := &model.TopUp{
		UserId:          id,
		Amount:          req.Amount,
		Money:           chargedMoney,
		TradeNo:         referenceId,
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	err = topUp.Insert()
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "stripe", "order_create_failed", paymentSecurityFields{
			UserID: id, OrderID: referenceId, Amount: req.Amount, Err: err,
		})
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}
	logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "stripe", "checkout_created", paymentSecurityFields{
		UserID: id, OrderID: referenceId, Amount: req.Amount, Money: chargedMoney,
	})
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_link": payLink,
		},
	})
}

func RequestStripeAmount(c *gin.Context) {
	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	stripeAdaptor.RequestAmount(c, &req)
}

func RequestStripePay(c *gin.Context) {
	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	stripeAdaptor.RequestPay(c, &req)
}

func StripeWebhook(c *gin.Context) {
	ctx := c.Request.Context()
	if !isStripeWebhookEnabled() {
		logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "webhook_rejected", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Reason: "webhook_disabled",
		})
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logPaymentSecurityEvent(ctx, paymentLogError, "stripe", "payload_read_failed", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Err: err,
		})
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	logPaymentSecurityEvent(ctx, paymentLogInfo, "stripe", "webhook_received", paymentSecurityFields{
		Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Payload: payload, Signature: signature,
	})
	event, err := webhook.ConstructEventWithOptions(payload, signature, setting.StripeWebhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})

	if err != nil {
		logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "signature_invalid", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Payload: payload, Signature: signature, Err: err,
		})
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	callerIp := c.ClientIP()
	logPaymentSecurityEvent(ctx, paymentLogInfo, "stripe", "signature_valid", paymentSecurityFields{
		Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID,
	})
	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		sessionCompleted(ctx, event, callerIp)
	case stripe.EventTypeCheckoutSessionExpired:
		sessionExpired(ctx, event)
	case stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded:
		sessionAsyncPaymentSucceeded(ctx, event, callerIp)
	case stripe.EventTypeCheckoutSessionAsyncPaymentFailed:
		sessionAsyncPaymentFailed(ctx, event, callerIp)
	default:
		logPaymentSecurityEvent(ctx, paymentLogInfo, "stripe", "event_ignored", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID,
		})
	}

	c.Status(http.StatusOK)
}

func sessionCompleted(ctx context.Context, event stripe.Event, callerIp string) {
	customerId := event.GetObjectValue("customer")
	referenceId := event.GetObjectValue("client_reference_id")
	status := event.GetObjectValue("status")
	if "complete" != status {
		logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "checkout_status_invalid", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId, OrderStatus: status,
		})
		return
	}

	paymentStatus := event.GetObjectValue("payment_status")
	if paymentStatus != "paid" {
		logPaymentSecurityEvent(ctx, paymentLogInfo, "stripe", "payment_pending", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId, OrderStatus: paymentStatus,
		})
		return
	}

	fulfillOrder(ctx, event, referenceId, customerId, callerIp)
}

// sessionAsyncPaymentSucceeded handles delayed payment methods (bank transfer, SEPA, etc.)
// that confirm payment after the checkout session completes.
func sessionAsyncPaymentSucceeded(ctx context.Context, event stripe.Event, callerIp string) {
	customerId := event.GetObjectValue("customer")
	referenceId := event.GetObjectValue("client_reference_id")
	logPaymentSecurityEvent(ctx, paymentLogInfo, "stripe", "async_payment_succeeded", paymentSecurityFields{
		ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId,
	})

	fulfillOrder(ctx, event, referenceId, customerId, callerIp)
}

// sessionAsyncPaymentFailed marks orders as failed when delayed payment methods
// ultimately fail (e.g. bank transfer not received, SEPA rejected).
func sessionAsyncPaymentFailed(ctx context.Context, event stripe.Event, callerIp string) {
	referenceId := event.GetObjectValue("client_reference_id")
	logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "async_payment_failed", paymentSecurityFields{
		ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId,
	})

	if len(referenceId) == 0 {
		logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "order_reference_missing", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID,
		})
		return
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)

	topUp := model.GetTopUpByTradeNo(referenceId)
	if topUp == nil {
		logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "order_not_found", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId,
		})
		return
	}

	if topUp.PaymentProvider != model.PaymentProviderStripe {
		logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "provider_mismatch", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId, Reason: "provider_mismatch",
		})
		return
	}

	if topUp.Status != common.TopUpStatusPending {
		logPaymentSecurityEvent(ctx, paymentLogInfo, "stripe", "order_already_terminal", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId, OrderStatus: topUp.Status,
		})
		return
	}

	topUp.Status = common.TopUpStatusFailed
	if err := topUp.Update(); err != nil {
		logPaymentSecurityEvent(ctx, paymentLogError, "stripe", "order_failure_update_failed", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId, Err: err,
		})
		return
	}
	logPaymentSecurityEvent(ctx, paymentLogInfo, "stripe", "order_marked_failed", paymentSecurityFields{
		ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId,
	})
}

// fulfillOrder is the shared logic for crediting quota after payment is confirmed.
func fulfillOrder(ctx context.Context, event stripe.Event, referenceId string, customerId string, callerIp string) {
	if len(referenceId) == 0 {
		logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "order_reference_missing", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID,
		})
		return
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)
	payload := map[string]any{
		"customer":     customerId,
		"amount_total": event.GetObjectValue("amount_total"),
		"currency":     strings.ToUpper(event.GetObjectValue("currency")),
		"event_type":   string(event.Type),
	}
	if err := model.CompleteSubscriptionOrder(referenceId, common.GetJsonString(payload), model.PaymentProviderStripe, ""); err == nil {
		logPaymentSecurityEvent(ctx, paymentLogInfo, "stripe", "subscription_completed", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId,
		})
		return
	} else if err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
		logPaymentSecurityEvent(ctx, paymentLogError, "stripe", "subscription_complete_failed", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId, Err: err,
		})
		return
	}

	err := model.Recharge(referenceId, customerId, callerIp)
	if err != nil {
		logPaymentSecurityEvent(ctx, paymentLogError, "stripe", "topup_complete_failed", paymentSecurityFields{
			ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId, Err: err,
		})
		return
	}

	total, _ := strconv.ParseFloat(event.GetObjectValue("amount_total"), 64)
	currency := strings.ToUpper(event.GetObjectValue("currency"))
	logPaymentSecurityEvent(ctx, paymentLogInfo, "stripe", "topup_completed", paymentSecurityFields{
		ClientIP: callerIp, EventType: string(event.Type), EventID: event.ID, OrderID: referenceId, Money: total / 100, Currency: currency,
	})
}

func sessionExpired(ctx context.Context, event stripe.Event) {
	referenceId := event.GetObjectValue("client_reference_id")
	status := event.GetObjectValue("status")
	if "expired" != status {
		logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "checkout_status_invalid", paymentSecurityFields{
			EventType: string(event.Type), EventID: event.ID, OrderID: referenceId, OrderStatus: status,
		})
		return
	}

	if len(referenceId) == 0 {
		logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "order_reference_missing", paymentSecurityFields{
			EventType: string(event.Type), EventID: event.ID,
		})
		return
	}

	// Subscription order expiration
	LockOrder(referenceId)
	defer UnlockOrder(referenceId)
	if err := model.ExpireSubscriptionOrder(referenceId, model.PaymentProviderStripe); err == nil {
		logPaymentSecurityEvent(ctx, paymentLogInfo, "stripe", "subscription_expired", paymentSecurityFields{
			EventType: string(event.Type), EventID: event.ID, OrderID: referenceId,
		})
		return
	} else if err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
		logPaymentSecurityEvent(ctx, paymentLogError, "stripe", "subscription_expire_failed", paymentSecurityFields{
			EventType: string(event.Type), EventID: event.ID, OrderID: referenceId, Err: err,
		})
		return
	}

	err := model.UpdatePendingTopUpStatus(referenceId, model.PaymentProviderStripe, common.TopUpStatusExpired)
	if errors.Is(err, model.ErrTopUpNotFound) {
		logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "order_not_found", paymentSecurityFields{
			EventType: string(event.Type), EventID: event.ID, OrderID: referenceId,
		})
		return
	}
	if err != nil {
		logPaymentSecurityEvent(ctx, paymentLogError, "stripe", "topup_expire_failed", paymentSecurityFields{
			EventType: string(event.Type), EventID: event.ID, OrderID: referenceId, Err: err,
		})
		return
	}

	logPaymentSecurityEvent(ctx, paymentLogInfo, "stripe", "topup_expired", paymentSecurityFields{
		EventType: string(event.Type), EventID: event.ID, OrderID: referenceId,
	})
}

// genStripeLink generates a Stripe Checkout session URL for payment.
// It creates a new checkout session with the specified parameters and returns the payment URL.
//
// Parameters:
//   - referenceId: unique reference identifier for the transaction
//   - customerId: existing Stripe customer ID (empty string if new customer)
//   - email: customer email address for new customer creation
//   - amount: quantity of units to purchase
//   - successURL: custom URL to redirect after successful payment (empty for default)
//   - cancelURL: custom URL to redirect when payment is canceled (empty for default)
//
// Returns the checkout session URL or an error if the session creation fails.
func genStripeLink(referenceId string, customerId string, email string, amount int64, successURL string, cancelURL string) (string, error) {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		return "", fmt.Errorf("无效的Stripe API密钥")
	}

	stripe.Key = setting.StripeApiSecret

	// Use custom URLs if provided, otherwise use defaults
	if successURL == "" {
		successURL = paymentReturnPath("/console/log")
	}
	if cancelURL == "" {
		cancelURL = paymentReturnPath("/console/topup")
	}

	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(referenceId),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(setting.StripePriceId),
				Quantity: stripe.Int64(amount),
			},
		},
		Mode:                stripe.String(string(stripe.CheckoutSessionModePayment)),
		AllowPromotionCodes: stripe.Bool(setting.StripePromotionCodesEnabled),
	}

	if "" == customerId {
		if "" != email {
			params.CustomerEmail = stripe.String(email)
		}

		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	} else {
		params.Customer = stripe.String(customerId)
	}

	result, err := session.New(params)
	if err != nil {
		return "", err
	}

	return result.URL, nil
}

func GetChargedAmount(count float64, user model.User) float64 {
	topUpGroupRatio := common.GetTopupGroupRatio(user.Group)
	if topUpGroupRatio == 0 {
		topUpGroupRatio = 1
	}

	return count * topUpGroupRatio
}

func getStripePayMoney(amount float64, group string) float64 {
	originalAmount := amount
	// Using float64 for monetary calculations is acceptable here due to the small amounts involved
	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	// apply optional preset discount by the original request amount (if configured), default 1.0
	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(originalAmount)]; ok {
		if ds > 0 {
			discount = ds
		}
	}
	payMoney := amount * setting.StripeUnitPrice * topupGroupRatio * discount
	return payMoney
}

func getStripeMinTopup() int64 {
	return int64(setting.StripeMinTopUp)
}
