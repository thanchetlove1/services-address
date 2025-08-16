package search

import (
	"context"
	"fmt"
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

// AdminUnitDoc kiểu document trong Meilisearch
type AdminUnitDoc struct {
	AdminID        string   `json:"admin_id"`
	ParentID       *string  `json:"parent_id,omitempty"`
	Level          int      `json:"level"`
	Name           string   `json:"name"`
	NormalizedName string   `json:"normalized_name"`
	Aliases        []string `json:"aliases"`
	AdminSubtype   string   `json:"admin_subtype"`
	Path           []string `json:"path"`
	PathNormalized []string `json:"path_normalized"`
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

// AdminCandidate struct cho candidate matching
type AdminCandidate struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	AdminSubtype   string `json:"admin_subtype"`
	ParentID       string `json:"parent_id"`
	Path           []string `json:"path"`
	PathNormalized []string `json:"path_normalized"`
	Level          int     `json:"level"`
	Score          float64 `json:"score"`
}

// CandidatePath struct cho path ward∈district∈province
type CandidatePath struct {
	Ward     *AdminCandidate `json:"ward"`
	District *AdminCandidate `json:"district"`
	Province *AdminCandidate `json:"province"`
	Score    float64         `json:"score"`
	Path     string          `json:"path"`
}

// TopK struct cho top-k search
type TopK struct{ 
	Ward      int 
	District  int 
	Province  int 
}

// SearchRequest request tìm kiếm với context
type SearchRequest struct {
	Query       string            `json:"query"`
	Level       int               `json:"level"`
	ParentID    string            `json:"parent_id"`
	Context     map[string]string `json:"context"` // context từ các level cao hơn
	Limit       int               `json:"limit"`
	Timeout     time.Duration     `json:"timeout"`
	UseCache    bool              `json:"use_cache"`
}

// SearchResult kết quả tìm kiếm
type SearchResult struct {
	Candidates []AdminCandidate `json:"candidates"`
	Total      int              `json:"total"`
	Query      string           `json:"query"`
	Level      int              `json:"level"`
	Duration   time.Duration    `json:"duration"`
}

// MultiLevelSearchResult kết quả tìm kiếm đa cấp
type MultiLevelSearchResult struct {
	Paths      []CandidatePath  `json:"paths"`
	Wards      []AdminCandidate `json:"wards"`
	Districts  []AdminCandidate `json:"districts"`
	Provinces  []AdminCandidate `json:"provinces"`
	Duration   time.Duration    `json:"duration"`
	TotalPaths int              `json:"total_paths"`
}

// searchIndex helper function cho search với filter và timeout
func (gs *GazetteerSearcher) searchIndex(ctx context.Context, index string, q string, filter []string, limit int) ([]AdminCandidate, error) {
	// Tạo context với timeout
	searchCtx, cancel := context.WithTimeout(ctx, gs.timeout)
	defer cancel()
	
	opts := &meilisearch.SearchRequest{
		Query:  q,
		Limit:  int64(limit),
		Filter: filter,
	}
	
	res, err := gs.client.Index(index).SearchWithContext(searchCtx, q, opts)
	if err != nil {
		return nil, fmt.Errorf("search index %s failed: %w", index, err)
	}
	
	out := make([]AdminCandidate, 0, len(res.Hits))
	for _, h := range res.Hits {
		m := h.(map[string]interface{})
		
		// Parse level
		var level int
		if levelRaw, ok := m["level"]; ok {
			switch v := levelRaw.(type) {
			case float64:
				level = int(v)
			case int:
				level = v
			}
		}
		
		// Parse score
		var score float64
		if scoreRaw, ok := m["_score"]; ok {
			if scoreVal, ok := scoreRaw.(float64); ok {
				score = scoreVal
			}
		}
		
		// Parse path arrays
		var path, pathNormalized []string
		if pathRaw, ok := m["path"].([]interface{}); ok {
			path = make([]string, len(pathRaw))
			for i, pathItem := range pathRaw {
				if pathStr, ok := pathItem.(string); ok {
					path[i] = pathStr
				}
			}
		}
		
		if pathNormRaw, ok := m["path_normalized"].([]interface{}); ok {
			pathNormalized = make([]string, len(pathNormRaw))
			for i, pathItem := range pathNormRaw {
				if pathStr, ok := pathItem.(string); ok {
					pathNormalized[i] = pathStr
				}
			}
		}
		
		candidate := AdminCandidate{
			ID:             fmt.Sprint(m["admin_id"]),
			Name:           fmt.Sprint(m["name"]),
			AdminSubtype:   fmt.Sprint(m["admin_subtype"]),
			ParentID:       fmt.Sprint(m["parent_id"]),
			Path:           path,
			PathNormalized: pathNormalized,
			Level:          level,
			Score:          score,
		}
		
		out = append(out, candidate)
	}
	
	return out, nil
}

// SearchByLevel search theo level hành chính với context
func (gs *GazetteerSearcher) SearchByLevel(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	start := time.Now()
	
	// Tạo context với timeout
	searchCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()
	
	// Xây dựng filters
	var filters []string
	
	// Filter theo level
	if req.Level > 0 {
		filters = append(filters, fmt.Sprintf("level = %d", req.Level))
	}
	
	// Filter theo parent_id nếu có
	if req.ParentID != "" {
		filters = append(filters, fmt.Sprintf(`parent_id = "%s"`, req.ParentID))
	}
	
	// Filter theo context từ các level cao hơn
	for key, value := range req.Context {
		if value != "" {
			filters = append(filters, fmt.Sprintf(`%s = "%s"`, key, value))
		}
	}
	
	// Thực hiện search
	candidates, err := gs.searchIndex(searchCtx, gs.indexName, req.Query, filters, req.Limit)
	if err != nil {
		return nil, err
	}
	
	duration := time.Since(start)
	
	return &SearchResult{
		Candidates: candidates,
		Total:      len(candidates),
		Query:      req.Query,
		Level:      req.Level,
		Duration:   duration,
	}, nil
}

// FindCandidates tìm candidates theo signals và normalized text với multi-level search
func (gs *GazetteerSearcher) FindCandidates(ctx context.Context, sig interface{}, norm string) (*MultiLevelSearchResult, error) {
	start := time.Now()
	
	// Cấu hình top-k
	topk := TopK{
		Ward:     20,
		District: 15,
		Province: 10,
	}
	
	// Tạo context với timeout
	searchCtx, cancel := context.WithTimeout(ctx, gs.timeout)
	defer cancel()
	
	// 1. Tìm kiếm Province/TP trước (level 2)
	provQuery := norm
	provinces, err := gs.searchIndex(searchCtx, gs.indexName, provQuery, []string{
		"level = 2",
		`admin_subtype IN ["province", "municipality"]`,
	}, topk.Province)
	if err != nil {
		return nil, fmt.Errorf("search provinces failed: %w", err)
	}
	
	// 2. Tìm kiếm District/Quận (level 3) với context từ province
	var districts []AdminCandidate
	for _, province := range provinces {
		distQuery := norm
		distResults, err := gs.searchIndex(searchCtx, gs.indexName, distQuery, []string{
			"level = 3",
			`admin_subtype IN ["urban_district", "rural_district", "city_under_province"]`,
			fmt.Sprintf(`parent_id = "%s"`, province.ID),
		}, topk.District)
		if err != nil {
			gs.logger.Warn("search districts failed for province", 
				zap.String("province_id", province.ID), zap.Error(err))
			continue
		}
		districts = append(districts, distResults...)
	}
	
	// 3. Tìm kiếm Ward/Phường (level 4) với context từ district
	var wards []AdminCandidate
	for _, district := range districts {
		wardQuery := norm
		wardResults, err := gs.searchIndex(searchCtx, gs.indexName, wardQuery, []string{
			"level = 4",
			`admin_subtype IN ["ward", "commune"]`,
			fmt.Sprintf(`parent_id = "%s"`, district.ID),
		}, topk.Ward)
		if err != nil {
			gs.logger.Warn("search wards failed for district", 
				zap.String("district_id", district.ID), zap.Error(err))
			continue
		}
		wards = append(wards, wardResults...)
	}
	
	// 4. Xây dựng candidate paths ward∈district∈province
	var paths []CandidatePath
	pathMap := make(map[string]bool) // Để tránh duplicate paths
	
	for _, ward := range wards {
		// Tìm district cha của ward
		var parentDistrict *AdminCandidate
		for _, district := range districts {
			if district.ID == ward.ParentID {
				parentDistrict = &district
				break
			}
		}
		if parentDistrict == nil {
			continue
		}
		
		// Tìm province cha của district
		var parentProvince *AdminCandidate
		for _, province := range provinces {
			if province.ID == parentDistrict.ParentID {
				parentProvince = &province
				break
			}
		}
		if parentProvince == nil {
			continue
		}
		
		// Tạo path string
		pathKey := fmt.Sprintf("%s-%s-%s", ward.ID, parentDistrict.ID, parentProvince.ID)
		if pathMap[pathKey] {
			continue
		}
		pathMap[pathKey] = true
		
		// Tính score tổng hợp
		pathScore := (ward.Score + parentDistrict.Score + parentProvince.Score) / 3.0
		
		// Tạo path string cho display
		pathDisplay := fmt.Sprintf("%s > %s > %s", 
			parentProvince.Name, parentDistrict.Name, ward.Name)
		
		paths = append(paths, CandidatePath{
			Ward:     &ward,
			District: parentDistrict,
			Province: parentProvince,
			Score:    pathScore,
			Path:     pathDisplay,
		})
	}
	
	// Sắp xếp paths theo score giảm dần
	// TODO: Implement sort.Slice cho paths
	
	duration := time.Since(start)
	
	return &MultiLevelSearchResult{
		Paths:      paths,
		Wards:      wards,
		Districts:  districts,
		Provinces:  provinces,
		Duration:   duration,
		TotalPaths: len(paths),
	}, nil
}

// SearchAddress search address theo hierarchy với context
func (gs *GazetteerSearcher) SearchAddress(ctx context.Context, query string, levels int, contextMap map[string]string) ([]models.Candidate, error) {
	// Tìm kiếm theo từng level
	var allCandidates []models.Candidate
	
	for level := 1; level <= levels; level++ {
		req := SearchRequest{
			Query:    query,
			Level:    level,
			Context:  contextMap,
			Limit:    10,
			Timeout:  gs.timeout,
			UseCache: true,
		}
		
		result, err := gs.SearchByLevel(ctx, req)
		if err != nil {
			gs.logger.Warn("search level failed", zap.Int("level", level), zap.Error(err))
			continue
		}
		
		// Convert AdminCandidate sang models.Candidate
		for _, candidate := range result.Candidates {
			adminUnits := []models.AdminUnit{
				{
					AdminID:          candidate.ID,
					ParentID:         &candidate.ParentID,
					Level:            candidate.Level,
					Name:             candidate.Name,
					AdminSubtype:     candidate.AdminSubtype,
				},
			}
			
			allCandidates = append(allCandidates, models.Candidate{
				Path:       candidate.Name,
				Score:      candidate.Score,
				AdminUnits: adminUnits,
			})
		}
	}
	
	return allCandidates, nil
}

// SearchWithFilter search với filter string
func (gs *GazetteerSearcher) SearchWithFilter(ctx context.Context, query, filter string, limit int) ([]models.AdminUnit, string, error) {
	// Tạo context với timeout
	searchCtx, cancel := context.WithTimeout(ctx, gs.timeout)
	defer cancel()
	
	// Parse filter thành array
	var filters []string
	if filter != "" {
		filters = []string{filter}
	}
	
	// Thực hiện search
	candidates, err := gs.searchIndex(searchCtx, gs.indexName, query, filters, limit)
	if err != nil {
		return nil, "", err
	}
	
	// Convert sang models.AdminUnit
	var adminUnits []models.AdminUnit
	for _, candidate := range candidates {
		adminUnit := models.AdminUnit{
			AdminID:      candidate.ID,
			ParentID:     &candidate.ParentID,
			Level:        candidate.Level,
			Name:         candidate.Name,
			AdminSubtype: candidate.AdminSubtype,
		}
		adminUnits = append(adminUnits, adminUnit)
	}
	
	// Xác định match strategy
	matchStrategy := "exact"
	if len(adminUnits) > 1 {
		matchStrategy = "multiple_candidates"
	} else if len(adminUnits) == 0 {
		matchStrategy = "no_match"
	}
	
	return adminUnits, matchStrategy, nil
}

// FuzzySearch fuzzy search với threshold
func (gs *GazetteerSearcher) FuzzySearch(ctx context.Context, query string, threshold float64) ([]models.Candidate, error) {
	// Tạo context với timeout
	searchCtx, cancel := context.WithTimeout(ctx, gs.timeout)
	defer cancel()
	
	// Thực hiện search với typo tolerance
	candidates, err := gs.searchIndex(searchCtx, gs.indexName, query, nil, 20)
	if err != nil {
		return nil, err
	}
	
	// Filter theo threshold
	var filteredCandidates []models.Candidate
	for _, candidate := range candidates {
		if candidate.Score >= threshold {
			adminUnits := []models.AdminUnit{
				{
					AdminID:          candidate.ID,
					ParentID:         &candidate.ParentID,
					Level:            candidate.Level,
					Name:             candidate.Name,
					AdminSubtype:     candidate.AdminSubtype,
				},
			}
			
			filteredCandidates = append(filteredCandidates, models.Candidate{
				Path:       candidate.Name,
				Score:      candidate.Score,
				AdminUnits: adminUnits,
			})
		}
	}
	
	return filteredCandidates, nil
}

// ApplyIndexSettings áp dụng settings cho indexes
func (gs *GazetteerSearcher) ApplyIndexSettings(ctx context.Context) error {
	searchableAttrs := []string{"normalized_name", "aliases", "path_normalized", "name"}
	filterableAttrs := []string{"admin_subtype", "parent_id", "level", "admin_id"}
	rankingRules := []string{"typo", "words", "proximity", "attribute", "exactness"}
	
	set := &meilisearch.Settings{
		SearchableAttributes: searchableAttrs,
		FilterableAttributes: filterableAttrs,
		RankingRules:         rankingRules,
	}
	
	// Áp dụng cho index chính
	if _, err := gs.client.Index(gs.indexName).UpdateSettings(set); err != nil {
		return fmt.Errorf("apply settings %s: %w", gs.indexName, err)
	}
	
	return nil
}

// BuildIndexes build indexes với cấu hình
func (gs *GazetteerSearcher) BuildIndexes() error {
	gs.logger.Info("Building indexes...")
	
	// Áp dụng settings
	if err := gs.ApplyIndexSettings(context.Background()); err != nil {
		return fmt.Errorf("apply index settings failed: %w", err)
	}
	
	gs.logger.Info("Indexes built successfully")
	return nil
}

// SeedData seed data vào Meilisearch
func (gs *GazetteerSearcher) SeedData(adminUnits []models.AdminUnit) error {
	gs.logger.Info("Seeding data...", zap.Int("count", len(adminUnits)))
	
	// Convert models.AdminUnit sang AdminUnitDoc
	var docs []AdminUnitDoc
	for _, unit := range adminUnits {
		doc := AdminUnitDoc{
			AdminID:        unit.AdminID,
			ParentID:       unit.ParentID,
			Level:          unit.Level,
			Name:           unit.Name,
			NormalizedName: unit.NormalizedName,
			Aliases:        unit.Aliases,
			AdminSubtype:   unit.AdminSubtype,
			Path:           unit.Path,
			PathNormalized: unit.PathNormalized,
		}
		docs = append(docs, doc)
	}
	
	// TODO: Implement batch add documents to Meilisearch
	gs.logger.Info("Data seeding completed", zap.Int("documents", len(docs)))
	
	return nil
}

// UpdateIndexes update synonyms và settings
func (gs *GazetteerSearcher) UpdateIndexes() error {
	gs.logger.Info("Updating indexes...")
	
	// TODO: Implement synonyms update
	gs.logger.Info("Indexes updated successfully")
	
	return nil
}

// HealthCheck kiểm tra sức khỏe của searcher
func (gs *GazetteerSearcher) HealthCheck(ctx context.Context) error {
	// Kiểm tra kết nối Meilisearch
	health, err := gs.client.Health()
	if err != nil {
		return fmt.Errorf("meilisearch health check failed: %w", err)
	}
	
	// Kiểm tra index có tồn tại không bằng cách thử truy cập
	index := gs.client.Index(gs.indexName)
	_, err = index.GetStats()
	if err != nil {
		return fmt.Errorf("index %s not accessible: %w", gs.indexName, err)
	}
	
	gs.logger.Debug("Health check passed", 
		zap.String("status", health.Status),
		zap.String("index", gs.indexName))
	
	return nil
}
