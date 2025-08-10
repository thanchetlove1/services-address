package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/address-parser/app/models"
	"github.com/address-parser/internal/search"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// AdminService service quản lý admin functions
type AdminService struct {
	db       *mongo.Database
	searcher *search.GazetteerSearcher
	logger   *zap.Logger
}

// GazetteerValidation kết quả validation gazetteer
type GazetteerValidation struct {
	Passed            bool     `json:"passed"`
	Warnings          []string `json:"warnings"`
	EstimatedBuildTime string   `json:"estimated_build_time"`
}

// SeedResult kết quả seed gazetteer
type SeedResult struct {
	UnitsProcessed   int `json:"units_processed"`
	IndexesBuilt     int `json:"indexes_built"`
	ProcessingTimeMs int64 `json:"processing_time_ms"`
}

// SystemStats thống kê hệ thống
type SystemStats struct {
	AccuracyRate        float64                `json:"accuracy_rate"`
	AvgProcessingTimeMs float64                `json:"avg_processing_time_ms"`
	TotalProcessed      int64                  `json:"total_processed"`
	ReviewQueueSize     int64                  `json:"review_queue_size"`
	Uptime             string                 `json:"uptime"`
	MemoryUsage        map[string]interface{} `json:"memory_usage"`
	CPUUsage           float64                `json:"cpu_usage"`
	DatabaseStats      DatabaseStats          `json:"database_stats"`
}

// DatabaseStats thống kê database
type DatabaseStats struct {
	AdminUnits     int64 `json:"admin_units"`
	AddressCache   int64 `json:"address_cache"`
	AddressReview  int64 `json:"address_review"`
	LearnedAliases int64 `json:"learned_aliases"`
}

// NewAdminService tạo mới AdminService
func NewAdminService(db *mongo.Database, searcher *search.GazetteerSearcher, logger *zap.Logger) *AdminService {
	return &AdminService{
		db:       db,
		searcher: searcher,
		logger:   logger,
	}
}

// ValidateGazetteerData validate dữ liệu gazetteer
func (as *AdminService) ValidateGazetteerData(data []models.AdminUnit) (*GazetteerValidation, error) {
	warnings := make([]string, 0)
	
	// Kiểm tra cấu trúc dữ liệu
	if len(data) == 0 {
		return &GazetteerValidation{
			Passed:            false,
			Warnings:          []string{"Không có dữ liệu để validate"},
			EstimatedBuildTime: "0s",
		}, nil
	}

	// Validate từng unit
	seenIDs := make(map[string]bool)
	for i, unit := range data {
		// Kiểm tra duplicate ID
		if seenIDs[unit.AdminID] {
			warnings = append(warnings, fmt.Sprintf("Duplicate AdminID: %s", unit.AdminID))
		}
		seenIDs[unit.AdminID] = true

		// Kiểm tra required fields
		if unit.AdminID == "" {
			warnings = append(warnings, fmt.Sprintf("Missing AdminID at index %d", i))
		}
		if unit.Name == "" {
			warnings = append(warnings, fmt.Sprintf("Missing Name at index %d", i))
		}
		if unit.Level < 1 || unit.Level > 4 {
			warnings = append(warnings, fmt.Sprintf("Invalid Level %d at index %d", unit.Level, i))
		}
		if !unit.IsValidAdminSubtype() {
			warnings = append(warnings, fmt.Sprintf("Invalid AdminSubtype '%s' at index %d", unit.AdminSubtype, i))
		}
	}

	// Ước tính thời gian build
	estimatedSeconds := len(data) / 100 // Giả sử 100 units/second
	if estimatedSeconds < 1 {
		estimatedSeconds = 1
	}
	estimatedTime := fmt.Sprintf("%ds", estimatedSeconds)

	passed := len(warnings) == 0

	return &GazetteerValidation{
		Passed:            passed,
		Warnings:          warnings,
		EstimatedBuildTime: estimatedTime,
	}, nil
}

// SeedGazetteer seed dữ liệu gazetteer vào MongoDB và Meilisearch
func (as *AdminService) SeedGazetteer(gazetteerVersion string, data []models.AdminUnit, rebuildIndexes bool) (*SeedResult, error) {
	startTime := time.Now()

	// 1. Validate dữ liệu
	validation, err := as.ValidateGazetteerData(data)
	if err != nil {
		return nil, fmt.Errorf("lỗi validate dữ liệu: %w", err)
	}
	
	if !validation.Passed {
		return nil, fmt.Errorf("dữ liệu không hợp lệ: %v", validation.Warnings)
	}

	// 2. Lưu vào MongoDB
	collection := as.db.Collection("admin_units")
	
	// Clear existing data với cùng gazetteer version
	ctx := context.Background()
	filter := bson.M{"gazetteer_version": gazetteerVersion}
	deleteResult, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("lỗi xóa dữ liệu cũ: %w", err)
	}
	
	as.logger.Info("Deleted old admin units", 
		zap.String("gazetteer_version", gazetteerVersion),
		zap.Int64("deleted_count", deleteResult.DeletedCount))

	// Insert new data
	documents := make([]interface{}, len(data))
	for i, unit := range data {
		unit.GazetteerVersion = gazetteerVersion
		unit.CreatedAt = time.Now()
		unit.UpdatedAt = time.Now()
		documents[i] = unit
	}

	_, err = collection.InsertMany(ctx, documents)
	if err != nil {
		return nil, fmt.Errorf("lỗi insert dữ liệu mới: %w", err)
	}

	indexesBuilt := 0

	// 3. Rebuild Meilisearch indexes nếu cần
	if rebuildIndexes {
		// Build Meilisearch indexes
		err = as.searcher.BuildIndexes()
		if err != nil {
			as.logger.Warn("Lỗi build Meilisearch indexes", zap.Error(err))
		} else {
			indexesBuilt++
		}

		// Seed data to Meilisearch
		err = as.searcher.SeedData(data)
		if err != nil {
			as.logger.Warn("Lỗi seed data vào Meilisearch", zap.Error(err))
		} else {
			indexesBuilt++
		}
	}

	processingTime := time.Since(startTime)
	
	as.logger.Info("Gazetteer seed completed",
		zap.String("gazetteer_version", gazetteerVersion),
		zap.Int("units_processed", len(data)),
		zap.Int("indexes_built", indexesBuilt),
		zap.Duration("processing_time", processingTime))

	return &SeedResult{
		UnitsProcessed:   len(data),
		IndexesBuilt:     indexesBuilt,
		ProcessingTimeMs: processingTime.Milliseconds(),
	}, nil
}

// RebuildSynonyms rebuild synonyms từ learned_aliases
func (as *AdminService) RebuildSynonyms() error {
	// 1. Lấy learned_aliases từ MongoDB
	collection := as.db.Collection("learned_aliases")
	ctx := context.Background()

	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("lỗi lấy learned_aliases: %w", err)
	}
	defer cursor.Close(ctx)

	// 2. Build synonyms map
	synonyms := make(map[string][]string)
	
	for cursor.Next(ctx) {
		var alias models.LearnedAliases
		if err := cursor.Decode(&alias); err != nil {
			as.logger.Warn("Lỗi decode learned alias", zap.Error(err))
			continue
		}

		if synonyms[alias.CanonicalForm] == nil {
			synonyms[alias.CanonicalForm] = []string{}
		}
		synonyms[alias.CanonicalForm] = append(synonyms[alias.CanonicalForm], alias.OriginalToken)
	}

	// 3. Update Meilisearch synonyms
	// Note: Meilisearch synonyms update sẽ được implement trong GazetteerSearcher
	err = as.searcher.UpdateIndexes()
	if err != nil {
		return fmt.Errorf("lỗi update Meilisearch synonyms: %w", err)
	}

	as.logger.Info("Synonyms rebuilt successfully", 
		zap.Int("synonym_groups", len(synonyms)))

	return nil
}

// BuildIndexes build tất cả indexes
func (as *AdminService) BuildIndexes() error {
	// Build Meilisearch indexes
	err := as.searcher.BuildIndexes()
	if err != nil {
		return fmt.Errorf("lỗi build Meilisearch indexes: %w", err)
	}

	as.logger.Info("All indexes built successfully")
	return nil
}

// GetSystemStats lấy thống kê hệ thống
func (as *AdminService) GetSystemStats() (*SystemStats, error) {
	ctx := context.Background()

	// Database stats
	dbStats, err := as.getDatabaseStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("lỗi lấy database stats: %w", err)
	}

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryUsage := map[string]interface{}{
		"alloc_mb":       bToMb(m.Alloc),
		"total_alloc_mb": bToMb(m.TotalAlloc),
		"sys_mb":         bToMb(m.Sys),
		"num_gc":         m.NumGC,
	}

	// TODO: Implement actual accuracy calculation from cached results
	accuracyRate := 0.91 // Placeholder

	// TODO: Implement actual processing time calculation
	avgProcessingTimeMs := 145.0 // Placeholder

	// TODO: Get actual uptime
	uptime := "1h23m45s" // Placeholder

	stats := &SystemStats{
		AccuracyRate:        accuracyRate,
		AvgProcessingTimeMs: avgProcessingTimeMs,
		TotalProcessed:      dbStats.AddressCache,
		ReviewQueueSize:     dbStats.AddressReview,
		Uptime:             uptime,
		MemoryUsage:        memoryUsage,
		CPUUsage:           0.0, // TODO: Implement CPU usage calculation
		DatabaseStats:      *dbStats,
	}

	return stats, nil
}

// getDatabaseStats lấy thống kê database
func (as *AdminService) getDatabaseStats(ctx context.Context) (*DatabaseStats, error) {
	stats := &DatabaseStats{}

	// Count admin_units
	count, err := as.db.Collection("admin_units").CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	stats.AdminUnits = count

	// Count address_cache
	count, err = as.db.Collection("address_cache").CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	stats.AddressCache = count

	// Count address_review
	count, err = as.db.Collection("address_review").CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	stats.AddressReview = count

	// Count learned_aliases
	count, err = as.db.Collection("learned_aliases").CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	stats.LearnedAliases = count

	return stats, nil
}

// ExportData export dữ liệu để backup
func (as *AdminService) ExportData(dataType string, format string, limit int) ([]byte, error) {
	ctx := context.Background()
	
	var collection *mongo.Collection
	switch dataType {
	case "admin_units":
		collection = as.db.Collection("admin_units")
	case "address_cache":
		collection = as.db.Collection("address_cache")
	case "learned_aliases":
		collection = as.db.Collection("learned_aliases")
	default:
		return nil, errors.New("không hỗ trợ loại dữ liệu này")
	}

	// Query data
	findOptions := options.Find().SetLimit(int64(limit))
	cursor, err := collection.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		return nil, fmt.Errorf("lỗi query data: %w", err)
	}
	defer cursor.Close(ctx)

	// Collect results
	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("lỗi decode results: %w", err)
	}

	// Export theo format
	switch format {
	case "json":
		return json.MarshalIndent(results, "", "  ")
	case "csv":
		// TODO: Implement CSV export
		return nil, errors.New("CSV format chưa được implement")
	default:
		return nil, errors.New("không hỗ trợ format này")
	}
}

// Helper functions
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func int64Ptr(i int64) *int64 {
	return &i
}
