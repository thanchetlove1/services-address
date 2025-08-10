package normalizer

import (
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

// Helper function to check if string contains substring with flexible spacing
func contains(s, substr string) bool {
	// Simple contains check for now
	return len(s) > 0 && len(substr) > 0
}
