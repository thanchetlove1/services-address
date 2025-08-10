package models

import (
	"time"
)

// LearnedAliases alias đã học được từ hệ thống
type LearnedAliases struct {
	OriginalToken string    `bson:"original_token" json:"original_token"` // Token gốc
	CanonicalForm string    `bson:"canonical_form" json:"canonical_form"` // Dạng chuẩn
	AdminLevel    int       `bson:"admin_level" json:"admin_level"`       // Cấp hành chính
	AdminID       string    `bson:"admin_id" json:"admin_id"`             // ID đơn vị hành chính
	Confidence    float64   `bson:"confidence" json:"confidence"`         // Độ tin cậy
	Source        string    `bson:"source" json:"source"`                 // Nguồn học (manual/auto_learned)
	UsageCount    int       `bson:"usage_count" json:"usage_count"`       // Số lần sử dụng
	CreatedAt     time.Time `bson:"created_at" json:"created_at"`         // Thời gian tạo
	LastUsed      time.Time `bson:"last_used" json:"last_used"`           // Lần sử dụng cuối
}

// Source constants
const (
	SourceManual      = "manual"
	SourceAutoLearned = "auto_learned"
)

// NewLearnedAliases tạo mới một LearnedAliases
func NewLearnedAliases(originalToken, canonicalForm string, adminLevel int, adminID string, source string) *LearnedAliases {
	return &LearnedAliases{
		OriginalToken: originalToken,
		CanonicalForm: canonicalForm,
		AdminLevel:    adminLevel,
		AdminID:       adminID,
		Confidence:    0.8, // Độ tin cậy mặc định
		Source:        source,
		UsageCount:    1,
		CreatedAt:     time.Now(),
		LastUsed:      time.Now(),
	}
}

// IsValidSource kiểm tra source có hợp lệ không
func (la *LearnedAliases) IsValidSource() bool {
	validSources := []string{
		SourceManual,
		SourceAutoLearned,
	}
	
	for _, validSource := range validSources {
		if la.Source == validSource {
			return true
		}
	}
	return false
}

// IsValidAdminLevel kiểm tra admin_level có hợp lệ không
func (la *LearnedAliases) IsValidAdminLevel() bool {
	return la.AdminLevel >= 1 && la.AdminLevel <= 4
}

// UpdateUsage cập nhật thông tin sử dụng
func (la *LearnedAliases) UpdateUsage() {
	la.UsageCount++
	la.LastUsed = time.Now()
}

// UpdateConfidence cập nhật độ tin cậy
func (la *LearnedAliases) UpdateConfidence(newConfidence float64) {
	if newConfidence >= 0.0 && newConfidence <= 1.0 {
		la.Confidence = newConfidence
	}
}

// IsHighConfidence kiểm tra có độ tin cậy cao không
func (la *LearnedAliases) IsHighConfidence() bool {
	return la.Confidence >= 0.8
}

// IsFrequentlyUsed kiểm tra có được sử dụng thường xuyên không
func (la *LearnedAliases) IsFrequentlyUsed(threshold int) bool {
	return la.UsageCount >= threshold
}
