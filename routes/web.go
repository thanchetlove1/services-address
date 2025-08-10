package routes

import (
	"github.com/gin-gonic/gin"
)

// SetupWebRoutes thiết lập web routes (nếu cần trong tương lai)
func SetupWebRoutes(router *gin.Engine) {
	// Web routes group
	web := router.Group("/")
	{
		// Home page
		web.GET("/", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "Address Parser Service",
				"version": "1.0.0",
				"docs":    "/api/v1/docs",
			})
		})
		
		// API documentation
		web.GET("/docs", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"api": "Address Parser API v1",
				"endpoints": map[string]string{
					"parse":     "POST /api/v1/parse",
					"batch":     "POST /api/v1/batch",
					"job_status": "GET /api/v1/job/:jobID/status",
					"job_results": "GET /api/v1/job/:jobID/results",
					"health":    "GET /api/v1/health",
				},
			})
		})
		
		// Status page
		web.GET("/status", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "running",
				"service": "Address Parser",
				"uptime": "to be implemented",
			})
		})
	}
}
