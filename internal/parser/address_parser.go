package parser

import (
	"context"
	"errors"
	"strings"

	"github.com/address-parser/app/models"
	"github.com/address-parser/internal/normalizer"
	"github.com/address-parser/internal/search"
	"go.uber.org/zap"
)

// AddressParser parser địa chỉ chính
type AddressParser struct {
	matcher    *AddressMatcher
	normalizer *normalizer.TextNormalizerV2
	logger     *zap.Logger
}

// NewAddressParser tạo mới AddressParser
func NewAddressParser(searcher *search.GazetteerSearcher, normalizer *normalizer.TextNormalizerV2, logger *zap.Logger) *AddressParser {
	matcher := NewAddressMatcher(searcher, normalizer, logger)
	
	return &AddressParser{
		matcher:    matcher,
		normalizer: normalizer,
		logger:     logger,
	}
}

// ParseAddress parse một địa chỉ với gazetteer version
func (ap *AddressParser) ParseAddress(rawAddress string, gazetteerVersion string) (*models.AddressResult, error) {
	if rawAddress == "" {
		return nil, errors.New("địa chỉ không được để trống")
	}

	// Sử dụng AddressMatcher để thực hiện matching
	matchResult, err := ap.matcher.MatchAddress(rawAddress, gazetteerVersion)
	if err != nil {
		ap.logger.Error("Lỗi matching địa chỉ", zap.Error(err))
		return nil, err
	}

	// Convert MatchResult sang AddressResult
	result := &models.AddressResult{
		Raw:                    matchResult.Raw,
		CanonicalText:          matchResult.CanonicalText,
		NormalizedNoDiacritics: matchResult.NormalizedNoDiacritics,
		Components:             matchResult.Components,
		Quality:                matchResult.Quality,
		Residual:               matchResult.Residual,
		RawFingerprint:         matchResult.RawFingerprint,
		Confidence:             matchResult.Confidence,
		MatchStrategy:          string(matchResult.MatchStrategy),
		AdminPath:              matchResult.AdminPath,
		Candidates:             matchResult.Candidates,
		Status:                 matchResult.Status,
	}

	ap.logger.Debug("Đã parse địa chỉ thành công",
		zap.String("raw", rawAddress),
		zap.Float64("confidence", result.Confidence),
		zap.String("status", result.Status))

	return result, nil
}

// ParseAddresses parse batch địa chỉ
func (ap *AddressParser) ParseAddresses(rawAddresses []string, gazetteerVersion string) ([]*models.AddressResult, error) {
	if len(rawAddresses) == 0 {
		return nil, errors.New("danh sách địa chỉ không được rỗng")
	}

	results := make([]*models.AddressResult, len(rawAddresses))
	
	for i, rawAddress := range rawAddresses {
		result, err := ap.ParseAddress(rawAddress, gazetteerVersion)
		if err != nil {
			ap.logger.Warn("Lỗi parse địa chỉ trong batch",
				zap.Int("index", i),
				zap.String("address", rawAddress),
				zap.Error(err))
			
			// Tạo result lỗi
			results[i] = &models.AddressResult{
				Raw:        rawAddress,
				Status:     models.StatusUnmatched,
				Confidence: 0.0,
			}
		} else {
			results[i] = result
		}
	}

	ap.logger.Info("Đã parse batch địa chỉ",
		zap.Int("total", len(rawAddresses)),
		zap.Int("processed", len(results)))

	return results, nil
}

// === ADDED FROM CODE-V1.MD ===

// ConfidenceParts struct cho confidence calculation
type ConfidenceParts struct {
	Score, Completeness, PathConsistency float64
}

// CalculateConfidence tính confidence từ các parts
func CalculateConfidence(parts ConfidenceParts) float64 {
	// TODO: Import config package để sử dụng weights
	w := struct {
		ScoreWeight, CompletenessWeight, PathWeight float64
	}{
		ScoreWeight: 0.60, CompletenessWeight: 0.25, PathWeight: 0.15,
	}
	return w.ScoreWeight*parts.Score +
		w.CompletenessWeight*parts.Completeness +
		w.PathWeight*parts.PathConsistency
}

// componentCompleteness tính completeness của components
func componentCompleteness(sig normalizer.Signals, gotPath bool) float64 {
	found := 0
	total := 4 // house/street + ward + district + province
	if sig.Road != "" || sig.House != "" {
		found++
	}
	if gotPath {
		found += 3
	}
	return float64(found) / float64(total)
}

// pathConsistency tính consistency của path
func pathConsistency(valid bool) float64 {
	if valid {
		return 1
	}
	return 0
}

// ParseDeps interface cho dependencies
type ParseDeps interface {
	FindCandidates(ctx context.Context, sig normalizer.Signals, norm string) ([]search.CandidatePath, []search.AdminCandidate, []search.AdminCandidate, []search.AdminCandidate, error)
}

// Result struct cho parse result
type Result struct {
	Path        search.CandidatePath
	Score       float64
	Confidence  float64
	MatchLevel  string
	Normalized  string
	Signals     normalizer.Signals
	HasPath     bool
}

// ParseOnce parse một lần
func ParseOnce(ctx context.Context, deps ParseDeps, raw string) (*Result, error) {
	norm, sig := normalizer.Normalize(raw)

	// Libpostal optional
	lpCov := 0.0
	// TODO: Import config và external packages để sử dụng libpostal

	paths, _, _, _, err := deps.FindCandidates(ctx, sig, norm)
	if err != nil {
		return nil, err
	}
	var best *search.CandidatePath
	bestScore := -1.0
	for i := range paths {
		s, _ := ScorePath(norm, paths[i], sig, lpCov)
		if s > bestScore {
			cp := paths[i]
			best = &cp
			bestScore = s
		}
	}

	if best == nil {
		conf := CalculateConfidence(ConfidenceParts{
			Score: 0, Completeness: componentCompleteness(sig, false), PathConsistency: pathConsistency(false),
		})
		return &Result{
			Score:      0,
			Confidence: conf,
			MatchLevel: "fuzzy",
			Normalized: norm,
			Signals:    sig,
			HasPath:    false,
		}, nil
	}

	conf := CalculateConfidence(ConfidenceParts{
		Score: bestScore, Completeness: componentCompleteness(sig, true), PathConsistency: pathConsistency(true),
	})

	return &Result{
		Path:       *best,
		Score:      bestScore,
		Confidence: conf,
		MatchLevel: "fuzzy", // sẽ set lại ở controller khi có canonical_text đầy đủ
		Normalized: norm,
		Signals:    sig,
		HasPath:    true,
	}, nil
}

// Parse parse với context
func Parse(ctx context.Context, deps ParseDeps, raw string) (*Result, error) {
	// request timeout lấy từ config.RequestTimeout() — tạo ở layer trên nếu cần
	return ParseOnce(ctx, deps, raw)
}

// TokenizeAddress chia địa chỉ thành các token
func (ap *AddressParser) TokenizeAddress(address string) []string {
	// Chia theo khoảng trắng và dấu câu
	tokens := strings.FieldsFunc(address, func(r rune) bool {
		return r == ' ' || r == ',' || r == ';' || r == '-'
	})
	
	// Loại bỏ token rỗng
	var result []string
	for _, token := range tokens {
		if strings.TrimSpace(token) != "" {
			result = append(result, strings.TrimSpace(token))
		}
	}
	
	return result
}

// ExtractComponents trích xuất các thành phần địa chỉ
func (ap *AddressParser) ExtractComponents(tokens []string) models.AddressComponents {
	components := models.AddressComponents{}
	
	// TODO: Implement component extraction logic
	// 1. Xác định loại token
	// 2. Phân loại vào component tương ứng
	// 3. Xử lý context và dependencies
	
	return components
}

// CalculateConfidence tính toán độ tin cậy
func (ap *AddressParser) CalculateConfidence(components models.AddressComponents, matchQuality float64) float64 {
	// TODO: Implement confidence calculation
	// 1. Dựa trên chất lượng match
	// 2. Dựa trên completeness của components
	// 3. Dựa trên historical accuracy
	
	return matchQuality
}
