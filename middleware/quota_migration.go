package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func QuotaMigrationGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !shouldBlockDuringQuotaMigration(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}
		if model.IsQuotaMigrationInProgress() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"message": "quota migration in progress; billing is temporarily unavailable",
				"error": gin.H{
					"message": "quota migration in progress; billing is temporarily unavailable",
					"type":    "new_api_error",
					"code":    "quota_migration_in_progress",
				},
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func shouldBlockDuringQuotaMigration(method string, path string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	switch {
	case strings.HasPrefix(path, "/api/"):
		return true
	case strings.HasPrefix(path, "/v1/"):
		return true
	case strings.HasPrefix(path, "/v1beta/"):
		return true
	case strings.HasPrefix(path, "/mj/"):
		return true
	case strings.Contains(path, "/mj/"):
		return true
	case strings.HasPrefix(path, "/suno/"):
		return true
	case strings.HasPrefix(path, "/kling/"):
		return true
	case strings.HasPrefix(path, "/jimeng"):
		return true
	default:
		return false
	}
}
