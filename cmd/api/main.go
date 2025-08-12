package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/address-parser/app/config"
	"github.com/address-parser/app/controllers"
	"github.com/address-parser/app/services"
	"github.com/address-parser/internal/normalizer"
	"github.com/address-parser/internal/parser"
	"github.com/address-parser/internal/search"
	"github.com/address-parser/routes"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	if err := config.Load("config/parser.yaml"); err != nil {
		panic(err)
	}

	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Address Parser Service...")

	// Initialize MongoDB connection
	mongoClient, err := initMongoDB(logger)
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			logger.Error("Failed to disconnect from MongoDB", zap.Error(err))
		}
	}()

	// Initialize services
	normalizerService := normalizer.NewTextNormalizerV2()
	
	// Initialize search components (temporarily without Meilisearch for MVP)
	searchConfig := search.SearchConfig{
		Host:          "http://localhost:7700",
		APIKey:        "",
		IndexName:     "admin_units",
		Timeout:       30 * time.Second,
		MaxCandidates: 100,
	}
	searcher, err := search.NewGazetteerSearcher(searchConfig, logger)
	if err != nil {
		logger.Fatal("Failed to create gazetteer searcher", zap.Error(err))
	}
	
	// Initialize parser
	addressParser := parser.NewAddressParser(searcher, normalizerService, logger)
	
	// Initialize address service
	addressService := services.NewAddressService(addressParser, normalizerService, searcher, logger)
	
	// Initialize cache service (using MongoDB cache for MVP)
	database := mongoClient.Database("address_parser")
	cacheService, err := services.NewMongoCacheService(database, 1000, logger)
	if err != nil {
		logger.Fatal("Failed to create cache service", zap.Error(err))
	}
	
	// Initialize admin service
	adminService := services.NewAdminService(database, searcher, logger)
	
	// Initialize controllers
	addressController := controllers.NewAddressController(addressService, cacheService, logger)
	adminController := controllers.NewAdminController(adminService, cacheService, logger)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	
	// Setup routes
	setupRoutes(router, addressController, adminController)

	// Start server
	port := getPort()
	go func() {
		logger.Info("Starting HTTP server", zap.String("port", port))
		if err := router.Run(":" + port); err != nil {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	
	// Give outstanding requests a deadline for completion
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// TODO: Add graceful shutdown logic here
	
	logger.Info("Server exited")
}

func initMongoDB(logger *zap.Logger) (*mongo.Client, error) {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}
	
	logger.Info("Connecting to MongoDB", zap.String("uri", mongoURI))
	
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, err
	}
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}
	
	logger.Info("Successfully connected to MongoDB")
	return client, nil
}

func setupRoutes(router *gin.Engine, addressController *controllers.AddressController, adminController *controllers.AdminController) {
	// Setup all routes using the existing routes package
	routes.SetupAllRoutes(router, addressController, adminController)
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}
