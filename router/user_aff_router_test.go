package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAffTransferAllRouter(t *testing.T) (*gin.Engine, model.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousGlobalRateLimit := common.GlobalApiRateLimitEnable
	previousCriticalRateLimit := common.CriticalRateLimitEnable
	paymentSetting := operation_setting.GetPaymentSetting()
	previousPaymentSetting := *paymentSetting

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.GlobalApiRateLimitEnable = false
	common.CriticalRateLimitEnable = false
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Option{}))
	require.NoError(t, db.Create(&model.Option{Key: common.QuotaMigrationInProgressKey, Value: "false"}).Error)

	user := model.User{
		Username: "aff-router-user",
		Password: "hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    125000,
		AffQuota: 250000,
	}
	require.NoError(t, db.Create(&user).Error)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("aff-transfer-all-test"))))
	engine.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", user.Username)
		session.Set("role", user.Role)
		session.Set("id", user.Id)
		session.Set("status", user.Status)
		session.Set("group", user.Group)
		if err := session.Save(); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	SetApiRouter(engine)

	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedisEnabled
		common.GlobalApiRateLimitEnable = previousGlobalRateLimit
		common.CriticalRateLimitEnable = previousCriticalRateLimit
		*paymentSetting = previousPaymentSetting
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return engine, user
}

func TestAffTransferAllRouteUsesCriticalRateLimit(t *testing.T) {
	source, err := os.ReadFile("api-router.go")
	require.NoError(t, err)
	require.Contains(t, string(source), `selfRoute.POST("/aff_transfer_all", middleware.CriticalRateLimit(), controller.TransferAllAffQuota)`)
}

func TestAffTransferAllRouteRequiresRealSession(t *testing.T) {
	engine, user := setupAffTransferAllRouter(t)

	anonymousRecorder := httptest.NewRecorder()
	anonymousRequest := httptest.NewRequest(http.MethodPost, "/api/user/aff_transfer_all", strings.NewReader(`{}`))
	engine.ServeHTTP(anonymousRecorder, anonymousRequest)
	require.Equal(t, http.StatusUnauthorized, anonymousRecorder.Code, anonymousRecorder.Body.String())

	loginRecorder := httptest.NewRecorder()
	engine.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodGet, "/login", nil))
	require.Equal(t, http.StatusNoContent, loginRecorder.Code)

	authenticatedRecorder := httptest.NewRecorder()
	authenticatedRequest := httptest.NewRequest(http.MethodPost, "/api/user/aff_transfer_all", strings.NewReader(`{}`))
	authenticatedRequest.Header.Set("New-Api-User", fmt.Sprint(user.Id))
	for _, sessionCookie := range loginRecorder.Result().Cookies() {
		authenticatedRequest.AddCookie(sessionCookie)
	}
	engine.ServeHTTP(authenticatedRecorder, authenticatedRequest)
	require.Equal(t, http.StatusOK, authenticatedRecorder.Code, authenticatedRecorder.Body.String())

	var response struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(authenticatedRecorder.Body.Bytes(), &response))
	require.True(t, response.Success, authenticatedRecorder.Body.String())
	require.Equal(t, "CNY", response.Data["currency"])
	require.InDelta(t, 2.5, response.Data["transferred_cny"], 0.000001)
}
