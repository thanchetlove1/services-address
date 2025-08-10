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
	fmt.Println("🔄 Preparing seed data...")

	// Read admin_units.json
	data, err := ioutil.ReadFile("storage/admin_units.json")
	if err != nil {
		log.Fatal("Error reading admin_units.json:", err)
	}

	var adminUnits []models.AdminUnit
	err = json.Unmarshal(data, &adminUnits)
	if err != nil {
		log.Fatal("Error unmarshaling admin units:", err)
	}

	fmt.Printf("✅ Loaded %d admin units\n", len(adminUnits))

	// Wrap in SeedGazetteerRequest
	seedRequest := requests.SeedGazetteerRequest{
		GazetteerVersion: "1.0.0",
		Data:            adminUnits,
		RebuildIndexes:  true,
	}

	// Marshal to JSON
	output, err := json.MarshalIndent(seedRequest, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling seed request:", err)
	}

	// Save to file
	err = ioutil.WriteFile("storage/seed_request.json", output, 0644)
	if err != nil {
		log.Fatal("Error writing seed request:", err)
	}

	fmt.Printf("✅ Prepared seed request with %d admin units\n", len(adminUnits))
	fmt.Printf("📁 Saved to storage/seed_request.json\n")
}
