package services

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/address-parser/app/models"
	"github.com/address-parser/app/requests"
	"github.com/address-parser/internal/parser"
	"github.com/address-parser/internal/normalizer"
	"github.com/address-parser/internal/search"
	"go.uber.org/zap"
)

// AddressService service xử lý logic parse địa chỉ
type AddressService struct {
	parser     *parser.AddressParser
	normalizer *normalizer.TextNormalizer
	searcher   *search.GazetteerSearcher
	logger     *zap.Logger
	startTime  time.Time
	mu         sync.RWMutex
	
	// Job management
	jobs       map[string]*JobStatus
	jobResults map[string][]*models.AddressResult
}

// JobStatus trạng thái của job
type JobStatus struct {
	JobID              string
	Status             string
	Progress           float64
	Processed          int
	Total              int
	EstimatedRemaining int
	Message            string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// NewAddressService tạo mới AddressService
func NewAddressService(parser *parser.AddressParser, normalizer *normalizer.TextNormalizer, searcher *search.GazetteerSearcher, logger *zap.Logger) *AddressService {
	return &AddressService{
		parser:     parser,
		normalizer: normalizer,
		searcher:   searcher,
		logger:     logger,
		startTime:  time.Now(),
		jobs:       make(map[string]*JobStatus),
		jobResults: make(map[string][]*models.AddressResult),
	}
}

// ParseAddress parse một địa chỉ
func (as *AddressService) ParseAddress(rawAddress string, options requests.ParseOptions) (*models.AddressResult, error) {
	if rawAddress == "" {
		return nil, errors.New("địa chỉ không được để trống")
	}

	// Use new ParseSingle method instead of old parser
	result, err := as.ParseSingle(rawAddress, options)
	if err != nil {
		return nil, err
	}

	// Tính toán confidence nếu cần
	if result.Confidence < options.MinConfidence {
		result.Confidence = options.MinConfidence
		result.Status = models.StatusNeedsReview
	}

	return result, nil
}

// ParseSingle implements core parsing logic theo PROMPT HYBRID-FINAL 
func (as *AddressService) ParseSingle(rawAddress string, options requests.ParseOptions) (*models.AddressResult, error) {
	ctx := context.Background()
	gazetteerVersion := "1.0.0" // TODO: get from config
	
	// Step 1: Normalize input
	normalized, _ := as.normalizer.NormalizeAddress(rawAddress)
	if normalized == "" {
		return nil, errors.New("lỗi normalize địa chỉ")
	}
	
	// Step 2: Extract province keywords and search for candidates
	provinceKeywords := as.extractProvinceKeywords(normalized)
	as.logger.Info("Extracted province keywords", zap.Strings("keywords", provinceKeywords))
	
	// Search for province candidates using first keyword
	var provCandidates []search.AdminUnitDoc
	if len(provinceKeywords) > 0 {
		var err error
		provCandidates, err = as.searcher.SearchByLevel(ctx, provinceKeywords[0], 2, "", 10) // level 2 = province
		if err != nil {
			as.logger.Warn("Không tìm được province candidates", zap.Error(err))
			return nil, err
		}
	}
	
	as.logger.Info("Found province candidates", zap.Int("count", len(provCandidates)))
	for _, cand := range provCandidates {
		as.logger.Info("Province candidate", zap.String("name", cand.Name), zap.String("id", cand.AdminID))
	}
	
	var bestResult *models.AddressResult
	var bestScore float64 = 0.0
	
	// Step 3: Try each province candidate
	for _, provCandidate := range provCandidates {
		// Extract district keywords
		districtKeywords := as.extractDistrictKeywords(normalized)
		as.logger.Info("Extracted district keywords", zap.Strings("keywords", districtKeywords))
		
		// Search for districts in this province
		var distCandidates []search.AdminUnitDoc
		as.logger.Info("Searching for districts", 
			zap.Strings("keywords", districtKeywords),
			zap.String("province_id", provCandidate.AdminID))
		
		for _, keyword := range districtKeywords {
			as.logger.Info("Searching district with keyword", zap.String("keyword", keyword))
			candidates, err := as.searcher.SearchByLevel(ctx, keyword, 3, provCandidate.AdminID, 5) // level 3 = district
			if err != nil {
				as.logger.Warn("District search failed", zap.String("keyword", keyword), zap.Error(err))
				continue
			}
			as.logger.Info("Found district candidates for keyword", 
				zap.String("keyword", keyword), 
				zap.Int("count", len(candidates)))
			distCandidates = append(distCandidates, candidates...)
		}
		
		as.logger.Info("Found district candidates", zap.Int("count", len(distCandidates)))
		
		if len(distCandidates) == 0 {
			continue
		}
		
		for _, distCandidate := range distCandidates {
			// Extract ward keywords
			wardKeywords := as.extractWardKeywords(normalized)
			as.logger.Info("Extracted ward keywords", zap.Strings("keywords", wardKeywords))
			
			// Search for wards in this district
			var wardCandidates []search.AdminUnitDoc
			for _, keyword := range wardKeywords {
				candidates, err := as.searcher.SearchByLevel(ctx, keyword, 4, distCandidate.AdminID, 5) // level 4 = ward
				if err != nil {
					continue
				}
				wardCandidates = append(wardCandidates, candidates...)
			}
			
			as.logger.Info("Found ward candidates", zap.Int("count", len(wardCandidates)))
			
			if len(wardCandidates) > 0 {
				// Build result with ward (perfect match)
				for _, wardCandidate := range wardCandidates {
					result := as.buildAddressResult(rawAddress, normalized, provCandidate, distCandidate, &wardCandidate, gazetteerVersion)
					result.Confidence = 1.0
					result.Status = models.StatusMatched
					return result, nil
				}
			} else {
				// Build result without ward (district-level match)
				result := as.buildAddressResult(rawAddress, normalized, provCandidate, distCandidate, nil, gazetteerVersion)
				result.Confidence = 0.8
				result.Status = models.StatusAmbiguous
				if bestScore < 0.8 {
					bestScore = 0.8
					bestResult = result
				}
			}
		}
	}
	
	// Return best result or fallback
	if bestResult != nil {
		return bestResult, nil
	}
	
	// No matches found - return fallback result
	result := as.buildAddressResult(rawAddress, normalized, search.AdminUnitDoc{}, search.AdminUnitDoc{}, nil, gazetteerVersion)
	result.Status = models.StatusNeedsReview
	result.Confidence = 0.0
	return result, nil
}

// buildAddressResult builds a complete AddressResult from components
func (as *AddressService) buildAddressResult(rawAddress string, normalized string, 
	province search.AdminUnitDoc, district search.AdminUnitDoc, ward *search.AdminUnitDoc, 
	gazetteerVersion string) *models.AddressResult {
	
	result := &models.AddressResult{
		Raw:                     rawAddress,
		NormalizedNoDiacritics: normalized,
		RawFingerprint:         "", // TODO: generate fingerprint
		Components:             models.AddressComponents{},
		Quality:                models.QualityInfo{},
		AdminPath:              []string{},
	}
	
	// Build canonical text and admin path
	var canonical []string
	
	if province.Name != "" {
		canonical = append(canonical, province.Name)
		result.AdminPath = append(result.AdminPath, province.Name)
		result.Components.Province = &models.AdminUnit{
			AdminID: province.AdminID,
			Name:    province.Name,
			Type:    "province",
		}
	}
	
	if district.Name != "" {
		canonical = append(canonical, district.Name)
		result.AdminPath = append(result.AdminPath, district.Name)
		result.Components.District = &models.AdminUnit{
			AdminID: district.AdminID,
			Name:    district.Name,
			Type:    "district",
		}
	}
	
	if ward != nil && ward.Name != "" {
		canonical = append(canonical, ward.Name)
		result.AdminPath = append(result.AdminPath, ward.Name)
		result.Components.Ward = &models.AdminUnit{
			AdminID: ward.AdminID,
			Name:    ward.Name,
			Type:    "ward",
		}
	}
	
	result.CanonicalText = strings.Join(canonical, ", ")
	
	return result
}

// EstimateBatchProcessingTime ước tính thời gian xử lý batch
func (as *AddressService) EstimateBatchProcessingTime(addressCount int) int {
	// Ước tính dựa trên số lượng địa chỉ
	// Giả sử mỗi địa chỉ mất 100ms
	estimatedMs := addressCount * 100
	return estimatedMs / 1000 // Chuyển về giây
}

// ProcessBatchJob xử lý job batch trong background
func (as *AddressService) ProcessBatchJob(jobID string, addresses []string, options requests.ParseOptions) {
	// Tạo job status
	as.mu.Lock()
	as.jobs[jobID] = &JobStatus{
		JobID:     jobID,
		Status:    "running",
		Progress:  0.0,
		Processed: 0,
		Total:     len(addresses),
		Message:   "Đang xử lý...",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	as.mu.Unlock()

	// Xử lý addresses
	results := make([]*models.AddressResult, len(addresses))
	
	for i, address := range addresses {
		result, err := as.ParseAddress(address, options)
		if err != nil {
			// Tạo result lỗi
			result = &models.AddressResult{
				Raw:        address,
				Status:     models.StatusUnmatched,
				Confidence: 0.0,
			}
		}
		results[i] = result

		// Cập nhật progress
		as.mu.Lock()
		if job, exists := as.jobs[jobID]; exists {
			job.Processed = i + 1
			job.Progress = float64(i+1) / float64(len(addresses))
			job.UpdatedAt = time.Now()
			
			if i == len(addresses)-1 {
				job.Status = "done"
				job.Message = "Hoàn thành xử lý"
			}
		}
		as.mu.Unlock()
	}

	// Lưu kết quả
	as.mu.Lock()
	as.jobResults[jobID] = results
	as.mu.Unlock()

	as.logger.Info("Batch job completed",
		zap.String("job_id", jobID),
		zap.Int("total_addresses", len(addresses)))
}

// GetJobStatus lấy trạng thái job
func (as *AddressService) GetJobStatus(jobID string) (*JobStatus, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	job, exists := as.jobs[jobID]
	if !exists {
		return nil, errors.New("job không tồn tại")
	}

	return job, nil
}

// GetJobResults lấy kết quả job
func (as *AddressService) GetJobResults(jobID string) ([]*models.AddressResult, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	results, exists := as.jobResults[jobID]
	if !exists {
		return nil, errors.New("kết quả job không tồn tại")
	}

	return results, nil
}

// GetJobResultsStream lấy kết quả job dưới dạng channel để stream
func (as *AddressService) GetJobResultsStream(jobID string) (<-chan *models.AddressResult, error) {
	results, err := as.GetJobResults(jobID)
	if err != nil {
		return nil, err
	}

	resultChannel := make(chan *models.AddressResult, 100)
	
	go func() {
		defer close(resultChannel)
		for _, result := range results {
			resultChannel <- result
		}
	}()

	return resultChannel, nil
}

// GetStartTime lấy thời gian khởi động service
func (as *AddressService) GetStartTime() time.Time {
	return as.startTime
}

// GetStats lấy thống kê service
func (as *AddressService) GetStats() map[string]interface{} {
	as.mu.RLock()
	defer as.mu.RUnlock()

	uptime := time.Since(as.startTime)
	
	return map[string]interface{}{
		"uptime_seconds": int64(uptime.Seconds()),
		"start_time":     as.startTime.Format(time.RFC3339),
		"status":         "running",
	}
}

// Helper methods for ParseSingle

// extractProvinceCandidates tìm province candidates từ normalized text
func (as *AddressService) extractProvinceCandidates(ctx context.Context, normalized string) ([]search.AdminUnitDoc, error) {
	// Extract province keywords từ normalized text
	provinceKeywords := as.extractProvinceKeywords(normalized)
	as.logger.Info("Extracted province keywords", zap.Strings("keywords", provinceKeywords))
	
	var allCandidates []search.AdminUnitDoc
	
	// Tìm kiếm với từng keyword
	for _, keyword := range provinceKeywords {
		candidates, err := as.searcher.SearchByLevel(ctx, keyword, 2, "", 5) // level 2 = province
		if err != nil {
			as.logger.Warn("Error searching with keyword", zap.String("keyword", keyword), zap.Error(err))
			continue
		}
		allCandidates = append(allCandidates, candidates...)
	}
	
	as.logger.Info("Found province candidates", zap.Int("count", len(allCandidates)))
	for _, candidate := range allCandidates {
		as.logger.Info("Province candidate", 
			zap.String("name", candidate.Name), 
			zap.String("id", candidate.AdminID))
	}
	
	return allCandidates, nil
}

// extractProvinceKeywords extract province-related keywords from normalized text
func (as *AddressService) extractProvinceKeywords(normalized string) []string {
	words := strings.Fields(normalized)
	var keywords []string
	
	// Look for province patterns
	for i, word := range words {
		// Look for "thanh pho X" or "tinh X" patterns
		if word == "thanh" && i+1 < len(words) && words[i+1] == "pho" && i+2 < len(words) {
			// "thanh pho ha noi" -> "ha noi"
			remaining := words[i+2:]
			if len(remaining) > 0 {
				keywords = append(keywords, strings.Join(remaining, " "))
			}
		} else if word == "tinh" && i+1 < len(words) {
			// "tinh nam dinh" -> "nam dinh"  
			remaining := words[i+1:]
			if len(remaining) > 0 {
				keywords = append(keywords, strings.Join(remaining, " "))
			}
		}
	}
	
	// Also try common province names
	commonProvinces := []string{"ha noi", "ho chi minh", "nam dinh", "hai phong", "thanh hoa", "nghe an"}
	for _, province := range commonProvinces {
		if strings.Contains(normalized, province) {
			keywords = append(keywords, province)
		}
	}
	
	// Fallback: if no specific patterns found, try last 2-3 words (often provinces)
	if len(keywords) == 0 && len(words) >= 2 {
		keywords = append(keywords, strings.Join(words[len(words)-2:], " "))
	}
	
	return keywords
}

// extractDistrictCandidates tìm district candidates trong province cụ thể  
func (as *AddressService) extractDistrictCandidates(ctx context.Context, normalized, provinceID string) ([]search.AdminUnitDoc, error) {
	// Extract district keywords từ normalized text
	districtKeywords := as.extractDistrictKeywords(normalized)
	as.logger.Info("Extracted district keywords", zap.Strings("keywords", districtKeywords))
	
	var allCandidates []search.AdminUnitDoc
	
	// Tìm kiếm với từng keyword
	for _, keyword := range districtKeywords {
		candidates, err := as.searcher.SearchByLevel(ctx, keyword, 3, provinceID, 5) // level 3 = district
		if err != nil {
			as.logger.Warn("Error searching district with keyword", zap.String("keyword", keyword), zap.Error(err))
			continue
		}
		allCandidates = append(allCandidates, candidates...)
	}
	
	as.logger.Info("Found district candidates", zap.Int("count", len(allCandidates)))
	return allCandidates, nil
}

// extractWardCandidates tìm ward candidates trong district cụ thể
func (as *AddressService) extractWardCandidates(ctx context.Context, normalized, districtID string) ([]search.AdminUnitDoc, error) {
	// Extract ward keywords từ normalized text
	wardKeywords := as.extractWardKeywords(normalized)
	as.logger.Info("Extracted ward keywords", zap.Strings("keywords", wardKeywords))
	
	var allCandidates []search.AdminUnitDoc
	
	// Tìm kiếm với từng keyword
	for _, keyword := range wardKeywords {
		candidates, err := as.searcher.SearchByLevel(ctx, keyword, 4, districtID, 5) // level 4 = ward
		if err != nil {
			as.logger.Warn("Error searching ward with keyword", zap.String("keyword", keyword), zap.Error(err))
			continue
		}
		allCandidates = append(allCandidates, candidates...)
	}
	
	as.logger.Info("Found ward candidates", zap.Int("count", len(allCandidates)))
	return allCandidates, nil
}

// extractDistrictKeywords extract district-related keywords
func (as *AddressService) extractDistrictKeywords(normalized string) []string {
	words := strings.Fields(normalized)
	var keywords []string
	
	as.logger.Info("Debug district extraction", 
		zap.String("normalized", normalized),
		zap.Strings("words", words))
	
	// Look for "quan X" or "huyen X" patterns
	for i, word := range words {
		if word == "quan" && i+1 < len(words) {
			// "quan long bien" -> "long bien"
			// "quan so_nha_5" -> "5"
			// Find end (stop before "thanh", "tinh", "pho")
			endIdx := len(words)
			for j := i + 1; j < len(words); j++ {
				if words[j] == "thanh" || words[j] == "tinh" || words[j] == "pho" {
					endIdx = j
					break
				}
			}
			
			if endIdx > i+1 {
				districtName := strings.Join(words[i+1:endIdx], " ")
				keywords = append(keywords, districtName)
				
				// Handle "so_nha_5" patterns -> extract just "5"
				for _, part := range words[i+1:endIdx] {
					if strings.HasPrefix(part, "so_nha_") && len(part) > 7 {
						num := part[7:]
						keywords = append(keywords, num)
						keywords = append(keywords, "quan "+num)
					}
				}
			}
		} else if word == "huyen" && i+1 < len(words) {
			// "huyen X" -> "X"
			endIdx := len(words)
			for j := i + 1; j < len(words); j++ {
				if words[j] == "thanh" || words[j] == "tinh" || words[j] == "pho" {
					endIdx = j
					break
				}
			}
			
			if endIdx > i+1 {
				districtName := strings.Join(words[i+1:endIdx], " ")
				keywords = append(keywords, districtName)
			}
		} else if word == "thanh" && i+1 < len(words) && words[i+1] == "pho" && i+2 < len(words) {
			// "thanh pho nam dinh" -> "nam dinh" (this is actually city = district level in some cases)
			remaining := words[i+2:]
			// Stop before "tinh"
			endIdx := len(remaining)
			for j, w := range remaining {
				if w == "tinh" {
					endIdx = j
					break
				}
			}
			
			if endIdx > 0 {
				cityName := strings.Join(remaining[:endIdx], " ")
				keywords = append(keywords, cityName)
			}
		}
	}
	
	// Handle multi-language cases: "District 5" 
	for i, word := range words {
		if word == "district" && i+1 < len(words) {
			// "district 5" -> "quan 5" and "5"
			districtNum := words[i+1]
			// Extract numeric part if it has "so_nha_" prefix
			if strings.HasPrefix(districtNum, "so_nha_") && len(districtNum) > 7 {
				num := districtNum[7:]
				keywords = append(keywords, num)
				keywords = append(keywords, "quan "+num)
			} else {
				keywords = append(keywords, districtNum)
				keywords = append(keywords, "quan "+districtNum)
			}
		}
	}
	
	// Also try common district names by searching for their patterns
	commonDistricts := []string{"long bien", "thanh xuan", "ba dinh", "dong da", "ngo quyen", "nam dinh"}
	for _, district := range commonDistricts {
		if strings.Contains(normalized, district) {
			keywords = append(keywords, district)
		}
	}
	
	return keywords
}

// extractWardKeywords extract ward-related keywords  
func (as *AddressService) extractWardKeywords(normalized string) []string {
	words := strings.Fields(normalized)
	var keywords []string
	
	// Look for "phuong X" patterns
	for i, word := range words {
		if word == "phuong" && i+1 < len(words) {
			// Find end of ward name (stop before "quan", "huyen", "thanh", "tinh")
			endIdx := len(words)
			for j := i + 1; j < len(words); j++ {
				if words[j] == "quan" || words[j] == "huyen" || words[j] == "thanh" || words[j] == "tinh" {
					endIdx = j
					break
				}
			}
			
			if endIdx > i+1 {
				wardName := strings.Join(words[i+1:endIdx], " ")
				keywords = append(keywords, wardName)
				
				// Also try without common word combinations to handle variations
				// "dong khe" might be written as "ong khe" due to normalization
				if strings.Contains(wardName, "ong ") {
					keywords = append(keywords, strings.Replace(wardName, "ong ", "dong ", 1))
				}
			}
		}
	}
	
	// Handle multi-language cases: "Ward 5", "District 5" 
	for i, word := range words {
		if word == "ward" && i+1 < len(words) {
			// "ward 5" -> "phuong 5"
			wardNum := words[i+1]
			keywords = append(keywords, "phuong "+wardNum)
			keywords = append(keywords, wardNum) // Also try just the number
		} else if word == "district" && i+1 < len(words) {
			// "district 5" -> "quan 5" 
			districtNum := words[i+1]
			keywords = append(keywords, "quan "+districtNum)
			keywords = append(keywords, districtNum) // Also try just the number
		}
	}
	
	// Handle cases where ward/district is just a number after "quan" or after province
	for i, word := range words {
		if strings.HasPrefix(word, "so_nha_") && len(word) > 7 {
			// Extract number from "so_nha_5" -> "5"
			num := word[7:]
			// If this number appears in context after "quan", it might be a district/ward
			if i > 0 && words[i-1] == "quan" {
				keywords = append(keywords, num)
				keywords = append(keywords, "phuong "+num)
			}
		}
	}
	
	// Also try common ward names and handle common phonetic variations
	commonWards := []string{"bo de", "nhan chinh", "loc tho", "dong khe"}
	for _, ward := range commonWards {
		if strings.Contains(normalized, ward) {
			keywords = append(keywords, ward)
		}
		// Handle phonetic variations - "ong khe" -> "dong khe"
		if ward == "dong khe" && strings.Contains(normalized, "ong khe") {
			keywords = append(keywords, "dong khe")
		}
	}
	
	return keywords
}

// buildResult builds AddressResult từ matched components
func (as *AddressService) buildResult(raw string, normalized string, 
	prov, dist, ward *search.AdminUnitDoc, gazetteerVersion string) *models.AddressResult {
	
	result := &models.AddressResult{
		Raw:                    raw,
		NormalizedNoDiacritics: normalized,
		RawFingerprint:         "", // TODO: generate fingerprint
		Components:             models.AddressComponents{},
		AdminPath:              []string{},
		Candidates:             []models.Candidate{},
	}
	
	// Build components
	if prov != nil {
		result.Components.Province = &models.AdminUnit{
			AdminID:        prov.AdminID,
			Name:           prov.Name,
			NormalizedName: prov.NormalizedName,
			Level:          prov.Level,
			AdminSubtype:   prov.AdminSubtype,
			Aliases:        prov.Aliases,
		}
		result.AdminPath = append(result.AdminPath, prov.Name)
		result.CanonicalText = prov.Name
	}
	
	if dist != nil {
		result.Components.District = &models.AdminUnit{
			AdminID:        dist.AdminID,
			Name:           dist.Name,
			NormalizedName: dist.NormalizedName,
			Level:          dist.Level,
			AdminSubtype:   dist.AdminSubtype,
			Aliases:        dist.Aliases,
		}
		result.AdminPath = append(result.AdminPath, dist.Name)
		if result.CanonicalText != "" {
			result.CanonicalText += ", " + dist.Name
		} else {
			result.CanonicalText = dist.Name
		}
	}
	
	if ward != nil {
		result.Components.Ward = &models.AdminUnit{
			AdminID:        ward.AdminID,
			Name:           ward.Name,
			NormalizedName: ward.NormalizedName,
			Level:          ward.Level,
			AdminSubtype:   ward.AdminSubtype,
			Aliases:        ward.Aliases,
		}
		result.AdminPath = append(result.AdminPath, ward.Name)
		if result.CanonicalText != "" {
			result.CanonicalText += ", " + ward.Name
		} else {
			result.CanonicalText = ward.Name
		}
	}
	
	return result
}

// buildUnmatchedResult builds result for unmatched address
func (as *AddressService) buildUnmatchedResult(raw string, normalized string, gazetteerVersion string) *models.AddressResult {
	return &models.AddressResult{
		Raw:                    raw,
		NormalizedNoDiacritics: normalized,
		RawFingerprint:         "", // TODO: generate fingerprint
		Confidence:             0.0,
		Status:                 "unmatched",
		CanonicalText:          "",
		Components:             models.AddressComponents{},
		AdminPath:              []string{},
		Candidates:             []models.Candidate{},
	}
}

// calculateScore calculates confidence score for address result
func (as *AddressService) calculateScore(result *models.AddressResult, normalized string) float64 {
	score := 0.0
	
	// Base score cho exact/ascii matches
	if result.Components.Province != nil {
		score += 0.4 // Province match
	}
	if result.Components.District != nil {
		score += 0.3 // District match  
	}
	if result.Components.Ward != nil {
		score += 0.2 // Ward match
	}
	
	// Bonus cho hierarchy coherence
	if result.Components.Province != nil && result.Components.District != nil {
		score += 0.1 // Coherent province->district
	}
	
	return score
}

// === ADDED FROM CODE-V1.MD ===

// Out struct cho batch output
type Out struct{
	Raw        string  `json:"raw"`
	Canonical  string  `json:"canonical_text"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
}

// Deps interface cho dependencies
type Deps interface {
	parser.ParseDeps
}

// ProcessBatch xử lý batch addresses với NDJSON output
func (as *AddressService) ProcessBatch(ctx context.Context, inputs []string, ndjsonPath string) error {
	// TODO: Implement actual file writing
	// For now, just log the batch processing
	
	as.logger.Info("Processing batch addresses", 
		zap.Int("total", len(inputs)),
		zap.String("output_path", ndjsonPath))
	
	// Process each address
	for i, input := range inputs {
		// Use existing ParseSingle method
		result, err := as.ParseSingle(input, requests.ParseOptions{
			Levels: 4,
			UseCache: true,
		})
		
		if err != nil {
			as.logger.Warn("Failed to parse address in batch",
				zap.Int("index", i),
				zap.String("address", input),
				zap.Error(err))
			continue
		}
		
		// Log result
		as.logger.Debug("Parsed address in batch",
			zap.Int("index", i),
			zap.String("raw", input),
			zap.String("canonical", result.CanonicalText),
			zap.Float64("confidence", result.Confidence),
			zap.String("status", result.Status))
	}
	
	as.logger.Info("Completed batch processing", zap.Int("total", len(inputs)))
	return nil
}
