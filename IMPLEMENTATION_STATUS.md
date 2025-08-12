# Implementation Status - Code V1

## Đã hoàn thành ✅

### 1. Config động
- ✅ `app/config/config.go` - struct config đầy đủ
- ✅ `config/parser.yaml` - file config YAML
- ✅ `cmd/api/main.go` - load config khi khởi động
- ✅ `cmd/worker/main.go` - load config khi khởi động

### 2. Normalizer V2
- ✅ `internal/normalizer/normalizer_simple.go` - struct Signals và hàm Normalize
- ✅ Hỗ trợ road code, house number, unit/level extraction
- ✅ Noise removal (điện thoại, mã đơn hàng)
- ✅ VN abbreviation expansion

### 3. Libpostal (optional)
- ✅ `internal/external/libpostal.go` - CGO version
- ✅ `internal/external/libpostal_stub.go` - no-CGO version
- ✅ Struct LP với coverage calculation

### 4. Meilisearch retrieval + path builder
- ✅ `internal/search/gazetteer_searcher.go` - thêm struct AdminCandidate, CandidatePath
- ✅ Hàm `FindCandidatesV2` và `ApplyIndexSettings`
- ✅ Search với filter theo admin_subtype

### 5. Scoring + match level
- ✅ `internal/parser/address_matcher.go` - thêm struct ScoreParts, hàm ScorePath
- ✅ Similarity calculation với Jaro-Winkler + Levenshtein
- ✅ Road bonus và POI bonus scoring

### 6. Confidence + Parse orchestrator
- ✅ `internal/parser/address_parser.go` - thêm struct ConfidenceParts, Result
- ✅ Hàm `ParseOnce`, `Parse` với context
- ✅ Confidence calculation từ score + completeness + path consistency

### 7. Controller helpers
- ✅ `app/controllers/address_controller.go` - thêm helper functions
- ✅ `statusFromConfidence`, `buildFlags`, `canonicalFromPath`
- ✅ Import parser package và strings

### 8. Batch worker
- ✅ `app/services/address_service.go` - thêm struct Out, Deps
- ✅ Hàm `ProcessBatch` với logging (placeholder cho file writing)

### 9. Golden tests
- ✅ `tests/golden/` - 4 test cases mẫu
- ✅ `tests/golden_test.go` - test runner với struct validation

## Cần hoàn thiện 🔄

### 1. Integration
- [ ] Kết nối `FindCandidatesV2` với `ParseOnce`
- [ ] Sử dụng config weights thay vì hardcode
- [ ] Implement libpostal integration trong parser

### 2. Search indexes
- [ ] Tạo indexes riêng cho wards, districts, provinces
- [ ] Seed data với normalized fields
- [ ] Gọi `ApplyIndexSettings` sau khi seed

### 3. API endpoints
- [ ] Test 4 golden cases qua `/v1/addresses/parse`
- [ ] Implement batch processing endpoint
- [ ] Admin endpoints cho seed/build-indexes

### 4. Performance
- [ ] Worker pool cho batch processing
- [ ] NDJSON streaming với gzip
- [ ] Cache warm-up và hit rate monitoring

## Cách chạy test

### 1. Build và chạy service
```bash
cd address-parser
go build ./cmd/api
./api
```

### 2. Chạy golden tests
```bash
go test ./tests -v
```

### 3. Test API endpoint
```bash
curl -X POST http://localhost:8080/v1/addresses/parse \
  -H "Content-Type: application/json" \
  -d '{"address": "SO 199 HOANG NHU TIEP PHUONG BO DE QUAN LONG BIEN TP HA NOI, HÀ NỘI"}'
```

## Next steps

1. **Seed data**: Tạo gazetteer data với normalized fields
2. **Build indexes**: Tạo Meilisearch indexes riêng biệt
3. **Integration test**: Kết nối các components
4. **Performance test**: Đo throughput 20k addresses
5. **Production tuning**: Optimize config và worker pools

## Notes

- Các hàm đã được thêm vào files hiện có thay vì tạo mới
- Sử dụng hardcode values tạm thời cho config weights
- TODO comments đánh dấu các phần cần hoàn thiện
- Golden tests hiện tại chỉ validate format, chưa gọi API thực
