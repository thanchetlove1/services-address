package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/address-parser/app/models"
	"github.com/address-parser/app/requests"
)

func main() {
	fmt.Println("🔄 Testing small seed...")

	// Read admin_units.json
	data, err := ioutil.ReadFile("../storage/admin_units.json")
	if err != nil {
		log.Fatal("Error reading admin_units.json:", err)
	}

	var adminUnits []models.AdminUnit
	err = json.Unmarshal(data, &adminUnits)
	if err != nil {
		log.Fatal("Error unmarshaling admin units:", err)
	}

	fmt.Printf("✅ Loaded %d admin units\n", len(adminUnits))

	// Take only first 10 units for testing
	testUnits := adminUnits[:10]
	
	// Wrap in SeedGazetteerRequest
	seedRequest := requests.SeedGazetteerRequest{
		GazetteerVersion: "1.0.0",
		Data:            testUnits,
		RebuildIndexes:  true,
	}

	// Marshal to JSON
	output, err := json.MarshalIndent(seedRequest, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling seed request:", err)
	}

	// Save to file
	err = ioutil.WriteFile("../storage/test_seed_small.json", output, 0644)
	if err != nil {
		log.Fatal("Error writing seed request:", err)
	}

	fmt.Printf("✅ Prepared test seed with %d admin units\n", len(testUnits))
	fmt.Printf("📁 Saved to storage/test_seed_small.json\n")
	
	// Print sample data
	fmt.Println("\n📋 Sample data:")
	for i, unit := range testUnits {
		fmt.Printf("%d. %s (%s) - Level %d\n", i+1, unit.Name, unit.AdminID, unit.Level)
	}
}
