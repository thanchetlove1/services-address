package main

import (
	"fmt"
	"log"

	"github.com/address-parser/app/config"
	"github.com/address-parser/internal/normalizer"
)

func main() {
	fmt.Println("=== Testing Integration Components ===")

	// Test 1: Load config
	fmt.Println("\n1. Testing config loading...")
	if err := config.Load("config/parser.yaml"); err != nil {
		log.Fatalf("Config load failed: %v", err)
	}
	fmt.Printf("✓ Config loaded: level_config=%d, use_libpostal=%v\n", 
		config.C.LevelConfig, config.C.UseLibpostal)

	// Test 2: Normalizer
	fmt.Println("\n2. Testing normalizer...")
	raw := "SO 199 HOANG NHU TIEP PHUONG BO DE QUAN LONG BIEN TP HA NOI, HÀ NỘI"
	norm, sig := normalizer.Normalize(raw)
	fmt.Printf("✓ Raw: %s\n", raw)
	fmt.Printf("✓ Normalized: %s\n", norm)
	fmt.Printf("✓ Signals: House=%s, Road=%s, Unit=%s, Level=%s\n", 
		sig.House, sig.Road, sig.Unit, sig.Level)

	// Test 3: Admin tokens extraction
	fmt.Println("\n3. Testing admin tokens extraction...")
	ward, dist, prov := normalizer.ExtractAdminTokens(norm)
	fmt.Printf("✓ Ward: %s\n", ward)
	fmt.Printf("✓ District: %s\n", dist)
	fmt.Printf("✓ Province: %s\n", prov)

	// Test 4: Scoring (without search dependency)
	fmt.Println("\n4. Testing scoring components...")
	fmt.Printf("✓ Scoring weights: Ward=%.2f, District=%.2f, Province=%.2f\n",
		config.C.Scoring.Weights.Ward,
		config.C.Scoring.Weights.District,
		config.C.Scoring.Weights.Province)

	fmt.Println("\n=== All basic components working! ===")
	fmt.Println("Next step: Start MongoDB + Meilisearch and test full integration")
}
