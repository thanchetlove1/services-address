package search

import (
	"context"
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

// === ADDED FROM CODE-V1.MD ===

// AdminCandidate struct cho candidate matching
type AdminCandidate struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	AdminSubtype   string `json:"admin_subtype"`
	ParentID       string `json:"parent_id"`
	Path           string `json:"path"`
	PathNormalized string `json:"path_normalized"`
}

// CandidatePath struct cho path ward∈district∈province
type CandidatePath struct {
	Ward, District, Province *AdminCandidate
}

// TopK struct cho top-k search
type TopK struct{ Ward, District, Province int }

// searchIndex helper function cho search với filter
func (gs *GazetteerSearcher) searchIndex(ctx context.Context, index string, q string, filter []string, limit int) ([]AdminCandidate, error) {
	opts := &meilisearch.SearchRequest{
		Query:  q,
		Limit:  int64(limit),
		Filter: filter,
	}
	res, err := gs.client.Index(index).Search(q, opts)
	if err != nil {
		return nil, err
	}
	out := make([]AdminCandidate, 0, len(res.Hits))
	for _, h := range res.Hits {
		m := h.(map[string]interface{})
		out = append(out, AdminCandidate{
			ID:             fmt.Sprint(m["id"]),
			Name:           fmt.Sprint(m["name"]),
			AdminSubtype:   fmt.Sprint(m["admin_subtype"]),
			ParentID:       fmt.Sprint(m["parent_id"]),
			Path:           fmt.Sprint(m["path"]),
			PathNormalized: fmt.Sprint(m["path_normalized"]),
		})
	}
	return out, nil
}

// FindCandidates tìm candidates theo signals và normalized text
func (gs *GazetteerSearcher) FindCandidates(ctx context.Context, sig interface{}, norm string) ([]CandidatePath, []AdminCandidate, []AdminCandidate, []AdminCandidate, error) {
	// Hardcode topk tạm thời
	topk := TopK{
		Ward:     10,
		District: 10,
		Province: 5,
	}

	// Convert sig to string for road extraction
	sigStr := ""
	if sig != nil {
		// TODO: Type assertion để lấy road từ signals
	}

	qWard := strings.TrimSpace(sigStr + " " + norm)
	wards, err := gs.searchIndex(ctx, "wards", qWard, []string{`admin_subtype = "ward"`}, topk.Ward)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	qDist := norm
	dists, err := gs.searchIndex(ctx, "districts", qDist, []string{`admin_subtype IN ["urban_district","rural_district","city_under_province"]`}, topk.District)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	qProv := norm
	provs, err := gs.searchIndex(ctx, "provinces", qProv, []string{`admin_subtype IN ["province","municipality"]`}, topk.Province)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// build paths ward∈district∈province
	var paths []CandidatePath
	for _, w := range wards {
		for _, d := range dists {
			if w.ParentID != d.ID {
				continue
			}
			for _, p := range provs {
				if d.ParentID != p.ID {
					continue
				}
				wc, dc, pc := w, d, p
				paths = append(paths, CandidatePath{
					Ward:     &wc,
					District: &dc,
					Province: &pc,
				})
			}
		}
	}

	return paths, wards, dists, provs, nil
}

// ApplyIndexSettings áp dụng settings cho indexes
func (gs *GazetteerSearcher) ApplyIndexSettings(ctx context.Context) error {
	searchableAttrs := []string{"normalized_name", "aliases", "path_normalized"}
	filterableAttrs := []string{"admin_subtype", "parent_id"}
	rankingRules := []string{"typo", "words", "proximity", "attribute", "exactness"}
	
	set := &meilisearch.Settings{
		SearchableAttributes: searchableAttrs,
		FilterableAttributes: filterableAttrs,
		RankingRules:         rankingRules,
	}
	for _, idx := range []string{"wards", "districts", "provinces"} {
		if _, err := gs.client.Index(idx).UpdateSettings(set); err != nil {
			return fmt.Errorf("apply settings %s: %w", idx, err)
		}
	}
	return nil
}

// === ADDITIONAL METHODS FOR ADDRESS_MATCHER.GO ===

// SearchByLevel search by administrative level
func (gs *GazetteerSearcher) SearchByLevel(ctx context.Context, q string, level int, parentID string, limit int) ([]AdminUnitDoc, error) {
	var filters []string
	if level > 0 {
		filters = append(filters, fmt.Sprintf("level = %d", level))
	}
	if parentID != "" {
		filters = append(filters, fmt.Sprintf(`parent_id = "%s"`, parentID))
	}
	
	// Use existing search method
	return gs.search(ctx, q, filters, limit)
}

// SearchAddress search address by hierarchy
func (gs *GazetteerSearcher) SearchAddress(query string, levels int) ([]models.Candidate, error) {
	// Placeholder implementation
	return []models.Candidate{}, nil
}

// SearchWithFilter search with filter
func (gs *GazetteerSearcher) SearchWithFilter(query, filter string, limit int) ([]models.AdminUnit, string, error) {
	// Placeholder implementation
	return []models.AdminUnit{}, "exact", nil
}

// FuzzySearch fuzzy search
func (gs *GazetteerSearcher) FuzzySearch(query string, threshold float64) ([]models.Candidate, error) {
	// Placeholder implementation
	return []models.Candidate{}, nil
}

// search method for SearchByLevel
func (gs *GazetteerSearcher) search(ctx context.Context, q string, filters []string, limit int) ([]AdminUnitDoc, error) {
	idx := gs.client.Index(gs.indexName)
	
	var filterStr string
	if len(filters) > 0 {
		filterStr = strings.Join(filters, " AND ")
	}
	
	req := &meilisearch.SearchRequest{
		Query:  q,
		Limit:  int64(limit),
		Filter: filterStr,
	}
	resp, err := idx.Search(q, req)
	if err != nil {
		return nil, err
	}
	
	var out []AdminUnitDoc
	for _, hit := range resp.Hits {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}
		
		doc := AdminUnitDoc{}
		if adminID, ok := hitMap["admin_id"].(string); ok {
			doc.AdminID = adminID
		}
		if name, ok := hitMap["name"].(string); ok {
			doc.Name = name
		}
		if normalizedName, ok := hitMap["normalized_name"].(string); ok {
			doc.NormalizedName = normalizedName
		}
		if level, ok := hitMap["level"].(float64); ok {
			doc.Level = int(level)
		}
		if adminSubtype, ok := hitMap["admin_subtype"].(string); ok {
			doc.AdminSubtype = adminSubtype
		}
		
		if parentID, ok := hitMap["parent_id"].(string); ok && parentID != "" {
			doc.ParentID = &parentID
		}
		
		if aliasesRaw, ok := hitMap["aliases"].([]interface{}); ok {
			aliases := make([]string, len(aliasesRaw))
			for i, alias := range aliasesRaw {
				if aliasStr, ok := alias.(string); ok {
					aliases[i] = aliasStr
				}
			}
			doc.Aliases = aliases
		}
		
		if pathRaw, ok := hitMap["path"].([]interface{}); ok {
			path := make([]string, len(pathRaw))
			for i, pathItem := range pathRaw {
				if pathStr, ok := pathItem.(string); ok {
					path[i] = pathStr
				}
			}
			doc.Path = path
		}
		
		out = append(out, doc)
	}
	
	return out, nil
}

// === ADDITIONAL METHODS FOR ADMIN_SERVICE.GO ===

// BuildIndexes build indexes với cấu hình
func (gs *GazetteerSearcher) BuildIndexes() error {
	// Placeholder implementation
	gs.logger.Info("Building indexes...")
	return nil
}

// SeedData seed data vào Meilisearch
func (gs *GazetteerSearcher) SeedData(adminUnits []models.AdminUnit) error {
	// Placeholder implementation
	gs.logger.Info("Seeding data...", zap.Int("count", len(adminUnits)))
	return nil
}

// UpdateIndexes update synonyms và settings
func (gs *GazetteerSearcher) UpdateIndexes() error {
	// Placeholder implementation
	gs.logger.Info("Updating indexes...")
	return nil
}
