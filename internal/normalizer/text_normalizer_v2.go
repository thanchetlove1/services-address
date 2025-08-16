package normalizer

import (
	"strings"
)

// NormalizationResult kết quả normalize từ V2
type NormalizationResult struct {
	OriginalCleaned      string `json:"original_cleaned"`
	NormalizedNoDiacritics string `json:"normalized_no_diacritics"`
	Fingerprint          string `json:"fingerprint"`
	ComponentTags        map[string]string `json:"component_tags"`
}

// TextNormalizerV2 normalizer phiên bản 2 với cải tiến
type TextNormalizerV2 struct {
	// Sử dụng TextNormalizer cũ làm base
	base *TextNormalizer
}

// NewTextNormalizerV2 tạo mới TextNormalizerV2
func NewTextNormalizerV2() *TextNormalizerV2 {
	return &TextNormalizerV2{
		base: NewTextNormalizer(),
	}
}

// NormalizeAddress normalize địa chỉ và trả về NormalizationResult
func (tn *TextNormalizerV2) NormalizeAddress(rawAddress string) *NormalizationResult {
	if rawAddress == "" {
		return &NormalizationResult{}
	}

	// Sử dụng base normalizer
	normalized, signals := tn.base.NormalizeAddress(rawAddress)
	
	// Tạo component tags từ signals
	componentTags := make(map[string]string)
	if signals != "" {
		// Parse signals nếu cần
		componentTags["signals"] = signals
	}

	// Tạo fingerprint đơn giản
	fingerprint := "sha256:" + strings.ToLower(strings.ReplaceAll(normalized, " ", ""))

	return &NormalizationResult{
		OriginalCleaned:       rawAddress,
		NormalizedNoDiacritics: normalized,
		Fingerprint:           fingerprint,
		ComponentTags:         componentTags,
	}
}
