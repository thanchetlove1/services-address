package normalizer

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/mozillazg/go-unidecode"
)

var rePhonesOrders = regexp.MustCompile(`(?i)(\+?84|0)\d{8,11}|[A-Z]{2}\d{6,}|CTN\w+`)
var reSpaces = regexp.MustCompile(`\s+`)
var reRoadCode = regexp.MustCompile(`\b(ql|dt|tl|hl|dh)\s*([0-9]{1,4}[a-z]?)\b`)
var reHouseNumber = regexp.MustCompile(`\b(\d{1,5}[A-Za-z]?)(?:/\d+)?\b`)
var reUnit = regexp.MustCompile(`(?i)\b(can ho|chung cu|apartment|unit|phong|p\.?)\s*([A-Za-z0-9\.\-]+)\b`)
var reLevel = regexp.MustCompile(`(?i)\b(tang|floor|t\.)\s*([0-9]{1,2})\b`)

var reWardTok = regexp.MustCompile(`\bphuong\s+([a-z0-9\s]+)`)
var reDistTok = regexp.MustCompile(`\b(quan|huyen|thi tran|thi xa|thanh pho)\s+([a-z0-9\s]+)`)
var reProvTok = regexp.MustCompile(`\b(tinh|thanh pho)\s+([a-z0-9\s]+)`)

type Signals struct {
	House     string
	Unit      string
	Level     string
	Road      string
	RoadType  string // ql/dt/tl/hl/dh
	RoadCode  string // 1a, 32...
	POI       string // nếu có danh mục, điền bằng Aho-Corasick
	WardHint  string
	DistHint  string
	ProvHint  string
	Residual  string
}

func unaccent(s string) string { return strings.ToLower(unidecode.Unidecode(s)) }

func expandVN(s string) string {
	r := " " + s + " "
	rep := map[string]string{
		" tp hcm ": " thanh pho ho chi minh ",
		" tphcm ":  " thanh pho ho chi minh ",
		" sai gon ": " thanh pho ho chi minh ",
		" sg ": " thanh pho ho chi minh ",
		" ha noi ": " ha noi ",
		" hn ": " ha noi ",
		" q.": " quan ",
		" q ": " quan ",
		" p.": " phuong ",
		" p ": " phuong ",
		" tt ": " thi tran ",
		" tx ": " thi xa ",
		" h.": " huyen ",
		" h ": " huyen ",
		" tp ": " thanh pho ",
	}
	for k, v := range rep {
		r = strings.ReplaceAll(r, k, v)
	}
	return strings.TrimSpace(reSpaces.ReplaceAllString(r, " "))
}

func Normalize(raw string) (string, Signals) {
	// cắt nhiễu (điện thoại, mã đơn)
	s := rePhonesOrders.ReplaceAllString(raw, " ")
	// về ascii + lower + gọn khoảng trắng
	s = strings.ToLower(unidecode.Unidecode(s))
	s = reSpaces.ReplaceAllString(strings.TrimSpace(s), " ")
	// mở rộng viết tắt
	s = expandVN(s)

	var sig Signals

	// road code
	if m := reRoadCode.FindStringSubmatch(s); len(m) == 3 {
		sig.RoadType, sig.RoadCode = m[1], m[2]
	}

	// house number (chọn số hợp lý đầu tiên)
	if m := reHouseNumber.FindStringSubmatch(s); len(m) > 1 {
		sig.House = m[1]
	}

	// unit/tầng
	if m := reUnit.FindStringSubmatch(s); len(m) > 2 {
		sig.Unit = m[2]
	}
	if m := reLevel.FindStringSubmatch(s); len(m) > 2 {
		sig.Level = m[2]
	}

	// hint thô
	if strings.Contains(s, " phuong ") {
		sig.WardHint = "phuong"
	}
	if strings.Contains(s, " quan ") || strings.Contains(s, " huyen ") || strings.Contains(s, " thi tran ") || strings.Contains(s, " thi xa ") {
		sig.DistHint = "district"
	}
	if strings.Contains(s, " thanh pho ") || strings.Contains(s, " tinh ") {
		sig.ProvHint = "province_or_muni"
	}

	// residual = phần đã cắt (điện thoại/mã đơn) theo raw
	sig.Residual = strings.TrimSpace(reSpaces.ReplaceAllString(rePhonesOrders.ReplaceAllString(raw, " "), " "))

	// ước lượng road thô (cắt trước các từ khóa hành chính)
	road := s
	for _, cut := range []string{" phuong ", " quan ", " huyen ", " thi tran ", " thi xa ", " tinh ", " thanh pho "} {
		if i := strings.Index(road, cut); i > 0 {
			road = strings.TrimSpace(road[:i])
			break
		}
	}
	// lọc ký tự rác
	road = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			return r
		}
		return -1
	}, road)
	if len(road) > 0 {
		sig.Road = road
	}

	return s, sig
}

// Trích token hành chính đơn giản từ normalized string
func ExtractAdminTokens(norm string) (ward, district, province string) {
	if m := reWardTok.FindStringSubmatch(norm); len(m) > 1 {
		ward = strings.TrimSpace(m[1])
	}
	if m := reDistTok.FindStringSubmatch(norm); len(m) > 2 {
		district = strings.TrimSpace(m[2])
	}
	if m := reProvTok.FindStringSubmatch(norm); len(m) > 2 {
		province = strings.TrimSpace(m[2])
	}
	return
}
