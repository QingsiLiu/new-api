package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
)

type paymentLogLevel uint8

const (
	paymentLogInfo paymentLogLevel = iota
	paymentLogWarn
	paymentLogError
)

type paymentSecurityFields struct {
	Method        string
	Path          string
	ClientIP      string
	Payload       []byte
	Signature     string
	EventType     string
	EventID       string
	OrderID       string
	OrderStatus   string
	CallbackType  string
	PaymentMethod string
	Currency      string
	ExpectedEnv   string
	ActualEnv     string
	Reason        string
	UserID        int
	Amount        int64
	Money         float64
	StatusCode    int
	DurationMS    int64
	PlanID        int
	ResourceID    string
	Err           error
}

func logPaymentSecurityEvent(ctx context.Context, level paymentLogLevel, provider, event string, fields paymentSecurityFields) {
	record := map[string]any{
		"component": "payment",
		"provider":  paymentLogToken(provider),
		"event":     paymentLogToken(event),
	}
	if method := paymentLogMethod(fields.Method); method != "" {
		record["method"] = method
	}
	if path := paymentLogPath(fields.Path); path != "" {
		record["path"] = path
	}
	if fields.ClientIP != "" {
		record["client_ip_sha256"] = paymentLogDigest(strings.TrimSpace(fields.ClientIP))
	}
	if fields.Payload != nil {
		record["payload_bytes"] = len(fields.Payload)
		record["payload_sha256"] = paymentLogDigest(string(fields.Payload))
	}
	record["signature_present"] = strings.TrimSpace(fields.Signature) != ""
	if fields.Signature != "" {
		record["signature_sha256"] = paymentLogDigest(strings.TrimSpace(fields.Signature))
	}
	if fields.EventType != "" {
		record["event_type"] = paymentLogToken(fields.EventType)
	}
	if fields.EventID != "" {
		record["event_ref"] = paymentLogReference(fields.EventID)
	}
	if fields.OrderID != "" {
		record["order_ref"] = paymentLogReference(fields.OrderID)
	}
	if fields.OrderStatus != "" {
		record["order_status"] = paymentLogToken(fields.OrderStatus)
	}
	if fields.CallbackType != "" {
		record["callback_type"] = paymentLogToken(fields.CallbackType)
	}
	if fields.PaymentMethod != "" {
		record["payment_method"] = paymentLogToken(fields.PaymentMethod)
	}
	if fields.Currency != "" {
		record["currency"] = paymentLogToken(fields.Currency)
	}
	if fields.ExpectedEnv != "" {
		record["expected_env"] = paymentLogToken(fields.ExpectedEnv)
	}
	if fields.ActualEnv != "" {
		record["actual_env"] = paymentLogToken(fields.ActualEnv)
	}
	if fields.Reason != "" {
		record["reason"] = paymentLogToken(fields.Reason)
	}
	if fields.UserID > 0 {
		record["user_id"] = fields.UserID
	}
	if fields.Amount != 0 {
		record["amount"] = fields.Amount
	}
	if fields.Money != 0 {
		record["money"] = fields.Money
	}
	if fields.StatusCode > 0 {
		record["status_code"] = fields.StatusCode
	}
	if fields.DurationMS > 0 {
		record["duration_ms"] = fields.DurationMS
	}
	if fields.PlanID > 0 {
		record["plan_id"] = fields.PlanID
	}
	if fields.ResourceID != "" {
		record["resource_ref"] = paymentLogReference(fields.ResourceID)
	}
	if fields.Err != nil {
		record["error_type"] = reflect.TypeOf(fields.Err).String()
		record["error_sha256"] = paymentLogDigest(fields.Err.Error())
	}

	encoded, err := common.Marshal(record)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("payment security log marshal failed: %T", err))
		return
	}
	message := string(encoded)
	switch level {
	case paymentLogWarn:
		logger.LogWarn(ctx, message)
	case paymentLogError:
		logger.LogError(ctx, message)
	default:
		logger.LogInfo(ctx, message)
	}
}

func paymentLogDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", digest[:8])
}

func paymentLogReference(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 8 {
		return "#" + paymentLogDigest(value)
	}
	suffix := value[len(value)-8:]
	if paymentLogToken(suffix) == "unknown" {
		return "#" + paymentLogDigest(value)
	}
	return "*" + suffix
}

func paymentLogPath(value string) string {
	value = strings.TrimSpace(value)
	if parsed, err := url.ParseRequestURI(value); err == nil && parsed.Path != "" {
		value = parsed.Path
	} else if path, _, ok := strings.Cut(value, "?"); ok {
		value = path
	}
	if value == "" || value[0] != '/' || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	return value
}

func paymentLogMethod(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return value
	default:
		return ""
	}
}

func paymentLogToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 80 {
		return "unknown"
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._:-/", r) {
			continue
		}
		return "unknown"
	}
	return value
}
