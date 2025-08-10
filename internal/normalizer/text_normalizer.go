package normalizer

import (
	"regexp"
	"strings"
	"github.com/mozillazg/go-unidecode"
)

// TextNormalizer service chuẩn hóa văn bản địa chỉ
type TextNormalizer struct {
	// Regex patterns đã compile sẵn
	noisePatterns     []*regexp.Regexp
	abbreviationMap   map[string]string
	unigramMap        map[string]string
	ngramMap          map[string]string
	provinceAliases   map[string][]string
	cityAliases       map[string][]string
	districtAliases   map[string][]string
}

// NewTextNormalizer tạo mới TextNormalizer
func NewTextNormalizer() *TextNormalizer {
	tn := &TextNormalizer{
		abbreviationMap: make(map[string]string),
		unigramMap:      make(map[string]string),
		ngramMap:        make(map[string]string),
		provinceAliases: make(map[string][]string),
		cityAliases:     make(map[string][]string),
		districtAliases: make(map[string][]string),
	}
	
	tn.initializePatterns()
	tn.initializeMaps()
	
	return tn
}

// initializePatterns khởi tạo các regex patterns
func (tn *TextNormalizer) initializePatterns() {
	// Noise patterns
	tn.noisePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:địa chỉ|mr|ms|cty|ngân hàng|trường|bệnh viện|trung tâm)\s*:?\s*`),
		regexp.MustCompile(`(?i)(?:số điện thoại|phone|tel)\s*:?\s*\d{9,}`),
		regexp.MustCompile(`(?i)(?:mã đơn hàng|order|mã)\s*:?\s*CT\d+`),
		regexp.MustCompile(`(?i)(?:tiền|giá|price)\s*:?\s*\d+`),
	}
}

// initializeMaps khởi tạo các bản đồ mapping
func (tn *TextNormalizer) initializeMaps() {
	// Unigram map
	tn.unigramMap = map[string]string{
		// Core administrative levels
		"tp": "thanh pho",
		"t.p": "thanh pho",
		"tp.": "thanh pho",
		"tphcm": "thanh pho ho chi minh",
		"hcm": "ho chi minh",
		"tp.hcm": "thanh pho ho chi minh",
		"q": "quan",
		"q.": "quan",
		"p": "phuong",
		"p.": "phuong",
		"px": "phuong",
		"tx": "thi xa",
		"tt": "thi tran",
		"ttg": "thi tran",
		"h": "huyen",
		"h.": "huyen",
		"xa": "xa",
		
		// Road types
		"dx": "duong",
		"d": "duong",
		"d.": "duong",
		"dg": "duong",
		"duong": "duong",
		"dt": "duong tinh",
		"dl": "dai lo",
		"ql": "quoc lo",
		"tl": "tinh lo",
		"pho": "pho",
		
		// Building & Complex
		"kdc": "khu dan cu",
		"kdt": "khu do thi",
		"kcn": "khu cong nghiep",
		"kp": "khu pho",
		"to": "to",
		"ap": "ap",
		"thon": "thon",
		"xom": "xom",
		"hem": "hem",
		"ngo": "ngo",
		"ngach": "ngach",
		"kiet": "kiet",
		"cum": "cum",
		"khoi": "khoi",
		"khom": "khom",
		"doi": "doi",
		"lo": "lo",
		
		// Building structures
		"cc": "chung cu",
		"toa": "toa nha",
		"toanha": "toa nha",
		"block": "block",
		"tang": "tang",
		"lau": "tang",
		"can": "can ho",
		"ch": "can ho",
		"vh": "van phong",
		
		// Business entities
		"cty": "cong ty",
		"tnhh": "trach nhiem huu han",
		"cp": "co phan",
		"cn": "chi nhanh",
		"tc": "trung tam",
		"bv": "benh vien",
		"nh": "ngan hang",
		"st": "sieu thi",
		"ks": "khach san",
		"truong": "truong",
	}
	
	// Ngram map
	tn.ngramMap = map[string]string{
		// Administrative levels
		"thanh pho": "thanh pho",
		"thi xa": "thi xa",
		"thi tran": "thi tran",
		"khu dan cu": "khu dan cu",
		"khu do thi": "khu do thi",
		"khu pho": "khu pho",
		"khu cong nghiep": "khu cong nghiep",
		
		// Road types
		"duong tinh": "dt",
		"dai lo": "dl",
		"quoc lo": "ql",
		"tinh lo": "tl",
		"huyen lo": "hl",
		
		// Building & Complex
		"toa nha": "toa nha",
		"chung cu": "chung cu",
		"can ho": "can ho",
		"van phong": "van phong",
		"nha ve sinh": "nha ve sinh",
		
		// Business entities
		"cong ty": "cong ty",
		"co phan": "co phan",
		"trach nhiem huu han": "trach nhiem huu han",
		"chi nhanh": "chi nhanh",
		"trung tam": "trung tam",
		"benh vien": "benh vien",
		"ngan hang": "ngan hang",
		"sieu thi": "sieu thi",
		"khach san": "khach san",
		
		// Geographic features
		"bai bien": "bai bien",
		"dong song": "dong song",
		"nui rung": "nui rung",
	}
	
	// Province aliases
	tn.provinceAliases = map[string][]string{
		"ho chi minh": {"tp hcm", "tp.hcm", "tphcm", "hcm", "sai gon", "sg", "thanh pho ho chi minh", "hcmc", "saigon"},
		"ha noi": {"hn", "tp ha noi", "hanoi", "ha-noi"},
		"da nang": {"dn", "tp da nang", "tp.danang", "danang", "da-nang"},
		"hai phong": {"hp", "tp hai phong", "haiphong", "hai-phong"},
		"can tho": {"ct", "tp can tho", "cantho", "can-tho"},
		"khanh hoa": {"kh", "khanh-hoa"},
		"lam dong": {"ld", "lam-dong"},
		"ba ria - vung tau": {"brvt", "ba ria vung tau", "ba-ria - vung-tau"},
		"dak lak": {"daklak", "dăk lak", "đăk lăk", "dăk lăk", "đắc lắc", "dac lac"},
		"dak nong": {"daknong", "đăk nông", "dac nong", "đắc nông", "dak-nong"},
	}
	
	// City aliases
	tn.cityAliases = map[string][]string{
		"da lat": {"dalat", "da-lat", "tp da lat", "tp.dalat"},
		"nha trang": {"nhatrang", "nha-trang", "tp nha trang"},
		"buon ma thuot": {"buonmathuot", "bmthuot", "dak lak city"},
		"vung tau": {"vung-tau", "vt"},
	}
	
	// District aliases
	tn.districtAliases = map[string][]string{
		"ba ria": {"ba-ria"},
		"long khanh": {"long-khanh"},
		"bien hoa": {"bien-hoa"},
		"thu dau mot": {"thu-dau-mot", "tdm"},
	}
}

// NormalizeAddress chuẩn hóa địa chỉ using V2 implementation
func (tn *TextNormalizer) NormalizeAddress(rawAddress string) (string, string) {
	// Use V2 normalizer for complete implementation
	normalizerV2 := NewTextNormalizerV2()
	result := normalizerV2.NormalizeAddress(rawAddress)
	
	return result.OriginalCleaned, result.NormalizedNoDiacritics
}

// removeNoise loại bỏ noise phi-địa-chỉ
func (tn *TextNormalizer) removeNoise(text string) string {
	result := text
	for _, pattern := range tn.noisePatterns {
		result = pattern.ReplaceAllString(result, "")
	}
	return result
}

// cleanPunctuation chuẩn hóa dấu câu
func (tn *TextNormalizer) cleanPunctuation(text string) string {
	// Gộp các dấu câu về khoảng trắng
	result := regexp.MustCompile(`[–—\-\/,\.;:]`).ReplaceAllString(text, " ")
	// Rút gọn đa khoảng trắng
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	return strings.TrimSpace(result)
}

// removeDiacritics loại bỏ dấu và chuyển về lowercase
func (tn *TextNormalizer) removeDiacritics(text string) string {
	// Sử dụng go-unidecode để loại bỏ dấu
	result := unidecode.Unidecode(text)
	// Chuyển về lowercase
	result = strings.ToLower(result)
	return result
}

// extractPOI trích xuất POI và thông tin công ty
func (tn *TextNormalizer) extractPOI(text string) string {
	// Các pattern POI
	poiPatterns := []struct {
		pattern *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`(?i)(?:công ty|cty|ngân hàng|trường|bệnh viện|trung tâm)\s+([a-z0-9\s]+)`), "poi_company_$1"},
		{regexp.MustCompile(`(?i)(?:tower|block|tòa|lầu|tầng|suite|unit|apt|room|ch|can\s*ho)\s+([a-z0-9\s\-\.]+)`), "poi_building_$1"},
		{regexp.MustCompile(`(?i)(?:vinhomes|royal\s*city|somerset|vcci|kcn|kdc|kđt)\s+([a-z0-9\s]+)`), "poi_complex_$1"},
	}
	
	result := text
	for _, poi := range poiPatterns {
		result = poi.pattern.ReplaceAllString(result, poi.replacement)
	}
	
	return result
}

// expandAbbreviations mở rộng các từ viết tắt
func (tn *TextNormalizer) expandAbbreviations(text string) string {
	result := text
	
	// Administrative levels với context-aware expansion
	adminPatterns := []struct {
		pattern *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`\b(?:tp|hcm)\b`), "thanh pho ho chi minh"},
		{regexp.MustCompile(`\b(?:q|quan)\b`), "quan"},
		{regexp.MustCompile(`\b(?:p|phuong)\b`), "phuong"},
		{regexp.MustCompile(`\b(?:h|huyen)\b`), "huyen"},
		{regexp.MustCompile(`\b(?:tx|thi xa)\b`), "thi xa"},
		{regexp.MustCompile(`\b(?:tt|thi tran)\b`), "thi tran"},
	}
	
	for _, admin := range adminPatterns {
		result = admin.pattern.ReplaceAllString(result, admin.replacement)
	}
	
	// Street types
	streetPatterns := []struct {
		pattern *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`\b(?:d|duong)\b`), "duong"},
		{regexp.MustCompile(`\b(?:ql|quoc lo)\b`), "quoc lo"},
		{regexp.MustCompile(`\b(?:dt|duong tinh)\b`), "duong tinh"},
		{regexp.MustCompile(`\b(?:tl|tinh lo)\b`), "tinh lo"},
		{regexp.MustCompile(`\b(?:dl|dai lo)\b`), "dai lo"},
	}
	
	for _, street := range streetPatterns {
		result = street.pattern.ReplaceAllString(result, street.replacement)
	}
	
	return result
}

// translateMultiLanguage hỗ trợ đa ngôn ngữ
func (tn *TextNormalizer) translateMultiLanguage(text string) string {
	// Map từ tiếng Anh sang tiếng Việt
	languageMap := map[string]string{
		"ward": "phuong",
		"district": "quan",
		"city": "thanh pho",
		"province": "tinh",
		"street": "duong",
		"road": "duong",
		"avenue": "dai lo",
		"industrial park": "khu cong nghiep",
		"export processing zone": "khu chuc xuat",
	}
	
	result := text
	for eng, vn := range languageMap {
		pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(eng) + `\b`)
		result = pattern.ReplaceAllString(result, vn)
	}
	
	return result
}

// disambiguateP phân biệt P. (Phường) và P (Phòng)
func (tn *TextNormalizer) disambiguateP(text string) string {
	// P. + số 1-20 hoặc tên chữ → Phường
	wardPattern := regexp.MustCompile(`\b(?:p|phuong)\.?\s*(?:(\d{1,2})|([a-z]+))\b`)
	result := wardPattern.ReplaceAllString(text, "phuong $1$2")
	
	// P + 3-5 chữ số trong context tòa/cc → Phòng
	roomPattern := regexp.MustCompile(`\b(?:p|phong)\s*(\d{3,5})\b`)
	result = roomPattern.ReplaceAllString(result, "phong $1")
	
	return result
}

// applyPatterns áp dụng các pattern recognition
func (tn *TextNormalizer) applyPatterns(text string) string {
	// Các pattern ưu tiên cao
	highPriorityPatterns := []struct {
		pattern *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`\b(?:ch|can\s*ho)\s*(\d+(?:-\d+)?)\b`), "can_ho_$1"},
		{regexp.MustCompile(`\b(?:tang|lau)\s*(\d+)\b`), "tang_$1"},
		{regexp.MustCompile(`\b(?:vh|van\s*phong)\s*(\d+)\b`), "van_phong_$1"},
		{regexp.MustCompile(`\b(?:block|toa)\s*([a-z0-9]+)\b`), "block_$1"},
	}
	
	result := text
	for _, pattern := range highPriorityPatterns {
		result = pattern.pattern.ReplaceAllString(result, pattern.replacement)
	}
	
	// Các pattern ưu tiên thường
	normalPriorityPatterns := []struct {
		pattern *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`\b(?:so\s*)?(\d+(?:[/-]\d+)*)\b`), "so_nha_$1"},
		{regexp.MustCompile(`\b(?:ql|dt|tl|hl|dh)\s*([0-9]+[a-z]?)\b`), "duong_$1"},
		{regexp.MustCompile(`\b(?:hem|hẻm|ngo|ngõ|ngach|ngách|kiet|kiệt)\s*(\d+)\b`), "hem_$1"},
		{regexp.MustCompile(`\b(?:q|quan)\s*(\d+)\b`), "quan_$1"},
		{regexp.MustCompile(`\b(?:p|phuong)\s*(\d+)\b`), "phuong_$1"},
	}
	
	for _, pattern := range normalPriorityPatterns {
		result = pattern.pattern.ReplaceAllString(result, pattern.replacement)
	}
	
	return result
}

// applyDictionaryMaps áp dụng các bản đồ mapping
func (tn *TextNormalizer) applyDictionaryMaps(text string) string {
	result := text
	
	// Unigram mapping
	for original, canonical := range tn.unigramMap {
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(original) + `\b`)
		result = pattern.ReplaceAllString(result, canonical)
	}
	
	// Ngram mapping
	for original, canonical := range tn.ngramMap {
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(original) + `\b`)
		result = pattern.ReplaceAllString(result, canonical)
	}
	
	return result
}

// finalCleanup dọn dẹp cuối cùng
func (tn *TextNormalizer) finalCleanup(text string) string {
	// Rút gọn đa khoảng trắng
	result := regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	// Loại bỏ khoảng trắng đầu cuối
	result = strings.TrimSpace(result)
	// Chuyển về lowercase
	result = strings.ToLower(result)
	// Replace spaces with underscores for consistency with data format
	result = strings.ReplaceAll(result, " ", "_")
	
	return result
}

// GetNormalizedText trả về văn bản đã chuẩn hóa
func (tn *TextNormalizer) GetNormalizedText(rawAddress string) string {
	_, normalized := tn.NormalizeAddress(rawAddress)
	return normalized
}

// GetOriginalText trả về văn bản gốc đã làm sạch
func (tn *TextNormalizer) GetOriginalText(rawAddress string) string {
	original, _ := tn.NormalizeAddress(rawAddress)
	return original
}
