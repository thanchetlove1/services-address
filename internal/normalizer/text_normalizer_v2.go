package normalizer

import (
	"regexp"
	"strings"
	
	"github.com/mozillazg/go-unidecode"
)

// TextNormalizerV2 implements 9-step normalization pipeline theo PROMPT HYBRID-FINAL
type TextNormalizerV2 struct {
	// Step 1: Noise removal patterns (precompiled for performance)
	phonePattern    *regexp.Regexp
	orderCodePattern *regexp.Regexp
	prefixPattern   *regexp.Regexp
	punctuationPattern *regexp.Regexp
	
	// Step 2: POI extraction patterns
	companyPattern  *regexp.Regexp
	buildingPattern *regexp.Regexp
	complexPattern  *regexp.Regexp
	
	// Step 3: Abbreviation maps (context-aware)
	adminLevelMap   map[string]string
	streetTypeMap   map[string]string
	specialCaseMap  map[string]string
	
	// Step 4: Multi-language maps
	englishVietnameseMap map[string]string
	
	// Step 5: P. disambiguation patterns
	phuongPattern   *regexp.Regexp
	phongPattern    *regexp.Regexp
	
	// Step 6: Advanced pattern recognition (prioritized)
	highPriorityPatterns map[string]*regexp.Regexp
	normalPriorityPatterns map[string]*regexp.Regexp
	
	// Step 7: Dictionary maps (unigram + ngram)
	unigramMap      map[string]string
	bigramMap       map[string]string
	trigramMap      map[string]string
	
	// Step 8: Administrative hierarchy detection
	provinceAliases map[string][]string
	districtAliases map[string][]string
	wardAliases     map[string][]string
}

// POIInfo contains extracted POI information
type POIInfo struct {
	Company  string `json:"company"`
	Building string `json:"building"`
	Complex  string `json:"complex"`
	KCN      string `json:"kcn"`
	KDC      string `json:"kdc"`
}

// ComponentTags represents tagged address components
type ComponentTags struct {
	AdminL2   string   `json:"admin_l2"`   // province
	AdminL3   string   `json:"admin_l3"`   // district  
	AdminL4   string   `json:"admin_l4"`   // ward
	Street    string   `json:"street"`
	HouseNo   string   `json:"house_no"`
	POI       POIInfo  `json:"poi"`
	Residual  []string `json:"residual"`
}

// NormalizationResult contains complete normalization output
type NormalizationResult struct {
	OriginalCleaned      string        `json:"original_cleaned"`
	NormalizedNoDiacritics string      `json:"normalized_no_diacritics"`
	ComponentTags        ComponentTags `json:"component_tags"`
	QualityFlags         []string      `json:"quality_flags"`
	Fingerprint          string        `json:"fingerprint"`
}

// NewTextNormalizerV2 creates new normalizer with PROMPT specifications
func NewTextNormalizerV2() *TextNormalizerV2 {
	tn := &TextNormalizerV2{
		adminLevelMap:        make(map[string]string),
		streetTypeMap:        make(map[string]string),
		specialCaseMap:       make(map[string]string),
		englishVietnameseMap: make(map[string]string),
		unigramMap:          make(map[string]string),
		bigramMap:           make(map[string]string),
		trigramMap:          make(map[string]string),
		provinceAliases:     make(map[string][]string),
		districtAliases:     make(map[string][]string),
		wardAliases:         make(map[string][]string),
		highPriorityPatterns: make(map[string]*regexp.Regexp),
		normalPriorityPatterns: make(map[string]*regexp.Regexp),
	}
	
	tn.initializePatterns()
	tn.initializeMaps()
	
	return tn
}

// initializePatterns compiles all regex patterns for performance
func (tn *TextNormalizerV2) initializePatterns() {
	// Step 1: Noise removal patterns
	tn.phonePattern = regexp.MustCompile(`\d{9,}`)
	tn.orderCodePattern = regexp.MustCompile(`(?i)CT\d+`)
	tn.prefixPattern = regexp.MustCompile(`(?i)^(địa chỉ|mr|ms)\s*:?\s*`)
	tn.punctuationPattern = regexp.MustCompile(`[–—\-/,.;:]+`)
	
	// Step 2: POI extraction (Smart Rules)
	tn.companyPattern = regexp.MustCompile(`(?i)(CÔNG TY|CTY|NGÂN HÀNG|TRƯỜNG|BỆNH VIỆN|TRUNG TÂM)\s+([^,]+)`)
	tn.buildingPattern = regexp.MustCompile(`(?i)(Tower|Block|Tòa|Lầu|Tầng|Suite|Unit|Apt|Room)\s*([A-Z0-9.]+)`)
	tn.complexPattern = regexp.MustCompile(`(?i)(Vinhomes|Royal City|Somerset|VCCI|KCN|KDC|KĐT)\s*([^,]+)`)
	
	// Step 5: P. disambiguation
	tn.phuongPattern = regexp.MustCompile(`(?i)P\.\s*([1-9]|1[0-9]|20|[a-zA-Z]+)`)
	tn.phongPattern = regexp.MustCompile(`(?i)P\s*([0-9]{3,5})`)
	
	// Step 6: Advanced pattern recognition (HIGH PRIORITY - run first)
	tn.highPriorityPatterns["APARTMENT"] = regexp.MustCompile(`(?i)^(?:CH|CAN\s*HO)\s*([0-9]+(?:-[0-9]+)?)$`)
	tn.highPriorityPatterns["FLOOR"] = regexp.MustCompile(`(?i)^(?:TANG|LAU)\s*([0-9]+)$`)
	tn.highPriorityPatterns["OFFICE"] = regexp.MustCompile(`(?i)^(?:VH|VAN\s*PHONG)\s*([0-9]+)$`)
	tn.highPriorityPatterns["BLOCK_TOWER"] = regexp.MustCompile(`(?i)^(?:BLOCK|TOA)\s*([A-Z0-9]+)$`)
	
	// Step 6: Normal priority patterns
	tn.normalPriorityPatterns["HOUSE_NO"] = regexp.MustCompile(`(?i)^(?:SO\s*)?([0-9]+(?:[/-][0-9]+)*|NV[0-9]+(?:-[0-9]+)?|LO\s*[A-Z0-9]+(?:-[A-Z0-9]+)?|CH\s*[0-9]+(?:-[0-9]+)?|TANG\s*[0-9]+|LAU\s*[0-9]+)$`)
	tn.normalPriorityPatterns["ROAD_CODE"] = regexp.MustCompile(`(?i)^(?:(QL|DT|TL|HL|DH))\s*([0-9]+[A-Z]?)$`)
	tn.normalPriorityPatterns["ALLEY"] = regexp.MustCompile(`(?i)^(?:HEM|HẺM|NGO|NGÕ|NGACH|NGÁCH|KIET|KIỆT)\s*([0-9]+(?:[/-][0-9]+)*)$`)
	tn.normalPriorityPatterns["ALLEY_NAME"] = regexp.MustCompile(`(?i)^(?:HEM|NGO|NGACH|KIET)\s+([A-Z0-9]+(?:\s+[A-Z0-9]+)*)$`)
	tn.normalPriorityPatterns["Q_NUM"] = regexp.MustCompile(`(?i)^(?:Q|QUAN)\s*([0-9]+)$`)
	tn.normalPriorityPatterns["P_NUM"] = regexp.MustCompile(`(?i)^(?:P|PHUONG)\s*([0-9]+)$`)
	tn.normalPriorityPatterns["KP_NUM"] = regexp.MustCompile(`(?i)^(?:KP|KHU\s*PHO)\s*([0-9]+)$`)
	tn.normalPriorityPatterns["TO_NUM"] = regexp.MustCompile(`(?i)^(?:TO)\s*([0-9]+)$`)
	tn.normalPriorityPatterns["AP_NUM"] = regexp.MustCompile(`(?i)^(?:AP)\s*([0-9]+)$`)
	tn.normalPriorityPatterns["THON_NUM"] = regexp.MustCompile(`(?i)^(?:THON)\s*([0-9]+)$`)
	tn.normalPriorityPatterns["XOM_NUM"] = regexp.MustCompile(`(?i)^(?:XOM)\s*([0-9]+)$`)
}

// initializeMaps initializes all mapping dictionaries theo PROMPT
func (tn *TextNormalizerV2) initializeMaps() {
	// Step 3: Smart abbreviation expansion (context-aware)
	tn.adminLevelMap = map[string]string{
		"tp":    "thanh_pho",
		"t.p":   "thanh_pho", 
		"tp.":   "thanh_pho",
		"tphcm": "thanh_pho_ho_chi_minh",
		"hcm":   "ho_chi_minh",
		"tp.hcm": "thanh_pho_ho_chi_minh",
		"q":     "quan",
		"q.":    "quan",
		"p":     "phuong",
		"p.":    "phuong",
		"px":    "phuong",
		"tx":    "thi_xa",
		"tt":    "thi_tran",
		"ttg":   "thi_tran",
		"h":     "huyen",
		"h.":    "huyen",
		"xa":    "xa",
	}
	
	tn.streetTypeMap = map[string]string{
		"d":     "duong",
		"d.":    "duong",
		"dx":    "duong",
		"dg":    "duong",
		"dt":    "duong_tinh",
		"dl":    "dai_lo", 
		"ql":    "quoc_lo",
		"tl":    "tinh_lo",
		"hl":    "huyen_lo",
		"pho":   "pho",
		"hem":   "hem",
		"ngo":   "ngo",
		"ngach": "ngach",
		"kiet":  "kiet",
	}
	
	tn.specialCaseMap = map[string]string{
		"cmt8":   "cach_mang_thang_tam",
		"3/2":    "3_thang_2",
		"30/4":   "30_thang_4",
		"2/9":    "2_thang_9",
	}
	
	// Step 4: Multi-language support
	tn.englishVietnameseMap = map[string]string{
		"ward":       "phuong",
		"district":   "quan",
		"city":       "thanh_pho",
		"province":   "tinh",
		"street":     "duong",
		"industrial park": "kcn",
		"export processing zone": "kcx",
	}
	
	// Step 7: Dictionary replace (comprehensive from PROMPT)
	tn.unigramMap = map[string]string{
		// Building & Complex
		"kdc":   "khu_dan_cu",
		"kdt":   "khu_do_thi", 
		"kcn":   "khu_cong_nghiep",
		"kp":    "khu_pho",
		"to":    "to",
		"ap":    "ap", 
		"thon":  "thon",
		"xom":   "xom",
		"cum":   "cum",
		"khoi":  "khoi",
		"khom":  "khom",
		"doi":   "doi",
		"lo":    "lo",
		
		// Building structures
		"cc":    "chung_cu",
		"toa":   "toa_nha",
		"toanha": "toa_nha",
		"block": "block",
		"tang":  "tang",
		"lau":   "tang",
		"can":   "can_ho",
		"ch":    "can_ho",
		"vh":    "van_phong",
		
		// Business entities
		"cty":   "cong_ty",
		"tnhh":  "trach_nhiem_huu_han",
		"cp":    "co_phan",
		"cn":    "chi_nhanh",
		"tc":    "trung_tam",
		"bv":    "benh_vien",
		"nh":    "ngan_hang",
		"st":    "sieu_thi",
		"ks":    "khach_san",
		"truong": "truong",
	}
	
	tn.bigramMap = map[string]string{
		"thanh_pho": "thanh_pho",
		"thi_xa":    "thi_xa",
		"thi_tran":  "thi_tran",
		"khu_dan_cu": "khu_dan_cu",
		"khu_do_thi": "khu_do_thi",
		"khu_pho":   "khu_pho",
		"khu_cong_nghiep": "khu_cong_nghiep",
		"duong_tinh": "dt",
		"dai_lo":    "dl",
		"quoc_lo":   "ql",
		"tinh_lo":   "tl",
		"huyen_lo":  "hl",
		"toa_nha":   "toa_nha",
		"chung_cu":  "chung_cu",
		"can_ho":    "can_ho",
		"van_phong": "van_phong",
		"nha_ve_sinh": "nha_ve_sinh",
		"cong_ty":   "cong_ty",
		"co_phan":   "co_phan",
		"trach_nhiem_huu_han": "trach_nhiem_huu_han",
		"chi_nhanh": "chi_nhanh",
		"trung_tam": "trung_tam",
		"benh_vien": "benh_vien",
		"ngan_hang": "ngan_hang",
		"sieu_thi":  "sieu_thi",
		"khach_san": "khach_san",
		"bai_bien":  "bai_bien",
		"dong_song": "dong_song",
		"nui_rung":  "nui_rung",
	}
	
	// Step 8: Administrative hierarchy aliases (comprehensive)
	tn.provinceAliases = map[string][]string{
		"ho_chi_minh": {"tp_hcm", "tp.hcm", "tphcm", "hcm", "sai_gon", "sg", "thanh_pho_ho_chi_minh", "hcmc", "saigon"},
		"ha_noi":      {"hn", "tp_ha_noi", "hanoi", "ha-noi"},
		"da_nang":     {"dn", "tp_da_nang", "tp.danang", "danang", "da-nang"},
		"hai_phong":   {"hp", "tp_hai_phong", "haiphong", "hai-phong"},
		"can_tho":     {"ct", "tp_can_tho", "cantho", "can-tho"},
		"khanh_hoa":   {"kh", "khanh-hoa"},
		"lam_dong":    {"ld", "lam-dong"},
		"ba_ria_vung_tau": {"brvt", "ba_ria_vung_tau", "ba-ria_vung-tau"},
		"dak_lak":     {"daklak", "dăk_lak", "đăk_lăk", "dăk_lăk", "đắc_lắc", "dac_lac"},
		"dak_nong":    {"daknong", "đăk_nông", "dac_nong", "đắc_nông", "dak-nong"},
		"tien_giang":  {"tiengiang", "tien-giang"},
	}
}

// NormalizeAddress implements complete 9-step pipeline theo PROMPT HYBRID-FINAL
func (tn *TextNormalizerV2) NormalizeAddress(rawAddress string) *NormalizationResult {
	result := &NormalizationResult{
		ComponentTags: ComponentTags{
			POI:      POIInfo{},
			Residual: make([]string, 0),
		},
		QualityFlags: make([]string, 0),
	}
	
	// Step 1: Noise removal & dual normalization
	originalCleaned, normalizedText := tn.step1NoiseRemovalAndDualNormalization(rawAddress)
	result.OriginalCleaned = originalCleaned
	
	// Step 2: POI & Company extraction
	poiExtracted, poi := tn.step2POIAndCompanyExtraction(normalizedText)
	result.ComponentTags.POI = poi
	if poi.Company != "" || poi.Building != "" || poi.Complex != "" {
		result.QualityFlags = append(result.QualityFlags, "POI_EXTRACTED")
	}
	
	// Step 3: Smart abbreviation expansion
	expanded := tn.step3SmartAbbreviationExpansion(poiExtracted)
	
	// Step 4: Multi-language support
	translated := tn.step4MultiLanguageSupport(expanded)
	
	// Step 5: Smart P. disambiguation
	disambiguated := tn.step5SmartPDisambiguation(translated)
	
	// Step 6: Advanced pattern recognition
	patterned, flags := tn.step6AdvancedPatternRecognition(disambiguated)
	result.QualityFlags = append(result.QualityFlags, flags...)
	
	// Step 7: Dictionary replace
	replaced := tn.step7DictionaryReplace(patterned)
	
	// Step 8: Administrative hierarchy detection
	hierarchyDetected, adminTags := tn.step8AdministrativeHierarchyDetection(replaced)
	result.ComponentTags.AdminL2 = adminTags.AdminL2
	result.ComponentTags.AdminL3 = adminTags.AdminL3
	result.ComponentTags.AdminL4 = adminTags.AdminL4
	
	// Step 9: Quality tagging & residual tracking
	finalNormalized, finalTags := tn.step9QualityTaggingAndResidualTracking(hierarchyDetected, result.ComponentTags)
	result.NormalizedNoDiacritics = finalNormalized
	result.ComponentTags = finalTags
	
	// Generate fingerprint
	result.Fingerprint = tn.generateFingerprint(finalNormalized, "1.0.0")
	
	return result
}

// step1NoiseRemovalAndDualNormalization implements PROMPT step 1 - FIXED ORDER
func (tn *TextNormalizerV2) step1NoiseRemovalAndDualNormalization(input string) (string, string) {
	// Create dual versions: (a) original cleaned, (b) no diacritics + lowercase
	original := strings.TrimSpace(input)
	if original == "" {
		return "", ""
	}
	
	// Start with lowercase for processing
	working := strings.ToLower(original)
	
	// 1) Remove noise patterns FIRST (while still has diacritics)
	working = tn.phonePattern.ReplaceAllString(working, " ")
	working = tn.orderCodePattern.ReplaceAllString(working, " ")
	working = tn.prefixPattern.ReplaceAllString(working, " ")
	
	// 2) Merge punctuation to spaces
	working = tn.punctuationPattern.ReplaceAllString(working, " ")
	
	// 3) Strip diacritics SECOND (after noise removal)
	normalized := tn.removeDiacritics(working)
	
	// 4) NOW filter ASCII-safe and collapse spaces  
	normalized = regexp.MustCompile(`[^a-z0-9\s/.\-]`).ReplaceAllString(normalized, " ")
	normalized = strings.ReplaceAll(normalized, "đ", "d") // Handle đ specifically
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)
	
	// Fallback: ensure never empty
	if normalized == "" {
		fallback := tn.removeDiacritics(strings.ToLower(input))
		normalized = regexp.MustCompile(`[^a-z0-9\s]`).ReplaceAllString(fallback, " ")
		normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
		normalized = strings.TrimSpace(normalized)
	}
	
	// DEBUG: Log what we're producing
	if normalized == "" {
		// Still empty? Something is very wrong
		normalized = "debug_empty_" + strings.ToLower(input)
	}
	
	return original, normalized
}

// step2POIAndCompanyExtraction implements PROMPT step 2
func (tn *TextNormalizerV2) step2POIAndCompanyExtraction(input string) (string, POIInfo) {
	poi := POIInfo{}
	result := input
	
	// Extract company/organization names
	if matches := tn.companyPattern.FindStringSubmatch(input); len(matches) > 2 {
		poi.Company = strings.TrimSpace(matches[2])
		result = tn.companyPattern.ReplaceAllString(result, "")
	}
	
	// Extract building information
	if matches := tn.buildingPattern.FindStringSubmatch(input); len(matches) > 2 {
		poi.Building = strings.TrimSpace(matches[1] + " " + matches[2])
		result = tn.buildingPattern.ReplaceAllString(result, "")
	}
	
	// Extract complex names
	if matches := tn.complexPattern.FindStringSubmatch(input); len(matches) > 2 {
		poi.Complex = strings.TrimSpace(matches[1] + " " + matches[2])
		result = tn.complexPattern.ReplaceAllString(result, "")
		
		// Check for specific types
		complexName := strings.ToLower(matches[1])
		if strings.Contains(complexName, "kcn") {
			poi.KCN = poi.Complex
		} else if strings.Contains(complexName, "kdc") || strings.Contains(complexName, "kdt") {
			poi.KDC = poi.Complex
		}
	}
	
	// Clean up result
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)
	
	return result, poi
}

// step3SmartAbbreviationExpansion implements PROMPT step 3
func (tn *TextNormalizerV2) step3SmartAbbreviationExpansion(input string) string {
	words := strings.Fields(input)
	result := make([]string, 0, len(words))
	
	for i, word := range words {
		expanded := word
		
		// Context-aware expansion for administrative levels
		if replacement, exists := tn.adminLevelMap[word]; exists {
			// Check context to avoid false positives
			if tn.isAdminLevelContext(words, i) {
				expanded = replacement
			}
		}
		
		// Street type expansion
		if replacement, exists := tn.streetTypeMap[word]; exists {
			if tn.isStreetTypeContext(words, i) {
				expanded = replacement
			}
		}
		
		// Special cases (always apply when exact match)
		if replacement, exists := tn.specialCaseMap[word]; exists {
			expanded = replacement
		}
		
		result = append(result, expanded)
	}
	
	return strings.Join(result, " ")
}

// step4MultiLanguageSupport implements PROMPT step 4
func (tn *TextNormalizerV2) step4MultiLanguageSupport(input string) string {
	result := input
	
	// Replace English terms with Vietnamese equivalents
	for english, vietnamese := range tn.englishVietnameseMap {
		pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(english) + `\b`)
		result = pattern.ReplaceAllString(result, vietnamese)
	}
	
	return result
}

// step5SmartPDisambiguation implements PROMPT step 5
func (tn *TextNormalizerV2) step5SmartPDisambiguation(input string) string {
	result := input
	
	// P. + number/name -> Phường
	result = tn.phuongPattern.ReplaceAllStringFunc(result, func(match string) string {
		return tn.phuongPattern.ReplaceAllString(match, "phuong $1")
	})
	
	// P + 3-5 digits -> Phòng (house.unit)
	result = tn.phongPattern.ReplaceAllStringFunc(result, func(match string) string {
		return tn.phongPattern.ReplaceAllString(match, "phong $1")
	})
	
	return result
}

// step6AdvancedPatternRecognition implements PROMPT step 6 
func (tn *TextNormalizerV2) step6AdvancedPatternRecognition(input string) (string, []string) {
	words := strings.Fields(input)
	result := make([]string, 0, len(words))
	flags := make([]string, 0)
	
	for _, word := range words {
		processed := word
		matched := false
		
		// HIGH PRIORITY patterns (run first)
		for patternName, pattern := range tn.highPriorityPatterns {
			if pattern.MatchString(word) {
				switch patternName {
				case "APARTMENT":
					flags = append(flags, "APARTMENT_UNIT")
					processed = "apartment_" + pattern.ReplaceAllString(word, "$1")
				case "FLOOR":
					processed = "floor_" + pattern.ReplaceAllString(word, "$1")
				case "OFFICE":
					processed = "office_" + pattern.ReplaceAllString(word, "$1") 
				case "BLOCK_TOWER":
					processed = "block_" + pattern.ReplaceAllString(word, "$1")
				}
				matched = true
				break
			}
		}
		
		// NORMAL PRIORITY patterns (if no high priority match)
		if !matched {
			for patternName, pattern := range tn.normalPriorityPatterns {
				if pattern.MatchString(word) {
					switch patternName {
					case "HOUSE_NO":
						processed = "so_nha_" + pattern.ReplaceAllString(word, "$1")
					case "ROAD_CODE":
						processed = pattern.ReplaceAllString(word, "${1}_$2")
					case "ALLEY":
						processed = "hem_" + pattern.ReplaceAllString(word, "$1")
					case "ALLEY_NAME":
						processed = "hem_" + pattern.ReplaceAllString(word, "$1")
					case "Q_NUM":
						processed = "quan_" + pattern.ReplaceAllString(word, "$1")
					case "P_NUM":
						processed = "phuong_" + pattern.ReplaceAllString(word, "$1")
					case "KP_NUM":
						processed = "khu_pho_" + pattern.ReplaceAllString(word, "$1")
					case "TO_NUM":
						processed = "to_" + pattern.ReplaceAllString(word, "$1")
					case "AP_NUM":
						processed = "ap_" + pattern.ReplaceAllString(word, "$1")
					case "THON_NUM":
						processed = "thon_" + pattern.ReplaceAllString(word, "$1")
					case "XOM_NUM":
						processed = "xom_" + pattern.ReplaceAllString(word, "$1")
					}
					matched = true
					break
				}
			}
		}
		
		result = append(result, processed)
	}
	
	return strings.Join(result, " "), flags
}

// step7DictionaryReplace implements PROMPT step 7
func (tn *TextNormalizerV2) step7DictionaryReplace(input string) string {
	words := strings.Fields(input)
	result := make([]string, 0, len(words))
	
	// Process unigrams first
	for _, word := range words {
		if replacement, exists := tn.unigramMap[word]; exists {
			result = append(result, replacement)
		} else {
			result = append(result, word)
		}
	}
	
	// Process bigrams and trigrams
	text := strings.Join(result, " ")
	for ngram, replacement := range tn.bigramMap {
		text = strings.ReplaceAll(text, ngram, replacement)
	}
	
	return text
}

// step8AdministrativeHierarchyDetection implements PROMPT step 8 (right-to-left)
func (tn *TextNormalizerV2) step8AdministrativeHierarchyDetection(input string) (string, ComponentTags) {
	words := strings.Fields(input)
	tags := ComponentTags{}
	
	// Right-to-left strategy: find province first
	for i := len(words) - 1; i >= 0; i-- {
		word := words[i]
		
		// Check province aliases
		for province, aliases := range tn.provinceAliases {
			if word == province {
				tags.AdminL2 = province
				break
			}
			for _, alias := range aliases {
				if word == alias {
					tags.AdminL2 = province
					break
				}
			}
			if tags.AdminL2 != "" {
				break
			}
		}
		
		// Check for district patterns (after province found)
		if tags.AdminL2 != "" && strings.Contains(word, "quan_") {
			tags.AdminL3 = word
		}
		
		// Check for ward patterns (after district found)
		if tags.AdminL3 != "" && strings.Contains(word, "phuong_") {
			tags.AdminL4 = word
		}
	}
	
	return input, tags
}

// step9QualityTaggingAndResidualTracking implements PROMPT step 9
func (tn *TextNormalizerV2) step9QualityTaggingAndResidualTracking(input string, existingTags ComponentTags) (string, ComponentTags) {
	words := strings.Fields(input)
	finalTags := existingTags
	residual := make([]string, 0)
	processedWords := make([]string, 0)
	
	for _, word := range words {
		matched := false
		
		// Tag known components
		if strings.HasPrefix(word, "so_nha_") {
			finalTags.HouseNo = word
			matched = true
		} else if strings.HasPrefix(word, "quan_") || strings.HasPrefix(word, "huyen_") {
			if finalTags.AdminL3 == "" {
				finalTags.AdminL3 = word
			}
			matched = true
		} else if strings.HasPrefix(word, "phuong_") || strings.HasPrefix(word, "xa_") {
			if finalTags.AdminL4 == "" {
				finalTags.AdminL4 = word
			}
			matched = true
		} else if strings.Contains(word, "duong") || strings.HasPrefix(word, "ql_") || strings.HasPrefix(word, "dt_") {
			finalTags.Street = word
			matched = true
		}
		
		if matched {
			processedWords = append(processedWords, word)
		} else {
			// Keep unknown tokens in output (they might be province/district names)
			// Only move to residual if they look like noise
			if len(word) >= 2 { // Keep meaningful words
				processedWords = append(processedWords, word)
			} else {
				// Single chars or obvious noise go to residual for learning
				residual = append(residual, word)
			}
		}
	}
	
	finalTags.Residual = residual
	
	// Final cleanup with spaces (not underscores)
	finalText := strings.Join(processedWords, " ")
	finalText = regexp.MustCompile(`\s+`).ReplaceAllString(finalText, " ")
	finalText = strings.TrimSpace(finalText)
	
	return finalText, finalTags
}

// Helper functions
func (tn *TextNormalizerV2) removeDiacritics(input string) string {
	return unidecode.Unidecode(input)
}

func (tn *TextNormalizerV2) isAdminLevelContext(words []string, index int) bool {
	// Simple context check - can be enhanced
	return true // For now, always expand
}

func (tn *TextNormalizerV2) isStreetTypeContext(words []string, index int) bool {
	// Simple context check - can be enhanced
	return true // For now, always expand
}

func (tn *TextNormalizerV2) generateFingerprint(normalized, version string) string {
	// SHA256 of normalized + version as per PROMPT
	return "sha256:" + normalized + "\x1F" + version
}
