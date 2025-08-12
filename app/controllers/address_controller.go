package controllers

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/address-parser/app/requests"
	"github.com/address-parser/app/responses"
	"github.com/address-parser/app/models"
	"github.com/address-parser/app/services"
	"github.com/address-parser/helpers/utils"
	"github.com/address-parser/internal/parser"
	"go.uber.org/zap"
)

// AddressController controller xử lý các request liên quan đến địa chỉ
type AddressController struct {
	addressService *services.AddressService
	cacheService   services.ICacheService
	logger         *zap.Logger
}

// NewAddressController tạo mới AddressController
func NewAddressController(addressService *services.AddressService, cacheService services.ICacheService, logger *zap.Logger) *AddressController {
	return &AddressController{
		addressService: addressService,
		cacheService:   cacheService,
		logger:         logger,
	}
}

// ParseAddress parse địa chỉ đơn lẻ
func (ac *AddressController) ParseAddress(c *gin.Context) {
	var req requests.ParseAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Request không hợp lệ: " + err.Error(),
		})
		return
	}

	startTime := time.Now()

			// Kiểm tra cache trước
		if req.Options.UseCache {
			if cached, found, err := ac.cacheService.Get(c.Request.Context(), req.Address); err == nil && found {
				c.JSON(http.StatusOK, responses.ParseAddressResponse{
					LevelConfigUsed:  req.Options.Levels,
					GazetteerVersion: "1.0.0", // TODO: Lấy từ config
					Results:          []models.AddressResult{*cached},
					ProcessingTimeMs: time.Since(startTime).Milliseconds(),
					CacheHit:         true,
				})
				return
			}
		}

	// Parse địa chỉ
	result, err := ac.addressService.ParseAddress(req.Address, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Error:   "PARSE_ERROR",
			Message: "Lỗi parse địa chỉ: " + err.Error(),
		})
		return
	}

	// Lưu vào cache nếu cần
	if req.Options.UseCache {
		ac.cacheService.Set(c.Request.Context(), req.Address, result)
	}

	c.JSON(http.StatusOK, responses.ParseAddressResponse{
		LevelConfigUsed:  req.Options.Levels,
		GazetteerVersion: "1.0.0", // TODO: Lấy từ config
		Results:          []models.AddressResult{*result},
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		CacheHit:         false,
	})
}

// BatchParse parse hàng loạt địa chỉ
func (ac *AddressController) BatchParse(c *gin.Context) {
	var req requests.BatchParseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Request không hợp lệ: " + err.Error(),
		})
		return
	}

	// Kiểm tra giới hạn số lượng
	if len(req.Addresses) > 20000 {
		c.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Error:   "TOO_MANY_ADDRESSES",
			Message: "Số lượng địa chỉ vượt quá giới hạn (20,000)",
		})
		return
	}

	// Tạo job xử lý
	jobID := utils.GenerateUUID()
	estimatedTime := ac.addressService.EstimateBatchProcessingTime(len(req.Addresses))

	// Khởi chạy job trong background
	go ac.addressService.ProcessBatchJob(jobID, req.Addresses, req.Options)

	c.JSON(http.StatusAccepted, responses.BatchParseResponse{
		JobID:            jobID,
		EstimatedSeconds: estimatedTime,
		TotalAddresses:   len(req.Addresses),
		Message:          "Job đã được tạo và đang xử lý",
	})
}

// GetJobStatus lấy trạng thái job
func (ac *AddressController) GetJobStatus(c *gin.Context) {
	jobID := c.Param("jobID")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Error:   "MISSING_JOB_ID",
			Message: "Thiếu Job ID",
		})
		return
	}

	status, err := ac.addressService.GetJobStatus(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, responses.ErrorResponse{
			Error:   "JOB_NOT_FOUND",
			Message: "Không tìm thấy job: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, responses.JobStatusResponse{
		JobID:              jobID,
		Status:             status.Status,
		Progress:           status.Progress,
		Processed:          status.Processed,
		Total:              status.Total,
		EstimatedRemaining: status.EstimatedRemaining,
		Message:            status.Message,
	})
}

// GetJobResults lấy kết quả job với hỗ trợ NDJSON + gzip streaming
func (ac *AddressController) GetJobResults(c *gin.Context) {
	jobID := c.Param("jobID")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Error:   "MISSING_JOB_ID",
			Message: "Thiếu Job ID",
		})
		return
	}

	// Kiểm tra format yêu cầu
	format := c.Query("format")
	gzipEnabled := c.Query("gzip") == "1"

	if format == "ndjson" {
		// Stream NDJSON results
		ac.streamNDJSONResults(c, jobID, gzipEnabled)
		return
	}

	// Default JSON response
	results, err := ac.addressService.GetJobResults(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, responses.ErrorResponse{
			Error:   "JOB_NOT_FOUND",
			Message: "Không tìm thấy job: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, responses.SuccessResponse{
		Success: true,
		Message: "Lấy kết quả thành công",
		Data:    results,
	})
}

// HealthCheck kiểm tra sức khỏe service
func (ac *AddressController) HealthCheck(c *gin.Context) {
	uptime := time.Since(ac.addressService.GetStartTime())
	
	c.JSON(http.StatusOK, responses.HealthCheckResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
		Uptime:    uptime.String(),
		Version:   "1.0.0",
		Services: map[string]string{
			"address_parser": "healthy",
			"cache":          "healthy",
			"database":       "healthy",
		},
	})
}

// streamNDJSONResults stream kết quả theo format NDJSON với hỗ trợ gzip
func (ac *AddressController) streamNDJSONResults(c *gin.Context, jobID string, gzipEnabled bool) {
	// Thiết lập headers
	if gzipEnabled {
		c.Header("Content-Type", "application/x-ndjson")
		c.Header("Content-Encoding", "gzip")
	} else {
		c.Header("Content-Type", "application/x-ndjson")
	}

	// Tạo writer
	var writer gin.ResponseWriter = c.Writer
	if gzipEnabled {
		gzWriter := gzip.NewWriter(c.Writer)
		defer gzWriter.Close()
		writer = &gzipResponseWriter{
			ResponseWriter: c.Writer,
			gzWriter:       gzWriter,
		}
	}

	// Stream results từ service
	resultChannel, err := ac.addressService.GetJobResultsStream(jobID)
	if err != nil {
		ac.logger.Error("Lỗi stream job results", zap.Error(err))
		c.JSON(http.StatusNotFound, responses.ErrorResponse{
			Error:   "JOB_NOT_FOUND",
			Message: "Không tìm thấy job: " + err.Error(),
		})
		return
	}

	encoder := json.NewEncoder(writer)
	for result := range resultChannel {
		if err := encoder.Encode(result); err != nil {
			ac.logger.Error("Lỗi encode NDJSON", zap.Error(err))
			break
		}
		
		// Flush để đảm bảo data được gửi ngay
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

// gzipResponseWriter wrapper cho gzip writer
type gzipResponseWriter struct {
	gin.ResponseWriter
	gzWriter *gzip.Writer
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	return w.gzWriter.Write(data)
}

func (w *gzipResponseWriter) Flush() {
	w.gzWriter.Flush()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// === ADDED FROM CODE-V1.MD ===

// statusFromConfidence helper nhỏ
func statusFromConfidence(conf float64) string {
	// TODO: Import config package để sử dụng thresholds
	thresholdHigh := 0.90
	thresholdReviewLow := 0.60
	
	if conf >= thresholdHigh {
		return "matched"
	}
	if conf >= thresholdReviewLow {
		return "needs_review"
	}
	return "unmatched"
}

// buildFlags build flags từ result
func buildFlags(res *parser.Result) []string {
	flags := []string{}
	if res.Signals.Unit != "" || res.Signals.Level != "" {
		flags = append(flags, "APARTMENT_UNIT")
	}
	if res.Signals.POI != "" {
		flags = append(flags, "POI_EXTRACTED")
	}
	// strategy flag
	switch res.MatchLevel {
	case "exact":
		flags = append(flags, "EXACT_MATCH")
	case "ascii_exact":
		flags = append(flags, "ASCII_EXACT")
	default:
		flags = append(flags, "FUZZY_MATCH")
	}
	return flags
}

// canonicalFromPath build canonical text từ path
func canonicalFromPath(house, street string, p parser.Result) string {
	if !p.HasPath {
		return strings.Join([]string{house, street}, " ")
	}
	w := p.Path.Ward.Name
	d := p.Path.District.Name
	pr := p.Path.Province.Name
	left := strings.Join([]string{house, street}, " ")
	parts := []string{}
	if left != "" { parts = append(parts, left) }
	parts = append(parts, w, d, pr, "Việt Nam")
	return strings.Join(parts, ", ")
}
