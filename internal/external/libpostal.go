//go:build cgo
package external

/*
#cgo pkg-config: libpostal
#include <libpostal/libpostal.h>
#include <stdlib.h>
*/
import "C"

import (
	"log"
	"strings"

	expand "github.com/openvenues/gopostal/expand"
	parser "github.com/openvenues/gopostal/parser"
)

// InitializeLibpostal - function for manual initialization if needed  
func InitializeLibpostal() {
	log.Println("Libpostal should be initialized by gopostal automatically")
}

// Removed init() - let gopostal handle initialization automatically

// LibpostalResult kết quả chi tiết từ libpostal
type LibpostalResult struct {
	House       string  `json:"house"`
	Road        string  `json:"road"`
	Unit        string  `json:"unit"`
	Level       string  `json:"level"`
	Ward        string  `json:"ward"`
	City        string  `json:"city"`
	Province    string  `json:"province"`
	Postcode    string  `json:"postcode"`
	Country     string  `json:"country"`
	Coverage    float64 `json:"coverage"`
	Confidence  float64 `json:"confidence"`
	RawResult   string  `json:"raw_result"`
}

// LP struct cũ để backward compatibility
type LP struct {
	House, Road, Unit, Level, Ward, City, Province string
	Coverage                                        float64
}

// ExtractWithLibpostal trích xuất địa chỉ sử dụng libpostal
func ExtractWithLibpostal(raw string) LibpostalResult {
	opts := expand.GetDefaultExpansionOptions()
	opts.Languages = []string{"vi"}
	exps := expand.ExpandAddressOptions(raw, opts)
	best := raw
	if len(exps) > 0 { 
		best = exps[0] 
	}

	comps := parser.ParseAddress(best)
	covered, total := 0, len(strings.Fields(best))
	lp := LibpostalResult{
		RawResult: best,
	}
	
	for _, c := range comps {
		switch c.Label {
		case "house_number": 
			lp.House = c.Value
		case "road":         
			lp.Road = c.Value
		case "unit":         
			lp.Unit = c.Value
		case "level":        
			lp.Level = c.Value
		case "suburb":       
			lp.Ward = c.Value
		case "city":         
			lp.City = c.Value
		case "state":        
			lp.Province = c.Value
		case "postcode":     
			lp.Postcode = c.Value
		case "country":      
			lp.Country = c.Value
		}
		covered += len(strings.Fields(c.Value))
	}
	
	if total > 0 { 
		lp.Coverage = float64(covered) / float64(total) 
	}
	
	// Tính confidence dựa trên coverage và số lượng components
	componentCount := 0
	if lp.House != "" { componentCount++ }
	if lp.Road != "" { componentCount++ }
	if lp.Unit != "" { componentCount++ }
	if lp.Level != "" { componentCount++ }
	if lp.Ward != "" { componentCount++ }
	if lp.City != "" { componentCount++ }
	if lp.Province != "" { componentCount++ }
	
	if componentCount > 0 {
		lp.Confidence = (lp.Coverage + float64(componentCount)/7.0) / 2.0
	} else {
		lp.Confidence = lp.Coverage
	}
	
	return lp
}

// ExtractWithLibpostalFallback trích xuất với fallback khi rule-based mơ hồ
func ExtractWithLibpostalFallback(raw string, ruleBasedConfidence float64) LibpostalResult {
	// Nếu rule-based confidence thấp, sử dụng libpostal
	if ruleBasedConfidence < 0.6 {
		return ExtractWithLibpostal(raw)
	}
	
	// Nếu rule-based confidence cao, vẫn dùng libpostal để bổ sung thông tin
	lp := ExtractWithLibpostal(raw)
	
	// Tăng confidence khi kết hợp cả hai
	if lp.Confidence > ruleBasedConfidence {
		lp.Confidence = (lp.Confidence + ruleBasedConfidence) / 2.0
	}
	
	return lp
}

// GetLPStruct trả về struct LP cũ để backward compatibility
func (lr LibpostalResult) GetLPStruct() LP {
	return LP{
		House:    lr.House,
		Road:     lr.Road,
		Unit:     lr.Unit,
		Level:    lr.Level,
		Ward:     lr.Ward,
		City:     lr.City,
		Province: lr.Province,
		Coverage: lr.Coverage,
	}
}