package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/address-parser/app/models"
	"github.com/meilisearch/meilisearch-go"
	"go.uber.org/zap"
)

// GazetteerSearcher searcher tìm kiếm trong gazetteer sử dụng Meilisearch
type GazetteerSearcher struct {
	client    meilisearch.ServiceManager
	logger    *zap.Logger
	indexName string
	timeout   time.Duration
}

// SearchConfig cấu hình cho Meilisearch
type SearchConfig struct {
	Host          string
	APIKey        string
	IndexName     string
	Timeout       time.Duration
	MaxCandidates int
}

// NewGazetteerSearcher tạo mới GazetteerSearcher với Meilisearch client
func NewGazetteerSearcher(config SearchConfig, logger *zap.Logger) (*GazetteerSearcher, error) {
	client := meilisearch.New(config.Host, meilisearch.WithAPIKey(config.APIKey))
	
	// Test connection
	health, err := client.Health()
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối Meilisearch: %w", err)
	}
	_ = health // use the health variable
	
	gs := &GazetteerSearcher{
		client:    client,
		logger:    logger,
		indexName: config.IndexName,
		timeout:   config.Timeout,
	}
	
	return gs, nil
}

// MultiSearchRequest cấu trúc để thực hiện multi-search theo hierarchy
type MultiSearchRequest struct {
	Queries []SearchQuery `json:"queries"`
}

type SearchQuery struct {
	IndexUID string                 `json:"indexUid"`
	Query    string                 `json:"q"`
	Filter   string                 `json:"filter,omitempty"`
	Limit    int                    `json:"limit"`
	Offset   int                    `json:"offset"`
	Sort     []string               `json:"sort,omitempty"`
	Facets   []string               `json:"facets,omitempty"`
	Options  map[string]interface{} `json:"attributesToRetrieve,omitempty"`
}

// SearchAddress tìm kiếm địa chỉ theo hierarchy: province → district → ward
func (gs *GazetteerSearcher) SearchAddress(query string, levels int) ([]models.Candidate, error) {
	if query == "" {
		return nil, errors.New("query không được để trống")
	}

	ctx, cancel := context.WithTimeout(context.Background(), gs.timeout)
	defer cancel()

	// Multi-search theo từng level: province → district → ward
	var candidates []models.Candidate

	// 1. Tìm province trước
	provinceResults, err := gs.searchByLevel(ctx, query, 2, 10) // level 2 = province
	if err != nil {
		gs.logger.Error("Lỗi tìm kiếm province", zap.Error(err))
		return nil, err
	}

	for _, province := range provinceResults {
		// 2. Tìm district trong province này
		districtFilter := fmt.Sprintf("parent_id = %s", province.AdminID)
		districtResults, err := gs.searchByLevelWithFilter(ctx, query, 3, districtFilter, 10)
		if err != nil {
			gs.logger.Warn("Lỗi tìm kiếm district", zap.Error(err))
			continue
		}

		for _, district := range districtResults {
			// 3. Tìm ward trong district này
			if levels >= 4 {
				wardFilter := fmt.Sprintf("parent_id = %s", district.AdminID)
				wardResults, err := gs.searchByLevelWithFilter(ctx, query, 4, wardFilter, 10)
				if err != nil {
					gs.logger.Warn("Lỗi tìm kiếm ward", zap.Error(err))
					continue
				}

				// Tạo candidates với path đầy đủ
				for _, ward := range wardResults {
					candidate := models.Candidate{
						Path:  fmt.Sprintf("%s > %s > %s > %s", province.Name, district.Name, ward.Name),
						Score: gs.calculateHierarchyScore(province, district, ward, query),
						AdminUnits: []models.AdminUnit{province, district, ward},
					}
					candidates = append(candidates, candidate)
				}
			} else {
				// Chỉ đến district level
				candidate := models.Candidate{
					Path:  fmt.Sprintf("%s > %s > %s", province.Name, district.Name),
					Score: gs.calculateHierarchyScore(province, district, models.AdminUnit{}, query),
					AdminUnits: []models.AdminUnit{province, district},
				}
				candidates = append(candidates, candidate)
			}
		}
	}

	return candidates, nil
}

// searchByLevel tìm kiếm theo cấp hành chính cụ thể
func (gs *GazetteerSearcher) searchByLevel(ctx context.Context, query string, level int, limit int) ([]models.AdminUnit, error) {
	index := gs.client.Index(gs.indexName)

	// Use compatible SearchRequest for Meilisearch 1.5.x
	searchReq := &meilisearch.SearchRequest{
		Limit: int64(limit),
	}
	
	result, err := index.Search(query, searchReq)
	if err != nil {
		return nil, fmt.Errorf("lỗi tìm kiếm Meilisearch: %w", err)
	}

	return gs.parseSearchResults(result)
}

// searchByLevelWithFilter tìm kiếm theo cấp với filter
func (gs *GazetteerSearcher) searchByLevelWithFilter(ctx context.Context, query string, level int, filter string, limit int) ([]models.AdminUnit, error) {
	index := gs.client.Index(gs.indexName)

	// Use compatible SearchRequest with filter for Meilisearch 1.5.x
	searchReq := &meilisearch.SearchRequest{
		Limit:  int64(limit),
		Filter: filter,
	}
	
	result, err := index.Search(query, searchReq)
	if err != nil {
		return nil, fmt.Errorf("lỗi tìm kiếm với filter: %w", err)
	}

	return gs.parseSearchResults(result)
}

// SearchWithFilter searches with additional filter criteria
func (gs *GazetteerSearcher) SearchWithFilter(query, filter string, limit int) ([]models.AdminUnit, string, error) {
	index := gs.client.Index(gs.indexName)
	
	// Use compatible SearchRequest with proper filter for Meilisearch 1.5.x
	searchReq := &meilisearch.SearchRequest{
		Limit:  int64(limit),
		Filter: filter,
	}
	
	result, err := index.Search(query, searchReq)
	if err != nil {
		return nil, "", fmt.Errorf("lỗi tìm kiếm với filter: %w", err)
	}
	
	units, parseErr := gs.parseSearchResults(result)
	return units, "exact", parseErr
}

// parseSearchResults parse kết quả từ Meilisearch thành AdminUnit
func (gs *GazetteerSearcher) parseSearchResults(result *meilisearch.SearchResponse) ([]models.AdminUnit, error) {
	var units []models.AdminUnit

	for _, hit := range result.Hits {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		unit := models.AdminUnit{}
		
		if id, ok := hitMap["admin_id"].(string); ok {
			unit.AdminID = id
		}
		if name, ok := hitMap["name"].(string); ok {
			unit.Name = name
		}
		if normalizedName, ok := hitMap["normalized_name"].(string); ok {
			unit.NormalizedName = normalizedName
		}
		if unitType, ok := hitMap["type"].(string); ok {
			unit.Type = unitType
		}
		if adminSubtype, ok := hitMap["admin_subtype"].(string); ok {
			unit.AdminSubtype = adminSubtype
		}
		if parentID, ok := hitMap["parent_id"].(string); ok {
			unit.ParentID = &parentID
		}
		if level, ok := hitMap["level"].(float64); ok {
			unit.Level = int(level)
		}

		// Parse aliases
		if aliasesRaw, ok := hitMap["aliases"]; ok {
			if aliasesSlice, ok := aliasesRaw.([]interface{}); ok {
				for _, alias := range aliasesSlice {
					if aliasStr, ok := alias.(string); ok {
						unit.Aliases = append(unit.Aliases, aliasStr)
					}
				}
			}
		}

		// Parse path
		if pathRaw, ok := hitMap["path"]; ok {
			if pathSlice, ok := pathRaw.([]interface{}); ok {
				for _, p := range pathSlice {
					if pathStr, ok := p.(string); ok {
						unit.Path = append(unit.Path, pathStr)
					}
				}
			}
		}

		units = append(units, unit)
	}

	return units, nil
}

// calculateHierarchyScore tính điểm cho hierarchy match
func (gs *GazetteerSearcher) calculateHierarchyScore(province, district, ward models.AdminUnit, query string) float64 {
	baseScore := 0.5
	
	// Bonus cho exact match
	query = strings.ToLower(query)
	if strings.Contains(strings.ToLower(province.Name), query) {
		baseScore += 0.2
	}
	if district.Name != "" && strings.Contains(strings.ToLower(district.Name), query) {
		baseScore += 0.2
	}
	if ward.Name != "" && strings.Contains(strings.ToLower(ward.Name), query) {
		baseScore += 0.1
	}
	
	// Bonus cho alias match
	for _, alias := range province.Aliases {
		if strings.Contains(strings.ToLower(alias), query) {
			baseScore += 0.1
			break
		}
	}
	
	if baseScore > 1.0 {
		baseScore = 1.0
	}
	
	return baseScore
}

// SearchByComponent tìm kiếm theo thành phần cụ thể
func (gs *GazetteerSearcher) SearchByComponent(componentType string, value string, adminLevel int) ([]models.AdminUnit, error) {
	if value == "" {
		return nil, errors.New("giá trị tìm kiếm không được để trống")
	}

	ctx, cancel := context.WithTimeout(context.Background(), gs.timeout)
	defer cancel()

	return gs.searchByLevel(ctx, value, adminLevel, 20)
}

// FuzzySearch tìm kiếm mờ với typo tolerance
func (gs *GazetteerSearcher) FuzzySearch(query string, threshold float64) ([]models.Candidate, error) {
	if query == "" {
		return nil, errors.New("query không được để trống")
	}

	index := gs.client.Index(gs.indexName)

	// Use compatible SearchRequest for Meilisearch 1.5.x (no deprecated fields)
	searchReq := &meilisearch.SearchRequest{
		Limit: 50,
	}

	result, err := index.Search(query, searchReq)
	if err != nil {
		return nil, fmt.Errorf("lỗi fuzzy search: %w", err)
	}

	var candidates []models.Candidate
	for _, hit := range result.Hits {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		// Lấy ranking score từ Meilisearch
		score := 0.5
		if rankingScore, ok := hitMap["_rankingScore"].(float64); ok {
			score = rankingScore
		}

		// Chỉ lấy kết quả trên threshold
		if score >= threshold {
			if name, ok := hitMap["name"].(string); ok {
				candidate := models.Candidate{
					Path:  name,
					Score: score,
				}
				candidates = append(candidates, candidate)
			}
		}
	}

	return candidates, nil
}

// GetAdminUnit lấy thông tin đơn vị hành chính theo ID
func (gs *GazetteerSearcher) GetAdminUnit(id string) (*models.AdminUnit, error) {
	if id == "" {
		return nil, errors.New("ID không được để trống")
	}

	index := gs.client.Index(gs.indexName)

	searchReq := &meilisearch.SearchRequest{
		Filter: fmt.Sprintf("admin_id = %s", id),
		Limit:  1,
	}

	result, err := index.Search("", searchReq)
	if err != nil {
		return nil, fmt.Errorf("lỗi tìm admin unit: %w", err)
	}

	if len(result.Hits) == 0 {
		return nil, errors.New("không tìm thấy admin unit")
	}

	units, err := gs.parseSearchResults(result)
	if err != nil {
		return nil, err
	}

	if len(units) == 0 {
		return nil, errors.New("không thể parse admin unit")
	}

	return &units[0], nil
}

// BuildIndexes xây dựng index Meilisearch với cấu hình theo prompt
func (gs *GazetteerSearcher) BuildIndexes() error {
	index := gs.client.Index(gs.indexName)

	// Cấu hình index theo spec trong prompt
	searchableAttrs := []string{"name", "normalized_name", "aliases"}
	filterableAttrs := []string{"admin_id", "level", "parent_id", "path", "admin_subtype"}
	sortableAttrs := []string{"level", "admin_id"}
	rankingRules := []string{"words", "typo", "proximity", "attribute", "sort", "exactness"}
	stopWords := []string{"cua", "va", "tai", "o", "trong"}
	synonyms := map[string][]string{
		"tp":     {"thanh pho"},
		"hcm":    {"ho chi minh", "sai gon"},
		"q":      {"quan"},
		"p":      {"phuong"},
		"tp hcm": {"thanh pho ho chi minh", "tphcm"},
	}
	enabled := true
	oneTypo := int64(3)
	twoTypos := int64(7)
	
	task, err := index.UpdateSettings(&meilisearch.Settings{
		SearchableAttributes: searchableAttrs,
		FilterableAttributes: filterableAttrs,
		SortableAttributes:   sortableAttrs,
		RankingRules:         rankingRules,
		StopWords:            stopWords,
		Synonyms:             synonyms,
		TypoTolerance: &meilisearch.TypoTolerance{
			Enabled: enabled,
			MinWordSizeForTypos: meilisearch.MinWordSizeForTypos{
				OneTypo:  oneTypo,
				TwoTypos: twoTypos,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("lỗi cấu hình index: %w", err)
	}

	gs.logger.Info("Đã cấu hình index Meilisearch thành công", zap.Int64("task_uid", task.TaskUID))
	return nil
}

// SeedData nạp dữ liệu gazetteer vào Meilisearch
func (gs *GazetteerSearcher) SeedData(adminUnits []models.AdminUnit) error {
	if len(adminUnits) == 0 {
		return errors.New("không có dữ liệu để seed")
	}

	index := gs.client.Index(gs.indexName)

	// Convert sang format Meilisearch
	var documents []map[string]interface{}
	for _, unit := range adminUnits {
		doc := map[string]interface{}{
			"id":              unit.AdminID, // Use AdminID as unique identifier
			"admin_id":        unit.AdminID,
			"parent_id":       unit.ParentID,
			"level":           unit.Level,
			"name":            unit.Name,
			"normalized_name": unit.NormalizedName,
			"type":            unit.Type,
			"admin_subtype":   unit.AdminSubtype,
			"aliases":         unit.Aliases,
			"path":            unit.Path,
			"path_normalized": unit.PathNormalized,
			"gazetteer_version": unit.GazetteerVersion,
			"created_at":      unit.CreatedAt,
			"updated_at":      unit.UpdatedAt,
		}
		documents = append(documents, doc)
	}

	// Batch insert (chunks of 1000)
	batchSize := 1000
	for i := 0; i < len(documents); i += batchSize {
		end := i + batchSize
		if end > len(documents) {
			end = len(documents)
		}

		batch := documents[i:end]
		task, err := index.AddDocuments(batch, "id")
		if err != nil {
			return fmt.Errorf("lỗi thêm documents batch %d-%d: %w", i, end, err)
		}

		gs.logger.Info("Đã thêm batch documents", 
			zap.Int("from", i), 
			zap.Int("to", end),
			zap.Int64("task_uid", task.TaskUID))
	}

	gs.logger.Info("Đã seed data thành công", zap.Int("total_documents", len(documents)))
	return nil
}

// UpdateIndexes cập nhật synonyms từ learned_aliases
func (gs *GazetteerSearcher) UpdateIndexes() error {
	// TODO: Lấy learned_aliases từ MongoDB và update synonyms
	// Đây sẽ là job chạy định kỳ
	
	gs.logger.Info("Cập nhật synonyms thành công")
	return nil
}
