package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type manageUserAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupUserControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserQuotaChangeRecord{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestManageUserQuotaIsIdempotentWithRequestID(t *testing.T) {
	db := setupUserControllerTestDB(t)

	target := &model.User{
		Username:    "bridge-user",
		Password:    "hash",
		DisplayName: "Bridge User",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       10,
		Group:       "default",
	}
	require.NoError(t, db.Create(target).Error)

	reqBody := `{"id":%d,"action":"add_quota","mode":"add","value":7,"request_id":"req-123"}`
	call := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/manage", strings.NewReader(fmt.Sprintf(reqBody, target.Id)))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Set("id", 1)
		ctx.Set("role", common.RoleRootUser)
		ManageUser(ctx)
		return rec
	}

	first := call()
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	second := call()
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())

	got, err := model.GetUserById(target.Id, false)
	require.NoError(t, err)
	require.Equal(t, 17, got.Quota)

	var records []model.UserQuotaChangeRecord
	require.NoError(t, db.Order("id asc").Find(&records).Error)
	require.Len(t, records, 1)
	require.Equal(t, "req-123", records[0].RequestId)
	require.Equal(t, 10, records[0].BeforeQuota)
	require.Equal(t, 17, records[0].AfterQuota)
}

func TestGetSelfReturnsCNYBalancesWithoutRawQuota(t *testing.T) {
	db := setupUserControllerTestDB(t)

	user := &model.User{
		Username:        "cny-self-user",
		Password:        "hash",
		DisplayName:     "CNY User",
		Role:            common.RoleCommonUser,
		Status:          common.UserStatusEnabled,
		Quota:           110000,
		UsedQuota:       242550,
		AffQuota:        333333,
		AffHistoryQuota: 444444,
		Group:           "default",
	}
	require.NoError(t, db.Create(user).Error)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	ctx.Set("id", user.Id)
	ctx.Set("role", common.RoleCommonUser)

	GetSelf(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var response struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success)

	require.Equal(t, "CNY", response.Data["currency"])
	require.InDelta(t, 1.1, response.Data["balance_cny"], 0.000001)
	require.InDelta(t, 2.4255, response.Data["used_cny"], 0.000001)
	require.InDelta(t, 3.3333, response.Data["aff_balance_cny"], 0.000001)
	require.InDelta(t, 4.4444, response.Data["aff_history_cny"], 0.000001)
	require.NotContains(t, response.Data, "quota")
	require.NotContains(t, response.Data, "used_quota")
	require.NotContains(t, response.Data, "aff_quota")
	require.NotContains(t, response.Data, "aff_history_quota")
}

func withPaymentCompliance(t *testing.T, enabled bool) {
	t.Helper()
	setting := operation_setting.GetPaymentSetting()
	previous := *setting
	setting.ComplianceConfirmed = enabled
	if enabled {
		setting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	} else {
		setting.ComplianceTermsVersion = ""
	}
	t.Cleanup(func() { *setting = previous })
}

func TestTransferAllAffQuotaReturnsOnlyPublicCNY(t *testing.T) {
	db := setupUserControllerTestDB(t)
	withPaymentCompliance(t, true)
	user := &model.User{Username: "aff-controller", Password: "hash", Status: common.UserStatusEnabled, Quota: 925000, AffQuota: 333333}
	require.NoError(t, db.Create(user).Error)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/aff_transfer_all", strings.NewReader(`{}`))
	ctx.Set("id", user.Id)
	TransferAllAffQuota(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	var response struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "CNY", response.Data["currency"])
	require.InDelta(t, 3.3333, response.Data["transferred_cny"], 0.000001)
	require.InDelta(t, 12.5833, response.Data["balance_cny"], 0.000001)
	require.Zero(t, response.Data["aff_balance_cny"])
	for _, forbidden := range []string{"quota", "aff_quota", "transferred_quota", "balance_quota"} {
		require.NotContains(t, response.Data, forbidden)
	}
}

func TestTransferAllAffQuotaRequiresPaymentComplianceWithoutMutation(t *testing.T) {
	db := setupUserControllerTestDB(t)
	withPaymentCompliance(t, false)
	user := &model.User{Username: "aff-compliance", Password: "hash", Status: common.UserStatusEnabled, Quota: 125000, AffQuota: 250000}
	require.NoError(t, db.Create(user).Error)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/aff_transfer_all", strings.NewReader(`{}`))
	ctx.Set("id", user.Id)
	TransferAllAffQuota(ctx)

	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.False(t, response.Success)

	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Equal(t, 125000, stored.Quota)
	require.Equal(t, 250000, stored.AffQuota)
}

func TestTransferAllAffQuotaRejectsNonEmptyOrMalformedBodiesWithoutMutation(t *testing.T) {
	for name, body := range map[string]string{
		"client quota":  `{"quota":1}`,
		"trailing JSON": `{} {}`,
		"over one KiB":  "{" + strings.Repeat(" ", 1024) + "}",
	} {
		t.Run(name, func(t *testing.T) {
			db := setupUserControllerTestDB(t)
			withPaymentCompliance(t, true)
			user := &model.User{Username: "aff-invalid-" + strings.ReplaceAll(name, " ", "-"), Password: "hash", Status: common.UserStatusEnabled, Quota: 125000, AffQuota: 250000}
			require.NoError(t, db.Create(user).Error)

			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/aff_transfer_all", strings.NewReader(body))
			ctx.Set("id", user.Id)
			TransferAllAffQuota(ctx)

			var response struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
			require.False(t, response.Success)

			var stored model.User
			require.NoError(t, db.First(&stored, user.Id).Error)
			require.Equal(t, 125000, stored.Quota)
			require.Equal(t, 250000, stored.AffQuota)
		})
	}
}

func TestTransferAllAffQuotaDoesNotExposeInternalErrors(t *testing.T) {
	setupUserControllerTestDB(t)
	withPaymentCompliance(t, true)
	require.NoError(t, i18n.Init())

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/aff_transfer_all", strings.NewReader(`{}`))
	ctx.Set("id", 999999)
	TransferAllAffQuota(ctx)

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.NotEmpty(t, response.Message)
	require.NotEqual(t, i18n.MsgUserTransferFailed, response.Message)
	require.NotContains(t, response.Message, "record not found")
	require.NotContains(t, response.Message, "<no value>")
	require.NotContains(t, response.Message, "{{")
}

func TestSSOExchangeAcceptsAdminAccessTokenAndConsumesCode(t *testing.T) {
	db := setupUserControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	setupSSOExchangeRedis(t)

	admin := seedSSOExchangeUser(t, db, common.RoleAdminUser, "admin-access-token")
	target := seedSSOExchangeUser(t, db, common.RoleCommonUser, "target-access-token")
	require.NoError(t, common.RedisSet("sso_code:valid-code", fmt.Sprintf("%d:%s", target.Id, target.GetAccessToken()), 60*time.Second))

	engine := setupSSOExchangeRouter()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/sso/exchange", strings.NewReader(`{"code":"valid-code"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+admin.GetAccessToken())
	request.Header.Set("New-Api-User", fmt.Sprint(admin.Id))

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response SSOExchangeResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)
	require.Equal(t, target.Id, response.UserID)
	require.Equal(t, target.GetAccessToken(), response.AccessToken)

	_, err := common.RedisGet("sso_code:valid-code")
	require.Error(t, err, "SSO code should be consumed after a successful exchange")
}

func TestSSOExchangeRejectsAPIKeyTokenBeforeCodeLookup(t *testing.T) {
	db := setupUserControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	setupSSOExchangeRedis(t)

	user := seedSSOExchangeUser(t, db, common.RoleCommonUser, "user-access-token")
	token := model.Token{
		UserId:         user.Id,
		Name:           "api-key",
		Key:            "api-key-token",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	require.NoError(t, db.Create(&token).Error)
	require.NoError(t, common.RedisSet("sso_code:api-key-code", fmt.Sprintf("%d:%s", user.Id, user.GetAccessToken()), 60*time.Second))

	engine := setupSSOExchangeRouter()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/sso/exchange", strings.NewReader(`{"code":"api-key-code"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token.GetFullKey())

	engine.ServeHTTP(recorder, request)

	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response["success"].(bool), recorder.Body.String())
	stored, err := common.RedisGet("sso_code:api-key-code")
	require.NoError(t, err)
	require.NotEmpty(t, stored, "invalid service auth must not consume SSO code")
}

func TestSSOExchangeRejectsMissingNewAPIUserHeaderBeforeCodeLookup(t *testing.T) {
	db := setupUserControllerTestDB(t)
	setupSSOExchangeRedis(t)

	admin := seedSSOExchangeUser(t, db, common.RoleAdminUser, "admin-access-token")
	require.NoError(t, common.RedisSet("sso_code:missing-user", "123:target-access-token", 60*time.Second))

	engine := setupSSOExchangeRouter()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/sso/exchange", strings.NewReader(`{"code":"missing-user"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+admin.GetAccessToken())

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	stored, err := common.RedisGet("sso_code:missing-user")
	require.NoError(t, err)
	require.NotEmpty(t, stored, "missing New-Api-User must not consume SSO code")
}

func TestSSORedirectStoresOneTimeCodeForSixtySeconds(t *testing.T) {
	db := setupUserControllerTestDB(t)
	redisServer := setupSSOExchangeRedis(t)
	user := seedSSOExchangeUser(t, db, common.RoleCommonUser, "")

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/sso?redirect_uri=https%3A%2F%2Fstudio.geiliapi.com%2Fauth%2Fsso-callback", nil)
	ctx.Set("id", user.Id)

	SSORedirect(ctx)

	require.Equal(t, http.StatusFound, recorder.Code, recorder.Body.String())
	location := recorder.Header().Get("Location")
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	code := parsed.Query().Get("code")
	require.NotEmpty(t, code)

	ttl := redisServer.TTL("sso_code:" + code)
	require.GreaterOrEqual(t, ttl, 59*time.Second)
	require.LessOrEqual(t, ttl, 60*time.Second)
}

func setupSSOExchangeRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	server := miniredis.RunT(t)
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
	})
	return server
}

func seedSSOExchangeUser(t *testing.T, db *gorm.DB, role int, accessToken string) model.User {
	t.Helper()

	user := model.User{
		Username:    fmt.Sprintf("sso-user-%d-%s", role, accessToken),
		Password:    "password",
		DisplayName: "SSO User",
		Role:        role,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     fmt.Sprintf("aff-%d-%s", role, strings.ReplaceAll(accessToken, "-", "")),
	}
	user.SetAccessToken(accessToken)
	require.NoError(t, db.Create(&user).Error)
	return user
}

func setupSSOExchangeRouter() *gin.Engine {
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("sso-exchange-test"))))
	engine.POST("/api/user/sso/exchange", middleware.AdminAuth(), SSOExchange)
	return engine
}
