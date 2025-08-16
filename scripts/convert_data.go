package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/address-parser/app/models"
	"github.com/mozillazg/go-unidecode"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Original data structures từ storage/address.json
type Province struct {
	ID        int    `json:"id"`
	CountryID int    `json:"country_id"`
	KeyWord   string `json:"key_word"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	ZipCode   string `json:"zipcode"`
	ZoneCode  string `json:"zonecode"`
	PostCode  string `json:"postcode"`
}

type District struct {
	ID         int         `json:"id"`
	ProvinceID int         `json:"province_id"`
	KeyWord    string      `json:"key_word"`
	PostCode   interface{} `json:"postcode"`
	Code       interface{} `json:"code"`
	Name       string      `json:"name"`
	Note       interface{} `json:"note"`
	InCity     interface{} `json:"in_city"`
}

type Ward struct {
	ID         int         `json:"id"`
	DistrictID int         `json:"district_id"`
	KeyWord    string      `json:"key_word"`
	Code       interface{} `json:"code"`
	PostCode   interface{} `json:"postcode"`
	Name       string      `json:"name"`
	Note       interface{} `json:"note"`
	Status     string      `json:"status"`
}

// New AdminUnit structure từ storage/admin_units.json
type NewAdminUnit struct {
	ID             string   `json:"id"`
	AdminID        string   `json:"admin_id"`
	ParentID       *string  `json:"parent_id,omitempty"`
	Level          int      `json:"level"`
	Name           string   `json:"name"`
	NormalizedName string   `json:"normalized_name"`
	Type           string   `json:"type"`
	AdminSubtype   string   `json:"admin_subtype"`
	Aliases        []string `json:"aliases"`
	Path           []string `json:"path"`
	PathNormalized []string `json:"path_normalized"`
	GazetteerVersion string `json:"gazetteer_version"`
}

func main() {
	fmt.Println("🔄 Converting address data to new Address Parser format...")
	fmt.Println("==========================================================")

	// 1. Load existing new format data để tham khảo
	newAdminUnits, err := loadNewAdminUnits()
	if err != nil {
		log.Fatal("Error loading new admin units:", err)
	}
	fmt.Printf("✅ Loaded %d new admin units for reference\n", len(newAdminUnits))

	// 2. Load old format data
	provinces, err := loadProvinces()
	if err != nil {
		log.Fatal("Error loading provinces:", err)
	}
	fmt.Printf("✅ Loaded %d provinces\n", len(provinces))

	districts, err := loadDistricts()
	if err != nil {
		log.Fatal("Error loading districts:", err)
	}
	fmt.Printf("✅ Loaded %d districts\n", len(districts))

	wards, err := loadWards()
	if err != nil {
		log.Fatal("Error loading wards:", err)
	}
	fmt.Printf("✅ Loaded %d wards\n", len(wards))

	// 3. Convert và tạo new admin units
	var convertedAdminUnits []models.AdminUnit
	now := time.Now()

	// Convert provinces (level 2)
	for _, p := range provinces {
		adminUnit := models.AdminUnit{
			ID:               primitive.NewObjectID(),
			AdminID:          fmt.Sprintf("P%02d", p.ID),
			Name:             p.Name,
			NormalizedName:   normalizeText(p.Name),
			Type:             detectProvinceType(p.Name),
			AdminSubtype:     detectProvinceSubtype(p.Name),
			Aliases:          generateAliases(p.Name, p.KeyWord),
			ParentID:         nil, // Province là level cao nhất
			Level:            models.LevelProvince,
			Path:             []string{fmt.Sprintf("P%02d", p.ID)},
			PathNormalized:   []string{normalizeText(p.Name)},
			GazetteerVersion: "1.0.0",
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		convertedAdminUnits = append(convertedAdminUnits, adminUnit)
	}

	// Convert districts (level 3)
	for _, d := range districts {
		parentID := fmt.Sprintf("P%02d", d.ProvinceID)
		adminUnit := models.AdminUnit{
			ID:               primitive.NewObjectID(),
			AdminID:          fmt.Sprintf("D%02d", d.ID),
			Name:             d.Name,
			NormalizedName:   normalizeText(d.Name),
			Type:             detectDistrictType(d.Name),
			AdminSubtype:     detectDistrictSubtype(d.Name),
			Aliases:          generateAliases(d.Name, d.KeyWord),
			ParentID:         &parentID,
			Level:            models.LevelDistrict,
			Path:             []string{parentID, fmt.Sprintf("D%02d", d.ID)},
			PathNormalized:   []string{normalizeText(getProvinceName(provinces, d.ProvinceID)), normalizeText(d.Name)},
			GazetteerVersion: "1.0.0",
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		convertedAdminUnits = append(convertedAdminUnits, adminUnit)
	}

	// Convert wards (level 4)
	for _, w := range wards {
		districtID := fmt.Sprintf("D%02d", w.DistrictID)
		district := getDistrict(districts, w.DistrictID)
		if district == nil {
			continue
		}
		
		parentID := fmt.Sprintf("P%02d", district.ProvinceID)
		adminUnit := models.AdminUnit{
			ID:               primitive.NewObjectID(),
			AdminID:          fmt.Sprintf("W%02d", w.ID),
			Name:             w.Name,
			NormalizedName:   normalizeText(w.Name),
			Type:             detectWardType(w.Name),
			AdminSubtype:     detectWardSubtype(w.Name),
			Aliases:          generateAliases(w.Name, w.KeyWord),
			ParentID:         &districtID,
			Level:            models.LevelWard,
			Path:             []string{parentID, districtID, fmt.Sprintf("W%02d", w.ID)},
			PathNormalized:   []string{normalizeText(getProvinceName(provinces, district.ProvinceID)), normalizeText(district.Name), normalizeText(w.Name)},
			GazetteerVersion: "1.0.0",
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		convertedAdminUnits = append(convertedAdminUnits, adminUnit)
	}

	// 4. Save converted data
	err = saveConvertedData(convertedAdminUnits)
	if err != nil {
		log.Fatal("Error saving converted data:", err)
	}

	fmt.Printf("🎉 Successfully converted %d admin units!\n", len(convertedAdminUnits))
	fmt.Println("📁 Output saved to: converted_admin_units.json")
	
	// 5. Print summary
	printSummary(convertedAdminUnits)
}

// Load new format admin units để tham khảo
func loadNewAdminUnits() ([]NewAdminUnit, error) {
	data, err := ioutil.ReadFile("storage/admin_units.json")
	if err != nil {
		return nil, fmt.Errorf("error reading admin_units.json: %w", err)
	}

	var adminUnits []NewAdminUnit
	err = json.Unmarshal(data, &adminUnits)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling admin_units.json: %w", err)
	}

	return adminUnits, nil
}

// Load provinces từ storage/address.json
func loadProvinces() ([]Province, error) {
	data, err := ioutil.ReadFile("storage/address.json")
	if err != nil {
		return nil, fmt.Errorf("error reading address.json: %w", err)
	}

	var rawData []map[string]interface{}
	err = json.Unmarshal(data, &rawData)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling address.json: %w", err)
	}

	var provinces []Province
	for _, item := range rawData {
		if unitLevel, ok := item["unit_level"].(float64); ok && unitLevel == 1 {
			province := Province{}
			if id, ok := item["id"].(float64); ok {
				province.ID = int(id)
			}
			if name, ok := item["name"].(string); ok {
				province.Name = name
			}
			if keyWord, ok := item["key_word"].(string); ok {
				province.KeyWord = keyWord
			}
			if code, ok := item["code"].(string); ok {
				province.Code = code
			}
			provinces = append(provinces, province)
		}
	}

	return provinces, nil
}

// Load districts từ storage/address.json
func loadDistricts() ([]District, error) {
	data, err := ioutil.ReadFile("storage/address.json")
	if err != nil {
		return nil, fmt.Errorf("error reading address.json: %w", err)
	}

	var rawData []map[string]interface{}
	err = json.Unmarshal(data, &rawData)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling address.json: %w", err)
	}

	var districts []District
	for _, item := range rawData {
		if unitLevel, ok := item["unit_level"].(float64); ok && unitLevel == 2 {
			district := District{}
			if id, ok := item["id"].(float64); ok {
				district.ID = int(id)
			}
			if name, ok := item["name"].(string); ok {
				district.Name = name
			}
			if keyWord, ok := item["key_word"].(string); ok {
				district.KeyWord = keyWord
			}
			if provinceID, ok := item["parent_id"].(float64); ok {
				district.ProvinceID = int(provinceID)
			}
			districts = append(districts, district)
		}
	}

	return districts, nil
}

// Load wards từ storage/address.json
func loadWards() ([]Ward, error) {
	data, err := ioutil.ReadFile("storage/address.json")
	if err != nil {
		return nil, fmt.Errorf("error reading address.json: %w", err)
	}

	var rawData []map[string]interface{}
	err = json.Unmarshal(data, &rawData)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling address.json: %w", err)
	}

	var wards []Ward
	for _, item := range rawData {
		if unitLevel, ok := item["unit_level"].(float64); ok && unitLevel == 3 {
			ward := Ward{}
			if id, ok := item["id"].(float64); ok {
				ward.ID = int(id)
			}
			if name, ok := item["name"].(string); ok {
				ward.Name = name
			}
			if keyWord, ok := item["key_word"].(string); ok {
				ward.KeyWord = keyWord
			}
			if districtID, ok := item["parent_id"].(float64); ok {
				ward.DistrictID = int(districtID)
			}
			wards = append(wards, ward)
		}
	}

	return wards, nil
}

// Helper functions
func getProvinceName(provinces []Province, id int) string {
	for _, p := range provinces {
		if p.ID == id {
			return p.Name
		}
	}
	return "Unknown"
}

func getDistrict(districts []District, id int) *District {
	for _, d := range districts {
		if d.ID == id {
			return &d
		}
	}
	return nil
}

// Normalize text (remove accents, lowercase, replace spaces with underscores)
func normalizeText(text string) string {
	// Remove accents
	normalized := unidecode.Unidecode(text)
	// Convert to lowercase
	normalized = strings.ToLower(normalized)
	// Replace spaces with underscores
	normalized = strings.ReplaceAll(normalized, " ", "_")
	// Remove special characters
	normalized = strings.ReplaceAll(normalized, "(", "")
	normalized = strings.ReplaceAll(normalized, ")", "")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	return normalized
}

// Detect province type
func detectProvinceType(name string) string {
	if strings.Contains(strings.ToLower(name), "thành phố") {
		return "Thành phố"
	}
	return "Tỉnh"
}

// Detect province subtype
func detectProvinceSubtype(name string) string {
	if strings.Contains(strings.ToLower(name), "thành phố") {
		return models.AdminSubtypeMunicipality
	}
	return models.AdminSubtypeProvince
}

// Detect district type
func detectDistrictType(name string) string {
	if strings.Contains(strings.ToLower(name), "thành phố") {
		return "Thành phố thuộc tỉnh"
	} else if strings.Contains(strings.ToLower(name), "thị xã") {
		return "Thị xã"
	}
	return "Huyện"
}

// Detect district subtype
func detectDistrictSubtype(name string) string {
	if strings.Contains(strings.ToLower(name), "thành phố") {
		return models.AdminSubtypeCityUnderProvince
	} else if strings.Contains(strings.ToLower(name), "thị xã") {
		return models.AdminSubtypeTown
	}
	return models.AdminSubtypeRuralDistrict
}

// Detect ward type
func detectWardType(name string) string {
	if strings.Contains(strings.ToLower(name), "phường") {
		return "Phường"
	}
	return "Xã"
}

// Detect ward subtype
func detectWardSubtype(name string) string {
	if strings.Contains(strings.ToLower(name), "phường") {
		return models.AdminSubtypeWard
	}
	return models.AdminSubtypeCommune
}

// Generate aliases
func generateAliases(name, keyWord string) []string {
	var aliases []string
	
	// Add keyword if exists
	if keyWord != "" {
		aliases = append(aliases, keyWord)
	}
	
	// Add normalized name without spaces
	normalized := strings.ReplaceAll(normalizeText(name), "_", "")
	aliases = append(aliases, normalized)
	
	// Add common abbreviations
	if strings.Contains(strings.ToLower(name), "thành phố") {
		aliases = append(aliases, "tp")
	}
	if strings.Contains(strings.ToLower(name), "quận") {
		aliases = append(aliases, "q")
	}
	if strings.Contains(strings.ToLower(name), "phường") {
		aliases = append(aliases, "p")
	}
	
	return aliases
}

// Save converted data
func saveConvertedData(adminUnits []models.AdminUnit) error {
	data, err := json.MarshalIndent(adminUnits, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling data: %w", err)
	}

	err = ioutil.WriteFile("converted_admin_units.json", data, 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

// Print summary
func printSummary(adminUnits []models.AdminUnit) {
	fmt.Println("\n📊 CONVERSION SUMMARY:")
	fmt.Println("========================")
	
	levelCounts := make(map[int]int)
	subtypeCounts := make(map[string]int)
	
	for _, unit := range adminUnits {
		levelCounts[unit.Level]++
		subtypeCounts[unit.AdminSubtype]++
	}
	
	fmt.Printf("🏛️  Level 1 (Country): %d\n", levelCounts[1])
	fmt.Printf("🏙️  Level 2 (Province): %d\n", levelCounts[2])
	fmt.Printf("🏘️  Level 3 (District): %d\n", levelCounts[3])
	fmt.Printf("🏠 Level 4 (Ward): %d\n", levelCounts[4])
	
	fmt.Println("\n📋 Admin Subtypes:")
	for subtype, count := range subtypeCounts {
		fmt.Printf("   %s: %d\n", subtype, count)
	}
}
