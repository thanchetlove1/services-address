package services

import (
	"context"
	"time"

	"github.com/address-parser/app/models"
)

// CacheStats thống kê cache
type CacheStats struct {
	HitRate    float64 `json:"hit_rate"`
	TotalHits  int64   `json:"total_hits"`
	TotalMiss  int64   `json:"total_miss"`
	TotalItems int64   `json:"total_items"`
}

// ICacheService interface định nghĩa các method cần thiết cho cache
type ICacheService interface {
	// Get lấy địa chỉ từ cache
	Get(ctx context.Context, key string) (*models.AddressResult, bool, error)
	
	// Set lưu địa chỉ vào cache
	Set(ctx context.Context, key string, result *models.AddressResult) error
	
	// Delete xóa địa chỉ khỏi cache
	Delete(ctx context.Context, key string) error
	
	// Clear xóa tất cả cache
	Clear(ctx context.Context) error
	
	// InvalidateByGazetteerVersion invalidate cache theo gazetteer version
	InvalidateByGazetteerVersion(ctx context.Context, gazetteerVersion string) error
	
	// GetStats lấy thống kê cache theo cấu trúc mới
	GetStats(ctx context.Context) (*CacheStats, error)
	
	// Exists kiểm tra key có tồn tại không
	Exists(ctx context.Context, key string) (bool, error)
	
	// GetTTL lấy TTL còn lại của key
	GetTTL(ctx context.Context, key string) (time.Duration, error)
	
	// Close đóng kết nối (nếu cần)
	Close() error
}
