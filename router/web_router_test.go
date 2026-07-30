package router

import "testing"

func TestIsGeiliAdminPublicPath(t *testing.T) {
	t.Parallel()

	allowed := []string{
		"/login",
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
		"/register",
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
