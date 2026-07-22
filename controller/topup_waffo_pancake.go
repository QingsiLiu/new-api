package controller

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/thanhpk/randstr"
)

type WaffoPancakePayRequest struct {
	Amount int64 `json:"amount"`
}

func RequestWaffoPancakeAmount(c *gin.Context) {
	var req WaffoPancakePayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	if req.Amount < int64(setting.WaffoPancakeMinTopUp) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", setting.WaffoPancakeMinTopUp)})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getWaffoPancakePayMoney(req.Amount, group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success", "data": fmt.Sprintf("%.2f", payMoney)})
}

func getWaffoPancakePayMoney(amount int64, group string) float64 {
	dAmount := decimal.NewFromInt(amount)

	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}

	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amount)]; ok && ds > 0 {
		discount = ds
	}

	payMoney := dAmount.
		Mul(decimal.NewFromFloat(setting.WaffoPancakeUnitPrice)).
		Mul(decimal.NewFromFloat(topupGroupRatio)).
		Mul(decimal.NewFromFloat(discount))

	return payMoney.InexactFloat64()
}

func normalizeWaffoPancakeTopUpAmount(amount int64) int64 {
	return amount
}

func formatWaffoPancakeAmount(payMoney float64) string {
	return decimal.NewFromFloat(payMoney).StringFixed(2)
}

func getWaffoPancakeBuyerEmail(user *model.User) string {
	if user != nil && strings.TrimSpace(user.Email) != "" {
		return user.Email
	}
	return ""
}

// The admin config endpoints below accept typed-but-not-yet-saved creds in
// the body and fall back to persisted creds when the body is blank (see
// resolveWaffoPancakeAdminCreds). Only SaveWaffoPancake writes to OptionMap.

type saveWaffoPancakeRequest struct {
	MerchantID string `json:"merchant_id"`
	PrivateKey string `json:"private_key"`
	ReturnURL  string `json:"return_url"`
	StoreID    string `json:"store_id"`
	ProductID  string `json:"product_id"`
}

type createWaffoPancakePairRequest struct {
	MerchantID string `json:"merchant_id"`
	PrivateKey string `json:"private_key"`
	ReturnURL  string `json:"return_url"`
}

// SaveWaffoPancake atomically persists all five operator-controlled fields.
// Catalog / pair endpoints are transient — only this one writes the OptionMap.
func SaveWaffoPancake(c *gin.Context) {
	var req saveWaffoPancakeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if err := service.SaveWaffoPancakeConfig(
		c.Request.Context(),
		req.MerchantID,
		req.PrivateKey,
		req.ReturnURL,
		req.StoreID,
		req.ProductID,
	); err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "config_save_failed", paymentSecurityFields{
			ResourceID: req.StoreID, Reason: "store_config", Err: err,
		})
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "保存配置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"product_id": setting.WaffoPancakeProductID,
			"store_id":   setting.WaffoPancakeStoreID,
		},
	})
}

// resolveWaffoPancakeAdminCreds prefers body creds (typed-but-not-yet-saved
// values, for verification) and falls back to persisted creds when the body
// is blank (so returning admins don't have to re-paste the private key,
// which is stripped from GET /api/option/).
func resolveWaffoPancakeAdminCreds(bodyMerchantID, bodyPrivateKey string) (string, string) {
	m := strings.TrimSpace(bodyMerchantID)
	k := strings.TrimSpace(bodyPrivateKey)
	if m == "" && k == "" {
		return setting.WaffoPancakeMerchantID, setting.WaffoPancakePrivateKey
	}
	return m, k
}

// CreateWaffoPancakePair mints a Store + OnetimeProduct pair in one round-
// trip. Surfaces an orphan-store flag when the product half fails so the
// frontend can preselect / retry without losing context.
func CreateWaffoPancakePair(c *gin.Context) {
	var req createWaffoPancakePairRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
			return
		}
	}
	merchantID, privateKey := resolveWaffoPancakeAdminCreds(req.MerchantID, req.PrivateKey)
	if merchantID == "" || privateKey == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Waffo Pancake 凭证未配置"})
		return
	}
	result, err := service.CreateWaffoPancakePrimaryPair(
		c.Request.Context(), merchantID, privateKey, req.ReturnURL,
	)
	if err != nil {
		orphan := result != nil && result.OrphanStore
		resourceID := ""
		if result != nil {
			resourceID = result.StoreID
		}
		reason := "pair_create_failed"
		if orphan {
			reason = "pair_create_orphan_store"
		}
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "pair_create_failed", paymentSecurityFields{
			ResourceID: resourceID, Reason: reason, Err: err,
		})
		data := gin.H{"error": err.Error()}
		if orphan {
			data["store_id"] = result.StoreID
			data["store_name"] = result.StoreName
			data["orphan_store"] = true
		}
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": data})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"store_id":     result.StoreID,
			"store_name":   result.StoreName,
			"product_id":   result.ProductID,
			"product_name": result.ProductName,
		},
	})
}

// ListWaffoPancakeCatalog returns the merchant's Stores + OnetimeProducts.
// Doubles as a credential probe (a successful 200 proves the resolved creds
// authenticate). See resolveWaffoPancakeAdminCreds for credential resolution.
func ListWaffoPancakeCatalog(c *gin.Context) {
	// Missing query creds mean "use persisted creds".
	merchantID, privateKey := resolveWaffoPancakeAdminCreds(
		strings.TrimSpace(c.Query("merchant_id")),
		strings.TrimSpace(c.Query("private_key")),
	)
	if merchantID == "" || privateKey == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Waffo Pancake 凭证未配置"})
		return
	}
	catalog, err := service.ListWaffoPancakeCatalog(c.Request.Context(), merchantID, privateKey)
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "catalog_list_failed", paymentSecurityFields{Err: err})
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉取目录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": catalog})
}

type createWaffoPancakeSubscriptionProductRequest struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
}

// CreateWaffoPancakeSubscriptionProduct mints an OnetimeProduct (not
// SubscriptionProduct — see service.CreateWaffoPancakeProductForPlan)
// sized to a plan's `name` + `amount`, using persisted Pancake credentials
// + StoreID. Reads from the form, not the plan row, so newly-typed unsaved
// plans can mint a product too.
func CreateWaffoPancakeSubscriptionProduct(c *gin.Context) {
	var req createWaffoPancakeSubscriptionProductRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
			return
		}
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "套餐名称不能为空"})
		return
	}
	if strings.TrimSpace(req.Amount) == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "套餐价格不能为空"})
		return
	}
	merchantID, privateKey := resolveWaffoPancakeAdminCreds("", "")
	storeID := strings.TrimSpace(setting.WaffoPancakeStoreID)
	if merchantID == "" || privateKey == "" || storeID == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Waffo Pancake 未完成配置，请先在支付设置中完成网关绑定"})
		return
	}
	productID, err := service.CreateWaffoPancakeProductForPlan(
		c.Request.Context(),
		merchantID,
		privateKey,
		storeID,
		req.Name,
		req.Amount,
		setting.WaffoPancakeReturnURL,
	)
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "subscription_product_create_failed", paymentSecurityFields{
			ResourceID: storeID, Err: err,
		})
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建套餐产品失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"product_id":   productID,
			"product_name": req.Name,
			"store_id":     storeID,
		},
	})
}

// ListWaffoPancakeSubscriptionProductOptions returns the OnetimeProducts
// in the saved Pancake store, for the subscription-plan dropdown. The name
// reflects new-api's plan concept; under the hood it's still OnetimeProducts.
func ListWaffoPancakeSubscriptionProductOptions(c *gin.Context) {
	merchantID, privateKey := resolveWaffoPancakeAdminCreds("", "")
	storeID := strings.TrimSpace(setting.WaffoPancakeStoreID)
	if merchantID == "" || privateKey == "" || storeID == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Waffo Pancake 未完成配置，请先在支付设置中完成网关绑定"})
		return
	}
	catalog, err := service.ListWaffoPancakeCatalog(c.Request.Context(), merchantID, privateKey)
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "subscription_catalog_list_failed", paymentSecurityFields{
			ResourceID: storeID, Err: err,
		})
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉取产品列表失败"})
		return
	}
	products := []service.WaffoPancakeCatalogProduct{}
	for _, store := range catalog.Stores {
		if store.ID == storeID {
			products = store.OnetimeProducts
			break
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"store_id": storeID,
			"products": products,
		},
	})
}

func getWaffoPancakeBuyerIdentity(user *model.User) string {
	if user == nil {
		return ""
	}
	return service.WaffoPancakeBuyerIdentityFromUserID(user.Id)
}

func RequestWaffoPancakePay(c *gin.Context) {
	if !isWaffoPancakeTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Waffo Pancake 配置不完整"})
		return
	}

	var req WaffoPancakePayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if req.Amount < int64(setting.WaffoPancakeMinTopUp) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", setting.WaffoPancakeMinTopUp)})
		return
	}

	id := c.GetInt("id")
	user, err := model.GetUserById(id, false)
	if err != nil || user == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "用户不存在"})
		return
	}

	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getWaffoPancakePayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	tradeNo := fmt.Sprintf("WAFFO_PANCAKE-%d-%d-%s", id, time.Now().UnixMilli(), randstr.String(6))
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          normalizeWaffoPancakeTopUpAmount(req.Amount),
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodWaffoPancake,
		PaymentProvider: model.PaymentProviderWaffoPancake,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "order_create_failed", paymentSecurityFields{
			UserID: id, OrderID: tradeNo, Amount: req.Amount, Err: err,
		})
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	expiresInSeconds := 45 * 60
	session, err := service.CreateWaffoPancakeCheckoutSession(c.Request.Context(), &service.WaffoPancakeCreateSessionParams{
		ProductID:     setting.WaffoPancakeProductID,
		BuyerIdentity: getWaffoPancakeBuyerIdentity(user),
		PriceSnapshot: &service.WaffoPancakePriceSnapshot{
			Amount:      formatWaffoPancakeAmount(payMoney),
			TaxCategory: "saas",
		},
		BuyerEmail:              getWaffoPancakeBuyerEmail(user),
		ExpiresInSeconds:        &expiresInSeconds,
		OrderMerchantExternalID: tradeNo,
	})
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "checkout_create_failed", paymentSecurityFields{
			UserID: id, OrderID: tradeNo, Amount: req.Amount, Err: err,
		})
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "waffo_pancake", "checkout_created", paymentSecurityFields{
		UserID: id, OrderID: tradeNo, EventID: session.SessionID, Amount: req.Amount, Money: payMoney,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"checkout_url":     session.CheckoutURL,
			"session_id":       session.SessionID,
			"expires_at":       session.ExpiresAt,
			"order_id":         tradeNo,
			"token":            session.Token,
			"token_expires_at": session.TokenExpiresAt,
		},
	})
}

func WaffoPancakeWebhook(c *gin.Context) {
	if !isWaffoPancakeWebhookEnabled() {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogWarn, "waffo_pancake", "webhook_rejected", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Reason: "webhook_disabled",
		})
		c.String(http.StatusForbidden, "webhook disabled")
		return
	}

	// :env splits test vs prod traffic at the routing layer — operator
	// registers each URL in the matching webhook slot in Pancake's dashboard.
	// We then enforce event.mode == expectedEnv to catch mis-registrations.
	expectedEnv := strings.TrimSpace(c.Param("env"))
	if expectedEnv != "test" && expectedEnv != "prod" {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogWarn, "waffo_pancake", "environment_invalid", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), ExpectedEnv: expectedEnv,
		})
		c.String(http.StatusNotFound, "unknown env")
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "payload_read_failed", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), ExpectedEnv: expectedEnv, Err: err,
		})
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	signature := c.GetHeader("X-Waffo-Signature")
	logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "waffo_pancake", "webhook_received", paymentSecurityFields{
		Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), ExpectedEnv: expectedEnv, Payload: bodyBytes, Signature: signature,
	})

	event, err := service.VerifyConfiguredWaffoPancakeWebhook(string(bodyBytes), signature)
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogWarn, "waffo_pancake", "signature_invalid", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), ExpectedEnv: expectedEnv, Payload: bodyBytes, Signature: signature, Err: err,
		})
		c.String(http.StatusUnauthorized, "invalid signature")
		return
	}

	if !strings.EqualFold(strings.TrimSpace(event.Mode), expectedEnv) {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "environment_mismatch", paymentSecurityFields{
			ClientIP: c.ClientIP(), ExpectedEnv: expectedEnv, ActualEnv: event.Mode, EventID: event.ID, OrderID: event.Data.OrderID,
		})
		c.String(http.StatusOK, "OK")
		return
	}

	logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "waffo_pancake", "signature_valid", paymentSecurityFields{
		ClientIP: c.ClientIP(), EventType: event.NormalizedEventType(), EventID: event.ID, OrderID: event.Data.OrderID,
	})
	if event.NormalizedEventType() != "order.completed" {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "waffo_pancake", "event_ignored", paymentSecurityFields{
			ClientIP: c.ClientIP(), EventType: event.NormalizedEventType(), EventID: event.ID, OrderID: event.Data.OrderID,
		})
		c.String(http.StatusOK, "OK")
		return
	}

	// Dispatch by trade_no prefix. OrderMerchantExternalID = our trade_no;
	// OrderID is Pancake's internal ORD_* (logs only).
	rawTradeNo := strings.TrimSpace(event.Data.OrderMerchantExternalID)
	isSubscription := strings.HasPrefix(rawTradeNo, "WAFFO_PANCAKE_SUB-")

	if isSubscription {
		tradeNo, err := service.ResolveWaffoPancakeSubscriptionTradeNo(event)
		if err != nil {
			logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "subscription_order_resolve_failed", paymentSecurityFields{
				ClientIP: c.ClientIP(), EventType: event.NormalizedEventType(), EventID: event.ID, OrderID: event.Data.OrderID, Err: err,
			})
			c.String(http.StatusOK, "OK")
			return
		}
		LockOrder(tradeNo)
		defer UnlockOrder(tradeNo)
		if err := model.CompleteSubscriptionOrder(tradeNo, string(bodyBytes), model.PaymentProviderWaffoPancake, ""); err != nil {
			logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "subscription_complete_failed", paymentSecurityFields{
				ClientIP: c.ClientIP(), EventType: event.NormalizedEventType(), EventID: event.ID, OrderID: tradeNo, Err: err,
			})
			c.String(http.StatusInternalServerError, "retry")
			return
		}
		logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "waffo_pancake", "subscription_completed", paymentSecurityFields{
			ClientIP: c.ClientIP(), EventType: event.NormalizedEventType(), EventID: event.ID, OrderID: tradeNo,
		})
		c.String(http.StatusOK, "OK")
		return
	}

	tradeNo, err := service.ResolveWaffoPancakeTradeNo(event)
	if err != nil {
		// LogError (not LogWarn): covers order-not-found and buyer-identity
		// mismatch — both warrant human attention. 200 OK so Waffo doesn't
		// retry a permanently-unresolvable webhook.
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "order_resolve_failed", paymentSecurityFields{
			ClientIP: c.ClientIP(), EventType: event.NormalizedEventType(), EventID: event.ID, OrderID: event.Data.OrderID, Err: err,
		})
		c.String(http.StatusOK, "OK")
		return
	}

	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	if err := model.RechargeWaffoPancake(tradeNo); err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "waffo_pancake", "topup_complete_failed", paymentSecurityFields{
			ClientIP: c.ClientIP(), EventType: event.NormalizedEventType(), EventID: event.ID, OrderID: tradeNo, Err: err,
		})
		c.String(http.StatusInternalServerError, "retry")
		return
	}

	logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "waffo_pancake", "topup_completed", paymentSecurityFields{
		ClientIP: c.ClientIP(), EventType: event.NormalizedEventType(), EventID: event.ID, OrderID: tradeNo,
	})
	c.String(http.StatusOK, "OK")
}
