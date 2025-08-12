package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type ScoringWeights struct {
	Ward           float64 `yaml:"ward" json:"ward"`
	District       float64 `yaml:"district" json:"district"`
	Province       float64 `yaml:"province" json:"province"`
	StructuralBonus float64 `yaml:"structural_bonus" json:"structural_bonus"`
	RoadcodeBonus   float64 `yaml:"roadcode_bonus" json:"roadcode_bonus"`
	PoiBonus        float64 `yaml:"poi_bonus" json:"poi_bonus"`
	LibpostalCoverage float64 `yaml:"libpostal_coverage" json:"libpostal_coverage"`
}

type Thresholds struct {
	High      float64 `yaml:"high" json:"high"`
	ReviewLow float64 `yaml:"review_low" json:"review_low"`
	// ReviewHigh không bắt buộc; nếu muốn dùng, thêm vào YAML và struct này
}

type ConfidenceWeights struct {
	ScoreWeight        float64 `yaml:"score_weight" json:"score_weight"`
	CompletenessWeight float64 `yaml:"completeness_weight" json:"completeness_weight"`
	PathWeight         float64 `yaml:"path_weight" json:"path_weight"`
}

type MeiliTopK struct {
	TopKWard     int `yaml:"topk_ward" json:"topk_ward"`
	TopKDistrict int `yaml:"topk_district" json:"topk_district"`
	TopKProvince int `yaml:"topk_province" json:"topk_province"`
}

type ParserCfg struct {
	LevelConfig     int     `yaml:"level_config" json:"level_config"`
	UseNormalizerV2 bool    `yaml:"use_normalizer_v2" json:"use_normalizer_v2"`
	UseLibpostal    bool    `yaml:"use_libpostal" json:"use_libpostal"`
	JWWeight        float64 `yaml:"jw_weight" json:"jw_weight"`
	LevWeight       float64 `yaml:"lev_weight" json:"lev_weight"`
	Scoring         struct {
		Weights ScoringWeights `yaml:"weights" json:"weights"`
	} `yaml:"scoring" json:"scoring"`
	Thresholds Thresholds        `yaml:"thresholds" json:"thresholds"`
	Confidence ConfidenceWeights `yaml:"confidence" json:"confidence"`
	Meili      MeiliTopK         `yaml:"meili" json:"meili"`
}

var C ParserCfg

func Load(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, &C); err != nil {
		return err
	}
	// ENV override
	if v := os.Getenv("USE_LIBPOSTAL"); v == "0" {
		C.UseLibpostal = false
	}
	if v := os.Getenv("USE_LIBPOSTAL"); v == "1" {
		C.UseLibpostal = true
	}
	return nil
}

// Timeout parse đơn lẻ
func RequestTimeout() time.Duration { return 1500 * time.Millisecond }
