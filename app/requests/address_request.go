package requests

import "github.com/address-parser/app/models"

// ParseAddressRequest request parse địa chỉ đơn lẻ
type ParseAddressRequest struct {
	Address string `json:"address" binding:"required"` // Địa chỉ cần parse
	Options ParseOptions `json:"options,omitempty"`    // Tùy chọn parse
}

// ParseOptions tùy chọn parse
type ParseOptions struct {
	Levels            int  `json:"levels,omitempty"`             // Số cấp hành chính (3-4)
	UseCache          bool `json:"use_cache,omitempty"`          // Có sử dụng cache không
	ReturnCandidates  bool `json:"return_candidates,omitempty"`  // Có trả về candidates không
	MinConfidence     float64 `json:"min_confidence,omitempty"`  // Độ tin cậy tối thiểu
	TopK              int  `json:"top_k,omitempty"`              // Số candidates tối đa
}

// BatchParseRequest request parse hàng loạt địa chỉ
type BatchParseRequest struct {
	Addresses []string      `json:"addresses" binding:"required,min=1,max=20000"` // Danh sách địa chỉ (tối đa 20k)
	Options   ParseOptions  `json:"options,omitempty"`                            // Tùy chọn parse
}

// SeedGazetteerRequest request seed gazetteer
type SeedGazetteerRequest struct {
	GazetteerVersion string             `json:"gazetteer_version" binding:"required"` // Phiên bản gazetteer
	Data            []models.AdminUnit  `json:"data" binding:"required"`              // Dữ liệu gazetteer
	RebuildIndexes  bool                `json:"rebuild_indexes,omitempty"`            // Có rebuild indexes không
}

// ReviewApproveRequest request phê duyệt review
type ReviewApproveRequest struct {
	ReviewerID string `json:"reviewer_id" binding:"required"` // ID người review
}

// ReviewCorrectRequest request chỉnh sửa review
type ReviewCorrectRequest struct {
	ManualResult  models.AddressResult `json:"manual_result" binding:"required"`  // Kết quả chỉnh sửa
	LearnAliases  bool                 `json:"learn_aliases,omitempty"`           // Có học aliases không
	ReviewerID    string               `json:"reviewer_id" binding:"required"`    // ID người review
}
