package services

import (
	"context"
	"sync"
	"time"

	"github.com/address-parser/app/models"
)

// CacheService service quản lý cache in-memory
type CacheService struct {
	cache      map[string]*models.AddressResult
	timestamps map[string]time.Time
	mu         sync.RWMutex
	ttl        time.Duration
}

// NewCacheService tạo mới CacheService
func NewCacheService(ttl time.Duration) *CacheService {
	return &CacheService{
		cache:      make(map[string]*models.AddressResult),
		timestamps: make(map[string]time.Time),
		ttl:        ttl,
	}
}

// Get lấy kết quả từ cache
func (cs *CacheService) Get(ctx context.Context, key string) (*models.AddressResult, bool, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if result, exists := cs.cache[key]; exists {
		// Kiểm tra TTL
		if cs.isExpired(key) {
			// Xóa item hết hạn
			go cs.deleteExpired(key)
			return nil, false, nil
		}
		return result, true, nil
	}

	return nil, false, nil
}

// Set lưu kết quả vào cache
func (cs *CacheService) Set(ctx context.Context, key string, result *models.AddressResult) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Lưu timestamp
	cs.timestamps[key] = time.Now()
	
	cs.cache[key] = result
	
	return nil
}

// Delete xóa item khỏi cache
func (cs *CacheService) Delete(ctx context.Context, key string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	delete(cs.cache, key)
	delete(cs.timestamps, key)
	
	return nil
}

// Clear xóa toàn bộ cache
func (cs *CacheService) Clear(ctx context.Context) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.cache = make(map[string]*models.AddressResult)
	cs.timestamps = make(map[string]time.Time)
	
	return nil
}

// Size lấy kích thước cache
func (cs *CacheService) Size() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return len(cs.cache)
}

// GetStats lấy thống kê cache
func (cs *CacheService) GetStats(ctx context.Context) (map[string]interface{}, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	total := len(cs.cache)
	expired := 0

	for key := range cs.cache {
		if cs.isExpired(key) {
			expired++
		}
	}

	return map[string]interface{}{
		"total_items":   total,
		"expired_items": expired,
		"active_items":  total - expired,
		"ttl_seconds":   int(cs.ttl.Seconds()),
	}, nil
}

// CleanupExpired xóa các item hết hạn
func (cs *CacheService) CleanupExpired() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for key := range cs.cache {
		if cs.isExpired(key) {
			delete(cs.cache, key)
			delete(cs.timestamps, key)
		}
	}
}

// isExpired kiểm tra item có hết hạn không
func (cs *CacheService) isExpired(key string) bool {
	timestamp, exists := cs.timestamps[key]
	if !exists {
		return true
	}
	return time.Since(timestamp) > cs.ttl
}

// deleteExpired xóa item hết hạn (async)
func (cs *CacheService) deleteExpired(key string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	delete(cs.cache, key)
	delete(cs.timestamps, key)
}

// Exists kiểm tra key có tồn tại không
func (cs *CacheService) Exists(ctx context.Context, key string) (bool, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	
	_, exists := cs.cache[key]
	return exists, nil
}

// GetTTL lấy TTL còn lại của key
func (cs *CacheService) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	
	timestamp, exists := cs.timestamps[key]
	if !exists {
		return 0, nil
	}
	
	remaining := cs.ttl - time.Since(timestamp)
	if remaining < 0 {
		return 0, nil
	}
	
	return remaining, nil
}

// StartCleanupWorker khởi động worker dọn dẹp cache
func (cs *CacheService) StartCleanupWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			cs.CleanupExpired()
		}
	}()
}

// Close đóng kết nối (không cần thiết cho in-memory cache)
func (cs *CacheService) Close() error {
	return nil
}
