package normalizer

import (
	"regexp"
	"strings"
)

// PatternExtractor service trích xuất patterns từ địa chỉ
type PatternExtractor struct {
	// Precompiled regex patterns theo thứ tự ưu tiên
	highPriorityPatterns map[string]*regexp.Regexp
	normalPriorityPatterns map[string]*regexp.Regexp
}

// PatternResult kết quả trích xuất pattern
type PatternResult struct {
	Type       string            `json:"type"`        // HOUSE_NO, ROAD_CODE, ALLEY, etc.
	Value      string            `json:"value"`       // Giá trị được trích xuất
	Tokens     []string          `json:"tokens"`      // Các token gốc
	Position   int               `json:"position"`    // Vị trí trong chuỗi
	Confidence float64           `json:"confidence"`  // Độ tin cậy (0-1)
	Metadata   map[string]string `json:"metadata"`    // Metadata bổ sung
}

// NewPatternExtractor tạo mới PatternExtractor
func NewPatternExtractor() *PatternExtractor {
	pe := &PatternExtractor{
		highPriorityPatterns: make(map[string]*regexp.Regexp),
		normalPriorityPatterns: make(map[string]*regexp.Regexp),
	}
	
	pe.initializePatterns()
	return pe
}

// initializePatterns khởi tạo các regex patterns theo prompt
func (pe *PatternExtractor) initializePatterns() {
	// High priority patterns (chạy trước)
	pe.highPriorityPatterns = map[string]*regexp.Regexp{
		"APARTMENT":    regexp.MustCompile(`(?i)\b(?:ch|can\s*ho)\s*([0-9]+(?:-[0-9]+)?)\b`),
		"FLOOR":        regexp.MustCompile(`(?i)\b(?:tang|lau)\s*([0-9]+)\b`),
		"OFFICE":       regexp.MustCompile(`(?i)\b(?:vh|van\s*phong)\s*([0-9]+)\b`),
		"BLOCK_TOWER":  regexp.MustCompile(`(?i)\b(?:block|toa)\s*([a-z0-9]+)\b`),
		"UNIT":         regexp.MustCompile(`(?i)\b(?:unit|apt|room|p)\s*([a-z0-9]+(?:\.[a-z0-9]+)*)\b`),
	}
	
	// Normal priority patterns
	pe.normalPriorityPatterns = map[string]*regexp.Regexp{
		"HOUSE_NO":     regexp.MustCompile(`(?i)\b(?:so\s*)?([0-9]+(?:[/-][0-9]+)*|nv[0-9]+(?:-[0-9]+)?|lo\s*[a-z0-9]+(?:-[a-z0-9]+)?|ch\s*[0-9]+(?:-[0-9]+)?)\b`),
		"ROAD_CODE":    regexp.MustCompile(`(?i)\b(?:(ql|dt|tl|hl|dh))\s*([0-9]+[a-z]?)\b`),
		"ALLEY":        regexp.MustCompile(`(?i)\b(?:hem|hẻm|ngo|ngõ|ngach|ngách|kiet|kiệt)\.?\s*([0-9]+(?:[/-][0-9]+)*)\b`),
		"ALLEY_NAME":   regexp.MustCompile(`(?i)\b(?:hem|ngo|ngach|kiet)\s+([a-z0-9]+(?:\s+[a-z0-9]+)*)\b`),
		"Q_NUM":        regexp.MustCompile(`(?i)\b(?:q|quan)\.?\s*(\d+)\b`),
		"P_NUM":        regexp.MustCompile(`(?i)\b(?:p|phuong)\.?\s*(\d+)\b`),
		"KP_NUM":       regexp.MustCompile(`(?i)\b(?:kp|khu\s*pho)\.?\s*(\d+)\b`),
		"TO_NUM":       regexp.MustCompile(`(?i)\b(?:to)\.?\s*(\d+)\b`),
		"AP_NUM":       regexp.MustCompile(`(?i)\b(?:ap)\.?\s*(\d+)\b`),
		"THON_NUM":     regexp.MustCompile(`(?i)\b(?:thon)\.?\s*(\d+)\b`),
		"XOM_NUM":      regexp.MustCompile(`(?i)\b(?:xom)\.?\s*(\d+)\b`),
	}
}

// ExtractPatterns trích xuất tất cả patterns từ văn bản
func (pe *PatternExtractor) ExtractPatterns(text string) []PatternResult {
	var results []PatternResult
	
	// Chạy high priority patterns trước
	for patternType, regex := range pe.highPriorityPatterns {
		matches := pe.findMatches(text, regex, patternType, 0.9)
		results = append(results, matches...)
	}
	
	// Chạy normal priority patterns
	for patternType, regex := range pe.normalPriorityPatterns {
		matches := pe.findMatches(text, regex, patternType, 0.8)
		results = append(results, matches...)
	}
	
	return results
}

// findMatches tìm tất cả matches cho một pattern
func (pe *PatternExtractor) findMatches(text string, regex *regexp.Regexp, patternType string, confidence float64) []PatternResult {
	var results []PatternResult
	
	matches := regex.FindAllStringSubmatch(text, -1)
	indices := regex.FindAllStringIndex(text, -1)
	
	for i, match := range matches {
		if len(match) < 2 {
			continue
		}
		
		result := PatternResult{
			Type:       patternType,
			Value:      match[1], // Captured group
			Tokens:     strings.Split(match[0], " "),
			Position:   indices[i][0],
			Confidence: confidence,
			Metadata:   make(map[string]string),
		}
		
		// Metadata cụ thể theo pattern
		pe.addPatternMetadata(&result, match)
		
		results = append(results, result)
	}
	
	return results
}

// addPatternMetadata thêm metadata cụ thể theo pattern type
func (pe *PatternExtractor) addPatternMetadata(result *PatternResult, match []string) {
	switch result.Type {
	case "ROAD_CODE":
		if len(match) >= 3 {
			result.Metadata["road_type"] = strings.ToLower(match[1]) // ql, dt, tl, etc.
			result.Metadata["road_number"] = strings.ToLower(match[2])
		}
	case "APARTMENT":
		result.Metadata["building_type"] = "apartment"
	case "FLOOR":
		result.Metadata["building_component"] = "floor"
	case "OFFICE":
		result.Metadata["building_type"] = "office"
	case "BLOCK_TOWER":
		result.Metadata["building_component"] = "block"
	case "UNIT":
		result.Metadata["building_component"] = "unit"
	case "HOUSE_NO":
		if strings.Contains(result.Value, "/") {
			result.Metadata["house_type"] = "complex"
		} else if strings.HasPrefix(strings.ToLower(result.Value), "nv") {
			result.Metadata["house_type"] = "special_code"
		} else {
			result.Metadata["house_type"] = "simple"
		}
	case "ALLEY":
		result.Metadata["street_component"] = "alley_number"
	case "ALLEY_NAME":
		result.Metadata["street_component"] = "alley_name"
	case "Q_NUM":
		result.Metadata["admin_level"] = "district"
		result.Metadata["admin_type"] = "urban_district"
	case "P_NUM":
		result.Metadata["admin_level"] = "ward"
		result.Metadata["admin_type"] = "ward"
	case "KP_NUM", "TO_NUM", "AP_NUM", "THON_NUM", "XOM_NUM":
		result.Metadata["admin_level"] = "locality"
		result.Metadata["locality_type"] = strings.ToLower(result.Type[:len(result.Type)-4]) // Remove _NUM
	}
}

// ExtractHouseNumber trích xuất số nhà cụ thể
func (pe *PatternExtractor) ExtractHouseNumber(text string) *PatternResult {
	patterns := pe.ExtractPatterns(text)
	
	for _, pattern := range patterns {
		if pattern.Type == "HOUSE_NO" {
			return &pattern
		}
	}
	
	return nil
}

// ExtractRoadCode trích xuất mã đường (QL, DT, TL, etc.)
func (pe *PatternExtractor) ExtractRoadCode(text string) *PatternResult {
	patterns := pe.ExtractPatterns(text)
	
	for _, pattern := range patterns {
		if pattern.Type == "ROAD_CODE" {
			return &pattern
		}
	}
	
	return nil
}

// ExtractAlley trích xuất thông tin hẻm/ngõ
func (pe *PatternExtractor) ExtractAlley(text string) *PatternResult {
	patterns := pe.ExtractPatterns(text)
	
	// Ưu tiên ALLEY_NAME trước ALLEY number
	for _, pattern := range patterns {
		if pattern.Type == "ALLEY_NAME" {
			return &pattern
		}
	}
	
	for _, pattern := range patterns {
		if pattern.Type == "ALLEY" {
			return &pattern
		}
	}
	
	return nil
}

// ExtractAdminNumbers trích xuất các số hành chính (quận/phường)
func (pe *PatternExtractor) ExtractAdminNumbers(text string) []PatternResult {
	var results []PatternResult
	patterns := pe.ExtractPatterns(text)
	
	for _, pattern := range patterns {
		if pattern.Type == "Q_NUM" || pattern.Type == "P_NUM" {
			results = append(results, pattern)
		}
	}
	
	return results
}

// ExtractLocalityNumbers trích xuất các số khu vực (khu phố, tổ, ấp, etc.)
func (pe *PatternExtractor) ExtractLocalityNumbers(text string) []PatternResult {
	var results []PatternResult
	patterns := pe.ExtractPatterns(text)
	
	localityTypes := []string{"KP_NUM", "TO_NUM", "AP_NUM", "THON_NUM", "XOM_NUM"}
	
	for _, pattern := range patterns {
		for _, lType := range localityTypes {
			if pattern.Type == lType {
				results = append(results, pattern)
				break
			}
		}
	}
	
	return results
}

// ExtractBuildingInfo trích xuất thông tin tòa nhà (tầng, block, căn hộ, etc.)
func (pe *PatternExtractor) ExtractBuildingInfo(text string) []PatternResult {
	var results []PatternResult
	patterns := pe.ExtractPatterns(text)
	
	buildingTypes := []string{"APARTMENT", "FLOOR", "OFFICE", "BLOCK_TOWER", "UNIT"}
	
	for _, pattern := range patterns {
		for _, bType := range buildingTypes {
			if pattern.Type == bType {
				results = append(results, pattern)
				break
			}
		}
	}
	
	return results
}

// RemoveExtractedPatterns loại bỏ các patterns đã trích xuất khỏi text để tránh double processing
func (pe *PatternExtractor) RemoveExtractedPatterns(text string, patterns []PatternResult) string {
	result := text
	
	// Sắp xếp patterns theo position giảm dần để xóa từ cuối lên đầu
	for i := len(patterns) - 1; i >= 0; i-- {
		// Tìm và replace pattern trong text
		for _, regex := range pe.highPriorityPatterns {
			result = regex.ReplaceAllStringFunc(result, func(match string) string {
				// Nếu match chứa pattern đã extract, thay bằng placeholder hoặc xóa
				return " " // Thay bằng space để giữ structure
			})
		}
		for _, regex := range pe.normalPriorityPatterns {
			result = regex.ReplaceAllStringFunc(result, func(match string) string {
				return " " // Thay bằng space để giữ structure
			})
		}
	}
	
	// Clean up multiple spaces
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	return strings.TrimSpace(result)
}
