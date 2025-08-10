package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/address-parser/app/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisCacheService cache service sử dụng Redis
type RedisCacheService struct {
	client  *redis.Client
	logger  *zap.Logger
	prefix  string
	ttl     time.Duration
	
	// Stats
	hits   int64
	misses int64
}

// NewRedisCacheService tạo mới Redis cache service
func NewRedisCacheService(redisURL string, logger *zap.Logger) (*RedisCacheService, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("lỗi parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	_, err = client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối Redis: %w", err)
	}

	return &RedisCacheService{
		client: client,
		logger: logger,
		prefix: "addr_parser:",
		ttl:    24 * time.Hour, // TTL mặc định 24h
		hits:   0,
		misses: 0,
	}, nil
}

// Get lấy address result từ cache
func (rcs *RedisCacheService) Get(ctx context.Context, key string) (*models.AddressResult, bool, error) {
	cacheKey := rcs.prefix + key
	
	val, err := rcs.client.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		rcs.misses++
		return nil, false, nil
	}
	if err != nil {
		rcs.logger.Error("Lỗi get từ Redis", zap.Error(err), zap.String("key", cacheKey))
		return nil, false, err
	}

	var result models.AddressResult
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		rcs.logger.Error("Lỗi unmarshal cache data", zap.Error(err))
		return nil, false, err
	}

	rcs.hits++
	rcs.logger.Debug("Redis cache hit", zap.String("key", key))
	return &result, true, nil
}

// Set lưu address result vào cache
func (rcs *RedisCacheService) Set(ctx context.Context, key string, result *models.AddressResult) error {
	cacheKey := rcs.prefix + key
	
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("lỗi marshal cache data: %w", err)
	}

	err = rcs.client.Set(ctx, cacheKey, data, rcs.ttl).Err()
	if err != nil {
		rcs.logger.Error("Lỗi set vào Redis", zap.Error(err), zap.String("key", cacheKey))
		return err
	}

	rcs.logger.Debug("Đã lưu vào Redis cache", zap.String("key", key))
	return nil
}

// Delete xóa key khỏi cache
func (rcs *RedisCacheService) Delete(ctx context.Context, key string) error {
	cacheKey := rcs.prefix + key
	
	err := rcs.client.Del(ctx, cacheKey).Err()
	if err != nil {
		rcs.logger.Error("Lỗi delete từ Redis", zap.Error(err), zap.String("key", cacheKey))
		return err
	}

	rcs.logger.Debug("Đã xóa khỏi Redis cache", zap.String("key", key))
	return nil
}

// Clear xóa toàn bộ cache
func (rcs *RedisCacheService) Clear(ctx context.Context) error {
	pattern := rcs.prefix + "*"
	keys, err := rcs.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("lỗi lấy danh sách keys: %w", err)
	}

	if len(keys) > 0 {
		err = rcs.client.Del(ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("lỗi xóa keys: %w", err)
		}
	}

	rcs.logger.Info("Đã clear Redis cache", zap.Int("keys_deleted", len(keys)))
	return nil
}

// InvalidateByGazetteerVersion xóa cache theo phiên bản gazetteer
func (rcs *RedisCacheService) InvalidateByGazetteerVersion(ctx context.Context, gazetteerVersion string) error {
	// Redis không lưu gazetteer version trong key, nên phải clear all
	// Hoặc có thể implement pattern key với gazetteer version
	return rcs.Clear(ctx)
}

// GetStats lấy thống kê cache
func (rcs *RedisCacheService) GetStats(ctx context.Context) (*CacheStats, error) {
	_, err := rcs.client.Info(ctx, "memory").Result()
	if err != nil {
		rcs.logger.Warn("Không thể lấy Redis memory info", zap.Error(err))
	}

	total := rcs.hits + rcs.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(rcs.hits) / float64(total)
	}

	// Estimate số items từ pattern
	keys, err := rcs.client.Keys(ctx, rcs.prefix+"*").Result()
	totalItems := int64(0)
	if err == nil {
		totalItems = int64(len(keys))
	}

	return &CacheStats{
		HitRate:    hitRate,
		TotalHits:  rcs.hits,
		TotalMiss:  rcs.misses,
		TotalItems: totalItems,
	}, nil
}

// Exists kiểm tra key có tồn tại không
func (rcs *RedisCacheService) Exists(ctx context.Context, key string) (bool, error) {
	cacheKey := rcs.prefix + key
	
	exists, err := rcs.client.Exists(ctx, cacheKey).Result()
	if err != nil {
		return false, err
	}
	
	return exists > 0, nil
}

// GetTTL lấy TTL của key
func (rcs *RedisCacheService) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	cacheKey := rcs.prefix + key
	
	ttl, err := rcs.client.TTL(ctx, cacheKey).Result()
	if err != nil {
		return 0, err
	}
	
	return ttl, nil
}

// Close đóng kết nối Redis
func (rcs *RedisCacheService) Close() error {
	return rcs.client.Close()
}

// SetTTL thiết lập TTL cho service
func (rcs *RedisCacheService) SetTTL(ttl time.Duration) {
	rcs.ttl = ttl
}

// GetClient lấy Redis client (cho debug)
func (rcs *RedisCacheService) GetClient() *redis.Client {
	return rcs.client
}
