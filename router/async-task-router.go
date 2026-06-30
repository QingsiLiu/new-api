package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/setting/operation_setting"

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

func SetAsyncTaskProductRouter(router *gin.Engine) {
	if !operation_setting.AsyncTaskProductRoutesEnabled {
		return
	}
	productRouter := router.Group("/v1")
	productRouter.Use(middleware.RouteTag("relay"))
	productRouter.Use(middleware.SystemPerformanceCheck())
	productRouter.Use(middleware.TokenAuth())
	productRouter.Use(middleware.ModelRequestRateLimit())
	{
		productRouter.POST("/images/tasks", controller.CreateAsyncImageTask)
		productRouter.POST("/videos/tasks", controller.CreateAsyncVideoTask)
		productRouter.GET("/tasks/:id", controller.GetAsyncTask)
		productRouter.POST("/tasks/:id/cancel", controller.CancelAsyncTask)
		productRouter.GET("/tasks/:id/content", controller.GetAsyncTaskContent)
		productRouter.POST("/pricing/estimate", controller.EstimateAsyncTaskPricing)
		productRouter.GET("/billing/balance", controller.GetAsyncBillingBalance)
		productRouter.GET("/billing/usage", controller.GetAsyncBillingUsage)
	}
}
