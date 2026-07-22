package controller

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func capturePaymentLog(t *testing.T, write func()) string {
	t.Helper()

	var output bytes.Buffer
	common.LogWriterMu.Lock()
	originalWriter := gin.DefaultWriter
	originalErrorWriter := gin.DefaultErrorWriter
	gin.DefaultWriter = &output
	gin.DefaultErrorWriter = &output
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = originalWriter
		gin.DefaultErrorWriter = originalErrorWriter
		common.LogWriterMu.Unlock()
	})

	write()
	return output.String()
}

func TestPaymentSecurityLogOnlyEmitsWhitelistedRedactedFields(t *testing.T) {
	payload := []byte(`{"token":"payload-secret","customer":{"email":"buyer@example.com"}}`)
	ctx := context.WithValue(context.Background(), common.RequestIdKey, "request-visible")
	output := capturePaymentLog(t, func() {
		logPaymentSecurityEvent(ctx, paymentLogWarn, "stripe", "signature_invalid", paymentSecurityFields{
			Method:      "POST",
			Path:        "/api/stripe/webhook?token=query-secret",
			ClientIP:    "203.0.113.42",
			Payload:     payload,
			Signature:   "signature-secret",
			EventType:   "checkout.completed",
			EventID:     "evt_private_12345678",
			OrderID:     "order_private_87654321",
			OrderStatus: "paid",
			Err:         errors.New("Authorization: Bearer error-secret"),
		})
	})

	for _, secret := range []string{
		"payload-secret",
		"buyer@example.com",
		"query-secret",
		"203.0.113.42",
		"signature-secret",
		"evt_private_12345678",
		"order_private_87654321",
		"error-secret",
	} {
		require.NotContains(t, output, secret)
	}

	for _, expected := range []string{
		"request-visible",
		`"component":"payment"`,
		`"provider":"stripe"`,
		`"event":"signature_invalid"`,
		`"method":"POST"`,
		`"path":"/api/stripe/webhook"`,
		`"payload_bytes":`,
		`"payload_sha256":"`,
		`"signature_present":true`,
		`"signature_sha256":"`,
		`"event_ref":"*12345678"`,
		`"order_ref":"*87654321"`,
		`"order_status":"paid"`,
		`"error_type":"*errors.errorString"`,
		`"error_sha256":"`,
		`"client_ip_sha256":"`,
	} {
		require.Contains(t, output, expected)
	}
}

func TestPaymentSecurityLogSourceDoesNotReintroduceRawSecrets(t *testing.T) {
	files := []string{
		"topup.go",
		"topup_stripe.go",
		"topup_creem.go",
		"topup_waffo.go",
		"topup_waffo_pancake.go",
		"subscription_payment_epay.go",
	}
	forbidden := []string{
		"c.Request.RequestURI",
		"signature=%q",
		"body=%q",
		"params=%q",
		"verify_info=%q",
		"topup=%q",
		"customer_email=%q",
		"customer_name=%q",
		"checkout_url=%q",
		"response=%q",
		"string(bodyBytes)",
		"common.GetJsonString(params)",
		"common.GetJsonString(verifyInfo)",
	}

	for _, file := range files {
		path := filepath.Join(".", file)
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, body, 0)
		require.NoError(t, err)
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isSecuritySensitiveLogCall(call) {
				return true
			}
			start := fileSet.Position(call.Pos()).Offset
			end := fileSet.Position(call.End()).Offset
			logCall := string(body[start:end])
			for _, fragment := range forbidden {
				if strings.Contains(logCall, fragment) {
					t.Errorf("%s contains forbidden payment-log fragment %q", file, fragment)
				}
			}
			return true
		})
	}
}

func isSecuritySensitiveLogCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	return (identifier.Name == "logger" && strings.HasPrefix(selector.Sel.Name, "Log")) ||
		(identifier.Name == "common" && selector.Sel.Name == "SysLog")
}
