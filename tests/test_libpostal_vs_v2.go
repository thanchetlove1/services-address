package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/address-parser/internal/normalizer"
	"github.com/address-parser/internal/external"
)

func main() {
	// Tạo normalizer V2
	norm := normalizer.NewTextNormalizer()
	
	// Test với libpostal
	norm.SetUseLibpostal(true)
	
	// Bộ test 10 địa chỉ dài để so sánh V2 vs libpostal
	testAddresses := []string{
		// Địa chỉ dài, phức tạp
		"TANG 8 UNIT 1, TOA NHA BITEXCO FINANCIAL, 2 HAI TRIEU, PHUONG BEN NGHE, QUAN 1, THANH PHO HO CHI MINH, VIET NAM",
		
		// Địa chỉ có nhiều thông tin
		"CAN HO A1.02 TANG 15, BLOCK A, CHUNG CU VINHOMES CENTRAL PARK, 208 NGUYEN HUU THO, PHUONG 22, QUAN BINH THANH, TP HCM",
		
		// Địa chỉ có road code và khu phố
		"SO 105/6 TL 37, KP THANH LOC, PHUONG THANH LOC, QUAN 12, THANH PHO HO CHI MINH, VIET NAM",
		
		// Địa chỉ có POI và building
		"LAU 23, UOA TOWER, 6 TAN TRAO, PHUONG TAN PHU, QUAN 7, THANH PHO HO CHI MINH",
		
		// Địa chỉ có nhiều từ khóa hành chính
		"TOA NHA VFC, TOWER, 29 TON DUC THANG, PHUONG BEN NGHE, QUAN 1, THANH PHO HO CHI MINH, VIET NAM",
		
		// Địa chỉ có thông tin chi tiết
		"CHUNG CU ECO GREEN SAIGON, BLOCK A, 23 NGUYEN HUU THO, PHUONG TAN HUNG, QUAN 7, THANH PHO HO CHI MINH",
		
		// Địa chỉ có khu đô thị
		"KDC ERA TOWN BLOCK B4, VINMARK DG 15B NOI DAI, PHUONG PHU MY, QUAN 7, THANH PHO HO CHI MINH",
		
		// Địa chỉ có ấp và xã
		"AP 3, TOC TIEN, THI XA PHU MY, TINH BA RIA VUNG TAU, VIET NAM",
		
		// Địa chỉ có đường tỉnh
		"194 QL 50, XA BINH HUNG, HUYEN BINH CHANH, THANH PHO HO CHI MINH, VIET NAM",
		
		// Địa chỉ có nhiều thông tin nhất
		"P 1205 PETRO TOWER, 1 LE DUAN, PHUONG BEN NGHE, QUAN 1, THANH PHO HO CHI MINH, VIET NAM, POSTCODE 70000",

		"Hiến-+84978615724-Daikin Service Từ Liêm, đường CN3, cụm CNTT vừa và nhỏ Từ Liêm, Phú Diễn, Phường Minh Khai, Quận Bắc Từ Liêm, Thành phố Hà Nội",
		"Ms Gấm-+84975593052-3 Cao Bá Quát, Phường Điện Biên, Quận Ba Đình, Thành phố Hà Nội",
		"Đào Duy Hoàng 0368000493 Thôn Tân Hòa, Xã Hợp Tiến, Huyện Đông Hưng, Tỉnh Thái Bình",
		"Mr Nam 0902217049- 170 Ỷ Lan Nguyên Phi, Phường Hòa Cường Bắc, Quận Hải Châu, Thành phố Đà Nẵng",
		"Hòa Việt Cường-0984018294-CTN0700184-Số 7, Lô 8A, Đường Lê Hồng Phong, Phường Đông Khê, Quận Ngô Quyền, Thành phố Hải Phòng",
		"Chị Thanh-0943.276.830-Số 25, Đốc Thiết, Phường Hưng Bình, Thành phố Vinh, Tỉnh Nghệ An",
		"Ms Oanh-0339847632-CTN0609018;HPF0-số 147 Giáp Hải, Phường Dĩnh Kế, Thành phố Bắc Giang, Tỉnh Bắc Giang",
		"PC-A. Vượng-0983661688-Số nhà 20 ngõ 61 đường Nguyễn văn giáp phường cầu diễn - quận nam từ liêm, hn",
		"Mr Việt-0906060290 - 167 An Thái; Phường Bình Hàn; Tp Hải Dương",
		"Mr Tân : 0986117231 Số 7 Lô 8A đường Lê Hồng Phong- Quận Ngô Quyền- Thành Phố Hải Phòng",
		"17.05 Tầng 17, Tháp 8, Khu 3, Khu phức hợp Thương mại – Dịch vụ, Văn phòng và Căn hộ, Số 28 Mai Chí Thọ, Phường An Phú, Thành phố Thủ Đức, Thành phố Hồ Chí Minh,  Việt Nam  Mr Duong: 0936977807",
		"Anh Đạt - CTN0700230-0935303367-25 Hòa Minh 19, Phường Hòa Minh, Quận Liên Chiểu, Thành phố Đà Nẵng",
		"Mr Đạt 0935303367-   25 Hòa Minh 19, Phường Hòa Minh, Quận Liên Chiểu, Thành phố Đà Nẵng",
		"MR TRUNG - 0902448073-48 Hưng Phú, Phường 8, Quận 8, TP Hồ Chí Minh",
		"72 TRAN THU DO, HAI HOA, TP MONG CAI, QU ANG NINH   , QUẢNG NINH",
		"21 TO 2 MINH XUAN, MINH XUAN,TUYEN QUANG .   , TUYÊN QUANG",
		"Toa nha Vien Dong, ngo 34 Hoang Cau , Q.Dong Da, Ha Noi   , HÀ NỘI",
		"50A - 50B duong so 18, phuong 8, Q. Go Vap, Ho Chi Minh   , HỒ CHÍ MINH",
		"So 5 ngach 31, ngo 165 duong Cau Gi ay, Q.Cau Giay, Ha Noi   , HÀ NỘI",
		"187 giang vo, Q.Dong Da, Ha Noi    , HÀ NỘI",
		"Thon Trung Hoa Xa Vu Ninh-Kien Xuon g-Thai Binh   , THÁI BÌNH",
		"Chung cu Ecogreen city 286 Nguyen X ien, H.Thanh Tri, Ha Noi   , HÀ NỘI",
		"726 Vo Nguyen Giap, Q.Le Chan, Hai Phong   , HẢI PHÒNG",
		"34 vu nang an p.ha long, TP.Nam Din h, Nam Dinh   , NAM ĐỊNH",
		"626/35 TRUNG NU VUONG Q HAI CHAU TP DA N ANG   , ĐÀ NẴNG",
		"21 MAN QUANG 17 P THO QUANG-Son Tra -Da Nang   , ĐÀ NẴNG",
		"32A ngo 54 Nam Yen Lung, H.Hoai Duc , Ha Noi   , HÀ NỘI",
		"26 DUONG UNG VAN KHIEM, P 25, BINH THANH , HO CHI MINH   , HỒ CHÍ MINH",
		"CHUNG CU EHOME 3 HO NGOC LAM, P AN LAC, Q BINH TAN, TP HCM   , HỒ CHÍ MINH",
		"34 NGO 42 VU NGOC PHAN LANG HA DONG DA H N   , HÀ NỘI",
		"SO 1 THUONG HIEN XA HA HOI-Thuong T in-Ha Noi   , HÀ NỘI",
		"THON 5. VIET TIEN. VINH BAO-Vinh Ba o-Hai Phong   , HẢI PHÒNG",
		"Tang 8 Capital Tower 109 tran hung dao-Hoan Kiem-Ha Noi   , HÀ NỘI",
		"85 NGUYEN BA TONG PHUONG TAN THANH QUAN TAN PHU HO CHI MINH   , HỒ CHÍ MINH",
		"So 24 ngach 5/12 Ngoa Long-Bac Tu L iem-Ha Noi   , HÀ NỘI",
		"HOI SO TCB SO 6 QUANG TRUNG, HOAN KIEM, HN   , HÀ NỘI",
		"458 MINH KHAI, ONEMOUNT BUILDING, Q.HAI BA TRUNG, HA NOI   , HÀ NỘI",
		"14/448 HA HUY TAP, TT. YEN VIEN, H.GIA L AM, TP.HA NOI   , HÀ NỘI",
		"NGO 30 THON MAI CHAU DAI MACH DONG ANH H A NOI   , HÀ NỘI",
		"69/5 TRAN VAN DANG P9 Q3 TP HCM .   , HỒ CHÍ MINH",
		"765/150 TO 7 PHUONG SAI DONG-Long B ien-Ha Noi   , HÀ NỘI",
		"khu Grandworld xa Ganh Dau-Phu Quoc -Kien Giang   , KIÊN GIANG",
		"toa richy tang 15 so 35 mac thai to yen hoa-Cau Giay-Ha Noi   , HÀ NỘI",
		"Cong ty Sasco, 45 Truong Son, P2, Q.Tan Binh, Ho Chi Minh   , HỒ CHÍ MINH",
	}

	fmt.Println("🧪 Testing Libpostal Integration: V2 vs Libpostal")
	fmt.Println(strings.Repeat("=", 80))

	for i, addr := range testAddresses {
		fmt.Printf("\n📍 Test %d: %s\n", i+1, addr)
		fmt.Println(strings.Repeat("-", 60))
		
		// Test với normalizer V2 + libpostal
		result := norm.NormalizeAddress(addr)
		
		// Test với libpostal trực tiếp
		libpostalResult := external.ExtractWithLibpostal(addr)
		
		// So sánh kết quả
		fmt.Printf("🔤 V2 Normalized: %s\n", result.NormalizedNoDiacritics)
		fmt.Printf("🎯 V2 Confidence: %.2f\n", result.Confidence)
		fmt.Printf("🏷️  V2 Components: %d\n", len(result.ComponentTags))
		
		if result.UseLibpostal && result.LibpostalResult != nil {
			fmt.Printf("📊 Libpostal Coverage: %.2f\n", result.LibpostalResult.Coverage)
			fmt.Printf("🔍 Libpostal Confidence: %.2f\n", result.LibpostalResult.Confidence)
			fmt.Printf("🏠 Libpostal House: %s\n", result.LibpostalResult.House)
			fmt.Printf("🛣️  Libpostal Road: %s\n", result.LibpostalResult.Road)
			fmt.Printf("🏢 Libpostal Unit: %s\n", result.LibpostalResult.Unit)
			fmt.Printf("📈 Libpostal Level: %s\n", result.LibpostalResult.Level)
			fmt.Printf("🏘️  Libpostal Ward: %s\n", result.LibpostalResult.Ward)
			fmt.Printf("🏙️  Libpostal City: %s\n", result.LibpostalResult.City)
			fmt.Printf("🌍 Libpostal Province: %s\n", result.LibpostalResult.Province)
		}
		
		// So sánh với libpostal trực tiếp
		fmt.Printf("\n📋 Libpostal Direct Result:\n")
		fmt.Printf("   🏠 House: %s\n", libpostalResult.House)
		fmt.Printf("   🛣️  Road: %s\n", libpostalResult.Road)
		fmt.Printf("   🏢 Unit: %s\n", libpostalResult.Unit)
		fmt.Printf("   📈 Level: %s\n", libpostalResult.Level)
		fmt.Printf("   🏘️  Ward: %s\n", libpostalResult.Ward)
		fmt.Printf("   🏙️  City: %s\n", libpostalResult.City)
		fmt.Printf("   🌍 Province: %s\n", libpostalResult.Province)
		fmt.Printf("   📊 Coverage: %.2f\n", libpostalResult.Coverage)
		
		fmt.Println(strings.Repeat("-", 60))
	}

	// Test toggle libpostal
	fmt.Println("\n🔄 Testing Libpostal Toggle...")
	
	// Tắt libpostal
	norm.SetUseLibpostal(false)
	resultOff := norm.NormalizeAddress(testAddresses[0])
	fmt.Printf("❌ Libpostal OFF - Confidence: %.2f, UseLibpostal: %v\n", 
		resultOff.Confidence, resultOff.UseLibpostal)
	
	// Bật libpostal
	norm.SetUseLibpostal(true)
	resultOn := norm.NormalizeAddress(testAddresses[0])
	fmt.Printf("✅ Libpostal ON - Confidence: %.2f, UseLibpostal: %v\n", 
		resultOn.Confidence, resultOn.UseLibpostal)
	
	// JSON output example
	fmt.Println("\n📋 JSON Output Example (with libpostal):")
	jsonData, err := json.MarshalIndent(resultOn, "", "  ")
	if err != nil {
		fmt.Printf("❌ Error marshaling JSON: %v\n", err)
	} else {
		fmt.Println(string(jsonData))
	}
	
	fmt.Println("\n🎉 Libpostal Integration Test Completed!")
}
