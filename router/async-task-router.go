package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetAsyncTaskRouter(router *gin.Engine) {
	asyncRouter := router.Group("/v1/async")
	asyncRouter.Use(middleware.RouteTag("relay"))
	asyncRouter.Use(middleware.SystemPerformanceCheck())
	asyncRouter.Use(middleware.TokenAuth())
	{
		asyncRouter.POST("/tasks", controller.CreateAsyncTask)
		asyncRouter.GET("/metrics", controller.GetAsyncTaskMetrics)
		asyncRouter.GET("/tasks/:id", controller.GetAsyncTask)
		asyncRouter.POST("/tasks/:id/cancel", controller.CancelAsyncTask)
		asyncRouter.GET("/tasks/:id/content", controller.GetAsyncTaskContent)
	}
}
