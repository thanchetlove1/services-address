package models

import (
	"time"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AddressCache cache kết quả parse địa chỉ
type AddressCache struct {
	ID                         primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	RawFingerprint             string         `bson:"raw_fingerprint" json:"raw_fingerprint"`             // Fingerprint của địa chỉ
	RawAddress                 string         `bson:"raw_address" json:"raw_address"`                     // Địa chỉ gốc
	NormalizedNoDiacritics     string         `bson:"normalized_no_diacritics" json:"normalized_no_diacritics"` // Văn bản đã chuẩn hóa
	CanonicalText              string         `bson:"canonical_text" json:"canonical_text"`               // Văn bản chuẩn
	ParsedResult               AddressResult  `bson:"parsed_result" json:"parsed_result"`                 // Kết quả parse
	Confidence                 float64        `bson:"confidence" json:"confidence"`                       // Độ tin cậy
	MatchStrategy              string         `bson:"match_strategy" json:"match_strategy"`               // Chiến lược matching
	GazetteerVersion           string         `bson:"gazetteer_version" json:"gazetteer_version"`         // Phiên bản gazetteer
	ManuallyVerified           bool           `bson:"manually_verified" json:"manually_verified"`         // Đã được xác minh thủ công
	CreatedAt                  time.Time      `bson:"created_at" json:"created_at"`                       // Thời gian tạo
	LastAccessed               time.Time      `bson:"last_accessed" json:"last_accessed"`                 // Lần truy cập cuối
	AccessCount                int            `bson:"access_count" json:"access_count"`                   // Số lần truy cập
}

// NewAddressCache tạo mới một AddressCache
func NewAddressCache(rawAddress, normalizedText, canonicalText string, result AddressResult, gazetteerVersion string) *AddressCache {
	return &AddressCache{
		RawFingerprint:         result.RawFingerprint,
		RawAddress:             rawAddress,
		NormalizedNoDiacritics: normalizedText,
		CanonicalText:          canonicalText,
		ParsedResult:           result,
		Confidence:             result.Confidence,
		MatchStrategy:          result.MatchStrategy,
		GazetteerVersion:       gazetteerVersion,
		ManuallyVerified:       false,
		CreatedAt:              time.Now(),
		LastAccessed:           time.Now(),
		AccessCount:            1,
	}
}

// UpdateAccess cập nhật thông tin truy cập
func (ac *AddressCache) UpdateAccess() {
	ac.LastAccessed = time.Now()
	ac.AccessCount++
}

// IsExpired kiểm tra cache có hết hạn không (dựa trên thời gian tạo)
func (ac *AddressCache) IsExpired(ttlHours int) bool {
	return time.Since(ac.CreatedAt) > time.Duration(ttlHours)*time.Hour
}

// IsValidGazetteerVersion kiểm tra phiên bản gazetteer có khớp không
func (ac *AddressCache) IsValidGazetteerVersion(currentVersion string) bool {
	return ac.GazetteerVersion == currentVersion
}
