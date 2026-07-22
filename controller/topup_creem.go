package controller

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/thanhpk/randstr"
)

const CreemSignatureHeader = "creem-signature"

var creemAdaptor = &CreemAdaptor{}

// 生成HMAC-SHA256签名
func generateCreemSignature(payload string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// 验证Creem webhook签名
func verifyCreemSignature(payload string, signature string, secret string) bool {
	if secret == "" {
		return setting.CreemTestMode
	}

	expectedSignature := generateCreemSignature(payload, secret)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

type CreemPayRequest struct {
	ProductId     string `json:"product_id"`
	PaymentMethod string `json:"payment_method"`
}

type CreemProduct struct {
	ProductId string  `json:"productId"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	Quota     int64   `json:"quota"`
}

type CreemAdaptor struct {
}

func (*CreemAdaptor) RequestPay(c *gin.Context, req *CreemPayRequest) {
	if req.PaymentMethod != model.PaymentMethodCreem {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付渠道"})
		return
	}

	if req.ProductId == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "请选择产品"})
		return
	}

	// 解析产品列表
	var products []CreemProduct
	err := common.Unmarshal([]byte(setting.CreemProducts), &products)
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "creem", "product_config_invalid", paymentSecurityFields{
			UserID: c.GetInt("id"), Err: err,
		})
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "产品配置错误"})
		return
	}

	// 查找对应的产品
	var selectedProduct *CreemProduct
	for _, product := range products {
		if product.ProductId == req.ProductId {
			selectedProduct = &product
			break
		}
	}

	if selectedProduct == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "产品不存在"})
		return
	}

	id := c.GetInt("id")
	user, _ := model.GetUserById(id, false)

	// 生成唯一的订单引用ID
	reference := fmt.Sprintf("creem-api-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "ref_" + common.Sha1([]byte(reference))

	// 先创建订单记录，使用产品配置的金额和充值额度
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          selectedProduct.Quota, // 充值额度
		Money:           selectedProduct.Price, // 支付金额
		TradeNo:         referenceId,
		PaymentMethod:   model.PaymentMethodCreem,
		PaymentProvider: model.PaymentProviderCreem,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	err = topUp.Insert()
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "creem", "order_create_failed", paymentSecurityFields{
			UserID: id, OrderID: referenceId, ResourceID: selectedProduct.ProductId, Amount: selectedProduct.Quota, Money: selectedProduct.Price, Err: err,
		})
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	// 创建支付链接，传入用户邮箱
	checkoutUrl, err := genCreemLink(c.Request.Context(), referenceId, selectedProduct, user.Email, user.Username)
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "creem", "checkout_create_failed", paymentSecurityFields{
			UserID: id, OrderID: referenceId, ResourceID: selectedProduct.ProductId, Amount: selectedProduct.Quota, Money: selectedProduct.Price, Err: err,
		})
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "creem", "checkout_created", paymentSecurityFields{
		UserID: id, OrderID: referenceId, ResourceID: selectedProduct.ProductId, Amount: selectedProduct.Quota, Money: selectedProduct.Price,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"checkout_url": checkoutUrl,
			"order_id":     referenceId,
		},
	})
}

func RequestCreemPay(c *gin.Context) {
	var req CreemPayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	creemAdaptor.RequestPay(c, &req)
}

// 新的Creem Webhook结构体，匹配实际的webhook数据格式
type CreemWebhookEvent struct {
	Id        string `json:"id"`
	EventType string `json:"eventType"`
	CreatedAt int64  `json:"created_at"`
	Object    struct {
		Id        string `json:"id"`
		Object    string `json:"object"`
		RequestId string `json:"request_id"`
		Order     struct {
			Object      string `json:"object"`
			Id          string `json:"id"`
			Customer    string `json:"customer"`
			Product     string `json:"product"`
			Amount      int    `json:"amount"`
			Currency    string `json:"currency"`
			SubTotal    int    `json:"sub_total"`
			TaxAmount   int    `json:"tax_amount"`
			AmountDue   int    `json:"amount_due"`
			AmountPaid  int    `json:"amount_paid"`
			Status      string `json:"status"`
			Type        string `json:"type"`
			Transaction string `json:"transaction"`
			CreatedAt   string `json:"created_at"`
			UpdatedAt   string `json:"updated_at"`
			Mode        string `json:"mode"`
		} `json:"order"`
		Product struct {
			Id                string  `json:"id"`
			Object            string  `json:"object"`
			Name              string  `json:"name"`
			Description       string  `json:"description"`
			Price             int     `json:"price"`
			Currency          string  `json:"currency"`
			BillingType       string  `json:"billing_type"`
			BillingPeriod     string  `json:"billing_period"`
			Status            string  `json:"status"`
			TaxMode           string  `json:"tax_mode"`
			TaxCategory       string  `json:"tax_category"`
			DefaultSuccessUrl *string `json:"default_success_url"`
			CreatedAt         string  `json:"created_at"`
			UpdatedAt         string  `json:"updated_at"`
			Mode              string  `json:"mode"`
		} `json:"product"`
		Units    int `json:"units"`
		Customer struct {
			Id        string `json:"id"`
			Object    string `json:"object"`
			Email     string `json:"email"`
			Name      string `json:"name"`
			Country   string `json:"country"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
			Mode      string `json:"mode"`
		} `json:"customer"`
		Status   string            `json:"status"`
		Metadata map[string]string `json:"metadata"`
		Mode     string            `json:"mode"`
	} `json:"object"`
}

func CreemWebhook(c *gin.Context) {
	if !isCreemWebhookEnabled() {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogWarn, "creem", "webhook_rejected", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Reason: "webhook_disabled",
		})
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	// 读取body内容用于打印，同时保留原始数据供后续使用
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "creem", "payload_read_failed", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Err: err,
		})
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// 获取签名头
	signature := c.GetHeader(CreemSignatureHeader)
	logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "creem", "webhook_received", paymentSecurityFields{
		Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Payload: bodyBytes, Signature: signature,
	})
	if signature == "" {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogWarn, "creem", "signature_missing", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Payload: bodyBytes,
		})
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// 验证签名
	if !verifyCreemSignature(string(bodyBytes), signature, setting.CreemWebhookSecret) {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogWarn, "creem", "signature_invalid", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Payload: bodyBytes, Signature: signature,
		})
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "creem", "signature_valid", paymentSecurityFields{
		Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Signature: signature,
	})

	// 重新设置body供后续的ShouldBindJSON使用
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// 解析新格式的webhook数据
	var webhookEvent CreemWebhookEvent
	if err := c.ShouldBindJSON(&webhookEvent); err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "creem", "payload_invalid", paymentSecurityFields{
			Method: c.Request.Method, Path: c.Request.URL.Path, ClientIP: c.ClientIP(), Payload: bodyBytes, Err: err,
		})
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "creem", "payload_valid", paymentSecurityFields{
		EventType: webhookEvent.EventType, EventID: webhookEvent.Id, OrderID: webhookEvent.Object.Order.Id, OrderStatus: webhookEvent.Object.Order.Status,
	})

	// 根据事件类型处理不同的webhook
	switch webhookEvent.EventType {
	case "checkout.completed":
		handleCheckoutCompleted(c, &webhookEvent)
	default:
		logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "creem", "event_ignored", paymentSecurityFields{
			EventType: webhookEvent.EventType, EventID: webhookEvent.Id, OrderID: webhookEvent.Object.Order.Id,
		})
		c.Status(http.StatusOK)
	}
}

// 处理支付完成事件
func handleCheckoutCompleted(c *gin.Context, event *CreemWebhookEvent) {
	// 验证订单状态
	if event.Object.Order.Status != "paid" {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "creem", "payment_pending", paymentSecurityFields{
			EventType: event.EventType, EventID: event.Id, OrderID: event.Object.RequestId, OrderStatus: event.Object.Order.Status,
		})
		c.Status(http.StatusOK)
		return
	}

	// 获取引用ID（这是我们创建订单时传递的request_id）
	referenceId := event.Object.RequestId
	if referenceId == "" {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogWarn, "creem", "order_reference_missing", paymentSecurityFields{
			EventType: event.EventType, EventID: event.Id, OrderID: event.Object.Order.Id,
		})
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Try complete subscription order first
	LockOrder(referenceId)
	defer UnlockOrder(referenceId)
	if err := model.CompleteSubscriptionOrder(referenceId, common.GetJsonString(event), model.PaymentProviderCreem, ""); err == nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "creem", "subscription_completed", paymentSecurityFields{
			EventType: event.EventType, EventID: event.Id, OrderID: referenceId,
		})
		c.Status(http.StatusOK)
		return
	} else if err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "creem", "subscription_complete_failed", paymentSecurityFields{
			EventType: event.EventType, EventID: event.Id, OrderID: referenceId, Err: err,
		})
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// 验证订单类型，目前只处理一次性付款（充值）
	if event.Object.Order.Type != "onetime" {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "creem", "order_type_ignored", paymentSecurityFields{
			EventType: event.EventType, EventID: event.Id, OrderID: referenceId, Reason: event.Object.Order.Type,
		})
		c.Status(http.StatusOK)
		return
	}

	logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "creem", "payment_confirmed", paymentSecurityFields{
		EventType: event.EventType, EventID: event.Id, OrderID: referenceId, Money: float64(event.Object.Order.AmountPaid), Currency: event.Object.Order.Currency,
	})

	// 查询本地订单确认存在
	topUp := model.GetTopUpByTradeNo(referenceId)
	if topUp == nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogWarn, "creem", "order_not_found", paymentSecurityFields{
			EventType: event.EventType, EventID: event.Id, OrderID: referenceId,
		})
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if topUp.Status != common.TopUpStatusPending {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "creem", "order_already_terminal", paymentSecurityFields{
			EventType: event.EventType, EventID: event.Id, OrderID: referenceId, OrderStatus: topUp.Status,
		})
		c.Status(http.StatusOK) // 已处理过的订单，返回成功避免重复处理
		return
	}

	// 处理充值，传入客户邮箱和姓名信息
	customerEmail := event.Object.Customer.Email
	customerName := event.Object.Customer.Name

	// 防护性检查，确保邮箱和姓名不为空字符串
	if customerEmail == "" {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogWarn, "creem", "customer_email_missing", paymentSecurityFields{
			EventType: event.EventType, EventID: event.Id, OrderID: referenceId,
		})
	}
	if customerName == "" {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogWarn, "creem", "customer_name_missing", paymentSecurityFields{
			EventType: event.EventType, EventID: event.Id, OrderID: referenceId,
		})
	}

	err := model.RechargeCreem(referenceId, customerEmail, customerName, c.ClientIP())
	if err != nil {
		logPaymentSecurityEvent(c.Request.Context(), paymentLogError, "creem", "topup_complete_failed", paymentSecurityFields{
			EventType: event.EventType, EventID: event.Id, OrderID: referenceId, ClientIP: c.ClientIP(), Err: err,
		})
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	logPaymentSecurityEvent(c.Request.Context(), paymentLogInfo, "creem", "topup_completed", paymentSecurityFields{
		EventType: event.EventType, EventID: event.Id, OrderID: referenceId, ClientIP: c.ClientIP(), Amount: int64(topUp.Amount), Money: topUp.Money,
	})
	c.Status(http.StatusOK)
}

type CreemCheckoutRequest struct {
	ProductId string `json:"product_id"`
	RequestId string `json:"request_id"`
	Customer  struct {
		Email string `json:"email"`
	} `json:"customer"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type CreemCheckoutResponse struct {
	CheckoutUrl string `json:"checkout_url"`
	Id          string `json:"id"`
}

func genCreemLink(ctx context.Context, referenceId string, product *CreemProduct, email string, username string) (string, error) {
	if setting.CreemApiKey == "" {
		return "", fmt.Errorf("未配置Creem API密钥")
	}

	// 根据测试模式选择 API 端点
	apiUrl := "https://api.creem.io/v1/checkouts"
	if setting.CreemTestMode {
		apiUrl = "https://test-api.creem.io/v1/checkouts"
		logPaymentSecurityEvent(ctx, paymentLogInfo, "creem", "api_environment_selected", paymentSecurityFields{ExpectedEnv: "test"})
	}

	// 构建请求数据，确保包含用户邮箱
	requestData := CreemCheckoutRequest{
		ProductId: product.ProductId,
		RequestId: referenceId, // 这个作为订单ID传递给Creem
		Customer: struct {
			Email string `json:"email"`
		}{
			Email: email, // 用户邮箱会在支付页面预填充
		},
		Metadata: map[string]string{
			"username":     username,
			"reference_id": referenceId,
			"product_name": product.Name,
			"quota":        fmt.Sprintf("%d", product.Quota),
		},
	}

	// 序列化请求数据
	jsonData, err := common.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("序列化请求数据失败: %v", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", setting.CreemApiKey)

	logPaymentSecurityEvent(ctx, paymentLogInfo, "creem", "checkout_request_sent", paymentSecurityFields{OrderID: referenceId})

	// 发送请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	logPaymentSecurityEvent(ctx, paymentLogInfo, "creem", "checkout_response_received", paymentSecurityFields{
		OrderID: referenceId, StatusCode: resp.StatusCode, Payload: body,
	})

	// 检查响应状态
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("Creem API http status %d ", resp.StatusCode)
	}
	// 解析响应
	var checkoutResp CreemCheckoutResponse
	err = common.Unmarshal(body, &checkoutResp)
	if err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if checkoutResp.CheckoutUrl == "" {
		return "", fmt.Errorf("Creem API resp no checkout url ")
	}

	logPaymentSecurityEvent(ctx, paymentLogInfo, "creem", "checkout_created", paymentSecurityFields{
		OrderID: referenceId, EventID: checkoutResp.Id,
	})
	return checkoutResp.CheckoutUrl, nil
}
