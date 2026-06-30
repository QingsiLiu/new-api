package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAsyncTaskProductRoutesDisabledByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	operation_setting.AsyncTaskProductRoutesEnabled = false
	t.Cleanup(func() {
		operation_setting.AsyncTaskProductRoutesEnabled = false
	})

	engine := gin.New()
	SetAsyncTaskProductRouter(engine)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/images/tasks", strings.NewReader(`{}`))
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAsyncTaskProductRoutesEnabledRequiresTokenAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	operation_setting.AsyncTaskProductRoutesEnabled = true
	t.Cleanup(func() {
		operation_setting.AsyncTaskProductRoutesEnabled = false
	})

	engine := gin.New()
	SetAsyncTaskProductRouter(engine)

	for _, target := range []string{
		"/v1/images/tasks",
		"/v1/videos/tasks",
		"/v1/tasks/task-1",
		"/v1/tasks/task-1/content",
		"/v1/tasks/task-1/cancel",
	} {
		t.Run(target, func(t *testing.T) {
			method := http.MethodGet
			if strings.HasSuffix(target, "/tasks") || strings.HasSuffix(target, "/cancel") {
				method = http.MethodPost
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(method, target, strings.NewReader(`{}`))
			engine.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
		})
	}
}
