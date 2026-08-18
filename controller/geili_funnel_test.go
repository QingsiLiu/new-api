package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const validFunnelEventBody = `{"event_id":"7dfb2d2c-7f40-4f39-b8f4-5fb27db06041","event":"slp_view","version":1,"environment":"production","visitor_hmac":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","locale":"zh","model":"gpt-image-2"}`

func setupFunnelControllerTestDB(t *testing.T) {
	t.Helper()
	setupModelRegistryTestDB(t)
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	model.LOG_DB = model.DB
	require.NoError(t, model.DB.AutoMigrate(
		&model.Option{}, &model.User{}, &model.Token{}, &model.TopUp{}, &model.Task{}, &model.Log{},
		&model.FunnelVisitor{}, &model.FunnelEvent{}, &model.FunnelActivityDay{},
	))
}

func funnelControllerRouter(t *testing.T, secret string) *gin.Engine {
	t.Helper()
	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", secret)
	t.Setenv("GEILI_FUNNEL_UNAUTHORIZED_RPM", "100000")
	t.Setenv("GEILI_FUNNEL_WRITE_RPM", "100000")
	t.Setenv("GEILI_FUNNEL_READ_RPM", "100000")
	router := gin.New()
	router.Use(middleware.GeiliFunnelNoStore())
	router.POST(
		"/events",
		middleware.FixedRequestBodyLimit(2048),
		middleware.GeiliFunnelSecretAuth(),
		middleware.GeiliFunnelWriteRateLimit(),
		IngestGeiliFunnelEvent,
	)
	router.GET(
		"/summary",
		middleware.GeiliFunnelSecretAuth(),
		middleware.GeiliFunnelReadRateLimit(),
		GetGeiliFunnelSummary,
	)
	router.GET(
		"/health",
		middleware.GeiliFunnelSecretAuth(),
		middleware.GeiliFunnelReadRateLimit(),
		GetGeiliFunnelHealth,
	)
	return router
}

func getFunnelController(router http.Handler, secret, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.RemoteAddr = "192.0.2.51:1234"
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func postFunnelController(router http.Handler, secret string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.RemoteAddr = "192.0.2.50:1234"
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestGeiliFunnelIngestStrictMatrix(t *testing.T) {
	setupFunnelControllerTestDB(t)
	secret := strings.Repeat("s", 32)
	router := funnelControllerRouter(t, secret)

	require.Equal(t, http.StatusNoContent, postFunnelController(router, secret, validFunnelEventBody).Code)
	require.Equal(t, http.StatusNoContent, postFunnelController(router, secret, validFunnelEventBody).Code)
	var count int64
	require.NoError(t, model.DB.Model(&model.FunnelEvent{}).Count(&count).Error)
	require.EqualValues(t, 1, count)

	cases := []struct {
		name   string
		body   string
		status int
	}{
		{"unknown key", strings.TrimSuffix(validFunnelEventBody, "}") + `,"prompt":"private"}`, http.StatusBadRequest},
		{"unknown event", strings.Replace(validFunnelEventBody, `"slp_view"`, `"unknown"`, 1), http.StatusUnprocessableEntity},
		{"unsupported version", strings.Replace(validFunnelEventBody, `"version":1`, `"version":2`, 1), http.StatusUnprocessableEntity},
		{"forbidden user id", strings.TrimSuffix(validFunnelEventBody, "}") + `,"user_id":0}`, http.StatusUnprocessableEntity},
		{"missing model", strings.Replace(validFunnelEventBody, `,"model":"gpt-image-2"`, "", 1), http.StatusUnprocessableEntity},
		{"malformed", `{`, http.StatusBadRequest},
	}
	for i, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			body := strings.Replace(test.body, "7dfb2d2c-7f40-4f39-b8f4-5fb27db06041", fmt.Sprintf("7dfb2d2c-7f40-4f39-b8f4-5fb27db0604%d", i+2), 1)
			require.Equal(t, test.status, postFunnelController(router, secret, body).Code)
		})
	}

	require.Equal(t, http.StatusUnauthorized, postFunnelController(router, "wrong", validFunnelEventBody).Code)
	large := strings.Repeat("x", 2049)
	largeResponse := postFunnelController(router, secret, large)
	require.Equal(t, http.StatusRequestEntityTooLarge, largeResponse.Code)
	require.Equal(t, "private, no-store", largeResponse.Header().Get("Cache-Control"))
}

func TestGeiliFunnelStoresSourceHostWithoutUserIdentifiers(t *testing.T) {
	setupFunnelControllerTestDB(t)
	secret := strings.Repeat("s", 32)
	router := funnelControllerRouter(t, secret)
	body := strings.Replace(
		validFunnelEventBody,
		`"model":"gpt-image-2"`,
		`"model":"gpt-image-2","source_host":"geiliapi.com"`,
		1,
	)
	require.Equal(t, http.StatusNoContent, postFunnelController(router, secret, body).Code)
	var event model.FunnelEvent
	require.NoError(t, model.DB.First(&event).Error)
	require.Equal(t, "geiliapi.com", event.SourceHost)
}

func TestGeiliFunnelIngestRejectsInvalidEnabledConfig(t *testing.T) {
	setupFunnelControllerTestDB(t)
	router := funnelControllerRouter(t, "short")
	require.Equal(t, http.StatusServiceUnavailable, postFunnelController(router, "short", validFunnelEventBody).Code)
}

func TestGeiliFunnelSummaryQueryParsingAndUTCDefaults(t *testing.T) {
	setupFunnelControllerTestDB(t)
	secret := strings.Repeat("s", 32)
	router := funnelControllerRouter(t, secret)
	fixedNow := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	previousNow := geiliFunnelCurrentTime
	geiliFunnelCurrentTime = func() time.Time { return fixedNow }
	t.Cleanup(func() { geiliFunnelCurrentTime = previousNow })

	defaultResponse := getFunnelController(router, secret, "/summary")
	require.Equal(t, http.StatusOK, defaultResponse.Code, defaultResponse.Body.String())
	var decoded dto.GeiliFunnelSummaryResponse
	require.NoError(t, json.Unmarshal(defaultResponse.Body.Bytes(), &decoded))
	require.Equal(t, "2026-06-23T00:00:00Z", decoded.Window.From)
	require.Equal(t, "2026-07-23T00:00:00Z", decoded.Window.To)
	require.Equal(t, "production", decoded.Window.Environment)
	require.Equal(t, "private, no-store", defaultResponse.Header().Get("Cache-Control"))

	inclusive := getFunnelController(router, secret, "/summary?from=2026-07-01&to=2026-07-22&environment=staging")
	require.Equal(t, http.StatusOK, inclusive.Code, inclusive.Body.String())
	require.NoError(t, json.Unmarshal(inclusive.Body.Bytes(), &decoded))
	require.Equal(t, "2026-07-01T00:00:00Z", decoded.Window.From)
	require.Equal(t, "2026-07-23T00:00:00Z", decoded.Window.To)
	require.Equal(t, "staging", decoded.Window.Environment)

	tooLongFrom := fixedNow.Truncate(24 * time.Hour).Add(-730 * 24 * time.Hour).Format("2006-01-02")
	invalidTargets := []string{
		"/summary?from=" + tooLongFrom + "&to=2026-07-22",
		"/summary?from=2026-07-22&to=2026-07-01",
		"/summary?from=not-a-date&to=2026-07-22",
		"/summary?from=2026-07-23&to=2026-07-23",
		"/summary?from=2026-07-01&to=2026-07-23",
		"/summary?dimension=campaign",
		"/summary?environment=preview",
		"/summary?value=private",
		"/summary?from=2026-07-01",
		"/summary?dimension=all&dimension=model",
	}
	for _, target := range invalidTargets {
		response := getFunnelController(router, secret, target)
		require.Equal(t, http.StatusBadRequest, response.Code, target+": "+response.Body.String())
	}
	require.Equal(t, http.StatusUnauthorized, getFunnelController(router, "wrong", "/summary").Code)
}

func TestGeiliFunnelSummaryLoaderFailureIsFixed503(t *testing.T) {
	setupFunnelControllerTestDB(t)
	secret := strings.Repeat("s", 32)
	router := funnelControllerRouter(t, secret)
	require.NoError(t, model.DB.Migrator().DropTable(&model.FunnelEvent{}))

	response := getFunnelController(router, secret, "/summary")
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Empty(t, response.Body.String())
}

func TestGeiliFunnelHealthEnvironmentAndMaintenanceStatus(t *testing.T) {
	setupFunnelControllerTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))
	secret := strings.Repeat("s", 32)
	router := funnelControllerRouter(t, secret)
	fixedNow := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	previousNow := geiliFunnelCurrentTime
	geiliFunnelCurrentTime = func() time.Time { return fixedNow }
	t.Cleanup(func() { geiliFunnelCurrentTime = previousNow })

	production := getFunnelController(router, secret, "/health")
	require.Equal(t, http.StatusOK, production.Code, production.Body.String())
	var decoded dto.GeiliFunnelHealthResponse
	require.NoError(t, json.Unmarshal(production.Body.Bytes(), &decoded))
	require.True(t, decoded.Healthy)
	require.Equal(t, "production", decoded.Environment)
	require.Equal(t, "pending_initial_run", decoded.Maintenance.Status)
	require.Len(t, decoded.Events, 5)

	staging := getFunnelController(router, secret, "/health?environment=staging")
	require.Equal(t, http.StatusOK, staging.Code, staging.Body.String())
	require.NoError(t, json.Unmarshal(staging.Body.Bytes(), &decoded))
	require.Equal(t, "staging", decoded.Environment)

	for _, target := range []string{
		"/health?environment=preview",
		"/health?environment=staging&environment=production",
		"/health?value=private",
	} {
		require.Equal(t, http.StatusBadRequest, getFunnelController(router, secret, target).Code, target)
	}
	require.Equal(t, http.StatusUnauthorized, getFunnelController(router, "wrong", "/health").Code)

	require.NoError(t, model.DB.Create(&model.Option{
		Key: "GeiliFunnelCollectionStartedAt.production", Value: fmt.Sprintf("%d", fixedNow.Unix()-40*3600),
	}).Error)
	stale := getFunnelController(router, secret, "/health")
	require.Equal(t, http.StatusServiceUnavailable, stale.Code, stale.Body.String())
	require.NoError(t, json.Unmarshal(stale.Body.Bytes(), &decoded))
	require.False(t, decoded.Healthy)
	require.Equal(t, "stale", decoded.Maintenance.Status)
}

func TestGeiliFunnelHealthSchemaFailureIsFixed503(t *testing.T) {
	setupFunnelControllerTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))
	secret := strings.Repeat("s", 32)
	router := funnelControllerRouter(t, secret)
	require.NoError(t, model.DB.Migrator().DropTable(&model.FunnelActivityDay{}))

	response := getFunnelController(router, secret, "/health")
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Empty(t, response.Body.String())
}
