package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestIsGeiliAdminPublicPath(t *testing.T) {
	t.Parallel()

	allowed := []string{
		"/sign-in",
		"/otp",
		"/forgot-password",
		"/reset",
		"/user/reset",
		"/oauth",
		"/oauth/github",
		"/logo.png",
		"/favicon.ico",
		"/manifest.json",
		"/static/js/index.js",
		"/static/css/index.css",
		"/assets/example.png",
		"/api/status",
		"/v1/models",
		"/v1beta/models",
		"/mj/submit",
		"/pg/chat",
		"/suno/fetch",
		"/kling/task",
		"/jimeng/task",
		"/relay/mj/submit",
	}
	for _, path := range allowed {
		if !isGeiliAdminPublicPath(path) {
			t.Errorf("expected %q to remain public", path)
		}
	}

	rejected := []string{
		"/",
		"/login",
		"/register",
		"/sign-up",
		"/dashboard",
		"/static-evil",
		"/v1-evil",
		"/mj-evil",
		"/relay/mj-evil",
		"/relay/not-mj",
	}
	for _, path := range rejected {
		if isGeiliAdminPublicPath(path) {
			t.Errorf("expected %q to remain admin-only", path)
		}
	}
}

func TestGeiliAdminOnlyUIGuardRoutesAnonymousEntrypoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("GEILI_ADMIN_ONLY_UI", "true")
	t.Setenv("GEILI_PORTAL_URL", "https://geiliapi.com")

	router := gin.New()
	router.Use(sessions.Sessions("test", cookie.NewStore([]byte("test-secret"))))
	router.Use(geiliAdminOnlyUIGuard())
	router.NoRoute(func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	tests := []struct {
		path     string
		status   int
		location string
	}{
		{path: "/", status: http.StatusFound, location: "/sign-in"},
		{path: "/login", status: http.StatusFound, location: "/sign-in"},
		{path: "/sign-in", status: http.StatusNoContent},
		{path: "/otp", status: http.StatusNoContent},
		{path: "/static/js/index.js", status: http.StatusNoContent},
		{path: "/register", status: http.StatusFound, location: "https://geiliapi.com"},
		{path: "/dashboard", status: http.StatusFound, location: "https://geiliapi.com"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status for %s = %d, want %d", test.path, response.Code, test.status)
			}
			if location := response.Header().Get("Location"); location != test.location {
				t.Fatalf("location for %s = %q, want %q", test.path, location, test.location)
			}
		})
	}
}
