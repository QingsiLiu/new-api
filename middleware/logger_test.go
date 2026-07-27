package middleware

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLoggerNeverIncludesRawQuery(t *testing.T) {
	const (
		requestID  = "edge-query-safe-0123456789"
		queryName  = "turnstile_canary_name"
		queryValue = "turnstile_canary_value_7f93"
	)
	line := formatRequestLog(gin.LogFormatterParams{
		TimeStamp:  time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC),
		StatusCode: http.StatusOK,
		Latency:    time.Millisecond,
		ClientIP:   "127.0.0.1",
		Method:     http.MethodPost,
		Path:       "/api/user/login?" + queryName + "=" + queryValue + "&encoded=" + strings.ReplaceAll(queryValue, "_", "%5F"),
		Keys: map[string]any{
			common.RequestIdKey: requestID,
			RouteTagKey:         "api",
		},
	})

	require.Contains(t, line, requestID)
	require.Contains(t, line, http.MethodPost)
	require.Contains(t, line, "/api/user/login")
	require.Contains(t, line, "200")
	require.NotContains(t, line, "?")
	require.NotContains(t, line, queryName)
	require.NotContains(t, line, queryValue)
	require.NotContains(t, line, "%5F")
}
