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
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestSSORedirectV2StoresOnlyMinimalAudienceBoundMetadata(t *testing.T) {
	t.Setenv(ssoV2EnabledEnv, "true")
	db := setupUserControllerTestDB(t)
	redisServer := setupSSOExchangeRedis(t)
	user := seedSSOExchangeUser(t, db, common.RoleCommonUser, "must-not-enter-ticket")

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/sso/v2?audience=studio&redirect_uri=https%3A%2F%2Fstudio.geiliapi.com%2Fauth%2Fsso-callback", nil)
	ctx.Set("id", user.Id)

	SSORedirectV2(ctx)

	require.Equal(t, http.StatusFound, recorder.Code, recorder.Body.String())
	location, err := url.Parse(recorder.Header().Get("Location"))
	require.NoError(t, err)
	ticket := location.Query().Get("ticket")
	require.GreaterOrEqual(t, len(ticket), 24)
	require.Equal(t, ssoStudioAudience, location.Query().Get("audience"))
	require.Empty(t, location.Query().Get("code"))
	require.NotContains(t, location.String(), "must-not-enter-ticket")

	payloadJSON, err := common.RedisGet(ssoTicketRedisPrefix + ticket)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal([]byte(payloadJSON), &payload))
	require.ElementsMatch(t, []string{"user_id", "audience", "expires_at", "nonce"}, mapKeys(payload))
	require.Equal(t, float64(user.Id), payload["user_id"])
	require.Equal(t, ssoStudioAudience, payload["audience"])
	require.NotEmpty(t, payload["nonce"])
	require.NotContains(t, payloadJSON, "access_token")
	require.NotContains(t, payloadJSON, "must-not-enter-ticket")
	require.GreaterOrEqual(t, redisServer.TTL(ssoTicketRedisPrefix+ticket), 59*time.Second)
	require.LessOrEqual(t, redisServer.TTL(ssoTicketRedisPrefix+ticket), ssoTicketTTL)
}

func TestSSORedirectV2RejectsUnsafeRedirectAndAudience(t *testing.T) {
	t.Setenv(ssoV2EnabledEnv, "true")
	setupUserControllerTestDB(t)
	setupSSOExchangeRedis(t)

	for _, rawURL := range []string{
		"/api/user/sso/v2?audience=studio&redirect_uri=http%3A%2F%2Fstudio.geiliapi.com%2Fcallback",
		"/api/user/sso/v2?audience=studio&redirect_uri=https%3A%2F%2Fevil.example%2Fcallback",
		"/api/user/sso/v2?audience=admin&redirect_uri=https%3A%2F%2Fstudio.geiliapi.com%2Fcallback",
	} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, rawURL, nil)
		ctx.Set("id", 1)
		SSORedirectV2(ctx)
		require.Contains(t, []int{http.StatusBadRequest, http.StatusForbidden}, recorder.Code, rawURL)
		require.Empty(t, recorder.Header().Get("Location"), rawURL)
	}
}

func TestSSOExchangeV2ReturnsIdentityWithoutAccessTokenAndPreventsReplay(t *testing.T) {
	t.Setenv(ssoV2EnabledEnv, "true")
	db := setupUserControllerTestDB(t)
	setupSSOExchangeRedis(t)
	admin := seedSSOExchangeUser(t, db, common.RoleAdminUser, "admin-access-token")
	target := seedSSOExchangeUser(t, db, common.RoleCommonUser, "target-access-token")
	ticket := "ticket-valid-for-studio-123456789"
	storeSSOV2TestTicket(t, ticket, target.Id, ssoStudioAudience, time.Now().Add(time.Minute))
	engine := setupSSOExchangeV2Router()

	first := exchangeSSOV2Request(engine, admin, ticket, ssoStudioAudience)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.NotContains(t, first.Body.String(), "access_token")
	require.NotContains(t, first.Body.String(), "target-access-token")
	var response SSOExchangeV2Response
	require.NoError(t, common.Unmarshal(first.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, target.Id, response.UserID)
	require.Equal(t, ssoStudioAudience, response.Audience)

	second := exchangeSSOV2Request(engine, admin, ticket, ssoStudioAudience)
	require.Equal(t, http.StatusUnauthorized, second.Code, second.Body.String())
}

func TestSSOExchangeV2RejectsExpiredOrWrongAudienceTickets(t *testing.T) {
	t.Setenv(ssoV2EnabledEnv, "true")
	db := setupUserControllerTestDB(t)
	setupSSOExchangeRedis(t)
	admin := seedSSOExchangeUser(t, db, common.RoleAdminUser, "admin-access-token")
	target := seedSSOExchangeUser(t, db, common.RoleCommonUser, "target-access-token")
	engine := setupSSOExchangeV2Router()

	storeSSOV2TestTicket(t, "expired-ticket-123456789", target.Id, ssoStudioAudience, time.Now().Add(-time.Second))
	expired := exchangeSSOV2Request(engine, admin, "expired-ticket-123456789", ssoStudioAudience)
	require.Equal(t, http.StatusUnauthorized, expired.Code, expired.Body.String())

	storeSSOV2TestTicket(t, "wrong-audience-ticket-123456", target.Id, ssoStudioAudience, time.Now().Add(time.Minute))
	wrongAudience := exchangeSSOV2Request(engine, admin, "wrong-audience-ticket-123456", "admin")
	require.Equal(t, http.StatusUnauthorized, wrongAudience.Code, wrongAudience.Body.String())
	_, err := common.RedisGet(ssoTicketRedisPrefix + "wrong-audience-ticket-123456")
	require.ErrorIs(t, err, redis.Nil, "a mismatched ticket must be burned to prevent audience probing")
}

func TestSSOExchangeV2FailsClosedWhenRedisIsUnavailable(t *testing.T) {
	t.Setenv(ssoV2EnabledEnv, "true")
	previous := common.RDB
	common.RDB = nil
	t.Cleanup(func() { common.RDB = previous })

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/sso/v2/exchange", strings.NewReader(`{"ticket":"ticket-long-enough-123456","audience":"studio"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	SSOExchangeV2(ctx)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "redis client")
}

func storeSSOV2TestTicket(t *testing.T, ticket string, userID int, audience string, expiresAt time.Time) {
	t.Helper()
	payload, err := common.Marshal(ssoTicketPayloadV2{
		UserID:    userID,
		Audience:  audience,
		ExpiresAt: expiresAt.Unix(),
		Nonce:     "test-nonce-123456789",
	})
	require.NoError(t, err)
	require.NoError(t, common.RedisSet(ssoTicketRedisPrefix+ticket, string(payload), time.Minute))
}

func setupSSOExchangeV2Router() *gin.Engine {
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("sso-v2-exchange-test"))))
	engine.POST("/api/user/sso/v2/exchange", middleware.AdminAuth(), SSOExchangeV2)
	return engine
}

func exchangeSSOV2Request(engine *gin.Engine, admin model.User, ticket string, audience string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	body := fmt.Sprintf(`{"ticket":%q,"audience":%q}`, ticket, audience)
	request := httptest.NewRequest(http.MethodPost, "/api/user/sso/v2/exchange", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+admin.GetAccessToken())
	request.Header.Set("New-Api-User", fmt.Sprint(admin.Id))
	engine.ServeHTTP(recorder, request)
	return recorder
}

func mapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
