package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AdminUnitDoc structure for Meilisearch
type AdminUnitDoc struct {
	AdminID        string   `json:"admin_id" bson:"admin_id"`
	ParentID       *string  `json:"parent_id,omitempty" bson:"parent_id,omitempty"`
	Level          int      `json:"level" bson:"level"`
	Name           string   `json:"name" bson:"name"`
	NormalizedName string   `json:"normalized_name" bson:"normalized_name"`
	Aliases        []string `json:"aliases" bson:"aliases"`
	AdminSubtype   string   `json:"admin_subtype" bson:"admin_subtype"`
	Path           []string `json:"path" bson:"path"`
	Type           string   `json:"type" bson:"type"`
}

func main() {
	// MongoDB connection
	mongoClient, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal("Không thể kết nối MongoDB:", err)
	}
	defer mongoClient.Disconnect(context.TODO())

	// Meilisearch connection
	meiliClient := meilisearch.New("http://localhost:7700", meilisearch.WithAPIKey("5pAVWqmP046jvNzQwD70n8b5AdEyhW3lwWUZ1g5CZ8k"))

	// Test Meilisearch connection
	health, err := meiliClient.Health()
	if err != nil {
		log.Fatal("Không thể kết nối Meilisearch:", err)
	}
	fmt.Printf("Meilisearch status: %s\n", health.Status)

	// Get/Create index
	indexName := "admin_units"
	index := meiliClient.Index(indexName)

	// Set index settings
	fmt.Println("Đang cấu hình Meilisearch index settings...")
	settings := &meilisearch.Settings{
		SearchableAttributes: []string{"name", "normalized_name", "aliases"},
		FilterableAttributes: []string{"level", "parent_id", "admin_id", "path", "admin_subtype"},
		SortableAttributes:   []string{"level", "name"},
		RankingRules: []string{
			"words",
			"typo", 
			"proximity",
			"attribute",
			"sort",
			"exactness",
		},
	}

	task, err := index.UpdateSettings(settings)
	if err != nil {
		log.Fatal("Lỗi cập nhật settings:", err)
	}
	fmt.Printf("Settings update task ID: %d\n", task.TaskUID)

	// Wait for settings update to complete
	fmt.Println("Đang chờ settings update hoàn thành...")
	for {
		taskInfo, err := meiliClient.GetTask(task.TaskUID)
		if err != nil {
			log.Fatal("Lỗi check task status:", err)
		}
		if taskInfo.Status == "succeeded" {
			fmt.Println("Settings update thành công!")
			break
		} else if taskInfo.Status == "failed" {
			log.Fatal("Settings update thất bại:", taskInfo.Error)
		}
		time.Sleep(1 * time.Second)
	}

	// Get data from MongoDB
	fmt.Println("Đang lấy dữ liệu từ MongoDB...")
	collection := mongoClient.Database("address_parser").Collection("admin_units")
	
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		log.Fatal("Lỗi query MongoDB:", err)
	}
	defer cursor.Close(context.TODO())

	var documents []AdminUnitDoc
	batchSize := 1000
	totalProcessed := 0

	fmt.Println("Đang xử lý và seed dữ liệu vào Meilisearch...")

	for cursor.Next(context.TODO()) {
		var doc AdminUnitDoc
		if err := cursor.Decode(&doc); err != nil {
			fmt.Printf("Lỗi decode document: %v\n", err)
			continue
		}

		// Ensure required fields
		if doc.AdminID == "" {
			continue
		}
		if doc.NormalizedName == "" {
			doc.NormalizedName = doc.Name
		}
		if doc.Aliases == nil {
			doc.Aliases = []string{}
		}

		documents = append(documents, doc)

		// Batch insert when reaching batch size
		if len(documents) >= batchSize {
			if err := insertBatch(index, documents); err != nil {
				log.Printf("Lỗi insert batch: %v", err)
			} else {
				totalProcessed += len(documents)
				fmt.Printf("Đã xử lý %d documents...\n", totalProcessed)
			}
			documents = []AdminUnitDoc{} // Reset batch
		}
	}

	// Insert remaining documents
	if len(documents) > 0 {
		if err := insertBatch(index, documents); err != nil {
			log.Printf("Lỗi insert batch cuối: %v", err)
		} else {
			totalProcessed += len(documents)
		}
	}

	if err := cursor.Err(); err != nil {
		log.Fatal("Lỗi cursor:", err)
	}

	fmt.Printf("Hoàn thành! Đã seed %d documents vào Meilisearch\n", totalProcessed)

	// Check final count
	fmt.Println("Đang kiểm tra số lượng documents trong Meilisearch...")
	time.Sleep(2 * time.Second) // Wait for indexing
	
	search, err := index.Search("", &meilisearch.SearchRequest{Limit: 1})
	if err != nil {
		log.Printf("Lỗi check count: %v", err)
	} else {
		fmt.Printf("Tổng số documents trong Meilisearch: %d\n", search.EstimatedTotalHits)
	}
}

func insertBatch(index meilisearch.IndexManager, documents []AdminUnitDoc) error {
	// Convert to interface slice for Meilisearch
	docs := make([]interface{}, len(documents))
	for i, doc := range documents {
		docs[i] = doc
	}

	_, err := index.AddDocuments(docs, "admin_id")
	if err != nil {
		return err
	}

	// Return immediately for faster processing
	return nil
}
