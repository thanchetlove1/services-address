package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/address-parser/app/controllers"
	"github.com/address-parser/app/services"
	"github.com/address-parser/internal/normalizer"
	"github.com/address-parser/internal/parser"
	"github.com/address-parser/internal/search"
	"github.com/address-parser/routes"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func main() {
	// 1. Load configuration
	loadConfig()

	// 2. Khởi tạo logger
	logger := initLogger()
	defer logger.Sync()

	logger.Info("Starting Address Parser Service")

	// 3. Kết nối MongoDB
	mongoDB := initMongoDB(logger)
	defer func() {
		if err := mongoDB.Client().Disconnect(context.Background()); err != nil {
			logger.Error("Error disconnecting MongoDB", zap.Error(err))
		}
	}()

	// 4. Khởi tạo Meilisearch
	searchConfig := search.SearchConfig{
		Host:          viper.GetString("meilisearch.url"),
		APIKey:        viper.GetString("meilisearch.master_key"),
		IndexName:     "admin_units",
		Timeout:       30 * time.Second,
		MaxCandidates: 20,
	}

	logger.Info("Meilisearch config", 
		zap.String("host", searchConfig.Host),
		zap.String("key", searchConfig.APIKey[:10]+"..."))  // Log only first 10 chars for security

	gazetteerSearcher, err := search.NewGazetteerSearcher(searchConfig, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Meilisearch", zap.Error(err))
	}

	// 5. Khởi tạo components
	textNormalizer := normalizer.NewTextNormalizerV2()
	addressParser := parser.NewAddressParser(gazetteerSearcher, textNormalizer, logger)

	// 6. Khởi tạo cache services (Redis L1 + MongoDB L2)
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	redisCache, err := services.NewRedisCacheService(redisURL, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Redis cache", zap.Error(err))
	}

	l1Size := getEnvInt("L1_CACHE_SIZE", 10000)
	mongoCache, err := services.NewMongoCacheService(mongoDB, l1Size, logger)
	if err != nil {
		logger.Fatal("Failed to initialize MongoDB cache", zap.Error(err))
	}

	// Hybrid cache service (Redis L1 + MongoDB L2)
	cacheService := services.NewHybridCacheService(redisCache, mongoCache, logger)

	// 7. Warm up cache từ MongoDB
	if err := mongoCache.WarmUp(context.Background(), l1Size/2); err != nil {
		logger.Warn("Failed to warm up cache", zap.Error(err))
	}

	// 8. Khởi tạo services
	addressService := services.NewAddressService(addressParser, textNormalizer, gazetteerSearcher, logger)
	adminService := services.NewAdminService(mongoDB, gazetteerSearcher, logger)

	// 9. Khởi tạo controllers
	addressController := controllers.NewAddressController(addressService, cacheService, logger)
	adminController := controllers.NewAdminController(adminService, cacheService, logger)

	// 10. Khởi tạo Gin router
	router := gin.Default()
	
	// Add middleware
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// 11. Thiết lập routes
	routes.SetupAllRoutes(router, addressController, adminController)

	// 12. Build Meilisearch indexes nếu cần
	if err := gazetteerSearcher.BuildIndexes(); err != nil {
		logger.Warn("Failed to build Meilisearch indexes", zap.Error(err))
	}

	// 13. Khởi động server
	port := getEnv("APP_PORT", "8080")
	logger.Info("Address Parser Service starting", zap.String("port", port))
	
	if err := router.Run(":" + port); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}

// loadConfig load configuration từ file và env vars
func loadConfig() {
	viper.SetConfigName("app")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	// Set defaults
	viper.SetDefault("app.port", "8080")
	viper.SetDefault("app.env", "development")
	viper.SetDefault("meilisearch.url", "http://meili:7700")
	viper.SetDefault("meilisearch.master_key", "5pAVWqmP046jvNzQwD70n8b5AdEyhW3lwWUZ1g5CZ8k")
	viper.SetDefault("mongo.url", "mongodb://localhost:27017/address_parser")
	viper.SetDefault("cache.l1_size", 10000)

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Cannot read config file: %v", err)
	}
}

// initLogger khởi tạo structured logger
func initLogger() *zap.Logger {
	env := getEnv("APP_ENV", "development")
	
	var config zap.Config
	if env == "production" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	logger, err := config.Build()
	if err != nil {
		log.Fatal("Cannot initialize logger:", err)
	}

	return logger
}

// initMongoDB khởi tạo kết nối MongoDB
func initMongoDB(logger *zap.Logger) *mongo.Database {
	mongoURL := getEnv("MONGO_URL", "mongodb://localhost:27017/address_parser")
	
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURL))
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		logger.Fatal("Failed to ping MongoDB", zap.Error(err))
	}

	// Extract database name from URI
	clientOpts := options.Client().ApplyURI(mongoURL)
	if clientOpts.Auth != nil && clientOpts.Auth.AuthSource != "" {
		// Use auth source as database name if available
	}

	dbName := "address_parser"
	if clientOpts.Auth != nil && clientOpts.Auth.AuthSource != "" {
		dbName = clientOpts.Auth.AuthSource
	}

	db := client.Database(dbName)
	logger.Info("Connected to MongoDB", zap.String("database", dbName))

	return db
}

// getEnv lấy environment variable với default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt lấy environment variable as int với default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
