package models

import (
	// time package removed as it's not used
)

// AddressResult kết quả parse địa chỉ
type AddressResult struct {
	Raw                        string                 `json:"raw"`                        // Địa chỉ gốc
	CanonicalText              string                 `json:"canonical_text"`              // Văn bản chuẩn có dấu
	NormalizedNoDiacritics     string                 `json:"normalized_no_diacritics"`     // Văn bản chuẩn không dấu
	Components                  AddressComponents      `json:"components"`                  // Các thành phần địa chỉ
	Quality                    QualityInfo            `json:"quality"`                     // Thông tin chất lượng
	Residual                   string                 `json:"residual"`                    // Phần còn lại không map được
	RawFingerprint             string                 `json:"raw_fingerprint"`             // Fingerprint của địa chỉ
	Confidence                 float64                `json:"confidence"`                  // Độ tin cậy
	MatchStrategy              string                 `json:"match_strategy"`              // Chiến lược matching
	AdminPath                  []string               `json:"admin_path"`                  // Đường dẫn hành chính
	Candidates                 []Candidate            `json:"candidates"`                  // Danh sách ứng viên
	Status                     string                 `json:"status"`                      // Trạng thái xử lý
}

// AddressComponents các thành phần của địa chỉ
type AddressComponents struct {
	House      *HouseInfo      `json:"house,omitempty"`      // Thông tin nhà
	Street     *StreetInfo     `json:"street,omitempty"`     // Thông tin đường
	Locality   *LocalityInfo   `json:"locality,omitempty"`   // Thông tin khu vực
	RoadCode   *RoadCodeInfo   `json:"road_code,omitempty"`  // Mã đường
	Ward       *AdminUnit      `json:"ward,omitempty"`       // Phường/xã
	District   *AdminUnit      `json:"district,omitempty"`   // Quận/huyện
	City       *AdminUnit      `json:"city,omitempty"`       // Thành phố
	Province   *AdminUnit      `json:"province,omitempty"`   // Tỉnh
	Country    *AdminUnit      `json:"country,omitempty"`    // Quốc gia
	POI        *POIInfo        `json:"poi,omitempty"`        // Điểm quan tâm
}

// HouseInfo thông tin nhà
type HouseInfo struct {
	Number *string    `json:"number,omitempty"` // Số nhà
	Alley  AlleyInfo  `json:"alley,omitempty"`  // Thông tin hẻm
	Unit   string     `json:"unit,omitempty"`   // Đơn vị (căn hộ, phòng)
	Floor  *string    `json:"floor,omitempty"`  // Tầng
}

// AlleyInfo thông tin hẻm
type AlleyInfo struct {
	Number *string `json:"number,omitempty"` // Số hẻm
	Name   *string `json:"name,omitempty"`   // Tên hẻm
}

// StreetInfo thông tin đường
type StreetInfo struct {
	Name   string `json:"name"`   // Tên đường
	Type   string `json:"type"`   // Loại đường
}

// LocalityInfo thông tin khu vực
type LocalityInfo struct {
	Type   *string `json:"type,omitempty"`   // Loại khu vực
	Number *string `json:"number,omitempty"` // Số khu vực
}

// RoadCodeInfo thông tin mã đường
type RoadCodeInfo struct {
	Type string `json:"type"` // Loại đường (ql, dt, tl, hl, dh)
	Code string `json:"code"` // Mã đường
}

// POIInfo điểm quan tâm
type POIInfo struct {
	Building *string `json:"building,omitempty"` // Tòa nhà
	Complex  *string `json:"complex,omitempty"`  // Khu phức hợp
	Company  *string `json:"company,omitempty"`  // Công ty
	Khu      *string `json:"khu,omitempty"`      // Khu
	KCN      *string `json:"kcn,omitempty"`      // Khu công nghiệp
}

// QualityInfo thông tin chất lượng
type QualityInfo struct {
	Score      float64  `json:"score"`       // Điểm chất lượng
	MatchLevel string   `json:"match_level"` // Mức độ matching
	Flags      []string `json:"flags"`       // Các cờ chất lượng
}

// QualityMetrics alias cho QualityInfo để tương thích với AddressMatcher
type QualityMetrics = QualityInfo

// RoadCode alias cho RoadCodeInfo để tương thích với AddressMatcher
type RoadCode = RoadCodeInfo

// Candidate ứng viên matching
type Candidate struct {
	Path       string      `json:"path"`        // Đường dẫn hành chính
	Score      float64     `json:"score"`       // Điểm số
	AdminUnits []AdminUnit `json:"admin_units"` // Các đơn vị hành chính
}

// Status constants
const (
	StatusMatched      = "matched"
	StatusAmbiguous    = "ambiguous"
	StatusNeedsReview  = "needs_review"
	StatusUnmatched    = "unmatched"
)

// MatchStrategy constants
const (
	MatchStrategyExact      = "exact"
	MatchStrategyAsciiExact = "ascii_exact"
	MatchStrategyFuzzy      = "fuzzy"
	MatchStrategyAlias      = "alias"
)

// MatchLevel constants
const (
	MatchLevelExact      = "exact"
	MatchLevelAsciiExact = "ascii_exact"
	MatchLevelFuzzy      = "fuzzy"
)

// Quality flags
const (
	FlagExactMatch        = "EXACT_MATCH"
	FlagAsciiExact        = "ASCII_EXACT"
	FlagFuzzyMatch        = "FUZZY_MATCH"
	FlagPOIExtracted      = "POI_EXTRACTED"
	FlagApartmentUnit     = "APARTMENT_UNIT"
	FlagMultiLanguage     = "MULTI_LANGUAGE"
	FlagAmbiguousWard     = "AMBIGUOUS_WARD"
	FlagMultipleCandidates = "MULTIPLE_CANDIDATES"
	FlagLowConfidence     = "LOW_CONFIDENCE"
	FlagMissingWard       = "MISSING_WARD"
)

// IsValidStatus kiểm tra status có hợp lệ không
func (ar *AddressResult) IsValidStatus() bool {
	validStatuses := []string{
		StatusMatched,
		StatusAmbiguous,
		StatusNeedsReview,
		StatusUnmatched,
	}
	
	for _, validStatus := range validStatuses {
		if ar.Status == validStatus {
			return true
		}
	}
	return false
}

// IsValidMatchStrategy kiểm tra match_strategy có hợp lệ không
func (ar *AddressResult) IsValidMatchStrategy() bool {
	validStrategies := []string{
		MatchStrategyExact,
		MatchStrategyAsciiExact,
		MatchStrategyFuzzy,
		MatchStrategyAlias,
	}
	
	for _, validStrategy := range validStrategies {
		if ar.MatchStrategy == validStrategy {
			return true
		}
	}
	return false
}

// IsValidMatchLevel kiểm tra match_level có hợp lệ không
func (ar *AddressResult) IsValidMatchLevel() bool {
	validLevels := []string{
		MatchLevelExact,
		MatchLevelAsciiExact,
		MatchLevelFuzzy,
	}
	
	for _, validLevel := range validLevels {
		if ar.Quality.MatchLevel == validLevel {
			return true
		}
	}
	return false
}
