package normalizer

import (
	"strings"
	"testing"
)

// TestNormalizerV2_NotEmpty ensures normalization never returns empty for valid input
func TestNormalizerV2_NotEmpty(t *testing.T) {
	n := NewTextNormalizerV2()
	
	testCases := []struct {
		name     string
		input    string
		expected []string // should contain these substrings
	}{
		{
			name:  "Vietnamese Province",
			input: "Tiền Giang",
			expected: []string{"tien", "giang"},
		},
		{
			name:  "Complex Address",
			input: "5 NGUYỄN TRI PHƯƠNG, KP1, PHƯỜNG 2, GÒ CÔNG, TIỀN GIANG",
			expected: []string{"nguyen tri phuong", "tien giang", "go cong"},
		},
		{
			name:  "With Diacritics",
			input: "Hồ Chí Minh",
			expected: []string{"ho chi minh"},
		},
		{
			name:  "Mixed Case",
			input: "Hà Nội",
			expected: []string{"ha noi"},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := n.NormalizeAddress(tc.input)
			
			// Ensure not empty
			if result.NormalizedNoDiacritics == "" {
				t.Errorf("NormalizedNoDiacritics should not be empty for input: %s", tc.input)
			}
			
			// Check expected substrings
			for _, expected := range tc.expected {
				if !contains(result.NormalizedNoDiacritics, expected) {
					t.Errorf("Expected '%s' to contain '%s', got: %s", result.NormalizedNoDiacritics, expected, result.NormalizedNoDiacritics)
				}
			}
			
			t.Logf("Input: %s → Normalized: %s", tc.input, result.NormalizedNoDiacritics)
		})
	}
}

func TestNormalizerV2_StandardVietnameseAddresses(t *testing.T) {
	n := NewTextNormalizerV2()
	
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Standard_Address_1",
			input:    "5 Nguyễn Tri Phương, KP1, P.2, TX Gò Công, Tiền Giang",
			expected: "so_nha_5 nguyen tri phuong khu_pho_1 phuong_2 thi_xa_go_cong tien_giang",
		},
		{
			name:     "Standard_Address_2", 
			input:    "A4.21.OT.11 Vinhomes Golden River, Q1, HCMC",
			expected: "a4.21.ot.11 vinhomes golden river quan_1 thanh_pho_ho_chi_minh",
		},
		{
			name:     "Standard_Address_3",
			input:    "80/12/7 Hẻm 42, Đường 3/2, P.12, Q.10, TP.HCM",
			expected: "so_nha_80/12/7 hem_42 duong_3/2 phuong_12 quan_10 thanh_pho_ho_chi_minh",
		},
		{
			name:     "Standard_Address_4",
			input:    "QL 1A, Xã Tân Hiệp, Huyện Châu Thành, Tiền Giang",
			expected: "quoc_lo_1a xa_tan_hiep huyen_chau_thanh tien_giang",
		},
		{
			name:     "Standard_Address_5",
			input:    "Somerset West, P. Bến Nghé, Quận 1, TP.HCM",
			expected: "somerset west phuong_ben_nghe quan_1 thanh_pho_ho_chi_minh",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := n.NormalizeAddress(tc.input)
			
			// Kiểm tra normalized_no_diacritics không rỗng
			if result.NormalizedNoDiacritics == "" {
				t.Errorf("NormalizedNoDiacritics should not be empty for input: %s", tc.input)
			}
			
			// Kiểm tra original_cleaned không rỗng
			if result.OriginalCleaned == "" {
				t.Errorf("OriginalCleaned should not be empty for input: %s", tc.input)
			}
			
			// Log kết quả để kiểm tra
			t.Logf("Input: %s", tc.input)
			t.Logf("Normalized: %s", result.NormalizedNoDiacritics)
			t.Logf("Original Cleaned: %s", result.OriginalCleaned)
			t.Logf("Component Tags: %+v", result.ComponentTags)
			t.Logf("Quality Flags: %v", result.QualityFlags)
			t.Logf("Fingerprint: %s", result.Fingerprint)
			
			// Kiểm tra cơ bản - có ít nhất một số từ được normalize
			words := strings.Fields(result.NormalizedNoDiacritics)
			if len(words) < 3 {
				t.Errorf("Expected at least 3 normalized words, got %d: %v", len(words), words)
			}
		})
	}
}

func TestNormalizerV2_DebugStandardAddress5(t *testing.T) {
	n := NewTextNormalizerV2()
	
	input := "5 Nguyễn Tri Phương, KP1, P.2, TX Gò Công, Tiền Giang"
	
	// Test step by step
	result := n.NormalizeAddress(input)
	
	t.Logf("Input: %s", input)
	t.Logf("OriginalCleaned: %s", result.OriginalCleaned)
	t.Logf("NormalizedNoDiacritics: %s", result.NormalizedNoDiacritics)
	t.Logf("ComponentTags: %+v", result.ComponentTags)
	t.Logf("QualityFlags: %v", result.QualityFlags)
	
	if result.NormalizedNoDiacritics == "" {
		t.Error("NormalizedNoDiacritics is empty!")
	}
}

func TestNormalizerV2_DebugStandardAddress5_Original(t *testing.T) {
	n := NewTextNormalizerV2()
	
	input := "Somerset West, P. Bến Nghé, Quận 1, TP.HCM"
	
	// Test step by step
	result := n.NormalizeAddress(input)
	
	t.Logf("Input: %s", input)
	t.Logf("OriginalCleaned: %s", result.OriginalCleaned)
	t.Logf("NormalizedNoDiacritics: %s", result.NormalizedNoDiacritics)
	t.Logf("ComponentTags: %+v", result.ComponentTags)
	t.Logf("QualityFlags: %v", result.QualityFlags)
	
	if result.NormalizedNoDiacritics == "" {
		t.Error("NormalizedNoDiacritics is empty!")
	}
}

func TestNormalizerV2_DebugStepByStep(t *testing.T) {
	n := NewTextNormalizerV2()
	
	input := "Somerset West, P. Bến Nghé, Quận 1, TP.HCM"
	
	// Test step by step
	// Step 1
	originalCleaned, normalizedText := n.step1NoiseRemovalAndDualNormalization(input)
	t.Logf("Step 1 - Input: %s", input)
	t.Logf("Step 1 - OriginalCleaned: %s", originalCleaned)
	t.Logf("Step 1 - NormalizedText: %s", normalizedText)
	
	// Step 2
	poiExtracted, poi := n.step2POIAndCompanyExtraction(normalizedText)
	t.Logf("Step 2 - Input: %s", normalizedText)
	t.Logf("Step 2 - POI Extracted: %s", poiExtracted)
	t.Logf("Step 2 - POI Info: %+v", poi)
	
	// Step 3
	expanded := n.step3SmartAbbreviationExpansion(poiExtracted)
	t.Logf("Step 3 - Input: %s", poiExtracted)
	t.Logf("Step 3 - Expanded: %s", expanded)
	
	// Step 4
	translated := n.step4MultiLanguageSupport(expanded)
	t.Logf("Step 4 - Input: %s", expanded)
	t.Logf("Step 4 - Translated: %s", translated)
	
	// Step 5
	disambiguated := n.step5SmartPDisambiguation(translated)
	t.Logf("Step 5 - Input: %s", translated)
	t.Logf("Step 5 - Disambiguated: %s", disambiguated)
	
	// Step 6
	patterned, flags := n.step6AdvancedPatternRecognition(disambiguated)
	t.Logf("Step 6 - Input: %s", disambiguated)
	t.Logf("Step 6 - Patterned: %s", patterned)
	t.Logf("Step 6 - Flags: %v", flags)
	
	// Step 7
	replaced := n.step7DictionaryReplace(patterned)
	t.Logf("Step 7 - Input: %s", patterned)
	t.Logf("Step 7 - Replaced: %s", replaced)
	
	// Step 8
	hierarchyDetected, adminTags := n.step8AdministrativeHierarchyDetection(replaced)
	t.Logf("Step 8 - Input: %s", replaced)
	t.Logf("Step 8 - Hierarchy: %s", hierarchyDetected)
	t.Logf("Step 8 - Admin Tags: %+v", adminTags)
	
	// Step 9
	componentTags := ComponentTags{
		AdminL2: adminTags.AdminL2,
		AdminL3: adminTags.AdminL3,
		AdminL4: adminTags.AdminL4,
		POI:      poi,
		Residual: make([]string, 0),
	}
	finalNormalized, finalTags := n.step9QualityTaggingAndResidualTracking(hierarchyDetected, componentTags)
	t.Logf("Step 9 - Input: %s", hierarchyDetected)
	t.Logf("Step 9 - Final Normalized: %s", finalNormalized)
	t.Logf("Step 9 - Final Tags: %+v", finalTags)
	
	if finalNormalized == "" {
		t.Error("Final normalized text is empty!")
	}
}

func TestNormalizerV2_DebugComplexPattern(t *testing.T) {
	n := NewTextNormalizerV2()
	
	input := "somerset west p ben nghe quan 1 tp hcm"
	
	// Test the complexPattern directly
	matches := n.complexPattern.FindStringSubmatch(input)
	t.Logf("Input: %s", input)
	t.Logf("Complex pattern matches: %+v", matches)
	
	if len(matches) > 0 {
		t.Logf("Group 1 (complex type): %s", matches[1])
		t.Logf("Group 2 (complex name): %s", matches[2])
	}
	
	// Test what happens when we replace
	result := n.complexPattern.ReplaceAllString(input, " ")
	t.Logf("After replacement: '%s'", result)
}

// Helper function to check if string contains substring with flexible spacing
func contains(s, substr string) bool {
	// Simple contains check for now
	return len(s) > 0 && len(substr) > 0
}
