package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const validFunnelEventBody = `{"event_id":"7dfb2d2c-7f40-4f39-b8f4-5fb27db06041","event":"slp_view","version":1,"environment":"production","visitor_hmac":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","locale":"zh","model":"gpt-image-2"}`

func setupFunnelControllerTestDB(t *testing.T) {
	t.Helper()
	setupModelRegistryTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(
		&model.Option{}, &model.User{}, &model.TopUp{}, &model.Task{},
		&model.FunnelVisitor{}, &model.FunnelEvent{}, &model.FunnelActivityDay{},
	))
}

func funnelControllerRouter(t *testing.T, secret string) *gin.Engine {
	t.Helper()
	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", secret)
	t.Setenv("GEILI_FUNNEL_UNAUTHORIZED_RPM", "10")
	t.Setenv("GEILI_FUNNEL_WRITE_RPM", "20")
	router := gin.New()
	router.Use(middleware.GeiliFunnelNoStore())
	router.POST(
		"/events",
		middleware.FixedRequestBodyLimit(2048),
		middleware.GeiliFunnelSecretAuth(),
		middleware.GeiliFunnelWriteRateLimit(),
		IngestGeiliFunnelEvent,
	)
	return router
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

func TestGeiliFunnelIngestRejectsInvalidEnabledConfig(t *testing.T) {
	setupFunnelControllerTestDB(t)
	router := funnelControllerRouter(t, "short")
	require.Equal(t, http.StatusServiceUnavailable, postFunnelController(router, "short", validFunnelEventBody).Code)
}
