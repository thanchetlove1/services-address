package responses

import (
	"github.com/address-parser/app/models"
)

// ParseAddressResponse response parse địa chỉ đơn lẻ
type ParseAddressResponse struct {
	LevelConfigUsed    int             `json:"level_config_used"`    // Số cấp hành chính sử dụng
	GazetteerVersion   string          `json:"gazetteer_version"`   // Phiên bản gazetteer
	Results            []models.AddressResult `json:"results"`       // Kết quả parse
	ProcessingTimeMs   int64           `json:"processing_time_ms"`  // Thời gian xử lý (ms)
	CacheHit           bool            `json:"cache_hit"`           // Có hit cache không
}

// BatchParseResponse response parse hàng loạt địa chỉ
type BatchParseResponse struct {
	JobID              string          `json:"job_id"`               // ID của job
	EstimatedSeconds   int             `json:"estimated_seconds"`    // Thời gian ước tính (giây)
	TotalAddresses     int             `json:"total_addresses"`      // Tổng số địa chỉ
	Message            string          `json:"message"`              // Thông báo
}

// JobStatusResponse response trạng thái job
type JobStatusResponse struct {
	JobID              string          `json:"job_id"`               // ID của job
	Status             string          `json:"status"`               // Trạng thái job
	Progress           float64         `json:"progress"`             // Tiến độ (0.0 - 1.0)
	Processed          int             `json:"processed"`            // Số địa chỉ đã xử lý
	Total              int             `json:"total"`                // Tổng số địa chỉ
	EstimatedRemaining int             `json:"estimated_remaining"`  // Thời gian còn lại ước tính (giây)
	Message            string          `json:"message"`              // Thông báo
}

// JobStatus constants
const (
	JobStatusPending = "pending"
	JobStatusRunning = "running"
	JobStatusDone    = "done"
	JobStatusFailed  = "failed"
)

// SeedGazetteerResponse response seed gazetteer
type SeedGazetteerResponse struct {
	ValidationPassed    bool     `json:"validation_passed"`    // Validation có pass không
	Warnings           []string  `json:"warnings,omitempty"`   // Cảnh báo
	EstimatedBuildTime string    `json:"estimated_build_time,omitempty"` // Thời gian build ước tính
	UnitsProcessed     int       `json:"units_processed,omitempty"`      // Số units đã xử lý
	IndexesBuilt       int       `json:"indexes_built,omitempty"`        // Số indexes đã build
	ProcessingTimeMs   int64     `json:"processing_time_ms,omitempty"`   // Thời gian xử lý (ms)
	DryRun             bool      `json:"dry_run"`              // Có phải dry run không
	Message            string    `json:"message"`              // Thông báo
}

// ReviewListResponse response danh sách review
type ReviewListResponse struct {
	Reviews            []models.AddressReview `json:"reviews"`             // Danh sách review
	Total             int                    `json:"total"`               // Tổng số review
	Pending           int                    `json:"pending"`             // Số review đang chờ
	InReview          int                    `json:"in_review"`           // Số review đang xử lý
	Completed         int                    `json:"completed"`           // Số review đã hoàn thành
	Limit             int                    `json:"limit"`               // Giới hạn số lượng
	Offset            int                    `json:"offset"`              // Offset
}

// ReviewActionResponse response thao tác review
type ReviewActionResponse struct {
	Success            bool      `json:"success"`             // Thao tác có thành công không
	ReviewID           string    `json:"review_id"`           // ID của review
	Action             string    `json:"action"`              // Hành động thực hiện
	Message            string    `json:"message"`             // Thông báo
	UpdatedAt          string    `json:"updated_at"`          // Thời gian cập nhật
}

// AdminStatsResponse response thống kê admin
type AdminStatsResponse struct {
	CacheHitRate       float64   `json:"cache_hit_rate"`      // Tỷ lệ hit cache
	AccuracyRate       float64   `json:"accuracy_rate"`       // Tỷ lệ chính xác
	AvgProcessingTimeMs int64    `json:"avg_processing_time_ms"` // Thời gian xử lý trung bình (ms)
	TotalProcessed     int64     `json:"total_processed"`     // Tổng số địa chỉ đã xử lý
	TotalCached        int64     `json:"total_cached"`        // Tổng số địa chỉ trong cache
	TotalReviews       int64     `json:"total_reviews"`       // Tổng số review
	PendingReviews     int64     `json:"pending_reviews"`     // Số review đang chờ
	UptimeSeconds      int64     `json:"uptime_seconds"`      // Thời gian hoạt động (giây)
	LastUpdated        string    `json:"last_updated"`        // Lần cập nhật cuối
}

// ErrorResponse response lỗi
type ErrorResponse struct {
	Error             string      `json:"error"`               // Mã lỗi
	Message           string      `json:"message"`             // Thông báo lỗi
	Details           interface{} `json:"details,omitempty"`   // Chi tiết lỗi
	Timestamp         string      `json:"timestamp"`           // Thời gian xảy ra lỗi
	RequestID         string      `json:"request_id,omitempty"` // ID của request
}

// SuccessResponse response thành công
type SuccessResponse struct {
	Success           bool        `json:"success"`             // Có thành công không
	Message           string      `json:"message"`             // Thông báo
	Data              interface{} `json:"data,omitempty"`      // Dữ liệu
	Timestamp         string      `json:"timestamp"`           // Thời gian
}

// HealthCheckResponse response kiểm tra sức khỏe
type HealthCheckResponse struct {
	Status            string      `json:"status"`              // Trạng thái sức khỏe
	Timestamp         string      `json:"timestamp"`           // Thời gian kiểm tra
	Uptime            string      `json:"uptime"`              // Thời gian hoạt động
	Version           string      `json:"version"`             // Phiên bản
	Services          map[string]string `json:"services"`      // Trạng thái các service
}

// SystemStatsResponse response thống kê hệ thống
type SystemStatsResponse struct {
	CacheHitRate       float64       `json:"cache_hit_rate"`       // Tỷ lệ hit cache
	AccuracyRate       float64       `json:"accuracy_rate"`        // Tỷ lệ chính xác
	AvgProcessingTimeMs float64       `json:"avg_processing_time_ms"` // Thời gian xử lý trung bình (ms)
	TotalProcessed     int64         `json:"total_processed"`      // Tổng số địa chỉ đã xử lý
	ReviewQueueSize    int64         `json:"review_queue_size"`    // Số lượng review đang chờ
	SystemInfo         SystemInfo    `json:"system_info"`          // Thông tin hệ thống
	DatabaseStats      DatabaseStats `json:"database_stats"`       // Thống kê database
}

// SystemInfo thông tin hệ thống
type SystemInfo struct {
	Version      string                 `json:"version"`       // Phiên bản
	Environment  string                 `json:"environment"`   // Môi trường
	Uptime      string                 `json:"uptime"`        // Thời gian hoạt động
	MemoryUsage map[string]interface{} `json:"memory_usage"`  // Sử dụng memory
	CPUUsage    float64                `json:"cpu_usage"`     // Sử dụng CPU
}

// DatabaseStats thống kê database
type DatabaseStats struct {
	AdminUnits     int64 `json:"admin_units"`      // Số lượng admin units
	AddressCache   int64 `json:"address_cache"`    // Số lượng address cache
	AddressReview  int64 `json:"address_review"`   // Số lượng address review
	LearnedAliases int64 `json:"learned_aliases"`  // Số lượng learned aliases
}
