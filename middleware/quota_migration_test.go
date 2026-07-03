package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func withQuotaMigrationFlag(t *testing.T, value string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	previous, hadPrevious := common.OptionMap[common.QuotaMigrationInProgressKey]
	common.OptionMap[common.QuotaMigrationInProgressKey] = value
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if hadPrevious {
			common.OptionMap[common.QuotaMigrationInProgressKey] = previous
			return
		}
		delete(common.OptionMap, common.QuotaMigrationInProgressKey)
	})
}

func performQuotaMigrationGuardRequest(method string, path string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(QuotaMigrationGuard())
	router.Any("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, nil)
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestQuotaMigrationGuardBlocksBillingWritesWhenFlagEnabled(t *testing.T) {
	withQuotaMigrationFlag(t, "true")

	recorder := performQuotaMigrationGuardRequest(http.MethodPost, "/v1/images/tasks")

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "quota_migration_in_progress")
}

func TestQuotaMigrationGuardAllowsReadsAndNonBillingPaths(t *testing.T) {
	withQuotaMigrationFlag(t, "true")

	require.Equal(t, http.StatusNoContent, performQuotaMigrationGuardRequest(http.MethodGet, "/v1/tasks/task-1").Code)
	require.Equal(t, http.StatusNoContent, performQuotaMigrationGuardRequest(http.MethodPost, "/login").Code)
}

func TestQuotaMigrationGuardAllowsWritesWhenFlagDisabled(t *testing.T) {
	withQuotaMigrationFlag(t, "false")

	recorder := performQuotaMigrationGuardRequest(http.MethodPost, "/api/user/self/pay")

	require.Equal(t, http.StatusNoContent, recorder.Code)
}
