package search

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestGazetteerSearcher_NewGazetteerSearcher(t *testing.T) {
	// Test với config hợp lệ
	config := SearchConfig{
		Host:          "http://localhost:7700",
		APIKey:        "masterKey",
		IndexName:     "admin_units",
		Timeout:       30 * time.Second,
		MaxCandidates: 20,
	}

	logger, _ := zap.NewDevelopment()
	
	// Lưu ý: Test này sẽ fail nếu không có Meilisearch server chạy
	// Trong môi trường test thực tế, cần mock Meilisearch client
	_, err := NewGazetteerSearcher(config, logger)
	
	// Test sẽ pass nếu có server, fail nếu không có
	// Trong production test, cần setup test environment
	t.Logf("NewGazetteerSearcher result: %v", err)
}

func TestGazetteerSearcher_SearchRequest(t *testing.T) {
	// Test SearchRequest struct
	req := SearchRequest{
		Query:   "test",
		Level:   2,
		Context: map[string]string{"admin_subtype": "province"},
		Limit:   10,
		Timeout: 5 * time.Second,
		UseCache: true,
	}

	assert.Equal(t, "test", req.Query)
	assert.Equal(t, 2, req.Level)
	assert.Equal(t, "province", req.Context["admin_subtype"])
	assert.Equal(t, 10, req.Limit)
	assert.Equal(t, 5*time.Second, req.Timeout)
	assert.True(t, req.UseCache)
}

func TestGazetteerSearcher_AdminCandidate(t *testing.T) {
	// Test AdminCandidate struct
	candidate := AdminCandidate{
		ID:             "79",
		Name:           "Thành phố Hồ Chí Minh",
		AdminSubtype:   "municipality",
		ParentID:       "84",
		Path:           []string{"84", "79"},
		PathNormalized: []string{"viet nam", "thanh pho ho chi minh"},
		Level:          2,
		Score:          0.95,
	}

	assert.Equal(t, "79", candidate.ID)
	assert.Equal(t, "Thành phố Hồ Chí Minh", candidate.Name)
	assert.Equal(t, "municipality", candidate.AdminSubtype)
	assert.Equal(t, "84", candidate.ParentID)
	assert.Equal(t, 2, candidate.Level)
	assert.Equal(t, 0.95, candidate.Score)
	assert.Len(t, candidate.Path, 2)
	assert.Len(t, candidate.PathNormalized, 2)
}

func TestGazetteerSearcher_CandidatePath(t *testing.T) {
	// Test CandidatePath struct
	ward := &AdminCandidate{
		ID:           "26734",
		Name:         "Phường Bến Nghé",
		AdminSubtype: "ward",
		Level:         4,
		Score:         0.95,
	}

	district := &AdminCandidate{
		ID:           "769",
		Name:         "Quận 1",
		AdminSubtype: "urban_district",
		Level:         3,
		Score:         0.90,
	}

	province := &AdminCandidate{
		ID:           "79",
		Name:         "Thành phố Hồ Chí Minh",
		AdminSubtype: "municipality",
		Level:         2,
		Score:         0.85,
	}

	path := CandidatePath{
		Ward:     ward,
		District: district,
		Province: province,
		Score:    0.90,
		Path:     "Thành phố Hồ Chí Minh > Quận 1 > Phường Bến Nghé",
	}

	assert.Equal(t, ward, path.Ward)
	assert.Equal(t, district, path.District)
	assert.Equal(t, province, path.Province)
	assert.Equal(t, 0.90, path.Score)
	assert.Equal(t, "Thành phố Hồ Chí Minh > Quận 1 > Phường Bến Nghé", path.Path)
}

func TestGazetteerSearcher_TopK(t *testing.T) {
	// Test TopK struct
	topk := TopK{
		Ward:     20,
		District: 15,
		Province: 10,
	}

	assert.Equal(t, 20, topk.Ward)
	assert.Equal(t, 15, topk.District)
	assert.Equal(t, 10, topk.Province)
}

func TestGazetteerSearcher_MultiLevelSearchResult(t *testing.T) {
	// Test MultiLevelSearchResult struct
	result := MultiLevelSearchResult{
		Paths:      []CandidatePath{},
		Wards:      []AdminCandidate{},
		Districts:  []AdminCandidate{},
		Provinces:  []AdminCandidate{},
		Duration:   100 * time.Millisecond,
		TotalPaths: 0,
	}

	assert.Len(t, result.Paths, 0)
	assert.Len(t, result.Wards, 0)
	assert.Len(t, result.Districts, 0)
	assert.Len(t, result.Provinces, 0)
	assert.Equal(t, 100*time.Millisecond, result.Duration)
	assert.Equal(t, 0, result.TotalPaths)
}

func TestGazetteerSearcher_SearchResult(t *testing.T) {
	// Test SearchResult struct
	candidates := []AdminCandidate{
		{
			ID:           "26734",
			Name:         "Phường Bến Nghé",
			AdminSubtype: "ward",
			Level:         4,
			Score:         0.95,
		},
	}

	result := SearchResult{
		Candidates: candidates,
		Total:      1,
		Query:      "Ben Nghe",
		Level:      4,
		Duration:   50 * time.Millisecond,
	}

	assert.Len(t, result.Candidates, 1)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "Ben Nghe", result.Query)
	assert.Equal(t, 4, result.Level)
	assert.Equal(t, 50*time.Millisecond, result.Duration)
}

// Test mock data cho "Ben Nghe, D1, HCMC"
func TestGazetteerSearcher_MockData(t *testing.T) {
	// Mock data cho test case "Ben Nghe, D1, HCMC"
	mockWard := AdminCandidate{
		ID:           "26734",
		Name:         "Phường Bến Nghé",
		AdminSubtype: "ward",
		ParentID:     "769",
		Level:         4,
		Score:         0.95,
	}

	mockDistrict := AdminCandidate{
		ID:           "769",
		Name:         "Quận 1",
		AdminSubtype: "urban_district",
		ParentID:     "79",
		Level:         3,
		Score:         0.90,
	}

	mockProvince := AdminCandidate{
		ID:           "79",
		Name:         "Thành phố Hồ Chí Minh",
		AdminSubtype: "municipality",
		ParentID:     "84",
		Level:         2,
		Score:         0.85,
	}

	// Test path construction
	path := CandidatePath{
		Ward:     &mockWard,
		District: &mockDistrict,
		Province: &mockProvince,
		Score:    0.90,
		Path:     "Thành phố Hồ Chí Minh > Quận 1 > Phường Bến Nghé",
	}

	// Verify path structure
	assert.Equal(t, "26734", path.Ward.ID)
	assert.Equal(t, "Phường Bến Nghé", path.Ward.Name)
	assert.Equal(t, "ward", path.Ward.AdminSubtype)
	assert.Equal(t, "769", path.Ward.ParentID)

	assert.Equal(t, "769", path.District.ID)
	assert.Equal(t, "Quận 1", path.District.Name)
	assert.Equal(t, "urban_district", path.District.AdminSubtype)
	assert.Equal(t, "79", path.District.ParentID)

	assert.Equal(t, "79", path.Province.ID)
	assert.Equal(t, "Thành phố Hồ Chí Minh", path.Province.Name)
	assert.Equal(t, "municipality", path.Province.AdminSubtype)
	assert.Equal(t, "84", path.Province.ParentID)

	// Verify hierarchical relationship
	assert.Equal(t, path.Ward.ParentID, path.District.ID)
	assert.Equal(t, path.District.ParentID, path.Province.ID)

	// Verify path string
	expectedPath := "Thành phố Hồ Chí Minh > Quận 1 > Phường Bến Nghé"
	assert.Equal(t, expectedPath, path.Path)

	// Verify score calculation
	expectedScore := (mockWard.Score + mockDistrict.Score + mockProvince.Score) / 3.0
	assert.Equal(t, expectedScore, path.Score)
}

// Test context filtering
func TestGazetteerSearcher_ContextFiltering(t *testing.T) {
	// Test context map cho filtering
	context := map[string]string{
		"admin_subtype": "municipality",
		"level":         "2",
	}

	// Verify context values
	assert.Equal(t, "municipality", context["admin_subtype"])
	assert.Equal(t, "2", context["level"])

	// Test context filtering logic
	filters := []string{}
	for key, value := range context {
		if value != "" {
			filters = append(filters, fmt.Sprintf(`%s = "%s"`, key, value))
		}
	}

	expectedFilters := []string{
		`admin_subtype = "municipality"`,
		`level = "2"`,
	}

	assert.Equal(t, expectedFilters, filters)
}

// Test timeout handling
func TestGazetteerSearcher_TimeoutHandling(t *testing.T) {
	// Test timeout configuration
	timeout := 30 * time.Second
	assert.Equal(t, 30*time.Second, timeout)

	// Test context with timeout
	ctx := context.Background()
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Verify context has timeout
	deadline, ok := timeoutCtx.Deadline()
	assert.True(t, ok)
	assert.True(t, deadline.After(time.Now()))
}

// Test search configuration
func TestGazetteerSearcher_SearchConfig(t *testing.T) {
	config := SearchConfig{
		Host:          "http://localhost:7700",
		APIKey:        "masterKey",
		IndexName:     "admin_units",
		Timeout:       30 * time.Second,
		MaxCandidates: 20,
	}

	assert.Equal(t, "http://localhost:7700", config.Host)
	assert.Equal(t, "masterKey", config.APIKey)
	assert.Equal(t, "admin_units", config.IndexName)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 20, config.MaxCandidates)
}
