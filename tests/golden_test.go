package tests

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
)

// GoldenTest struct cho golden test
type GoldenTest struct {
	Raw  string `json:"raw"`
	Expect struct {
		Status           string   `json:"status"`
		CanonicalText   string   `json:"canonical_text"`
		ResidualContains []string `json:"residual_contains,omitempty"`
	} `json:"expect"`
}

// TestGoldenTests chạy tất cả golden tests
func TestGoldenTests(t *testing.T) {
	goldenDir := "golden"
	files, err := ioutil.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("Không thể đọc thư mục golden: %v", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			testGoldenFile(t, filepath.Join(goldenDir, file.Name()))
		})
	}
}

// testGoldenFile test một file golden cụ thể
func testGoldenFile(t *testing.T, filepath string) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		t.Fatalf("Không thể đọc file %s: %v", filepath, err)
	}

	var test GoldenTest
	if err := json.Unmarshal(data, &test); err != nil {
		t.Fatalf("Không thể parse JSON từ %s: %v", filepath, err)
	}

	// TODO: Gọi API parse thực tế và so sánh kết quả
	// Hiện tại chỉ log để verify format
	t.Logf("Testing: %s", test.Raw)
	t.Logf("Expected status: %s", test.Expect.Status)
	t.Logf("Expected canonical: %s", test.Expect.CanonicalText)
	
	if len(test.Expect.ResidualContains) > 0 {
		t.Logf("Expected residual contains: %v", test.Expect.ResidualContains)
	}

	// Placeholder test - luôn pass
	// TODO: Implement actual API call và assertion
	t.Log("Golden test format verified - implement actual API testing")
}

// TestGoldenTestStructure test cấu trúc của golden tests
func TestGoldenTestStructure(t *testing.T) {
	// Test cấu trúc cơ bản
	test := GoldenTest{
		Raw: "test address",
		Expect: struct {
			Status           string   `json:"status"`
			CanonicalText   string   `json:"canonical_text"`
			ResidualContains []string `json:"residual_contains,omitempty"`
		}{
			Status:         "matched",
			CanonicalText: "test canonical",
		},
	}

	// Verify struct có thể serialize/deserialize
	data, err := json.Marshal(test)
	if err != nil {
		t.Fatalf("Không thể serialize GoldenTest: %v", err)
	}

	var decoded GoldenTest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Không thể deserialize GoldenTest: %v", err)
	}

	if decoded.Raw != test.Raw {
		t.Errorf("Raw mismatch: got %s, want %s", decoded.Raw, test.Raw)
	}

	if decoded.Expect.Status != test.Expect.Status {
		t.Errorf("Status mismatch: got %s, want %s", decoded.Expect.Status, test.Expect.Status)
	}
}
