package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func performRequestIDRequest(headers map[string]string) (*httptest.ResponseRecorder, string) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var contextID string
	router.Use(RequestId())
	router.GET("/request-id", func(c *gin.Context) {
		contextID = c.GetString(common.RequestIdKey)
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/request-id", nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	router.ServeHTTP(recorder, request)
	return recorder, contextID
}

func TestRequestIDPreservesValidatedCanonicalHeader(t *testing.T) {
	recorder, contextID := performRequestIDRequest(map[string]string{
		common.ExternalRequestIdKey: "edge-0123456789",
		common.RequestIdKey:         "legacy-9876543210",
	})

	require.Equal(t, "edge-0123456789", contextID)
	require.Equal(t, contextID, recorder.Header().Get(common.ExternalRequestIdKey))
	require.Equal(t, contextID, recorder.Header().Get(common.RequestIdKey))
}

func TestRequestIDAcceptsValidatedLegacyHeader(t *testing.T) {
	recorder, contextID := performRequestIDRequest(map[string]string{
		common.RequestIdKey: "legacy-9876543210",
	})

	require.Equal(t, "legacy-9876543210", contextID)
	require.Equal(t, contextID, recorder.Header().Get(common.ExternalRequestIdKey))
}

func TestRequestIDReplacesInvalidInboundValue(t *testing.T) {
	recorder, contextID := performRequestIDRequest(map[string]string{
		common.ExternalRequestIdKey: strings.Repeat("x", 65),
		common.RequestIdKey:         "contains a space",
	})

	require.True(t, common.IsValidRequestId(contextID))
	require.NotEqual(t, strings.Repeat("x", 65), contextID)
	require.Equal(t, contextID, recorder.Header().Get(common.ExternalRequestIdKey))
}
