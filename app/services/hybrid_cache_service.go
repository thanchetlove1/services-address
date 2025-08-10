package services

import (
	"context"
	"fmt"
	"time"

	"github.com/address-parser/app/models"
	"go.uber.org/zap"
)

// HybridCacheService cache service kết hợp Redis (L1) + MongoDB (L2)
type HybridCacheService struct {
	redisCache *RedisCacheService  // L1 cache - nhanh
	mongoCache *MongoCacheService  // L2 cache - persistent
	logger     *zap.Logger
}

// NewHybridCacheService tạo mới hybrid cache service
func NewHybridCacheService(redisCache *RedisCacheService, mongoCache *MongoCacheService, logger *zap.Logger) *HybridCacheService {
	return &HybridCacheService{
		redisCache: redisCache,
		mongoCache: mongoCache,
		logger:     logger,
	}
}

// Get lấy address result từ cache (Redis trước, MongoDB sau)
func (hcs *HybridCacheService) Get(ctx context.Context, key string) (*models.AddressResult, bool, error) {
	// 1. Thử Redis cache trước (L1)
	result, found, err := hcs.redisCache.Get(ctx, key)
	if err != nil {
		hcs.logger.Warn("Lỗi Redis cache, fallback MongoDB", zap.Error(err))
	} else if found {
		hcs.logger.Debug("L1 cache hit (Redis)", zap.String("key", key))
		return result, true, nil
	}

	// 2. Nếu không có trong Redis, thử MongoDB (L2)
	result, found, err = hcs.mongoCache.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}
	if !found {
		hcs.logger.Debug("Cache miss (both Redis & MongoDB)", zap.String("key", key))
		return nil, false, nil
	}

	// 3. Nếu có trong MongoDB, đồng bộ lên Redis
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := hcs.redisCache.Set(bgCtx, key, result); err != nil {
			hcs.logger.Warn("Lỗi sync MongoDB->Redis", zap.Error(err), zap.String("key", key))
		} else {
			hcs.logger.Debug("Synced MongoDB->Redis", zap.String("key", key))
		}
	}()

	hcs.logger.Debug("L2 cache hit (MongoDB)", zap.String("key", key))
	return result, true, nil
}

// Set lưu address result vào cache (cả Redis và MongoDB)
func (hcs *HybridCacheService) Set(ctx context.Context, key string, result *models.AddressResult) error {
	// Lưu vào cả 2 cache song song
	errCh := make(chan error, 2)

	// Save to Redis (L1)
	go func() {
		err := hcs.redisCache.Set(ctx, key, result)
		if err != nil {
			hcs.logger.Warn("Lỗi lưu vào Redis", zap.Error(err))
		}
		errCh <- err
	}()

	// Save to MongoDB (L2)
	go func() {
		err := hcs.mongoCache.Set(ctx, key, result)
		if err != nil {
			hcs.logger.Warn("Lỗi lưu vào MongoDB", zap.Error(err))
		}
		errCh <- err
	}()

	// Đợi cả 2 hoàn thành
	var errs []error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cache errors: %v", errs)
	}

	hcs.logger.Debug("Saved to hybrid cache", zap.String("key", key))
	return nil
}

// Delete xóa key khỏi cache (cả Redis và MongoDB)
func (hcs *HybridCacheService) Delete(ctx context.Context, key string) error {
	// Xóa từ cả 2 cache
	errCh := make(chan error, 2)

	go func() {
		errCh <- hcs.redisCache.Delete(ctx, key)
	}()

	go func() {
		errCh <- hcs.mongoCache.Delete(ctx, key)
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("delete errors: %v", errs)
	}

	return nil
}

// Clear xóa toàn bộ cache (cả Redis và MongoDB)
func (hcs *HybridCacheService) Clear(ctx context.Context) error {
	errCh := make(chan error, 2)

	go func() {
		errCh <- hcs.redisCache.Clear(ctx)
	}()

	go func() {
		errCh <- hcs.mongoCache.Clear(ctx)
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("clear errors: %v", errs)
	}

	hcs.logger.Info("Cleared hybrid cache (Redis + MongoDB)")
	return nil
}

// InvalidateByGazetteerVersion xóa cache theo phiên bản gazetteer
func (hcs *HybridCacheService) InvalidateByGazetteerVersion(ctx context.Context, gazetteerVersion string) error {
	errCh := make(chan error, 2)

	go func() {
		errCh <- hcs.redisCache.InvalidateByGazetteerVersion(ctx, gazetteerVersion)
	}()

	go func() {
		errCh <- hcs.mongoCache.InvalidateByGazetteerVersion(ctx, gazetteerVersion)
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalidate errors: %v", errs)
	}

	hcs.logger.Info("Invalidated hybrid cache", zap.String("gazetteer_version", gazetteerVersion))
	return nil
}

// GetStats lấy thống kê cache (kết hợp từ cả 2)
func (hcs *HybridCacheService) GetStats(ctx context.Context) (*CacheStats, error) {
	redisStats, redisErr := hcs.redisCache.GetStats(ctx)
	mongoStats, mongoErr := hcs.mongoCache.GetStats(ctx)

	if redisErr != nil && mongoErr != nil {
		return nil, fmt.Errorf("cả Redis và MongoDB đều lỗi: %v, %v", redisErr, mongoErr)
	}

	// Kết hợp stats từ cả 2
	combinedStats := &CacheStats{}

	if redisErr == nil && mongoErr == nil {
		// Cả 2 đều OK
		totalHits := redisStats.TotalHits + mongoStats.TotalHits
		totalMiss := redisStats.TotalMiss + mongoStats.TotalMiss
		total := totalHits + totalMiss
		
		if total > 0 {
			combinedStats.HitRate = float64(totalHits) / float64(total)
		}
		combinedStats.TotalHits = totalHits
		combinedStats.TotalMiss = totalMiss
		combinedStats.TotalItems = redisStats.TotalItems + mongoStats.TotalItems
	} else if redisErr == nil {
		// Chỉ Redis OK
		*combinedStats = *redisStats
	} else {
		// Chỉ MongoDB OK
		*combinedStats = *mongoStats
	}

	return combinedStats, nil
}

// Exists kiểm tra key có tồn tại không (Redis trước, MongoDB sau)
func (hcs *HybridCacheService) Exists(ctx context.Context, key string) (bool, error) {
	// Kiểm tra Redis trước
	exists, err := hcs.redisCache.Exists(ctx, key)
	if err != nil {
		hcs.logger.Warn("Lỗi check Redis exists, fallback MongoDB", zap.Error(err))
	} else if exists {
		return true, nil
	}

	// Fallback MongoDB
	return hcs.mongoCache.Exists(ctx, key)
}

// GetTTL lấy TTL của key (từ Redis)
func (hcs *HybridCacheService) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return hcs.redisCache.GetTTL(ctx, key)
}

// Close đóng kết nối cả 2 cache
func (hcs *HybridCacheService) Close() error {
	errCh := make(chan error, 2)

	go func() {
		errCh <- hcs.redisCache.Close()
	}()

	go func() {
		errCh <- hcs.mongoCache.Close()
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}

// WarmUpFromMongoDB làm nóng Redis cache từ MongoDB
func (hcs *HybridCacheService) WarmUpFromMongoDB(ctx context.Context, limit int) error {
	return hcs.mongoCache.WarmUp(ctx, limit)
}
