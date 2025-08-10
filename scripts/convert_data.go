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

// Original data structures từ storage
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

func main() {
	fmt.Println("🔄 Converting address data to Address Parser format...")

	var adminUnits []models.AdminUnit

	// 1. Load provinces
	provinces, err := loadProvinces()
	if err != nil {
		log.Fatal("Error loading provinces:", err)
	}
	fmt.Printf("✅ Loaded %d provinces\n", len(provinces))

	// 2. Load districts
	districts, err := loadDistricts()
	if err != nil {
		log.Fatal("Error loading districts:", err)
	}
	fmt.Printf("✅ Loaded %d districts\n", len(districts))

	// 3. Load wards
	wards, err := loadWards()
	if err != nil {
		log.Fatal("Error loading wards:", err)
	}
	fmt.Printf("✅ Loaded %d wards\n", len(wards))

	// 4. Convert provinces
	for _, p := range provinces {
		now := time.Now()
		adminUnit := models.AdminUnit{
			ID:              primitive.NewObjectID(),
			AdminID:         fmt.Sprintf("P%02d", p.ID),
			Name:            p.Name,
			NormalizedName:  normalizeText(p.Name),
			Type:            detectProvinceType(p.Name),
			AdminSubtype:    detectProvinceSubtype(p.Name),
			Aliases:         generateAliases(p.Name, p.KeyWord),
			ParentID:        nil, // Province là level cao nhất
			Level:           models.LevelProvince,
			Path:            []string{fmt.Sprintf("P%02d", p.ID)},
			PathNormalized:  []string{normalizeText(p.Name)},
			GazetteerVersion: "1.0.0",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		adminUnits = append(adminUnits, adminUnit)
	}

	// 5. Convert districts
	for _, d := range districts {
		parentID := fmt.Sprintf("P%02d", d.ProvinceID)
		now := time.Now()
		
		// Find parent province for path
		var provinceName string
		for _, p := range provinces {
			if p.ID == d.ProvinceID {
				provinceName = normalizeText(p.Name)
				break
			}
		}
		
		adminUnit := models.AdminUnit{
			ID:              primitive.NewObjectID(),
			AdminID:         fmt.Sprintf("D%04d", d.ID),
			Name:            d.Name,
			NormalizedName:  normalizeText(d.Name),
			Type:            detectDistrictType(d.Name),
			AdminSubtype:    detectDistrictSubtype(d.Name),
			Aliases:         generateAliases(d.Name, d.KeyWord),
			ParentID:        &parentID,
			Level:           models.LevelDistrict,
			Path:            []string{fmt.Sprintf("P%02d", d.ProvinceID), fmt.Sprintf("D%04d", d.ID)},
			PathNormalized:  []string{provinceName, normalizeText(d.Name)},
			GazetteerVersion: "1.0.0",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		adminUnits = append(adminUnits, adminUnit)
	}

	// 6. Convert wards
	for _, w := range wards {
		if w.Status != "1" {
			continue // Skip inactive wards
		}
		
		parentID := fmt.Sprintf("D%04d", w.DistrictID)
		now := time.Now()

		// Find district and province for path
		var provinceName, districtName string
		var provinceID int
		for _, d := range districts {
			if d.ID == w.DistrictID {
				districtName = normalizeText(d.Name)
				provinceID = d.ProvinceID
				break
			}
		}
		
		for _, p := range provinces {
			if p.ID == provinceID {
				provinceName = normalizeText(p.Name)
				break
			}
		}
		
		adminUnit := models.AdminUnit{
			ID:              primitive.NewObjectID(),
			AdminID:         fmt.Sprintf("W%05d", w.ID),
			Name:            w.Name,
			NormalizedName:  normalizeText(w.Name),
			Type:            detectWardType(w.Name),
			AdminSubtype:    detectWardSubtype(w.Name),
			Aliases:         generateAliases(w.Name, w.KeyWord),
			ParentID:        &parentID,
			Level:           models.LevelWard,
			Path:            []string{fmt.Sprintf("P%02d", provinceID), fmt.Sprintf("D%04d", w.DistrictID), fmt.Sprintf("W%05d", w.ID)},
			PathNormalized:  []string{provinceName, districtName, normalizeText(w.Name)},
			GazetteerVersion: "1.0.0",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		adminUnits = append(adminUnits, adminUnit)
	}

	// 7. Save converted data
	fmt.Printf("🔄 Converting %d admin units...\n", len(adminUnits))
	
	output, err := json.MarshalIndent(adminUnits, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling data:", err)
	}

	err = ioutil.WriteFile("../storage/admin_units.json", output, 0644)
	if err != nil {
		log.Fatal("Error writing file:", err)
	}

	fmt.Printf("✅ Conversion complete! Saved %d admin units to storage/admin_units.json\n", len(adminUnits))
	
	// Stats
	provinces_count := 0
	districts_count := 0
	wards_count := 0
	for _, unit := range adminUnits {
		switch unit.Level {
		case 1:
			provinces_count++
		case 2:
			districts_count++
		case 3:
			wards_count++
		}
	}
	
	fmt.Printf("📊 Stats: %d provinces, %d districts, %d wards\n", provinces_count, districts_count, wards_count)
}

func loadProvinces() ([]Province, error) {
	data, err := ioutil.ReadFile("../storage/province.json")
	if err != nil {
		return nil, err
	}

	var provinces []Province
	err = json.Unmarshal(data, &provinces)
	return provinces, err
}

func loadDistricts() ([]District, error) {
	data, err := ioutil.ReadFile("../storage/district.json")
	if err != nil {
		return nil, err
	}

	var districts []District
	err = json.Unmarshal(data, &districts)
	return districts, err
}

func loadWards() ([]Ward, error) {
	data, err := ioutil.ReadFile("../storage/ward.json")
	if err != nil {
		return nil, err
	}

	var wards []Ward
	err = json.Unmarshal(data, &wards)
	return wards, err
}

func normalizeText(text string) string {
	// NFD normalization + remove diacritics + lowercase
	normalized := unidecode.Unidecode(text)
	normalized = strings.ToLower(normalized)
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

func detectProvinceType(name string) string {
	name = strings.ToLower(name)
	if strings.Contains(name, "thành phố") {
		return "municipality"
	}
	return "province"
}

func detectProvinceSubtype(name string) string {
	name = strings.ToLower(name)
	if strings.Contains(name, "thành phố") {
		return models.AdminSubtypeMunicipality
	}
	return models.AdminSubtypeProvince
}

func detectDistrictType(name string) string {
	name = strings.ToLower(name)
	if strings.Contains(name, "quận") {
		return "urban_district"
	} else if strings.Contains(name, "huyện") {
		return "rural_district"
	} else if strings.Contains(name, "thành phố") {
		return "city_under_province"
	} else if strings.Contains(name, "thị xã") {
		return "town"
	}
	return "rural_district" // default
}

func detectDistrictSubtype(name string) string {
	name = strings.ToLower(name)
	if strings.Contains(name, "quận") {
		return models.AdminSubtypeUrbanDistrict
	} else if strings.Contains(name, "huyện") {
		return models.AdminSubtypeRuralDistrict
	} else if strings.Contains(name, "thành phố") {
		return models.AdminSubtypeCityUnderProvince
	} else if strings.Contains(name, "thị xã") {
		return models.AdminSubtypeTown
	}
	return models.AdminSubtypeRuralDistrict // default
}

func detectWardType(name string) string {
	name = strings.ToLower(name)
	if strings.Contains(name, "phường") {
		return "ward"
	} else if strings.Contains(name, "xã") {
		return "commune"
	} else if strings.Contains(name, "thị trấn") {
		return "township"
	}
	return "commune" // default
}

func detectWardSubtype(name string) string {
	name = strings.ToLower(name)
	if strings.Contains(name, "phường") {
		return models.AdminSubtypeWard
	} else if strings.Contains(name, "xã") {
		return models.AdminSubtypeCommune
	} else if strings.Contains(name, "thị trấn") {
		return models.AdminSubtypeTownship
	}
	return models.AdminSubtypeCommune // default
}

func generateAliases(name, keyword string) []string {
	var aliases []string
	
	// Add keyword if different from name
	if keyword != "" && strings.ToLower(keyword) != strings.ToLower(name) {
		aliases = append(aliases, keyword)
	}
	
	// Add common abbreviations
	name_lower := strings.ToLower(name)
	
	// Province abbreviations
	if strings.Contains(name_lower, "thành phố hồ chí minh") {
		aliases = append(aliases, "tp hcm", "hcm", "sài gòn", "tphcm")
	} else if strings.Contains(name_lower, "thành phố hà nội") {
		aliases = append(aliases, "hà nội", "hn", "ha noi")
	} else if strings.Contains(name_lower, "thành phố") {
		// Remove "thành phố" prefix
		short := strings.TrimPrefix(name_lower, "thành phố ")
		aliases = append(aliases, short, "tp "+short)
	}
	
	// District abbreviations
	if strings.HasPrefix(name_lower, "quận ") {
		num := strings.TrimPrefix(name_lower, "quận ")
		aliases = append(aliases, "q "+num, "q."+num)
	} else if strings.HasPrefix(name_lower, "huyện ") {
		short := strings.TrimPrefix(name_lower, "huyện ")
		aliases = append(aliases, "h "+short, "h."+short)
	}
	
	// Ward abbreviations  
	if strings.HasPrefix(name_lower, "phường ") {
		num := strings.TrimPrefix(name_lower, "phường ")
		aliases = append(aliases, "p "+num, "p."+num)
	}
	
	return aliases
}
