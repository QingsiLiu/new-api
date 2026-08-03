package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func resetFunnelLimiters(t *testing.T) {
	t.Helper()
	previousRedisEnabled := common.RedisEnabled
	previousRedis := common.RDB
	common.RedisEnabled = false
	funnelUnauthorizedLimiter = newFunnelMemoryLimiter()
	funnelWriteLimiter = newFunnelMemoryLimiter()
	funnelReadLimiter = newFunnelMemoryLimiter()
	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRedis
	})
}

func configureFunnelMiddleware(t *testing.T) string {
	t.Helper()
	secret := strings.Repeat("s", 32)
	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", secret)
	t.Setenv("GEILI_FUNNEL_UNAUTHORIZED_RPM", "1")
	t.Setenv("GEILI_FUNNEL_WRITE_RPM", "1")
	t.Setenv("GEILI_FUNNEL_READ_RPM", "1")
	return secret
}

func TestGeiliFunnelUnauthorizedDoesNotConsumeAuthorizedBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetFunnelLimiters(t)
	secret := configureFunnelMiddleware(t)
	router := gin.New()
	router.Use(GeiliFunnelNoStore())
	router.POST("/events", GeiliFunnelSecretAuth(), GeiliFunnelWriteRateLimit(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	require.Equal(t, http.StatusUnauthorized, funnelMiddlewareRequest(router, "wrong").Code)
	require.Equal(t, http.StatusNoContent, funnelMiddlewareRequest(router, secret).Code)
	limited := funnelMiddlewareRequest(router, secret)
	require.Equal(t, http.StatusTooManyRequests, limited.Code)
	require.Equal(t, "60", limited.Header().Get("Retry-After"))
	require.Equal(t, "private, no-store", limited.Header().Get("Cache-Control"))
	require.NotContains(t, limited.Body.String(), secret)
}

func TestGeiliFunnelReadAndWriteBucketsAreIndependent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetFunnelLimiters(t)
	secret := configureFunnelMiddleware(t)
	router := gin.New()
	router.Use(GeiliFunnelNoStore())
	router.GET("/summary", GeiliFunnelSecretAuth(), GeiliFunnelReadRateLimit(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.POST("/events", GeiliFunnelSecretAuth(), GeiliFunnelWriteRateLimit(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	require.Equal(t, http.StatusNoContent, funnelMiddlewareMethodRequest(router, http.MethodGet, "/summary", secret).Code)
	require.Equal(t, http.StatusTooManyRequests, funnelMiddlewareMethodRequest(router, http.MethodGet, "/summary", secret).Code)
	require.Equal(t, http.StatusNoContent, funnelMiddlewareMethodRequest(router, http.MethodPost, "/events", secret).Code)
}

func TestGeiliFunnelAuthFailsClosedAndRejectsDuplicateHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetFunnelLimiters(t)
	secret := configureFunnelMiddleware(t)

	t.Setenv("GEILI_FUNNEL_ENABLED", "false")
	disabled := gin.New()
	disabled.POST("/events", GeiliFunnelSecretAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	require.Equal(t, http.StatusServiceUnavailable, funnelMiddlewareRequest(disabled, secret).Code)

	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", "short")
	invalid := gin.New()
	invalid.POST("/events", GeiliFunnelSecretAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	require.Equal(t, http.StatusServiceUnavailable, funnelMiddlewareRequest(invalid, secret).Code)

	t.Setenv("GEILI_FUNNEL_SECRET", secret)
	duplicate := gin.New()
	duplicate.POST("/events", GeiliFunnelSecretAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodPost, "/events", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Add("Authorization", "Bearer "+secret)
	req.Header.Add("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	duplicate.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGeiliFunnelRedisBucketsAreSharedAndIsolated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetFunnelLimiters(t)
	secret := configureFunnelMiddleware(t)
	mini := miniredis.RunT(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: mini.Addr()})
	common.RedisEnabled = true

	writeRouter := func() *gin.Engine {
		router := gin.New()
		router.POST("/events", GeiliFunnelSecretAuth(), GeiliFunnelWriteRateLimit(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		return router
	}
	first := writeRouter()
	second := writeRouter()
	require.Equal(t, http.StatusNoContent, funnelMiddlewareRequest(first, secret).Code)
	require.Equal(t, http.StatusTooManyRequests, funnelMiddlewareRequest(second, secret).Code)

	read := gin.New()
	read.GET("/summary", GeiliFunnelSecretAuth(), GeiliFunnelReadRateLimit(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	require.Equal(t, http.StatusNoContent, funnelMiddlewareMethodRequest(read, http.MethodGet, "/summary", secret).Code)
}

func funnelMiddlewareRequest(router http.Handler, secret string) *httptest.ResponseRecorder {
	return funnelMiddlewareMethodRequest(router, http.MethodPost, "/events", secret)
}

func funnelMiddlewareMethodRequest(router http.Handler, method string, path string, secret string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "192.0.2.10:1234"
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
