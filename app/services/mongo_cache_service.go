package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/address-parser/app/models"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// MongoCacheService persistent cache service sử dụng MongoDB + LRU in-memory
type MongoCacheService struct {
	db                *mongo.Database
	collection        *mongo.Collection
	l1Cache           *lru.Cache[string, *models.AddressResult] // LRU in-memory cache
	logger            *zap.Logger
	
	// Metrics
	totalHits   int64
	totalMiss   int64
	l1Hits      int64
	l1Miss      int64
	mongoHits   int64
	mongoMiss   int64
}

// NewMongoCacheService tạo mới MongoCacheService
func NewMongoCacheService(db *mongo.Database, l1Size int, logger *zap.Logger) (*MongoCacheService, error) {
	// Tạo LRU cache
	l1Cache, err := lru.New[string, *models.AddressResult](l1Size)
	if err != nil {
		return nil, fmt.Errorf("không thể tạo LRU cache: %w", err)
	}

	collection := db.Collection("address_cache")
	
	// Tạo indexes cho performance
	indexModels := []mongo.IndexModel{
		{
			Keys: bson.D{bson.E{Key: "raw_fingerprint", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{bson.E{Key: "gazetteer_version", Value: 1}},
		},
		{
			Keys: bson.D{bson.E{Key: "created_at", Value: 1}},
		},
		{
			Keys: bson.D{bson.E{Key: "last_accessed", Value: 1}},
		},
		{
			Keys: bson.D{bson.E{Key: "manually_verified", Value: 1}},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = collection.Indexes().CreateMany(ctx, indexModels)
	if err != nil {
		logger.Warn("Không thể tạo indexes cho address_cache", zap.Error(err))
	}

	service := &MongoCacheService{
		db:         db,
		collection: collection,
		l1Cache:    l1Cache,
		logger:     logger,
	}

	return service, nil
}

// Get lấy địa chỉ từ cache (L1 → MongoDB)
func (mcs *MongoCacheService) Get(ctx context.Context, key string) (*models.AddressResult, bool, error) {
	// 1. Thử L1 cache trước (in-memory LRU)
	if result, found := mcs.l1Cache.Get(key); found {
		mcs.l1Hits++
		mcs.totalHits++
		mcs.logger.Debug("L1 cache hit", zap.String("key", key))
		return result, true, nil
	}
	mcs.l1Miss++

	// 2. Thử MongoDB persistent cache
	fingerprint := mcs.generateFingerprint(key)
	
	var cacheEntry models.AddressCache
	filter := bson.M{"raw_fingerprint": fingerprint}
	
	err := mcs.collection.FindOne(ctx, filter).Decode(&cacheEntry)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			mcs.mongoMiss++
			mcs.totalMiss++
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("lỗi query MongoDB cache: %w", err)
	}

	mcs.mongoHits++
	mcs.totalHits++

	// Update last_accessed và access_count
	go mcs.updateAccessStats(ctx, cacheEntry.ID)

	// Lưu vào L1 cache cho lần sau
	mcs.l1Cache.Add(key, &cacheEntry.ParsedResult)

	mcs.logger.Debug("MongoDB cache hit",
		zap.String("key", key),
		zap.String("fingerprint", fingerprint))

	return &cacheEntry.ParsedResult, true, nil
}

// Set lưu địa chỉ vào cache (L1 + MongoDB)
func (mcs *MongoCacheService) Set(ctx context.Context, key string, result *models.AddressResult) error {
	// 1. Lưu vào L1 cache
	mcs.l1Cache.Add(key, result)

	// 2. Lưu vào MongoDB persistent cache
	fingerprint := mcs.generateFingerprint(key)
	
	cacheEntry := models.AddressCache{
		RawFingerprint:         fingerprint,
		RawAddress:             result.Raw,
		NormalizedNoDiacritics: result.NormalizedNoDiacritics,
		CanonicalText:          result.CanonicalText,
		ParsedResult:           *result,
		Confidence:             result.Confidence,
		MatchStrategy:          result.MatchStrategy,
		GazetteerVersion:       mcs.extractGazetteerVersion(result),
		ManuallyVerified:       false,
		CreatedAt:              time.Now(),
		LastAccessed:           time.Now(),
		AccessCount:            1,
	}

	// Upsert to MongoDB
	opts := options.Replace().SetUpsert(true)
	filter := bson.M{"raw_fingerprint": fingerprint}
	
	_, err := mcs.collection.ReplaceOne(ctx, filter, cacheEntry, opts)
	if err != nil {
		mcs.logger.Error("Lỗi lưu vào MongoDB cache",
			zap.Error(err),
			zap.String("fingerprint", fingerprint))
		return fmt.Errorf("lỗi lưu vào MongoDB cache: %w", err)
	}

	mcs.logger.Debug("Đã lưu vào cache",
		zap.String("key", key),
		zap.String("fingerprint", fingerprint),
		zap.Float64("confidence", result.Confidence))

	return nil
}

// Delete xóa địa chỉ khỏi cache
func (mcs *MongoCacheService) Delete(ctx context.Context, key string) error {
	// 1. Xóa khỏi L1 cache
	mcs.l1Cache.Remove(key)

	// 2. Xóa khỏi MongoDB
	fingerprint := mcs.generateFingerprint(key)
	filter := bson.M{"raw_fingerprint": fingerprint}
	
	_, err := mcs.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("lỗi xóa khỏi MongoDB cache: %w", err)
	}

	return nil
}

// Clear xóa tất cả cache
func (mcs *MongoCacheService) Clear(ctx context.Context) error {
	// 1. Clear L1 cache
	mcs.l1Cache.Purge()

	// 2. Clear MongoDB cache
	_, err := mcs.collection.DeleteMany(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("lỗi clear MongoDB cache: %w", err)
	}

	// Reset metrics
	mcs.totalHits = 0
	mcs.totalMiss = 0
	mcs.l1Hits = 0
	mcs.l1Miss = 0
	mcs.mongoHits = 0
	mcs.mongoMiss = 0

	return nil
}

// InvalidateByGazetteerVersion invalidate cache theo gazetteer version
func (mcs *MongoCacheService) InvalidateByGazetteerVersion(ctx context.Context, gazetteerVersion string) error {
	// 1. Clear toàn bộ L1 cache (đơn giản nhất)
	mcs.l1Cache.Purge()

	// 2. Xóa records trong MongoDB có gazetteer_version cũ
	filter := bson.M{"gazetteer_version": bson.M{"$ne": gazetteerVersion}}
	
	result, err := mcs.collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("lỗi invalidate cache theo gazetteer version: %w", err)
	}

	mcs.logger.Info("Đã invalidate cache",
		zap.String("gazetteer_version", gazetteerVersion),
		zap.Int64("deleted_count", result.DeletedCount))

	return nil
}

// GetStats lấy thống kê cache
func (mcs *MongoCacheService) GetStats(ctx context.Context) (*CacheStats, error) {
	// L1 cache stats
	l1Size := mcs.l1Cache.Len()

	// MongoDB cache stats
	mongoCount, err := mcs.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("lỗi đếm documents trong MongoDB cache: %w", err)
	}

	// Calculate hit rate
	total := mcs.totalHits + mcs.totalMiss
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(mcs.totalHits) / float64(total)
	}

	stats := &CacheStats{
		HitRate:    hitRate,
		TotalHits:  mcs.totalHits,
		TotalMiss:  mcs.totalMiss,
		TotalItems: mongoCount,
	}

	mcs.logger.Debug("Cache stats",
		zap.Float64("hit_rate", hitRate),
		zap.Int64("total_hits", mcs.totalHits),
		zap.Int64("total_miss", mcs.totalMiss),
		zap.Int("l1_size", l1Size),
		zap.Int64("mongo_count", mongoCount))

	return stats, nil
}

// Exists kiểm tra key có tồn tại không
func (mcs *MongoCacheService) Exists(ctx context.Context, key string) (bool, error) {
	// Check L1 first
	if mcs.l1Cache.Contains(key) {
		return true, nil
	}

	// Check MongoDB
	fingerprint := mcs.generateFingerprint(key)
	filter := bson.M{"raw_fingerprint": fingerprint}
	
	count, err := mcs.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("lỗi check exists trong MongoDB: %w", err)
	}

	return count > 0, nil
}

// GetTTL lấy TTL còn lại của key (MongoDB cache không có TTL, luôn trả về 0)
func (mcs *MongoCacheService) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	// MongoDB persistent cache không có TTL
	return 0, nil
}

// Close đóng kết nối
func (mcs *MongoCacheService) Close() error {
	// L1 cache không cần close
	// MongoDB connection được quản lý bởi caller
	return nil
}

// generateFingerprint sinh fingerprint cho cache key
func (mcs *MongoCacheService) generateFingerprint(key string) string {
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("sha256:%x", hash)
}

// extractGazetteerVersion trích xuất gazetteer version từ result
func (mcs *MongoCacheService) extractGazetteerVersion(result *models.AddressResult) string {
	// Extractfrom fingerprint hoặc từ result nếu có
	// Hiện tại return default version
	return "1.0.0" // TODO: implement proper extraction
}

// updateAccessStats cập nhật thống kê truy cập (async)
func (mcs *MongoCacheService) updateAccessStats(ctx context.Context, id primitive.ObjectID) {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{"last_accessed": time.Now()},
		"$inc": bson.M{"access_count": 1},
	}

	_, err := mcs.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		mcs.logger.Warn("Lỗi update access stats", zap.Error(err))
	}
}

// GetL1Stats lấy thống kê L1 cache
func (mcs *MongoCacheService) GetL1Stats() map[string]interface{} {
	return map[string]interface{}{
		"l1_size":     mcs.l1Cache.Len(),
		"l1_hits":     mcs.l1Hits,
		"l1_miss":     mcs.l1Miss,
		"mongo_hits":  mcs.mongoHits,
		"mongo_miss":  mcs.mongoMiss,
		"total_hits":  mcs.totalHits,
		"total_miss":  mcs.totalMiss,
	}
}

// WarmUp làm nóng cache từ MongoDB vào L1
func (mcs *MongoCacheService) WarmUp(ctx context.Context, limit int) error {
	// Lấy các records được truy cập nhiều nhất từ MongoDB
	opts := options.Find().
		SetSort(bson.D{bson.E{Key: "access_count", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := mcs.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return fmt.Errorf("lỗi warm up cache: %w", err)
	}
	defer cursor.Close(ctx)

	count := 0
	for cursor.Next(ctx) {
		var cacheEntry models.AddressCache
		if err := cursor.Decode(&cacheEntry); err != nil {
			mcs.logger.Warn("Lỗi decode cache entry trong warm up", zap.Error(err))
			continue
		}

		// Add vào L1 cache
		mcs.l1Cache.Add(cacheEntry.RawAddress, &cacheEntry.ParsedResult)
		count++
	}

	mcs.logger.Info("Cache warm up hoàn thành",
		zap.Int("loaded_items", count),
		zap.Int("l1_size", mcs.l1Cache.Len()))

	return nil
}
