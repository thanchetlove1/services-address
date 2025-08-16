// SPDX-License-Identifier: MIT
package parser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/address-parser/app/config"
	"github.com/address-parser/app/models"
	"github.com/address-parser/internal/normalizer"
	"github.com/address-parser/internal/search"
	"github.com/agnivade/levenshtein"
	"github.com/mozillazg/go-unidecode"
	"github.com/xrash/smetrics"
	"go.uber.org/zap"
)

/* ================= Enums ================= */

type MatchStrategy string

const (
	MatchStrategyExact MatchStrategy = "exact"
	MatchStrategyAscii MatchStrategy = "ascii_exact"
	MatchStrategyAlias MatchStrategy = "alias"
	MatchStrategyFuzzy MatchStrategy = "fuzzy"
)

type MatchLevel string

const (
	MatchLevelExact MatchLevel = "exact"
	MatchLevelAscii MatchLevel = "ascii_exact"
	MatchLevelFuzzy MatchLevel = "fuzzy"
)

type QualityFlag string

const (
	FlagExactMatch         QualityFlag = "EXACT_MATCH"
	FlagAsciiExact         QualityFlag = "ASCII_EXACT"
	FlagFuzzyMatch         QualityFlag = "FUZZY_MATCH"
	FlagPOIExtracted       QualityFlag = "POI_EXTRACTED"
	FlagApartmentUnit      QualityFlag = "APARTMENT_UNIT"
	FlagMultiLanguage      QualityFlag = "MULTI_LANGUAGE"
	FlagAmbiguousWard      QualityFlag = "AMBIGUOUS_WARD"
	FlagMultipleCandidates QualityFlag = "MULTIPLE_CANDIDATES"
	FlagLowConfidence      QualityFlag = "LOW_CONFIDENCE"
	FlagMissingWard        QualityFlag = "MISSING_WARD"
)

/* ================ Matcher ================ */

type AddressMatcher struct {
	searcher   *search.GazetteerSearcher
	normalizer *normalizer.TextNormalizerV2
	extractor  *normalizer.PatternExtractor
	logger     *zap.Logger

	thresholdHigh   float64 // >= matched
	thresholdMedium float64 // >= needs_review
	maxCandidates   int
}

func NewAddressMatcher(searcher *search.GazetteerSearcher, textNormalizer *normalizer.TextNormalizerV2, logger *zap.Logger) *AddressMatcher {
	thHigh := 0.90
	thMed := 0.60
	if config.C.Thresholds.High > 0 {
		thHigh = config.C.Thresholds.High
	}
	if config.C.Thresholds.ReviewLow > 0 {
		thMed = config.C.Thresholds.ReviewLow
	}
	return &AddressMatcher{
		searcher:        searcher,
		normalizer:      textNormalizer,
		extractor:       normalizer.NewPatternExtractor(),
		logger:          logger,
		thresholdHigh:   thHigh,
		thresholdMedium: thMed,
		maxCandidates:   20,
	}
}

/* ================ Result ================ */

type MatchResult struct {
	Raw                    string                   `json:"raw"`
	CanonicalText          string                   `json:"canonical_text"`
	NormalizedNoDiacritics string                   `json:"normalized_no_diacritics"`
	Components             models.AddressComponents `json:"components"`
	Quality                models.QualityMetrics    `json:"quality"`
	Residual               string                   `json:"residual"`
	RawFingerprint         string                   `json:"raw_fingerprint"`
	Confidence             float64                  `json:"confidence"`
	MatchStrategy          MatchStrategy            `json:"match_strategy"`
	AdminPath              []string                 `json:"admin_path"`
	Candidates             []models.Candidate       `json:"candidates"`
	Status                 string                   `json:"status"` // matched | needs_review | unmatched
}

/* ================ Public ================ */

func (am *AddressMatcher) MatchAddress(rawAddress string, gazetteerVersion string) (*MatchResult, error) {
	start := time.Now()

	// 1) Normalize + patterns
	nr := am.normalizer.NormalizeAddress(rawAddress) // *NormalizationResult
	normalized := nr.NormalizedNoDiacritics
	patterns := am.extractor.ExtractPatterns(normalized)
	sig := am.signalsFromPatterns(patterns)

	// 2) Fingerprint
	fingerprint := am.generateFingerprint(normalized, gazetteerVersion)

	// 3) Candidate paths ward∈district∈province
	ctx := context.Background()
	paths, err := am.buildCandidatePaths(ctx, normalized)
	if err != nil {
		am.logger.Warn("buildCandidatePaths failed", zap.Error(err))
	}

	// 4) Score & rank
	cands := make([]models.Candidate, 0, len(paths))
	bestScore := -1.0
	for _, p := range paths {
		score, _ := am.scorePath(normalized, p, sig, 0 /* lpCov */)
		c := models.Candidate{
			AdminUnits: []models.AdminUnit{p.Ward, p.District, p.Province},
			Score:      score,
		}
		cands = append(cands, c)
		if score > bestScore {
			bestScore = score
		}
	}

	// 4.1) Dedup path theo chuỗi AdminID
	if len(cands) > 1 {
		seen := make(map[string]bool, len(cands))
		dedup := cands[:0]
		for _, c := range cands {
			ids := make([]string, 0, len(c.AdminUnits))
			for _, u := range c.AdminUnits {
				ids = append(ids, u.AdminID)
			}
			key := strings.Join(ids, ">")
			if !seen[key] {
				seen[key] = true
				dedup = append(dedup, c)
			}
		}
		cands = dedup
	}

	sort.Slice(cands, func(i, j int) bool { return cands[i].Score > cands[j].Score })

	// 5) Build result
	res := &MatchResult{
		Raw:                    rawAddress,
		NormalizedNoDiacritics: normalized,
		RawFingerprint:         fingerprint,
		Candidates:             cands,
		Quality: models.QualityMetrics{
			Flags: make([]string, 0),
		},
	}

	// 6) Best candidate → components/canonical
	if len(cands) > 0 {
		best := cands[0]
		res.Confidence = best.Score
		res.AdminPath = am.buildAdminPath(best.AdminUnits)

		res.Components = am.extractComponents(patterns, res.AdminPath)
		am.fillAdminComponentsFromPath(&res.Components, best.AdminUnits)

		street := am.streetFromPatterns(patterns)
		res.CanonicalText = am.buildCanonicalText(res.Components, best.AdminUnits, street)

		res.MatchStrategy = am.inferStrategyFromScore(best.Score)
		res.Quality.MatchLevel = string(am.matchLevelHeuristic(res.NormalizedNoDiacritics, res.CanonicalText))
	} else {
		res.Confidence = 0
		res.Status = "unmatched"
	}

	// 7) Residual & Flags & Status
	res.Residual = am.calculateResidual(normalized, patterns)
	am.assignQualityFlags(res, patterns)
	if len(cands) > 0 {
		am.determineStatus(res)
	}

	am.logger.Debug("Address matching done",
		zap.String("raw", rawAddress),
		zap.Float64("confidence", res.Confidence),
		zap.String("status", res.Status),
		zap.Int("candidates", len(res.Candidates)),
		zap.Duration("took", time.Since(start)),
	)
	return res, nil
}

/* ========== Candidate path building ========== */

type pathCandidate struct {
	Ward, District, Province models.AdminUnit
}

func (am *AddressMatcher) buildCandidatePaths(ctx context.Context, norm string) ([]pathCandidate, error) {
	paths := []pathCandidate{}

	provFilter := `admin_subtype IN ["province","municipality"]`
	provinces, _, err := am.searcher.SearchWithFilter(norm, provFilter, 10)
	if err != nil {
		return nil, err
	}
	distFilter := `admin_subtype IN ["urban_district","rural_district","city_under_province"]`
	districts, _, err := am.searcher.SearchWithFilter(norm, distFilter, 20)
	if err != nil {
		return nil, err
	}
	wardFilter := `admin_subtype IN ["ward","commune","township"]`
	wards, _, err := am.searcher.SearchWithFilter(norm, wardFilter, 30)
	if err != nil {
		return nil, err
	}

	distByID := map[string]models.AdminUnit{}
	for _, d := range districts {
		distByID[d.AdminID] = d
	}
	provByID := map[string]models.AdminUnit{}
	for _, p := range provinces {
		provByID[p.AdminID] = p
	}

	for _, w := range wards {
		if w.ParentID == nil {
			continue
		}
		d, ok := distByID[*w.ParentID]
		if !ok || d.ParentID == nil {
			continue
		}
		p, ok2 := provByID[*d.ParentID]
		if !ok2 {
			continue
		}
		paths = append(paths, pathCandidate{Ward: w, District: d, Province: p})
	}

	if len(paths) > am.maxCandidates {
		paths = paths[:am.maxCandidates]
	}
	return paths, nil
}

/* ================= Scoring ================= */

type ScoreParts struct {
	SimWard, SimDistrict, SimProvince float64
	Structural, RoadBonus, PoiBonus   float64
	LPCoverage                        float64
}

func (am *AddressMatcher) scorePath(norm string, path pathCandidate, sig normalizer.Signals, lpCov float64) (float64, ScoreParts) {
	wTok, dTok, pTok := normalizer.ExtractAdminTokens(norm)
	parts := ScoreParts{
		SimWard:     sim(wTok, path.Ward.Name),
		SimDistrict: sim(dTok, path.District.Name),
		SimProvince: sim(pTok, path.Province.Name),
		Structural:  1.0,
		LPCoverage:  lpCov,
	}

	pathNorm := unaccent(strings.Join([]string{path.Ward.Name, path.District.Name, path.Province.Name}, " "))
	if sig.RoadType != "" && sig.RoadCode != "" && strings.Contains(pathNorm, strings.ToLower(sig.RoadType+sig.RoadCode)) {
		parts.RoadBonus = 1.0
	}
	if sig.POI != "" && strings.Contains(pathNorm, unaccent(sig.POI)) {
		parts.PoiBonus = 1.0
	}

	w := config.C.Scoring.Weights
	if w.Ward == 0 && w.District == 0 && w.Province == 0 {
		w.Ward, w.District, w.Province = 0.35, 0.25, 0.15
		w.StructuralBonus, w.RoadcodeBonus, w.PoiBonus, w.LibpostalCoverage = 0.10, 0.07, 0.05, 0.03
	}

	score := w.Ward*parts.SimWard +
		w.District*parts.SimDistrict +
		w.Province*parts.SimProvince +
		w.StructuralBonus*parts.Structural +
		w.RoadcodeBonus*parts.RoadBonus +
		w.PoiBonus*parts.PoiBonus +
		w.LibpostalCoverage*parts.LPCoverage

	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score, parts
}

func sim(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	a, b = unaccent(a), unaccent(b)
	j := smetrics.JaroWinkler(a, b, 0.7, 4)
	ld := levenshtein.ComputeDistance(a, b)
	den := float64(max(len(a), len(b)))
	lev := 1.0 - float64(ld)/den
	jwW, lvW := config.C.JWWeight, config.C.LevWeight
	if jwW == 0 && lvW == 0 {
		jwW, lvW = 0.7, 0.3
	}
	return jwW*j + lvW*lev
}

func unaccent(s string) string { return strings.ToLower(unidecode.Unidecode(s)) }
func max(a, b int) int         { if a > b { return a }; return b }

/* ====== Components / Canonical / Residual ====== */

func (am *AddressMatcher) extractComponents(patterns []normalizer.PatternResult, _ []string) models.AddressComponents {
	components := models.AddressComponents{}
	for _, pattern := range patterns {
		switch pattern.Type {
		case "HOUSE_NO":
			if v := pattern.Value; v != "" {
				components.House.Number = &v
			}
		case "APARTMENT", "UNIT", "OFFICE":
			components.House.Unit = pattern.Value
		case "FLOOR":
			if floor := pattern.Value; floor != "" {
				components.House.Floor = &floor
			}
		case "ROAD_CODE":
			rt, _ := pattern.Metadata["road_type"]
			rn, _ := pattern.Metadata["road_number"]
			if rt != "" || rn != "" {
				components.RoadCode = &models.RoadCode{Type: rt, Code: rn}
			}
		case "ALLEY":
			if v := pattern.Value; v != "" {
				components.House.Alley.Number = &v
			}
		case "ALLEY_NAME":
			if v := pattern.Value; v != "" {
				components.House.Alley.Name = &v
			}
		}
	}
	return components
}

func (am *AddressMatcher) fillAdminComponentsFromPath(comp *models.AddressComponents, units []models.AdminUnit) {
	for _, u := range units {
		switch u.AdminSubtype {
		case "ward", "commune", "township":
			comp.Ward = &u
		case "urban_district", "rural_district", "city_under_province":
			comp.District = &u
		case "municipality":
			comp.City = &u
		case "province":
			comp.Province = &u
		}
	}
	if comp.Country == nil {
		comp.Country = &models.AdminUnit{
			AdminID:      "84",
			Name:         "Việt Nam",
			AdminSubtype: "country",
		}
	}
}

func (am *AddressMatcher) streetFromPatterns(patterns []normalizer.PatternResult) string {
	for _, p := range patterns {
		if p.Type == "STREET" || p.Type == "STREET_NAME" {
			return strings.TrimSpace(p.Value)
		}
	}
	return ""
}

func (am *AddressMatcher) buildCanonicalText(comp models.AddressComponents, units []models.AdminUnit, street string) string {
	parts := []string{}
	left := []string{}
	if comp.House.Number != nil {
		left = append(left, *comp.House.Number)
	}
	if street != "" {
		left = append(left, street)
	}
	if s := strings.TrimSpace(strings.Join(left, " ")); s != "" {
		parts = append(parts, s)
	}
	for _, u := range units {
		parts = append(parts, u.Name)
	}
	parts = append(parts, "Việt Nam")
	return strings.Join(parts, ", ")
}

func (am *AddressMatcher) calculateResidual(normalized string, patterns []normalizer.PatternResult) string {
	residual := normalized
	for _, p := range patterns {
		for _, t := range p.Tokens {
			if t == "" {
				continue
			}
			residual = strings.ReplaceAll(residual, t, " ")
		}
	}
	residual = regexp.MustCompile(`\s+`).ReplaceAllString(residual, " ")
	return strings.TrimSpace(residual)
}

func (am *AddressMatcher) buildAdminPath(units []models.AdminUnit) []string {
	out := make([]string, 0, len(units))
	for _, u := range units {
		out = append(out, u.Name)
	}
	return out
}

/* ===== Flags / Status / Strategy / Level ===== */

func (am *AddressMatcher) assignQualityFlags(result *MatchResult, patterns []normalizer.PatternResult) {
	flags := make([]string, 0, 4)

	switch result.Quality.MatchLevel {
	case string(MatchLevelExact):
		flags = append(flags, string(FlagExactMatch))
	case string(MatchLevelAscii):
		flags = append(flags, string(FlagAsciiExact))
	default:
		flags = append(flags, string(FlagFuzzyMatch))
	}

	for _, p := range patterns {
		switch p.Type {
		case "APARTMENT", "FLOOR", "OFFICE", "UNIT":
			flags = append(flags, string(FlagApartmentUnit))
		case "POI":
			flags = append(flags, string(FlagPOIExtracted))
		}
	}

	if len(result.Candidates) > 1 {
		flags = append(flags, string(FlagMultipleCandidates))
	}
	if result.Confidence < am.thresholdMedium {
		flags = append(flags, string(FlagLowConfidence))
	}

	result.Quality.Flags = flags
	result.Quality.Score = result.Confidence
}

func (am *AddressMatcher) determineStatus(r *MatchResult) {
	switch {
	case r.Confidence >= am.thresholdHigh:
		r.Status = "matched"
	case r.Confidence >= am.thresholdMedium:
		r.Status = "needs_review"
	default:
		r.Status = "unmatched"
	}
	if len(r.Candidates) == 0 {
		r.Status = "unmatched"
	}
}

func (am *AddressMatcher) inferStrategyFromScore(score float64) MatchStrategy {
	switch {
	case score >= 0.95:
		return MatchStrategyExact
	case score >= 0.85:
		return MatchStrategyAscii
	case score >= 0.70:
		return MatchStrategyAlias
	default:
		return MatchStrategyFuzzy
	}
}

func (am *AddressMatcher) matchLevelHeuristic(normalizedNoDia, canonical string) MatchLevel {
	n := unaccent(normalizedNoDia)
	c := unaccent(canonical)
	if n == c {
		return MatchLevelAscii
	}
	return MatchLevelFuzzy
}

/* =========== Signals / Fingerprint =========== */

func (am *AddressMatcher) signalsFromPatterns(patterns []normalizer.PatternResult) (sig normalizer.Signals) {
	for _, p := range patterns {
		switch p.Type {
		case "ROAD_CODE":
			if t, ok := p.Metadata["road_type"]; ok {
				sig.RoadType = t
			}
			if c, ok := p.Metadata["road_number"]; ok {
				sig.RoadCode = c
			}
		case "POI":
			sig.POI = p.Value
		}
	}
	return
}

func (am *AddressMatcher) generateFingerprint(normalized, gazetteerVersion string) string {
	sum := sha256.Sum256([]byte(normalized + "\x1F" + gazetteerVersion))
	return "sha256:" + hex.EncodeToString(sum[:])
}
