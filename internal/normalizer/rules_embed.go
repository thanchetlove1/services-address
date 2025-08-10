package normalizer

import (
	"embed"
	"gopkg.in/yaml.v3"
)

//go:embed data/regex.yaml
var regexYAML []byte

//go:embed data/unigram_map.yaml
var unigramYAML []byte

//go:embed data/ngram_map.yaml
var ngramYAML []byte

// _embedDummy sử dụng để tránh lỗi linter về import embed không sử dụng
var _embedDummy = embed.FS{}

// RulesConfig chứa cấu hình rules được load từ YAML
type RulesConfig struct {
	NoisePatterns    map[string]string `yaml:"noise_patterns"`
	POIPatterns      map[string]string `yaml:"poi_patterns"`
	AdminPatterns    map[string]string `yaml:"admin_patterns"`
	StreetPatterns   map[string]string `yaml:"street_patterns"`
	HousePatterns    map[string]string `yaml:"house_patterns"`
	UnigramMap       map[string]string `yaml:"admin_abbreviations,omitempty"`
	BigramMap        map[string]string `yaml:"bigram_patterns,omitempty"`
	TrigramMap       map[string]string `yaml:"trigram_patterns,omitempty"`
	EnglishVietnamese map[string]string `yaml:"english_vietnamese,omitempty"`
}

// LoadRulesConfig load cấu hình rules từ embedded YAML files
func LoadRulesConfig() (*RulesConfig, error) {
	config := &RulesConfig{}
	
	// Load regex patterns
	if err := yaml.Unmarshal(regexYAML, config); err != nil {
		return nil, err
	}
	
	// Load unigram map
	if err := yaml.Unmarshal(unigramYAML, config); err != nil {
		return nil, err
	}
	
	// Load ngram map
	if err := yaml.Unmarshal(ngramYAML, config); err != nil {
		return nil, err
	}
	
	return config, nil
}
