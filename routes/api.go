package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/address-parser/app/controllers"
)

// SetupAPIRoutes thiết lập tất cả API routes
func SetupAPIRoutes(router *gin.Engine, addressController *controllers.AddressController, adminController *controllers.AdminController) {
	// API v1 group
	v1 := router.Group("/v1")
	{
		// Address parsing routes
		addresses := v1.Group("/addresses")
		{
			addresses.POST("/parse", addressController.ParseAddress)
			addresses.POST("/jobs", addressController.BatchParse)
			addresses.GET("/jobs/:jobID/status", addressController.GetJobStatus)
			addresses.GET("/jobs/:jobID/results", addressController.GetJobResults)
		}
		
		// Admin routes
		admin := v1.Group("/admin")
		{
			admin.POST("/seed", adminController.SeedGazetteer)
			admin.POST("/meili/synonyms/rebuild", adminController.RebuildSynonyms)
			admin.POST("/cache/invalidate", adminController.InvalidateCache)
			admin.GET("/stats", adminController.GetStats)
			admin.POST("/indexes/build", adminController.BuildIndexes)
			admin.GET("/export/:type", adminController.ExportData)
		}
		
		// Health check route
		v1.GET("/health", addressController.HealthCheck)
	}
}

// SetupHealthRoutes thiết lập health check routes
func SetupHealthRoutes(router *gin.Engine, addressController *controllers.AddressController) {
	// Root health check
	router.GET("/health", addressController.HealthCheck)
	
	// Readiness check
	router.GET("/ready", addressController.HealthCheck)
	
	// Liveness check
	router.GET("/live", addressController.HealthCheck)
}

// SetupMetricsRoutes thiết lập metrics routes (cho Prometheus)
func SetupMetricsRoutes(router *gin.Engine) {
	// Metrics endpoint cho Prometheus
	router.GET("/metrics", func(c *gin.Context) {
		// TODO: Implement Prometheus metrics
		c.JSON(200, gin.H{
			"status": "metrics endpoint - to be implemented",
		})
	})
}

// SetupAllRoutes thiết lập tất cả routes
func SetupAllRoutes(router *gin.Engine, addressController *controllers.AddressController, adminController *controllers.AdminController) {
	// Thiết lập middleware
	setupMiddleware(router)
	
	// Thiết lập các loại routes
	SetupWebRoutes(router)
	SetupHealthRoutes(router, addressController)
	SetupAPIRoutes(router, addressController, adminController)
	SetupMetricsRoutes(router)
	
	// 404 handler
	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{
			"error": "Route not found",
			"path":  c.Request.URL.Path,
			"method": c.Request.Method,
		})
	})
}

// setupMiddleware thiết lập middleware cho router
func setupMiddleware(router *gin.Engine) {
	// Recovery middleware
	router.Use(gin.Recovery())
	
	// Logger middleware
	router.Use(gin.Logger())
	
	// CORS middleware (nếu cần)
	// router.Use(cors.Default())
	
	// Rate limiting middleware (nếu cần)
	// router.Use(rateLimit())
	
	// Authentication middleware (nếu cần)
	// router.Use(authMiddleware())
}
