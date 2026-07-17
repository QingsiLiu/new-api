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

// SetGeiliPublicModelRouter Geili 自有：模型注册表公开只读 API（无鉴权，供公开层 SLP/market/pricing 消费）。
// 不受 AsyncTaskProductRoutesEnabled 门控：纯只读展示数据，与产品生成路由独立。
func SetGeiliPublicModelRouter(router *gin.Engine) {
	publicRouter := router.Group("/v1/public")
	publicRouter.Use(middleware.RouteTag("api"))
	publicRouter.Use(middleware.GlobalAPIRateLimit())
	{
		publicRouter.GET("/models", controller.GetPublicModels)
		publicRouter.GET("/models/:slug/rating", controller.GetPublicModelRating)
		publicRouter.GET("/models/:slug", controller.GetPublicModelBySlug)
	}
}

// SetGeiliModelRegistryAdminRouter Geili 自有：注册表运营录入接口（Admin）。
func SetGeiliModelRegistryAdminRouter(apiRouter *gin.RouterGroup) {
	registryRoute := apiRouter.Group("/geili/model-registry")
	registryRoute.Use(middleware.AdminAuth())
	{
		registryRoute.GET("", controller.AdminListModelRegistry)
		registryRoute.POST("", controller.AdminUpsertModelRegistry)
		registryRoute.DELETE("/:model", controller.AdminDeleteModelRegistry)
	}
	pricingRoute := apiRouter.Group("/geili/text-category-pricing")
	pricingRoute.Use(middleware.AdminAuth())
	{
		pricingRoute.GET("", controller.AdminListTextCategoryPricing)
		pricingRoute.PUT("", controller.AdminUpsertTextCategoryPricing)
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
