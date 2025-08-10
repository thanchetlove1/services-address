package utils

import (
	"crypto/rand"
	"fmt"
)

// GenerateUUID tạo UUID v4
func GenerateUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback nếu crypto/rand fail
		return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	}
	
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// GenerateShortID tạo ID ngắn (8 ký tự)
func GenerateShortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// GenerateNumericID tạo ID số
func GenerateNumericID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%d", b)
}
