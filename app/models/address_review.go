package models

import (
	"time"
)

// AddressReview địa chỉ cần review
type AddressReview struct {
	RawAddress             string         `bson:"raw_address" json:"raw_address"`                         // Địa chỉ gốc
	NormalizedNoDiacritics string         `bson:"normalized_no_diacritics" json:"normalized_no_diacritics"` // Văn bản đã chuẩn hóa
	AutoParsedResult       AddressResult  `bson:"auto_parsed_result" json:"auto_parsed_result"`           // Kết quả parse tự động
	Confidence             float64        `bson:"confidence" json:"confidence"`                           // Độ tin cậy
	Candidates             []Candidate    `bson:"candidates" json:"candidates"`                           // Danh sách ứng viên
	Status                 string         `bson:"status" json:"status"`                                   // Trạng thái review
	ManualResult           *AddressResult `bson:"manual_result,omitempty" json:"manual_result,omitempty"`  // Kết quả review thủ công
	ReviewerID             *string        `bson:"reviewer_id,omitempty" json:"reviewer_id,omitempty"`      // ID người review
	ReviewedAt             *time.Time     `bson:"reviewed_at,omitempty" json:"reviewed_at,omitempty"`      // Thời gian review
	CreatedAt              time.Time      `bson:"created_at" json:"created_at"`                           // Thời gian tạo
}

// Status constants
const (
	ReviewStatusPending   = "pending"
	ReviewStatusInReview  = "in_review"
	ReviewStatusApproved  = "approved"
	ReviewStatusRejected  = "rejected"
)

// NewAddressReview tạo mới một AddressReview
func NewAddressReview(rawAddress, normalizedText string, result AddressResult, candidates []Candidate) *AddressReview {
	return &AddressReview{
		RawAddress:             rawAddress,
		NormalizedNoDiacritics: normalizedText,
		AutoParsedResult:       result,
		Confidence:             result.Confidence,
		Candidates:             candidates,
		Status:                 ReviewStatusPending,
		CreatedAt:              time.Now(),
	}
}

// IsValidStatus kiểm tra status có hợp lệ không
func (ar *AddressReview) IsValidStatus() bool {
	validStatuses := []string{
		ReviewStatusPending,
		ReviewStatusInReview,
		ReviewStatusApproved,
		ReviewStatusRejected,
	}
	
	for _, validStatus := range validStatuses {
		if ar.Status == validStatus {
			return true
		}
	}
	return false
}

// Approve phê duyệt kết quả tự động
func (ar *AddressReview) Approve(reviewerID string) {
	ar.Status = ReviewStatusApproved
	ar.ReviewerID = &reviewerID
	now := time.Now()
	ar.ReviewedAt = &now
}

// Reject từ chối kết quả tự động
func (ar *AddressReview) Reject(reviewerID string) {
	ar.Status = ReviewStatusRejected
	ar.ReviewerID = &reviewerID
	now := time.Now()
	ar.ReviewedAt = &now
}

// SetManualResult thiết lập kết quả review thủ công
func (ar *AddressReview) SetManualResult(result AddressResult, reviewerID string) {
	ar.ManualResult = &result
	ar.Status = ReviewStatusApproved
	ar.ReviewerID = &reviewerID
	now := time.Now()
	ar.ReviewedAt = &now
}

// IsPending kiểm tra có đang chờ review không
func (ar *AddressReview) IsPending() bool {
	return ar.Status == ReviewStatusPending
}

// IsCompleted kiểm tra đã hoàn thành review chưa
func (ar *AddressReview) IsCompleted() bool {
	return ar.Status == ReviewStatusApproved || ar.Status == ReviewStatusRejected
}
