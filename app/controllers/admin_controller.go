package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/address-parser/app/requests"
	"github.com/address-parser/app/responses"
	"github.com/address-parser/app/services"
	"go.uber.org/zap"
)

// AdminController controller xử lý các request admin
type AdminController struct {
	adminService  *services.AdminService
	cacheService  services.ICacheService
	logger        *zap.Logger
}

// NewAdminController tạo mới AdminController
func NewAdminController(adminService *services.AdminService, cacheService services.ICacheService, logger *zap.Logger) *AdminController {
	return &AdminController{
		adminService: adminService,
		cacheService: cacheService,
		logger:       logger,
	}
}

// SeedGazetteer seed dữ liệu gazetteer
func (ac *AdminController) SeedGazetteer(c *gin.Context) {
	var req requests.SeedGazetteerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Request không hợp lệ: " + err.Error(),
		})
		return
	}

	// Kiểm tra dry run
	dryRun := c.Query("dry_run") == "true"

	startTime := time.Now()

	if dryRun {
		// Validate dữ liệu nhưng không thực thi
		validation, err := ac.adminService.ValidateGazetteerData(req.Data)
		if err != nil {
			c.JSON(http.StatusBadRequest, responses.ErrorResponse{
				Error:   "VALIDATION_ERROR", 
				Message: "Lỗi validate dữ liệu: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, responses.SeedGazetteerResponse{
			ValidationPassed:    validation.Passed,
			Warnings:           validation.Warnings,
			EstimatedBuildTime: validation.EstimatedBuildTime,
			DryRun:             true,
			Message:            "Validation hoàn thành thành công",
		})
		return
	}

	// Thực thi seed
	result, err := ac.adminService.SeedGazetteer(req.GazetteerVersion, req.Data, req.RebuildIndexes)
	if err != nil {
		ac.logger.Error("Lỗi seed gazetteer", zap.Error(err))
		c.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Error:   "SEED_ERROR",
			Message: "Lỗi seed gazetteer: " + err.Error(),
		})
		return
	}

	// Invalidate cache nếu cần
	if req.RebuildIndexes {
		err = ac.cacheService.InvalidateByGazetteerVersion(c.Request.Context(), req.GazetteerVersion)
		if err != nil {
			ac.logger.Warn("Lỗi invalidate cache", zap.Error(err))
		}
	}

	processingTime := time.Since(startTime)
	ac.logger.Info("Seed gazetteer thành công",
		zap.String("version", req.GazetteerVersion),
		zap.Int("records", len(req.Data)),
		zap.Duration("duration", processingTime))

	c.JSON(http.StatusOK, responses.SeedGazetteerResponse{
		ValidationPassed:    true,
		UnitsProcessed:     result.UnitsProcessed,
		IndexesBuilt:       result.IndexesBuilt,
		ProcessingTimeMs:   processingTime.Milliseconds(),
		DryRun:             false,
		Message:            "Seed gazetteer thành công",
	})
}

// RebuildSynonyms rebuild synonyms từ learned_aliases
func (ac *AdminController) RebuildSynonyms(c *gin.Context) {
	startTime := time.Now()

	err := ac.adminService.RebuildSynonyms()
	if err != nil {
		ac.logger.Error("Lỗi rebuild synonyms", zap.Error(err))
		c.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Error:   "REBUILD_ERROR",
			Message: "Lỗi rebuild synonyms: " + err.Error(),
		})
		return
	}

	processingTime := time.Since(startTime)
	ac.logger.Info("Rebuild synonyms thành công", zap.Duration("duration", processingTime))

	c.JSON(http.StatusOK, responses.SuccessResponse{
		Success: true,
		Message: "Rebuild synonyms thành công",
		Data: map[string]interface{}{
			"processing_time_ms": processingTime.Milliseconds(),
		},
	})
}

// InvalidateCache invalidate cache theo gazetteer version
func (ac *AdminController) InvalidateCache(c *gin.Context) {
	gazetteerVersion := c.Query("gazetteer_version")
	if gazetteerVersion == "" {
		c.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Error:   "MISSING_VERSION",
			Message: "Thiếu gazetteer_version",
		})
		return
	}

	startTime := time.Now()

	err := ac.cacheService.InvalidateByGazetteerVersion(c.Request.Context(), gazetteerVersion)
	if err != nil {
		ac.logger.Error("Lỗi invalidate cache", zap.Error(err))
		c.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Error:   "INVALIDATE_ERROR",
			Message: "Lỗi invalidate cache: " + err.Error(),
		})
		return
	}

	processingTime := time.Since(startTime)
	ac.logger.Info("Invalidate cache thành công",
		zap.String("version", gazetteerVersion),
		zap.Duration("duration", processingTime))

	c.JSON(http.StatusOK, responses.SuccessResponse{
		Success: true,
		Message: "Invalidate cache thành công",
		Data: map[string]interface{}{
			"gazetteer_version":  gazetteerVersion,
			"processing_time_ms": processingTime.Milliseconds(),
		},
	})
}

// GetStats lấy thống kê hệ thống
func (ac *AdminController) GetStats(c *gin.Context) {
	// Lấy stats từ service
	stats, err := ac.adminService.GetSystemStats()
	if err != nil {
		ac.logger.Error("Lỗi lấy stats", zap.Error(err))
		c.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Error:   "STATS_ERROR",
			Message: "Lỗi lấy stats: " + err.Error(),
		})
		return
	}

	// Lấy cache stats
	cacheStats, err := ac.cacheService.GetStats(c.Request.Context())
	if err != nil {
		ac.logger.Warn("Lỗi lấy cache stats", zap.Error(err))
		cacheStats = &services.CacheStats{} // Default empty stats
	}

	c.JSON(http.StatusOK, responses.SystemStatsResponse{
		CacheHitRate:       cacheStats.HitRate,
		AccuracyRate:       stats.AccuracyRate,
		AvgProcessingTimeMs: stats.AvgProcessingTimeMs,
		TotalProcessed:     stats.TotalProcessed,
		ReviewQueueSize:    stats.ReviewQueueSize,
		SystemInfo: responses.SystemInfo{
			Version:      "1.0.0",
			Environment:  "production",
			Uptime:      stats.Uptime,
			MemoryUsage: stats.MemoryUsage,
			CPUUsage:    stats.CPUUsage,
		},
		DatabaseStats: responses.DatabaseStats{
			AdminUnits:      stats.DatabaseStats.AdminUnits,
			AddressCache:    stats.DatabaseStats.AddressCache,
			AddressReview:   stats.DatabaseStats.AddressReview,
			LearnedAliases:  stats.DatabaseStats.LearnedAliases,
		},
	})
}

// BuildIndexes build lại toàn bộ indexes
func (ac *AdminController) BuildIndexes(c *gin.Context) {
	startTime := time.Now()

	err := ac.adminService.BuildIndexes()
	if err != nil {
		ac.logger.Error("Lỗi build indexes", zap.Error(err))
		c.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Error:   "BUILD_ERROR",
			Message: "Lỗi build indexes: " + err.Error(),
		})
		return
	}

	processingTime := time.Since(startTime)
	ac.logger.Info("Build indexes thành công", zap.Duration("duration", processingTime))

	c.JSON(http.StatusOK, responses.SuccessResponse{
		Success: true,
		Message: "Build indexes thành công",
		Data: map[string]interface{}{
			"processing_time_ms": processingTime.Milliseconds(),
		},
	})
}

// ExportData export dữ liệu để backup
func (ac *AdminController) ExportData(c *gin.Context) {
	dataType := c.Param("type") // admin_units, address_cache, learned_aliases
	if dataType == "" {
		c.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Error:   "MISSING_TYPE",
			Message: "Thiếu loại dữ liệu cần export",
		})
		return
	}

	// Kiểm tra format
	format := c.Query("format") // json, csv
	if format == "" {
		format = "json"
	}

	// Kiểm tra limit
	limitStr := c.Query("limit")
	limit := 10000 // Default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Export data
	data, err := ac.adminService.ExportData(dataType, format, limit)
	if err != nil {
		ac.logger.Error("Lỗi export data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Error:   "EXPORT_ERROR",
			Message: "Lỗi export data: " + err.Error(),
		})
		return
	}

	// Set headers for download
	filename := fmt.Sprintf("%s_export_%s.%s", dataType, time.Now().Format("20060102_150405"), format)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	
	if format == "csv" {
		c.Header("Content-Type", "text/csv")
	} else {
		c.Header("Content-Type", "application/json")
	}

	c.String(http.StatusOK, string(data))
}
