package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTurnstileCheckAcceptsHeaderAndLegacyQueryButRejectsConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousEnabled := common.TurnstileCheckEnabled
	previousSecret := common.TurnstileSecretKey
	previousURL := turnstileVerifyURL
	previousClient := turnstileVerifyClient
	common.TurnstileCheckEnabled = true
	common.TurnstileSecretKey = "test-only-secret"

	var mu sync.Mutex
	var responses []string
	verifier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		mu.Lock()
		responses = append(responses, r.Form.Get("response"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	turnstileVerifyURL = verifier.URL
	turnstileVerifyClient = verifier.Client()
	t.Cleanup(func() {
		verifier.Close()
		common.TurnstileCheckEnabled = previousEnabled
		common.TurnstileSecretKey = previousSecret
		turnstileVerifyURL = previousURL
		turnstileVerifyClient = previousClient
	})

	tests := []struct {
		name       string
		header     string
		query      string
		wantStatus int
		wantCalls  int
	}{
		{name: "header", header: "header-canary", wantStatus: http.StatusNoContent, wantCalls: 1},
		{name: "legacy query", query: "query-canary", wantStatus: http.StatusNoContent, wantCalls: 1},
		{name: "matching dual transport", header: "same-canary", query: "same-canary", wantStatus: http.StatusNoContent, wantCalls: 1},
		{name: "conflicting dual transport", header: "header-conflict-canary", query: "query-conflict-canary", wantStatus: http.StatusOK, wantCalls: 0},
		{name: "missing", wantStatus: http.StatusOK, wantCalls: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mu.Lock()
			responses = nil
			mu.Unlock()
			engine := gin.New()
			engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("turnstile-header-test"))))
			engine.POST("/protected", TurnstileCheck(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
			requestURL := "/protected"
			if tt.query != "" {
				requestURL += "?turnstile=" + url.QueryEscape(tt.query)
			}
			request := httptest.NewRequest(http.MethodPost, requestURL, strings.NewReader(""))
			if tt.header != "" {
				request.Header.Set(GeiliTurnstileHeader, tt.header)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			require.Equal(t, tt.wantStatus, recorder.Code, recorder.Body.String())
			mu.Lock()
			require.Len(t, responses, tt.wantCalls)
			mu.Unlock()
			if tt.header != "" {
				require.NotContains(t, recorder.Body.String(), tt.header)
			}
			if tt.query != "" {
				require.NotContains(t, recorder.Body.String(), tt.query)
			}
		})
	}
}
