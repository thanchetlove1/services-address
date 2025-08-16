package normalizer

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	
	"github.com/address-parser/internal/external"
)

// NormalizationResult kết quả normalize từ V2
type NormalizationResult struct {
	OriginalCleaned      string            `json:"original_cleaned"`
	NormalizedNoDiacritics string          `json:"normalized_no_diacritics"`
	Fingerprint          string            `json:"fingerprint"`
	ComponentTags        map[string]string `json:"component_tags"`
	LibpostalResult      *external.LibpostalResult `json:"libpostal_result,omitempty"`
	Confidence           float64           `json:"confidence"`
	UseLibpostal         bool              `json:"use_libpostal"`
}

// TextNormalizer normalizer phiên bản 2 với cải tiến
type TextNormalizer struct {
	// Regex patterns từ data/regex.yaml
	rePhonesOrders *regexp.Regexp
	reSpaces       *regexp.Regexp
	reRoadCode     *regexp.Regexp
	reHouseNumber  *regexp.Regexp
	reUnit         *regexp.Regexp
	reLevel        *regexp.Regexp
	rePOI          *regexp.Regexp
	reWardTok      *regexp.Regexp
	reDistTok      *regexp.Regexp
	reProvTok      *regexp.Regexp
	
	// Thêm patterns từ data
	reNoisePatterns *regexp.Regexp
	reBuilding      *regexp.Regexp
	reComplex       *regexp.Regexp
	reKhuPho        *regexp.Regexp
	reNgach         *regexp.Regexp
	
	// Cấu hình
	useLibpostal bool
}

// NewTextNormalizer tạo mới TextNormalizer
func NewTextNormalizer() *TextNormalizer {
	return &TextNormalizer{
		// Patterns cơ bản
		rePhonesOrders: regexp.MustCompile(`(?i)(\+?84|0)\d{8,11}|[A-Z]{2}\d{6,}|CTN\w+`),
		reSpaces:       regexp.MustCompile(`\s+`),
		reRoadCode:     regexp.MustCompile(`\b(ql|dt|tl|hl|dh)\s*([0-9]{1,4}[a-z]?)\b`),
		reHouseNumber:  regexp.MustCompile(`\b(\d{1,5}[A-Za-z]?)(?:/\d+)?\b`),
		reUnit:         regexp.MustCompile(`(?i)\b(can ho|chung cu|apartment|unit|phong|p\.?|tang|floor|t\.)\s*([A-Za-z0-9\.\-]+)\b`),
		reLevel:        regexp.MustCompile(`(?i)\b(tang|floor|t\.)\s*([0-9]{1,2})\b`),
		rePOI:          regexp.MustCompile(`(?i)\b(toa nha|chung cu|building|tower|plaza|center|trung tam|sieu thi|nha hang|quan|cafe|ngan hang|benh vien|truong|khu|kdc|cc|ct|tn|toa|nha)\b`),
		reWardTok:      regexp.MustCompile(`\bphuong\s+([a-z0-9\s]+)`),
		reDistTok:      regexp.MustCompile(`\b(quan|huyen|thi tran|thi xa|thanh pho)\s+([a-z0-9\s]+)`),
		reProvTok:      regexp.MustCompile(`\b(tinh|thanh pho)\s+([a-z0-9\s]+)`),
		
		// Patterns từ data/regex.yaml
		reNoisePatterns: regexp.MustCompile(`(?i)\b(?:địa chỉ|address|địa điểm|location):\s*`),
		reBuilding:      regexp.MustCompile(`(?i)(?:tower|block|tòa|lầu|tầng|suite|unit|apt|room|ch|can\s*ho)\s+([a-z0-9\s\-\.]+)`),
		reComplex:       regexp.MustCompile(`(?i)(?:vinhomes|royal\s*city|somerset|vcci|kcn|kdc|kđt)\s+([a-z0-9\s]+)`),
		reKhuPho:        regexp.MustCompile(`(?i)\b(?:kp|khu phố)\s*(\d+)\b`),
		reNgach:         regexp.MustCompile(`(?i)\b(?:ngách|ngõ|hẻm|hem)\s*(\d+)\b`),
		
		// Mặc định bật libpostal
		useLibpostal: true,
	}
}

// SetUseLibpostal cài đặt có sử dụng libpostal hay không
func (tn *TextNormalizer) SetUseLibpostal(use bool) {
	tn.useLibpostal = use
}

// stripDiacritics loại bỏ dấu tiếng Việt một cách an toàn
func (tn *TextNormalizer) stripDiacritics(s string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(func(r rune) bool {
		return unicode.Is(unicode.Mn, r)
	}), norm.NFC)
	out, _, _ := transform.String(t, s)
	return out
}

// expandVN mở rộng các viết tắt tiếng Việt từ data/unigram_map.yaml
func (tn *TextNormalizer) expandVN(s string) string {
	r := " " + s + " "
	replacements := map[string]string{
		// Administrative abbreviations từ unigram_map.yaml
		" tp ": " thanh pho ",
		" hcm ": " ho chi minh ",
		" q ": " quan ",
		" p ": " phuong ",
		" h ": " huyen ",
		" tx ": " thi xa ",
		" tt ": " thi tran ",
		" x ": " xa ",
		
		// Street abbreviations từ unigram_map.yaml
		" d ": " duong ",
		" duong ": " duong ",
		" ql ": " quoc lo ",
		" dt ": " duong tinh ",
		" tl ": " tinh lo ",
		" dl ": " dai lo ",
		
		// Common abbreviations từ unigram_map.yaml
		" kp ": " khu pho ",
		" hem ": " hem ",
		" ngach ": " ngach ",
		" ngo ": " ngo ",
		" kcn ": " khu cong nghiep ",
		" kdc ": " khu dan cu ",
		" kdt ": " khu do thi ",
		
		// Thành phố (từ ngram_map.yaml)
		" tp hcm ": " thanh pho ho chi minh ",
		" tphcm ":  " thanh pho ho chi minh ",
		" sai gon ": " thanh pho ho chi minh ",
		" sg ": " thanh pho ho chi minh ",
		" ha noi ": " ha noi ",
		" hn ": " ha noi ",
		" da nang ": " thanh pho da nang ",
		" dn ": " thanh pho da nang ",
		" can tho ": " thanh pho can tho ",
		" ct ": " thanh pho can tho ",
		" hue ": " thanh pho hue ",
		
		// Quận/Huyện
		" q.": " quan ",
		" h.": " huyen ",
		
		// Phường/Xã
		" p.": " phuong ",
		" x.": " xa ",
		
		// Thành phố/Tỉnh
		" t.": " tinh ",
		" t ": " tinh ",
		
		// Đường
		" dg ": " duong ",
		" dg.": " duong ",
		
		// Khu phố
		" kp.": " khu pho ",
		
		// Ấp
		" ap.": " ap ",
		
		// Tổ
		" to.": " to ",
		
		// Khóm
		" khom.": " khom ",
		
		// Thôn
		" thon.": " thon ",
		
		// Xóm
		" xom.": " xom ",
	}
	
	for k, v := range replacements {
		r = strings.ReplaceAll(r, k, v)
	}
	
	return strings.TrimSpace(tn.reSpaces.ReplaceAllString(r, " "))
}

// extractComponents trích xuất các thành phần từ địa chỉ
func (tn *TextNormalizer) extractComponents(s string) map[string]string {
	components := make(map[string]string)
	
	// Road code (QL/DT/TL/HL/DH) từ data/regex.yaml
	if m := tn.reRoadCode.FindStringSubmatch(s); len(m) == 3 {
		components["road_type"] = strings.ToUpper(m[1])
		components["road_code"] = m[2]
	}
	
	// House number từ data/regex.yaml
	if m := tn.reHouseNumber.FindStringSubmatch(s); len(m) > 1 {
		components["house_number"] = m[1]
	}
	
	// Khu phố từ data/regex.yaml
	if m := tn.reKhuPho.FindStringSubmatch(s); len(m) > 1 {
		components["khu_pho"] = m[1]
	}
	
	// Ngách/Hẻm từ data/regex.yaml
	if m := tn.reNgach.FindStringSubmatch(s); len(m) > 1 {
		components["ngach"] = m[1]
	}
	
	// Unit/Phòng
	if m := tn.reUnit.FindStringSubmatch(s); len(m) > 2 {
		components["unit"] = m[2]
	}
	
	// Level/Tầng
	if m := tn.reLevel.FindStringSubmatch(s); len(m) > 2 {
		components["level"] = m[2]
	}
	
	// POI (Point of Interest) từ data/regex.yaml
	if m := tn.rePOI.FindStringSubmatch(s); len(m) > 1 {
		components["poi_type"] = m[1]
	}
	
	// Building từ data/regex.yaml
	if m := tn.reBuilding.FindStringSubmatch(s); len(m) > 1 {
		components["building"] = m[1]
	}
	
	// Complex từ data/regex.yaml
	if m := tn.reComplex.FindStringSubmatch(s); len(m) > 1 {
		components["complex"] = m[1]
	}
	
	// Ward/Phường
	if m := tn.reWardTok.FindStringSubmatch(s); len(m) > 1 {
		components["ward"] = strings.TrimSpace(m[1])
	}
	
	// District/Quận/Huyện
	if m := tn.reDistTok.FindStringSubmatch(s); len(m) > 2 {
		components["district"] = strings.TrimSpace(m[2])
	}
	
	// Province/Tỉnh/Thành phố
	if m := tn.reProvTok.FindStringSubmatch(s); len(m) > 2 {
		components["province"] = strings.TrimSpace(m[2])
	}
	
	return components
}

// calculateRuleBasedConfidence tính confidence dựa trên rule-based
func (tn *TextNormalizer) calculateRuleBasedConfidence(components map[string]string, normalized string) float64 {
	score := 0.0
	total := 0.0
	
	// Điểm cho các components chính
	if components["house_number"] != "" { score += 0.2; total += 0.2 }
	if components["road_type"] != "" { score += 0.15; total += 0.15 }
	if components["ward"] != "" { score += 0.2; total += 0.2 }
	if components["district"] != "" { score += 0.2; total += 0.2 }
	if components["province"] != "" { score += 0.15; total += 0.15 }
	if components["unit"] != "" { score += 0.1; total += 0.1 }
	
	// Điểm cho độ dài và cấu trúc
	if len(normalized) > 50 { score += 0.1; total += 0.1 }
	if strings.Contains(normalized, "phuong") && strings.Contains(normalized, "quan") { score += 0.1; total += 0.1 }
	
	if total == 0 {
		return 0.0
	}
	
	return score / total
}

// NormalizeAddress normalize địa chỉ và trả về NormalizationResult
func (tn *TextNormalizer) NormalizeAddress(rawAddress string) *NormalizationResult {
	if rawAddress == "" {
		return &NormalizationResult{
			ComponentTags: make(map[string]string),
			Confidence:    0.0,
			UseLibpostal:  false,
		}
	}

	// 1. Cắt nhiễu (điện thoại, mã đơn hàng, prefix)
	cleaned := tn.rePhonesOrders.ReplaceAllString(rawAddress, " ")
	cleaned = tn.reNoisePatterns.ReplaceAllString(cleaned, "")
	
	// 2. NFD → bỏ dấu → lower → collapse space
	normalized := tn.stripDiacritics(cleaned)
	normalized = strings.ToLower(normalized)
	normalized = tn.reSpaces.ReplaceAllString(strings.TrimSpace(normalized), " ")
	
	// 3. Mở rộng viết tắt
	normalized = tn.expandVN(normalized)
	
	// 4. Trích xuất components
	components := tn.extractComponents(normalized)
	
	// 5. Tính confidence rule-based
	ruleBasedConfidence := tn.calculateRuleBasedConfidence(components, normalized)
	
	// 6. Tạo fingerprint
	fingerprint := "sha256:" + strings.ToLower(strings.ReplaceAll(normalized, " ", ""))
	
	result := &NormalizationResult{
		OriginalCleaned:       cleaned,
		NormalizedNoDiacritics: normalized,
		Fingerprint:           fingerprint,
		ComponentTags:         components,
		Confidence:            ruleBasedConfidence,
		UseLibpostal:          false,
	}
	
	// 7. Tích hợp libpostal nếu cần
	if tn.useLibpostal {
		libpostalResult := external.ExtractWithLibpostalFallback(rawAddress, ruleBasedConfidence)
		result.LibpostalResult = &libpostalResult
		result.UseLibpostal = true
		
		// Cập nhật confidence khi kết hợp cả hai
		if libpostalResult.Confidence > ruleBasedConfidence {
			result.Confidence = (ruleBasedConfidence + libpostalResult.Confidence) / 2.0
		}
		
		// Bổ sung thông tin từ libpostal nếu rule-based thiếu
		if components["house_number"] == "" && libpostalResult.House != "" {
			components["house_number"] = libpostalResult.House
		}
		if components["ward"] == "" && libpostalResult.Ward != "" {
			components["ward"] = libpostalResult.Ward
		}
		if components["district"] == "" && libpostalResult.City != "" {
			components["district"] = libpostalResult.City
		}
		if components["province"] == "" && libpostalResult.Province != "" {
			components["province"] = libpostalResult.Province
		}
	}
	
	return result
}

// NormalizeBatch normalize nhiều địa chỉ cùng lúc
func (tn *TextNormalizer) NormalizeBatch(addresses []string) []*NormalizationResult {
	results := make([]*NormalizationResult, len(addresses))
	for i, addr := range addresses {
		results[i] = tn.NormalizeAddress(addr)
	}
	return results
}
