package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const funnelRateWindowSeconds int64 = 60

var (
	funnelUnauthorizedLimiter = newFunnelMemoryLimiter()
	funnelWriteLimiter        = newFunnelMemoryLimiter()
	funnelReadLimiter         = newFunnelMemoryLimiter()
)

func newFunnelMemoryLimiter() *common.InMemoryRateLimiter {
	limiter := &common.InMemoryRateLimiter{}
	limiter.Init(common.RateLimitKeyExpirationDuration)
	return limiter
}

func GeiliFunnelSecretAuth() gin.HandlerFunc {
	config, configErr := service.LoadGeiliFunnelConfig()
	return func(c *gin.Context) {
		if configErr != nil || !config.Enabled {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}

		values := c.Request.Header.Values("Authorization")
		bearer := ""
		if len(values) == 1 && strings.HasPrefix(values[0], "Bearer ") {
			bearer = strings.TrimPrefix(values[0], "Bearer ")
		}
		expectedDigest := sha256.Sum256([]byte(config.Secret))
		submittedDigest := sha256.Sum256([]byte(bearer))
		if bearer == "" || subtle.ConstantTimeCompare(expectedDigest[:], submittedDigest[:]) != 1 {
			if !allowFunnelRequest(c, funnelUnauthorizedLimiter, "rateLimit:GFU:"+c.ClientIP(), config.UnauthorizedRPM) {
				abortFunnelRateLimit(c)
				return
			}
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}

func GeiliFunnelWriteRateLimit() gin.HandlerFunc {
	config, configErr := service.LoadGeiliFunnelConfig()
	return fixedFunnelRateLimit(configErr, config.Enabled, funnelWriteLimiter, "rateLimit:GFW:service", config.WriteRPM)
}

func GeiliFunnelReadRateLimit() gin.HandlerFunc {
	config, configErr := service.LoadGeiliFunnelConfig()
	return fixedFunnelRateLimit(configErr, config.Enabled, funnelReadLimiter, "rateLimit:GFR:service", config.ReadRPM)
}

func fixedFunnelRateLimit(configErr error, enabled bool, limiter *common.InMemoryRateLimiter, key string, rpm int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if configErr != nil || !enabled {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		if !allowFunnelRequest(c, limiter, key, rpm) {
			abortFunnelRateLimit(c)
			return
		}
		c.Next()
	}
}

func allowFunnelRequest(c *gin.Context, limiter *common.InMemoryRateLimiter, key string, rpm int) bool {
	if common.RedisEnabled {
		allowed, err := common.RDB.Eval(
			context.Background(),
			`local n = redis.call('INCR', KEYS[1]); if n == 1 then redis.call('EXPIRE', KEYS[1], ARGV[2]) end; if n > tonumber(ARGV[1]) then return 0 end; return 1`,
			[]string{key}, rpm, funnelRateWindowSeconds,
		).Int()
		if err != nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return false
		}
		return allowed == 1
	}
	return limiter.Request(key, rpm, funnelRateWindowSeconds)
}

func abortFunnelRateLimit(c *gin.Context) {
	if c.IsAborted() {
		return
	}
	c.Header("Retry-After", "60")
	c.AbortWithStatus(http.StatusTooManyRequests)
}

func GeiliFunnelNoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		c.Header("Pragma", "no-cache")
		c.Next()
	}
}
