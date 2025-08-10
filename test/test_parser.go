package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/address-parser/app/models"
	"github.com/address-parser/app/requests"
	"github.com/address-parser/app/services"
	"github.com/address-parser/internal/normalizer"
	"github.com/address-parser/internal/parser"
	"github.com/address-parser/internal/search"
	"go.uber.org/zap"
)

func main() {
	// Setup logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Setup normalizer
	norm := normalizer.NewTextNormalizerV2()

	// Setup Meilisearch searcher
	searchConfig := search.SearchConfig{
		Host:          "http://localhost:7700",
		APIKey:        "5pAVWqmP046jvNzQwD70n8b5AdEyhW3lwWUZ1g5CZ8k",
		IndexName:     "admin_units",
		Timeout:       30 * time.Second,
		MaxCandidates: 20,
	}
	searcher, err := search.NewGazetteerSearcher(searchConfig, logger)
	if err != nil {
		log.Fatal("Lỗi tạo searcher:", err)
	}

	// Setup parser (can be nil for now since we're using new logic)
	parser := &parser.AddressParser{}

	// Setup service
	addressService := services.NewAddressService(parser, norm, searcher, logger)

	// Test addresses theo yêu cầu trong PROMPT
	testAddresses := []string{
		"SO 199 HOANG NHU TIEP, PHUONG BO DE, QUAN LONG BIEN, THANH PHO HA NOI",
		"46 NHAN HOA, PHUONG NHAN CHINH, QUAN THANH XUAN, THANH PHO HA NOI",
		"123 Trần Hưng Đạo, Phường Lộc Thọ, Thành phố Nam Định, Tỉnh Nam Định",
		"456/78 Lê Lợi, Phường Đông Khê, Quận Ngô Quyền, Thành phố Hải Phòng",
		"789 Nguyễn Trãi Ward 5 District 5 Ho Chi Minh City Vietnam",
	}

	fmt.Println("=== TESTING ADDRESS PARSER ===")
	fmt.Println()

	for i, addr := range testAddresses {
		fmt.Printf("🔍 TEST %d: %s\n", i+1, addr)
		fmt.Println("────────────────────────────────────────────────────────────")

		// Parse address
		options := requests.ParseOptions{
			UseCache:      false,
			MinConfidence: 0.6,
		}

		result, err := addressService.ParseSingle(addr, options)
		if err != nil {
			fmt.Printf("❌ LỖI: %v\n", err)
			fmt.Println()
			continue
		}

		// Print result
		printResult(result)
		fmt.Println()
	}

	fmt.Println("=== TESTING NORMALIZER DIRECTLY ===")
	fmt.Println()

	// Test normalizer directly
	for i, addr := range testAddresses[:2] { // Test 2 addresses only
		fmt.Printf("🔧 NORMALIZER TEST %d: %s\n", i+1, addr)
		fmt.Println("────────────────────────────────────────────────────────────")

		normResult := norm.NormalizeAddress(addr)
		if normResult != nil {
			fmt.Printf("✅ Normalized: %s\n", normResult.NormalizedNoDiacritics)
			fmt.Printf("📍 Fingerprint: %s\n", normResult.Fingerprint)
		} else {
			fmt.Printf("❌ Normalizer trả về nil\n")
		}
		fmt.Println()
	}

	fmt.Println("=== TESTING MEILISEARCH DIRECTLY ===")
	fmt.Println()

	// Test Meilisearch search directly
	testQueries := []string{
		"ha noi",
		"long bien",
		"bo de",
		"thanh xuan",
		"nam dinh",
	}

	for i, query := range testQueries {
		fmt.Printf("🔎 SEARCH TEST %d: %s\n", i+1, query)
		fmt.Println("────────────────────────────────────────────────────────────")

		// Search by different levels
		for level := 2; level <= 4; level++ {
			results, err := searcher.SearchByLevel(context.Background(), query, level, "", 3)
			if err != nil {
				fmt.Printf("❌ Lỗi search level %d: %v\n", level, err)
				continue
			}

			fmt.Printf("🎯 Level %d results (%d found):\n", level, len(results))
			for _, result := range results {
				fmt.Printf("  - %s (ID: %s, Type: %s)\n", result.Name, result.AdminID, result.AdminSubtype)
			}
		}
		fmt.Println()
	}
}

func printResult(result *models.AddressResult) {
	fmt.Printf("📊 STATUS: %s (%.2f confidence)\n", result.Status, result.Confidence)
	fmt.Printf("📝 CANONICAL: %s\n", result.CanonicalText)
	fmt.Printf("🔗 NORMALIZED: %s\n", result.NormalizedNoDiacritics)

	if len(result.AdminPath) > 0 {
		fmt.Printf("🗺️  ADMIN PATH: %v\n", result.AdminPath)
	}

	// Components
	if result.Components.Province != nil {
		fmt.Printf("🏛️  PROVINCE: %s (ID: %s)\n", result.Components.Province.Name, result.Components.Province.AdminID)
	}
	if result.Components.District != nil {
		fmt.Printf("🏙️  DISTRICT: %s (ID: %s)\n", result.Components.District.Name, result.Components.District.AdminID)
	}
	if result.Components.Ward != nil {
		fmt.Printf("🏘️  WARD: %s (ID: %s)\n", result.Components.Ward.Name, result.Components.Ward.AdminID)
	}

	if result.Components.House != nil && result.Components.House.Number != nil {
		fmt.Printf("🏠 HOUSE: %s\n", *result.Components.House.Number)
	}

	if result.Components.Street != nil && result.Components.Street.Name != "" {
		fmt.Printf("🛣️  STREET: %s\n", result.Components.Street.Name)
	}

	if result.RawFingerprint != "" {
		fmt.Printf("🔒 FINGERPRINT: %s\n", result.RawFingerprint)
	}

	if len(result.Candidates) > 0 {
		fmt.Printf("👥 CANDIDATES: %d\n", len(result.Candidates))
	}
}
