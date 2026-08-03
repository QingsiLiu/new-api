package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFunnelRouterTestDB(t *testing.T) {
	t.Helper()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(
		&model.Option{}, &model.User{}, &model.TopUp{}, &model.Task{},
		&model.FunnelVisitor{}, &model.FunnelEvent{}, &model.FunnelActivityDay{},
	))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestGeiliFunnelDedicatedRouterBypassesGlobalIPBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupFunnelRouterTestDB(t)
	secret := strings.Repeat("s", 32)
	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", secret)
	t.Setenv("GEILI_FUNNEL_WRITE_RPM", "2")

	previousEnabled := common.GlobalApiRateLimitEnable
	previousCount := common.GlobalApiRateLimitNum
	previousDuration := common.GlobalApiRateLimitDuration
	common.GlobalApiRateLimitEnable = true
	common.GlobalApiRateLimitNum = 1
	common.GlobalApiRateLimitDuration = 60
	t.Cleanup(func() {
		common.GlobalApiRateLimitEnable = previousEnabled
		common.GlobalApiRateLimitNum = previousCount
		common.GlobalApiRateLimitDuration = previousDuration
	})

	router := gin.New()
	ordinary := router.Group("/ordinary", middlewareForGlobalAPI())
	ordinary.GET("/status", func(c *gin.Context) { c.Status(http.StatusOK) })
	SetGeiliFunnelRouter(router)

	request := func(method string, path string, body string, auth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "192.0.2.60:1234"
		if auth {
			req.Header.Set("Authorization", "Bearer "+secret)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	require.Equal(t, http.StatusOK, request(http.MethodGet, "/ordinary/status", "", false).Code)
	require.Equal(t, http.StatusTooManyRequests, request(http.MethodGet, "/ordinary/status", "", false).Code)
	require.Equal(t, http.StatusNoContent, request(http.MethodPost, "/api/geili/funnel/events", routerFunnelBody("7dfb2d2c-7f40-4f39-b8f4-5fb27db06041"), true).Code)
	require.Equal(t, http.StatusNoContent, request(http.MethodPost, "/api/geili/funnel/events", routerFunnelBody("7dfb2d2c-7f40-4f39-b8f4-5fb27db06042"), true).Code)
}

func middlewareForGlobalAPI() gin.HandlerFunc {
	return middleware.GlobalAPIRateLimit()
}

func routerFunnelBody(eventID string) string {
	return `{"event_id":"` + eventID + `","event":"slp_view","version":1,"environment":"production","visitor_hmac":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","locale":"zh","model":"gpt-image-2"}`
}
