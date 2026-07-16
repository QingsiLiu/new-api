package router

import (
	"embed"
	"net/http"
	"os"
	"strings"

	"github.com/gin-contrib/sessions"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// ThemeAssets holds the embedded default frontend assets.
type ThemeAssets struct {
	DefaultBuildFS   embed.FS
	DefaultIndexPage []byte
}

// geiliAdminOnlyUIGuard M3-A 开关（默认关）：GEILI_ADMIN_ONLY_UI=true 时原生 UI 退居
// admin-only——非管理员会话的页面请求 302 到门户（GEILI_PORTAL_URL，默认 geiliapi.com）。
// 放行：API 命名空间、静态资产、/login（管理员登录入口）。默认关=零行为变化，
// 翻转属终局典礼动作（建造目标 §2 冻结面）。
func geiliAdminOnlyUIGuard() gin.HandlerFunc {
	enabled := strings.EqualFold(os.Getenv("GEILI_ADMIN_ONLY_UI"), "true")
	portal := os.Getenv("GEILI_PORTAL_URL")
	if portal == "" {
		portal = "https://geiliapi.com"
	}
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/v1") || strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/assets") ||
			strings.HasPrefix(p, "/mj") || strings.HasPrefix(p, "/pg") || p == "/login" {
			c.Next()
			return
		}
		session := sessions.Default(c)
		if role, ok := session.Get("role").(int); ok && role >= common.RoleAdminUser {
			c.Next()
			return
		}
		c.Redirect(http.StatusFound, portal)
		c.Abort()
	}
}

func SetWebRouter(router *gin.Engine, assets ThemeAssets) {
	defaultFS := common.EmbedFolder(assets.DefaultBuildFS, "web/default/dist")

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(geiliAdminOnlyUIGuard())
	router.Use(middleware.Cache())
	router.Use(static.Serve("/", defaultFS))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", assets.DefaultIndexPage)
	})
}
