package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetGeiliFunnelRouter(engine *gin.Engine) {
	group := engine.Group("/api/geili/funnel")
	group.Use(middleware.RouteTag("api"), middleware.GeiliFunnelNoStore())
	group.POST(
		"/events",
		middleware.FixedRequestBodyLimit(2048),
		middleware.GeiliFunnelSecretAuth(),
		middleware.GeiliFunnelWriteRateLimit(),
		controller.IngestGeiliFunnelEvent,
	)
	group.GET(
		"/summary",
		middleware.GeiliFunnelSecretAuth(),
		middleware.GeiliFunnelReadRateLimit(),
		controller.GetGeiliFunnelSummary,
	)
}
