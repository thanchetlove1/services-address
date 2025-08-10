package parser

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/address-parser/app/models"
	"github.com/address-parser/internal/normalizer"
	"github.com/address-parser/internal/search"
	"github.com/agnivade/levenshtein"
	"github.com/xrash/smetrics"
	"go.uber.org/zap"
)

// MatchStrategy enum cho các chiến lược matching
type MatchStrategy string

const (
	MatchStrategyExact     MatchStrategy = "exact"
	MatchStrategyAscii     MatchStrategy = "ascii_exact"
	MatchStrategyAlias     MatchStrategy = "alias"
	MatchStrategyFuzzy     MatchStrategy = "fuzzy"
)

// MatchLevel enum cho mức độ match
type MatchLevel string

const (
	MatchLevelExact     MatchLevel = "exact"
	MatchLevelAscii     MatchLevel = "ascii_exact"
	MatchLevelFuzzy     MatchLevel = "fuzzy"
)

// QualityFlag enum cho các cờ chất lượng
type QualityFlag string

const (
	FlagExactMatch        QualityFlag = "EXACT_MATCH"
	FlagAsciiExact        QualityFlag = "ASCII_EXACT"
	FlagFuzzyMatch        QualityFlag = "FUZZY_MATCH"
	FlagPOIExtracted      QualityFlag = "POI_EXTRACTED"
	FlagApartmentUnit     QualityFlag = "APARTMENT_UNIT"
	FlagMultiLanguage     QualityFlag = "MULTI_LANGUAGE"
	FlagAmbiguousWard     QualityFlag = "AMBIGUOUS_WARD"
	FlagMultipleCandidates QualityFlag = "MULTIPLE_CANDIDATES"
	FlagLowConfidence     QualityFlag = "LOW_CONFIDENCE"
	FlagMissingWard       QualityFlag = "MISSING_WARD"
)

// AddressMatcher service thực hiện matching và scoring
type AddressMatcher struct {
	searcher    *search.GazetteerSearcher
	normalizer  *normalizer.TextNormalizer
	extractor   *normalizer.PatternExtractor
	logger      *zap.Logger
	
	// Configuration
	thresholdHigh   float64 // 0.9 - matched
	thresholdMedium float64 // 0.6 - ambiguous
	maxCandidates   int     // 20 per level
}

// MatchResult kết quả matching của một địa chỉ
type MatchResult struct {
	Raw                   string                 `json:"raw"`
	CanonicalText         string                 `json:"canonical_text"`
	NormalizedNoDiacritics string                `json:"normalized_no_diacritics"`
	Components            models.AddressComponents `json:"components"`
	Quality               models.QualityMetrics  `json:"quality"`
	Residual              string                 `json:"residual"`
	RawFingerprint        string                 `json:"raw_fingerprint"`
	Confidence            float64                `json:"confidence"`
	MatchStrategy         MatchStrategy          `json:"match_strategy"`
	AdminPath             []string               `json:"admin_path"`
	Candidates            []models.Candidate     `json:"candidates"`
	Status                string                 `json:"status"` // matched, ambiguous, needs_review, unmatched
}

// NewAddressMatcher tạo mới AddressMatcher
func NewAddressMatcher(searcher *search.GazetteerSearcher, textNormalizer *normalizer.TextNormalizer, logger *zap.Logger) *AddressMatcher {
	return &AddressMatcher{
		searcher:        searcher,
		normalizer:      textNormalizer,
		extractor:       normalizer.NewPatternExtractor(),
		logger:          logger,
		thresholdHigh:   0.9,
		thresholdMedium: 0.6,
		maxCandidates:   20,
	}
}

// MatchAddress thực hiện matching địa chỉ với multi-stage strategy
func (am *AddressMatcher) MatchAddress(rawAddress string, gazetteerVersion string) (*MatchResult, error) {
	start := time.Now()
	
	// 1. Normalize address
	_, normalized := am.normalizer.NormalizeAddress(rawAddress)
	
	// 2. Extract patterns
	patterns := am.extractor.ExtractPatterns(normalized)
	
	// 3. Generate fingerprint
	fingerprint := am.generateFingerprint(normalized, gazetteerVersion)
	
	// 4. Multi-stage matching (exact → ascii → alias → fuzzy)
	result := &MatchResult{
		Raw:                   rawAddress,
		NormalizedNoDiacritics: normalized,
		RawFingerprint:        fingerprint,
		Candidates:            make([]models.Candidate, 0),
		Quality: models.QualityMetrics{
			Flags: make([]string, 0),
		},
	}
	
	// 5. Right-to-left hierarchy detection theo PROMPT (Province-first strategy)
	var candidates []models.Candidate
	var strategy MatchStrategy
	
	// Extract administrative components
	adminComponents := am.extractAdministrativeComponents(normalized)
	
	// Province-first: Find province to narrow down candidates
	if adminComponents.Province != "" {
		provinceCandidates, provinceStrategy, err := am.tryExactMatch(adminComponents.Province)
		if err != nil {
			am.logger.Warn("Lỗi province match", zap.String("province", adminComponents.Province), zap.Error(err))
		} else if len(provinceCandidates) > 0 {
			candidates = append(candidates, provinceCandidates...)
			strategy = provinceStrategy
			
			// District filtering: only consider districts within found province
			if adminComponents.District != "" {
				districtCandidates := am.filterDistrictsByProvince(provinceCandidates, adminComponents.District)
				if len(districtCandidates) > 0 {
					candidates = append(candidates, districtCandidates...)
				}
			}
			
			// Ward filtering: only consider wards within found district  
			if adminComponents.Ward != "" && len(candidates) > 0 {
				wardCandidates := am.filterWardsByDistrict(candidates, adminComponents.Ward)
				if len(wardCandidates) > 0 {
					candidates = append(candidates, wardCandidates...)
				}
			}
		}
	}
	
	// 6. Try ASCII matching if no exact match (fast, score=0.9)
	if len(candidates) == 0 {
		candidates, strategy, err = am.tryAsciiMatch(normalized)
		if err != nil {
			am.logger.Warn("Lỗi ASCII match", zap.Error(err))
		}
	}
	
	// 7. Try alias matching (medium, score=0.8)
	if len(candidates) == 0 {
		candidates, strategy, err = am.tryAliasMatch(normalized)
		if err != nil {
			am.logger.Warn("Lỗi alias match", zap.Error(err))
		}
	}
	
	// 8. Try controlled fuzzy matching (slowest, score=0.5-0.7, chỉ khi cần thiết)
	if len(candidates) == 0 {
		candidates, strategy, err = am.tryFuzzyMatch(normalized)
		if err != nil {
			am.logger.Warn("Lỗi fuzzy match", zap.Error(err))
		}
	}
	
	// 9. Score and rank candidates
	if len(candidates) > 0 {
		am.scoreAndRankCandidates(candidates, normalized, patterns)
		result.Candidates = candidates
		result.MatchStrategy = strategy
		
		// Take best candidate
		bestCandidate := candidates[0]
		result.Confidence = bestCandidate.Score
		result.AdminPath = am.buildAdminPath(bestCandidate.AdminUnits)
		result.CanonicalText = am.buildCanonicalText(bestCandidate.AdminUnits)
	}
	
	// 10. Extract components and build result
	result.Components = am.extractComponents(patterns, result.AdminPath)
	result.Residual = am.calculateResidual(normalized, patterns)
	
	// 11. Determine quality flags and status
	am.assignQualityFlags(result, patterns)
	am.determineStatus(result)
	
	// 12. Log performance
	duration := time.Since(start)
	am.logger.Debug("Address matching completed",
		zap.String("raw", rawAddress),
		zap.Float64("confidence", result.Confidence),
		zap.String("strategy", string(result.MatchStrategy)),
		zap.String("status", result.Status),
		zap.Duration("duration", duration))
	
	return result, nil
}

// tryExactMatch thử exact matching (có dấu)
func (am *AddressMatcher) tryExactMatch(normalized string) ([]models.Candidate, MatchStrategy, error) {
	// Direct search in Meilisearch for exact matches
	adminUnits, _, err := am.searcher.SearchWithFilter(normalized, "", 10)
	if err != nil {
		return nil, MatchStrategyExact, err
	}
	
	// Convert AdminUnits to Candidates with exact match scoring
	var exactCandidates []models.Candidate
	for _, unit := range adminUnits {
		candidate := models.Candidate{
			AdminUnit: unit,
			Score:     1.0, // Exact match gets highest score
		}
		exactCandidates = append(exactCandidates, candidate)
	}
	
	return exactCandidates, MatchStrategyExact, nil
}

// tryExactMatchOld - old method using complex SearchAddress
func (am *AddressMatcher) tryExactMatchOld(normalized string) ([]models.Candidate, MatchStrategy, error) {
	candidates, err := am.searcher.SearchAddress(normalized, 4)
	if err != nil {
		return nil, MatchStrategyExact, err
	}
	
	// Filter cho exact matches
	var exactCandidates []models.Candidate
	for _, candidate := range candidates {
		// Check exact match trong name hoặc aliases
		if am.isExactMatch(normalized, candidate) {
			candidate.Score = 1.0
			exactCandidates = append(exactCandidates, candidate)
		}
	}
	
	return exactCandidates, MatchStrategyExact, nil
}

// tryAsciiMatch thử ASCII matching (không dấu)
func (am *AddressMatcher) tryAsciiMatch(normalized string) ([]models.Candidate, MatchStrategy, error) {
	candidates, err := am.searcher.SearchAddress(normalized, 4)
	if err != nil {
		return nil, MatchStrategyAscii, err
	}
	
	// Filter cho ASCII exact matches
	var asciiCandidates []models.Candidate
	for _, candidate := range candidates {
		if am.isAsciiMatch(normalized, candidate) {
			candidate.Score = 0.9
			asciiCandidates = append(asciiCandidates, candidate)
		}
	}
	
	return asciiCandidates, MatchStrategyAscii, nil
}

// tryAliasMatch thử alias expansion matching
func (am *AddressMatcher) tryAliasMatch(normalized string) ([]models.Candidate, MatchStrategy, error) {
	candidates, err := am.searcher.SearchAddress(normalized, 4)
	if err != nil {
		return nil, MatchStrategyAlias, err
	}
	
	// Filter cho alias matches
	var aliasCandidates []models.Candidate
	for _, candidate := range candidates {
		if am.isAliasMatch(normalized, candidate) {
			candidate.Score = 0.8
			aliasCandidates = append(aliasCandidates, candidate)
		}
	}
	
	return aliasCandidates, MatchStrategyAlias, nil
}

// tryFuzzyMatch thử controlled fuzzy matching
func (am *AddressMatcher) tryFuzzyMatch(normalized string) ([]models.Candidate, MatchStrategy, error) {
	candidates, err := am.searcher.FuzzySearch(normalized, 0.5)
	if err != nil {
		return nil, MatchStrategyFuzzy, err
	}
	
	// Apply fuzzy optimization rules
	var fuzzyCandidates []models.Candidate
	for _, candidate := range candidates {
		score := am.calculateFuzzyScore(normalized, candidate)
		if score >= 0.5 {
			candidate.Score = score
			fuzzyCandidates = append(fuzzyCandidates, candidate)
		}
	}
	
	return fuzzyCandidates, MatchStrategyFuzzy, nil
}

// isExactMatch kiểm tra exact match
func (am *AddressMatcher) isExactMatch(query string, candidate models.Candidate) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	
	for _, unit := range candidate.AdminUnits {
		if strings.ToLower(strings.TrimSpace(unit.Name)) == query {
			return true
		}
		if strings.ToLower(strings.TrimSpace(unit.NormalizedName)) == query {
			return true
		}
	}
	
	return false
}

// isAsciiMatch kiểm tra ASCII match
func (am *AddressMatcher) isAsciiMatch(query string, candidate models.Candidate) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	
	for _, unit := range candidate.AdminUnits {
		if strings.ToLower(strings.TrimSpace(unit.NormalizedName)) == query {
			return true
		}
	}
	
	return false
}

// isAliasMatch kiểm tra alias match
func (am *AddressMatcher) isAliasMatch(query string, candidate models.Candidate) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	
	for _, unit := range candidate.AdminUnits {
		for _, alias := range unit.Aliases {
			if strings.ToLower(strings.TrimSpace(alias)) == query {
				return true
			}
		}
	}
	
	return false
}

// calculateFuzzyScore tính điểm fuzzy với edit distance
func (am *AddressMatcher) calculateFuzzyScore(query string, candidate models.Candidate) float64 {
	maxScore := 0.0
	
	for _, unit := range candidate.AdminUnits {
		// Jaro-Winkler distance
		jaroScore := smetrics.JaroWinkler(query, strings.ToLower(unit.Name), 0.7, 4)
		if jaroScore > maxScore {
			maxScore = jaroScore
		}
		
		// Levenshtein distance with length penalty
		levDist := levenshtein.ComputeDistance(query, strings.ToLower(unit.Name))
		maxLen := math.Max(float64(len(query)), float64(len(unit.Name)))
		levScore := 1.0 - (float64(levDist) / maxLen)
		
		if levScore > maxScore {
			maxScore = levScore
		}
		
		// Check aliases
		for _, alias := range unit.Aliases {
			aliasScore := smetrics.JaroWinkler(query, strings.ToLower(alias), 0.7, 4)
			if aliasScore > maxScore {
				maxScore = aliasScore
			}
		}
	}
	
	// Apply fuzzy optimization rules
	queryLen := len(query)
	if queryLen <= 10 && maxScore > 0.8 { // Short names need higher accuracy
		return maxScore
	} else if queryLen > 10 && maxScore > 0.6 { // Long names more tolerant
		return maxScore
	}
	
	return 0.0 // Below threshold
}

// scoreAndRankCandidates tính điểm và xếp hạng candidates
func (am *AddressMatcher) scoreAndRankCandidates(candidates []models.Candidate, query string, patterns []normalizer.PatternResult) {
	for i := range candidates {
		baseScore := candidates[i].Score
		
		// Enhanced scoring formula từ prompt
		structuralBonus := am.calculateStructuralBonus(candidates[i])
		geographicBonus := am.calculateGeographicBonus(candidates[i])
		hierarchyBonus := am.calculateHierarchyBonus(candidates[i])
		lengthPenalty := am.calculateLengthPenalty(query, candidates[i])
		
		finalScore := baseScore + structuralBonus + geographicBonus + hierarchyBonus - lengthPenalty
		
		// Clamp to [0.0, 1.0]
		if finalScore < 0.0 {
			finalScore = 0.0
		}
		if finalScore > 1.0 {
			finalScore = 1.0
		}
		
		candidates[i].Score = finalScore
	}
	
	// Sort by score descending
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[i].Score < candidates[j].Score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
}

// calculateStructuralBonus tính bonus cho đúng admin_subtype
func (am *AddressMatcher) calculateStructuralBonus(candidate models.Candidate) float64 {
	bonus := 0.0
	
	for _, unit := range candidate.AdminUnits {
		switch unit.AdminSubtype {
		case "country", "province", "municipality", "urban_district", "rural_district", "ward":
			bonus += 0.05 // +0.15 total maximum for good structure
		}
	}
	
	if bonus > 0.15 {
		bonus = 0.15
	}
	
	return bonus
}

// calculateGeographicBonus tính bonus cho đường phổ biến trong quận
func (am *AddressMatcher) calculateGeographicBonus(candidate models.Candidate) float64 {
	// TODO: Implement street popularity in district logic
	// Hiện tại return 0.0, sẽ implement sau khi có data thống kê
	return 0.0
}

// calculateHierarchyBonus tính bonus cho đúng parent-child relationship
func (am *AddressMatcher) calculateHierarchyBonus(candidate models.Candidate) float64 {
	bonus := 0.0
	
	// Check hierarchy consistency
	units := candidate.AdminUnits
	for i := 0; i < len(units)-1; i++ {
		if len(units) > i+1 {
			if units[i+1].ParentID != nil && *units[i+1].ParentID == units[i].AdminID {
				bonus += 0.05
			}
		}
	}
	
	if bonus > 0.1 {
		bonus = 0.1
	}
	
	return bonus
}

// calculateLengthPenalty tính penalty cho unmatched tokens
func (am *AddressMatcher) calculateLengthPenalty(query string, candidate models.Candidate) float64 {
	queryTokens := strings.Fields(query)
	matchedTokens := 0
	
	for _, token := range queryTokens {
		for _, unit := range candidate.AdminUnits {
			if strings.Contains(strings.ToLower(unit.Name), strings.ToLower(token)) {
				matchedTokens++
				break
			}
		}
	}
	
	unmatchedTokens := len(queryTokens) - matchedTokens
	penalty := float64(unmatchedTokens) * 0.05
	
	if penalty > 0.3 {
		penalty = 0.3 // Max penalty
	}
	
	return penalty
}

// extractComponents trích xuất components từ patterns và admin path
func (am *AddressMatcher) extractComponents(patterns []normalizer.PatternResult, adminPath []string) models.AddressComponents {
	components := models.AddressComponents{}
	
	// Extract từ patterns
	for _, pattern := range patterns {
		switch pattern.Type {
		case "HOUSE_NO":
			components.House.Number = &pattern.Value
		case "APARTMENT":
			components.House.Unit = pattern.Value
		case "FLOOR":
			if floor := pattern.Value; floor != "" {
				components.House.Floor = &floor
			}
		case "ROAD_CODE":
			if roadType, ok := pattern.Metadata["road_type"]; ok {
				if roadNumber, ok := pattern.Metadata["road_number"]; ok {
					components.RoadCode = &models.RoadCode{
						Type: roadType,
						Code: roadNumber,
					}
				}
			}
		case "ALLEY":
			if alleyNum := pattern.Value; alleyNum != "" {
				components.House.Alley.Number = &alleyNum
			}
		case "ALLEY_NAME":
			if alleyName := pattern.Value; alleyName != "" {
				components.House.Alley.Name = &alleyName
			}
		}
	}
	
	return components
}

// calculateResidual tính phần residual không được map
func (am *AddressMatcher) calculateResidual(normalized string, patterns []normalizer.PatternResult) string {
	residual := normalized
	
	// Remove extracted patterns
	for _, pattern := range patterns {
		// Remove pattern tokens from residual
		for _, token := range pattern.Tokens {
			residual = strings.ReplaceAll(residual, token, " ")
		}
	}
	
	// Clean up
	residual = regexp.MustCompile(`\s+`).ReplaceAllString(residual, " ")
	residual = strings.TrimSpace(residual)
	
	return residual
}

// assignQualityFlags gán các quality flags
func (am *AddressMatcher) assignQualityFlags(result *MatchResult, patterns []normalizer.PatternResult) {
	flags := make([]string, 0)
	
	// Match level flags
	switch result.MatchStrategy {
	case MatchStrategyExact:
		flags = append(flags, string(FlagExactMatch))
		result.Quality.MatchLevel = string(MatchLevelExact)
	case MatchStrategyAscii:
		flags = append(flags, string(FlagAsciiExact))
		result.Quality.MatchLevel = string(MatchLevelAscii)
	case MatchStrategyFuzzy:
		flags = append(flags, string(FlagFuzzyMatch))
		result.Quality.MatchLevel = string(MatchLevelFuzzy)
	}
	
	// Pattern-based flags
	for _, pattern := range patterns {
		switch pattern.Type {
		case "APARTMENT", "FLOOR", "OFFICE", "UNIT":
			flags = append(flags, string(FlagApartmentUnit))
		}
	}
	
	// Multiple candidates flag
	if len(result.Candidates) > 1 {
		flags = append(flags, string(FlagMultipleCandidates))
	}
	
	// Confidence-based flags
	if result.Confidence < am.thresholdMedium {
		flags = append(flags, string(FlagLowConfidence))
	}
	
	result.Quality.Flags = flags
	result.Quality.Score = result.Confidence
}

// determineStatus xác định status của kết quả
func (am *AddressMatcher) determineStatus(result *MatchResult) {
	if result.Confidence >= am.thresholdHigh {
		result.Status = "matched"
	} else if result.Confidence >= am.thresholdMedium {
		result.Status = "ambiguous"
	} else {
		result.Status = "needs_review"
	}
	
	if len(result.Candidates) == 0 {
		result.Status = "unmatched"
	}
}

// buildAdminPath xây dựng admin path từ AdminUnits
func (am *AddressMatcher) buildAdminPath(units []models.AdminUnit) []string {
	path := make([]string, len(units))
	for i, unit := range units {
		path[i] = unit.Name
	}
	return path
}

// AdministrativeComponents holds extracted admin components
type AdministrativeComponents struct {
	Province string
	District string
	Ward     string
	Street   string
	HouseNo  string
}

// extractAdministrativeComponents extracts admin components from normalized address
func (am *AddressMatcher) extractAdministrativeComponents(normalized string) AdministrativeComponents {
	components := AdministrativeComponents{}
	words := strings.Split(normalized, "_")
	
	// Right-to-left scan for administrative components
	for i := len(words) - 1; i >= 0; i-- {
		word := words[i]
		
		// Look for province patterns (rightmost administrative level)
		if am.isProvincePattern(word) {
			components.Province = word
			break
		}
	}
	
	// Look for district patterns (left of province)
	for _, word := range words {
		if am.isDistrictPattern(word) {
			components.District = word
			break
		}
	}
	
	// Look for ward patterns (left of district)
	for _, word := range words {
		if am.isWardPattern(word) {
			components.Ward = word
			break
		}
	}
	
	// Look for street patterns
	for _, word := range words {
		if am.isStreetPattern(word) {
			components.Street = word
			break
		}
	}
	
	// Look for house number patterns
	for _, word := range words {
		if am.isHouseNoPattern(word) {
			components.HouseNo = word
			break
		}
	}
	
	return components
}

// Helper pattern detection methods
func (am *AddressMatcher) isProvincePattern(word string) bool {
	// Common province names and patterns
	provincePatterns := []string{
		"tien_giang", "ho_chi_minh", "ha_noi", "da_nang", "hai_phong", "can_tho",
		"an_giang", "bac_giang", "ben_tre", "binh_duong", "binh_phuoc", "binh_thuan",
		"ca_mau", "cao_bang", "dak_lak", "dak_nong", "dong_nai", "dong_thap",
		"gia_lai", "ha_giang", "ha_nam", "ha_tinh", "hau_giang", "hoa_binh",
		"hung_yen", "khanh_hoa", "kien_giang", "kon_tum", "lai_chau", "lam_dong",
		"lang_son", "lao_cai", "long_an", "nam_dinh", "nghe_an", "ninh_binh",
		"ninh_thuan", "phu_tho", "phu_yen", "quang_binh", "quang_nam", "quang_ngai",
		"quang_ninh", "quang_tri", "soc_trang", "son_la", "tay_ninh", "thai_binh",
		"thai_nguyen", "thanh_hoa", "thua_thien_hue", "tra_vinh", "tuyen_quang",
		"vinh_long", "vinh_phuc", "yen_bai", "bac_kan", "bac_ninh",
	}
	
	for _, pattern := range provincePatterns {
		if word == pattern {
			return true
		}
	}
	return false
}

func (am *AddressMatcher) isDistrictPattern(word string) bool {
	return strings.HasPrefix(word, "quan_") || 
		   strings.HasPrefix(word, "huyen_") ||
		   strings.HasPrefix(word, "thi_xa_") ||
		   strings.HasPrefix(word, "thanh_pho_")
}

func (am *AddressMatcher) isWardPattern(word string) bool {
	return strings.HasPrefix(word, "phuong_") || 
		   strings.HasPrefix(word, "xa_") ||
		   strings.HasPrefix(word, "thi_tran_")
}

func (am *AddressMatcher) isStreetPattern(word string) bool {
	return strings.Contains(word, "duong") ||
		   strings.HasPrefix(word, "ql_") ||
		   strings.HasPrefix(word, "dt_") ||
		   strings.HasPrefix(word, "tl_") ||
		   strings.Contains(word, "hem") ||
		   strings.Contains(word, "ngo")
}

func (am *AddressMatcher) isHouseNoPattern(word string) bool {
	return strings.HasPrefix(word, "so_nha_")
}

// filterDistrictsByProvince filters districts that belong to given province candidates
func (am *AddressMatcher) filterDistrictsByProvince(provinceCandidates []models.Candidate, district string) []models.Candidate {
	var results []models.Candidate
	
	for _, provinceCandidate := range provinceCandidates {
		// Search for districts within this province
		districtCandidates, _, err := am.searcher.SearchWithFilter(district, 
			fmt.Sprintf("level = 3 AND parent_id = '%s'", provinceCandidate.AdminUnit.AdminID), 5)
		if err != nil {
			am.logger.Warn("Error filtering districts", zap.Error(err))
			continue
		}
		
		// Convert AdminUnits to Candidates
		for _, adminUnit := range districtCandidates {
			candidate := models.Candidate{
				AdminUnit: adminUnit,
				Score:     0.8, // District match score
			}
			results = append(results, candidate)
		}
	}
	
	return results
}

// filterWardsByDistrict filters wards that belong to given district candidates
func (am *AddressMatcher) filterWardsByDistrict(districtCandidates []models.Candidate, ward string) []models.Candidate {
	var results []models.Candidate
	
	for _, districtCandidate := range districtCandidates {
		// Search for wards within this district
		wardUnits, _, err := am.searcher.SearchWithFilter(ward,
			fmt.Sprintf("level = 4 AND parent_id = '%s'", districtCandidate.AdminUnit.AdminID), 5)
		if err != nil {
			am.logger.Warn("Error filtering wards", zap.Error(err))
			continue
		}
		
		// Convert AdminUnits to Candidates
		for _, adminUnit := range wardUnits {
			candidate := models.Candidate{
				AdminUnit: adminUnit,
				Score:     0.9, // Ward match score
			}
			results = append(results, candidate)
		}
	}
	
	return results
}

// buildCanonicalText xây dựng canonical text từ AdminUnits
func (am *AddressMatcher) buildCanonicalText(units []models.AdminUnit) string {
	var parts []string
	for _, unit := range units {
		parts = append(parts, unit.Name)
	}
	return strings.Join(parts, ", ")
}

// generateFingerprint sinh fingerprint cho cache
func (am *AddressMatcher) generateFingerprint(normalized, gazetteerVersion string) string {
	// SHA256(normalized + "\x1F" + gazetteer_version)
	input := normalized + "\x1F" + gazetteerVersion
	// TODO: Implement actual SHA256 hashing
	return fmt.Sprintf("sha256:%x", input) // Placeholder
}
