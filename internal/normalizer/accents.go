package normalizer

import (
	"strings"
	"unicode"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// StripDiacritics loại bỏ dấu tiếng Việt một cách an toàn
func StripDiacritics(s string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
	out, _, _ := transform.String(t, s)
	return out
}

// isMn kiểm tra xem rune có phải là diacritic mark không
func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r)
}

// RemoveAccentsAndLowercase loại bỏ dấu và chuyển về lowercase
func RemoveAccentsAndLowercase(s string) string {
	// Loại bỏ dấu
	noAccents := StripDiacritics(s)
	// Chuyển về lowercase
	return strings.ToLower(noAccents)
}
