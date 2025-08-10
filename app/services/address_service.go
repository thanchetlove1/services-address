package services

import (
	"errors"
	"sync"
	"time"

	"github.com/address-parser/app/models"
	"github.com/address-parser/app/requests"
	"github.com/address-parser/internal/parser"
	"github.com/address-parser/internal/normalizer"
	"github.com/address-parser/internal/search"
	"go.uber.org/zap"
)

// AddressService service xử lý logic parse địa chỉ
type AddressService struct {
	parser     *parser.AddressParser
	normalizer *normalizer.TextNormalizerV2
	searcher   *search.GazetteerSearcher
	logger     *zap.Logger
	startTime  time.Time
	mu         sync.RWMutex
	
	// Job management
	jobs       map[string]*JobStatus
	jobResults map[string][]*models.AddressResult
}

// JobStatus trạng thái của job
type JobStatus struct {
	JobID              string
	Status             string
	Progress           float64
	Processed          int
	Total              int
	EstimatedRemaining int
	Message            string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// NewAddressService tạo mới AddressService
func NewAddressService(parser *parser.AddressParser, normalizer *normalizer.TextNormalizerV2, searcher *search.GazetteerSearcher, logger *zap.Logger) *AddressService {
	return &AddressService{
		parser:     parser,
		normalizer: normalizer,
		searcher:   searcher,
		logger:     logger,
		startTime:  time.Now(),
		jobs:       make(map[string]*JobStatus),
		jobResults: make(map[string][]*models.AddressResult),
	}
}

// ParseAddress parse một địa chỉ
func (as *AddressService) ParseAddress(rawAddress string, options requests.ParseOptions) (*models.AddressResult, error) {
	if rawAddress == "" {
		return nil, errors.New("địa chỉ không được để trống")
	}

	// Parse địa chỉ với gazetteer version mặc định
	result, err := as.parser.ParseAddress(rawAddress, "1.0.0")
	if err != nil {
		return nil, err
	}

	// Tính toán confidence nếu cần
	if result.Confidence < options.MinConfidence {
		result.Confidence = options.MinConfidence
		result.Status = models.StatusNeedsReview
	}

	return result, nil
}

// EstimateBatchProcessingTime ước tính thời gian xử lý batch
func (as *AddressService) EstimateBatchProcessingTime(addressCount int) int {
	// Ước tính dựa trên số lượng địa chỉ
	// Giả sử mỗi địa chỉ mất 100ms
	estimatedMs := addressCount * 100
	return estimatedMs / 1000 // Chuyển về giây
}

// ProcessBatchJob xử lý job batch trong background
func (as *AddressService) ProcessBatchJob(jobID string, addresses []string, options requests.ParseOptions) {
	// Tạo job status
	as.mu.Lock()
	as.jobs[jobID] = &JobStatus{
		JobID:     jobID,
		Status:    "running",
		Progress:  0.0,
		Processed: 0,
		Total:     len(addresses),
		Message:   "Đang xử lý...",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	as.mu.Unlock()

	// Xử lý addresses
	results := make([]*models.AddressResult, len(addresses))
	
	for i, address := range addresses {
		result, err := as.ParseAddress(address, options)
		if err != nil {
			// Tạo result lỗi
			result = &models.AddressResult{
				Raw:        address,
				Status:     models.StatusUnmatched,
				Confidence: 0.0,
			}
		}
		results[i] = result

		// Cập nhật progress
		as.mu.Lock()
		if job, exists := as.jobs[jobID]; exists {
			job.Processed = i + 1
			job.Progress = float64(i+1) / float64(len(addresses))
			job.UpdatedAt = time.Now()
			
			if i == len(addresses)-1 {
				job.Status = "done"
				job.Message = "Hoàn thành xử lý"
			}
		}
		as.mu.Unlock()
	}

	// Lưu kết quả
	as.mu.Lock()
	as.jobResults[jobID] = results
	as.mu.Unlock()

	as.logger.Info("Batch job completed",
		zap.String("job_id", jobID),
		zap.Int("total_addresses", len(addresses)))
}

// GetJobStatus lấy trạng thái job
func (as *AddressService) GetJobStatus(jobID string) (*JobStatus, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	job, exists := as.jobs[jobID]
	if !exists {
		return nil, errors.New("job không tồn tại")
	}

	return job, nil
}

// GetJobResults lấy kết quả job
func (as *AddressService) GetJobResults(jobID string) ([]*models.AddressResult, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	results, exists := as.jobResults[jobID]
	if !exists {
		return nil, errors.New("kết quả job không tồn tại")
	}

	return results, nil
}

// GetJobResultsStream lấy kết quả job dưới dạng channel để stream
func (as *AddressService) GetJobResultsStream(jobID string) (<-chan *models.AddressResult, error) {
	results, err := as.GetJobResults(jobID)
	if err != nil {
		return nil, err
	}

	resultChannel := make(chan *models.AddressResult, 100)
	
	go func() {
		defer close(resultChannel)
		for _, result := range results {
			resultChannel <- result
		}
	}()

	return resultChannel, nil
}

// GetStartTime lấy thời gian khởi động service
func (as *AddressService) GetStartTime() time.Time {
	return as.startTime
}

// GetStats lấy thống kê service
func (as *AddressService) GetStats() map[string]interface{} {
	as.mu.RLock()
	defer as.mu.RUnlock()

	uptime := time.Since(as.startTime)
	
	return map[string]interface{}{
		"uptime_seconds": int64(uptime.Seconds()),
		"start_time":     as.startTime.Format(time.RFC3339),
		"status":         "running",
	}
}
