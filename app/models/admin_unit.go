package models

import (
	"time"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdminUnit đại diện cho đơn vị hành chính (quốc gia, tỉnh, quận, phường)
type AdminUnit struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	AdminID            string             `bson:"admin_id" json:"admin_id"`                       // ID theo cấp hành chính
	ParentID           *string            `bson:"parent_id,omitempty" json:"parent_id,omitempty"` // ID của đơn vị cha
	Level              int                `bson:"level" json:"level"`                             // 1=country, 2=province, 3=district, 4=ward
	Name               string             `bson:"name" json:"name"`                               // Tên đơn vị hành chính
	NormalizedName     string             `bson:"normalized_name" json:"normalized_name"`         // Tên đã chuẩn hóa (không dấu, lowercase)
	Type               string             `bson:"type" json:"type"`                               // Loại đơn vị (Quốc gia, Tỉnh, Quận, Phường)
	AdminSubtype       string             `bson:"admin_subtype" json:"admin_subtype"`             // Loại phụ (country, province, municipality, urban_district, rural_district, city_under_province, town, ward, commune, township)
	Aliases            []string           `bson:"aliases,omitempty" json:"aliases,omitempty"`     // Các tên gọi khác
	Path               []string           `bson:"path" json:"path"`                               // Đường dẫn ID từ gốc đến đơn vị hiện tại
	PathNormalized     []string           `bson:"path_normalized" json:"path_normalized"`         // Đường dẫn tên đã chuẩn hóa
	GazetteerVersion   string             `bson:"gazetteer_version" json:"gazetteer_version"`     // Phiên bản gazetteer (SHA256)
	CreatedAt          time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt          time.Time          `bson:"updated_at" json:"updated_at"`
}

// AdminSubtype constants
const (
	AdminSubtypeCountry            = "country"
	AdminSubtypeProvince           = "province"
	AdminSubtypeMunicipality       = "municipality"
	AdminSubtypeUrbanDistrict      = "urban_district"
	AdminSubtypeRuralDistrict      = "rural_district"
	AdminSubtypeCityUnderProvince  = "city_under_province"
	AdminSubtypeTown               = "town"
	AdminSubtypeWard               = "ward"
	AdminSubtypeCommune            = "commune"
	AdminSubtypeTownship           = "township"
)

// Level constants
const (
	LevelCountry  = 1
	LevelProvince = 2
	LevelDistrict = 3
	LevelWard     = 4
)

// IsValidAdminSubtype kiểm tra admin_subtype có hợp lệ không
func (au *AdminUnit) IsValidAdminSubtype() bool {
	validTypes := []string{
		AdminSubtypeCountry,
		AdminSubtypeProvince,
		AdminSubtypeMunicipality,
		AdminSubtypeUrbanDistrict,
		AdminSubtypeRuralDistrict,
		AdminSubtypeCityUnderProvince,
		AdminSubtypeTown,
		AdminSubtypeWard,
		AdminSubtypeCommune,
		AdminSubtypeTownship,
	}
	
	for _, validType := range validTypes {
		if au.AdminSubtype == validType {
			return true
		}
	}
	return false
}

// IsValidLevel kiểm tra level có hợp lệ không
func (au *AdminUnit) IsValidLevel() bool {
	return au.Level >= LevelCountry && au.Level <= LevelWard
}

// GetFullPath trả về đường dẫn đầy đủ từ gốc
func (au *AdminUnit) GetFullPath() string {
	if len(au.PathNormalized) == 0 {
		return au.NormalizedName
	}
	
	result := ""
	for i, name := range au.PathNormalized {
		if i > 0 {
			result += " > "
		}
		result += name
	}
	result += " > " + au.NormalizedName
	return result
}
