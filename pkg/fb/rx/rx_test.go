package rx

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/newrelic/nrdot-internal-devlab/internal/common/resilience"
	"github.com/newrelic/nrdot-internal-devlab/internal/config"
	"github.com/newrelic/nrdot-internal-devlab/pkg/fb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockChainPushServiceClient is a mock client for the ChainPushService
type MockChainPushServiceClient struct {
	mock.Mock
}

func (m *MockChainPushServiceClient) PushMetrics(ctx context.Context, in *fb.MetricBatchRequest, opts ...interface{}) (*fb.MetricBatchResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*fb.MetricBatchResponse), args.Error(1)
}

// MockCircuitBreaker is a mock circuit breaker for testing
type MockCircuitBreaker struct {
	mock.Mock
}

func (m *MockCircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *MockCircuitBreaker) State() resilience.CircuitBreakerState {
	args := m.Called()
	return args.Get(0).(resilience.CircuitBreakerState)
}

func TestRX_Initialize(t *testing.T) {
	r := NewRX()
	err := r.Initialize(context.Background())
	assert.NoError(t, err)
	assert.True(t, r.Ready())
}

func TestRX_UpdateConfig(t *testing.T) {
	r := NewRX()
	err := r.Initialize(context.Background())
	assert.NoError(t, err)

	// Test with valid config
	validConfig := RXConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    "fb-dlq:5000",
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		Endpoints: []Endpoint{
			{
				Protocol: "otlp/grpc",
				Port:     4317,
				Enabled:  true,
			},
		},
	}

	configBytes, err := json.Marshal(validConfig)
	assert.NoError(t, err)

	// Skip actual connection attempts by setting clients directly
	mockNextFB := new(MockChainPushServiceClient)
	mockDLQ := new(MockChainPushServiceClient)
	r.nextFBClient = mockNextFB
	r.dlqClient = mockDLQ

	err = r.UpdateConfig(context.Background(), configBytes, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), r.GetConfigGeneration())

	// Test with invalid config (missing endpoints)
	invalidConfig := RXConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    "fb-dlq:5000",
		},
		Endpoints: []Endpoint{}, // Invalid - empty endpoints
	}

	configBytes, err = json.Marshal(invalidConfig)
	assert.NoError(t, err)

	err = r.UpdateConfig(context.Background(), configBytes, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no endpoints configured")
}

func TestRX_ProcessBatch_Success(t *testing.T) {
	r := NewRX()
	err := r.Initialize(context.Background())
	assert.NoError(t, err)

	// Set up mock next FB client
	mockNextFB := new(MockChainPushServiceClient)
	r.nextFBClient = mockNextFB

	// Configure with valid config
	validConfig := RXConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    "fb-dlq:5000",
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		Endpoints: []Endpoint{
			{
				Protocol: "otlp/grpc",
				Port:     4317,
				Enabled:  true,
			},
		},
	}

	configBytes, err := json.Marshal(validConfig)
	assert.NoError(t, err)

	r.UpdateConfig(context.Background(), configBytes, 1)

	// Mock a successful response from the next FB
	mockNextFB.On("PushMetrics", mock.Anything, mock.MatchedBy(func(req *fb.MetricBatchRequest) bool {
		return req.BatchId == "test-batch-id"
	})).Return(&fb.MetricBatchResponse{
		Status:  fb.StatusSuccess,
		BatchId: "test-batch-id",
	}, nil)

	// Create a test batch
	batch := &fb.MetricBatch{
		BatchID: "test-batch-id",
		Data:    []byte(`{"resource_metrics":[{"resource":{"attributes":{"service.name":"test-service"}}}]}`),
		Format:  "otlp",
	}

	// Process the batch
	result, err := r.ProcessBatch(context.Background(), batch)
	assert.NoError(t, err)
	assert.Equal(t, fb.StatusSuccess, result.Status)
	mockNextFB.AssertExpectations(t)
}

func TestRX_ProcessBatch_NextFBFailure(t *testing.T) {
	r := NewRX()
	err := r.Initialize(context.Background())
	assert.NoError(t, err)

	// Set up mock clients
	mockNextFB := new(MockChainPushServiceClient)
	mockDLQ := new(MockChainPushServiceClient)
	r.nextFBClient = mockNextFB
	r.dlqClient = mockDLQ

	// Configure with valid config
	validConfig := RXConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    "fb-dlq:5000",
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		Endpoints: []Endpoint{
			{
				Protocol: "otlp/grpc",
				Port:     4317,
				Enabled:  true,
			},
		},
	}

	configBytes, err := json.Marshal(validConfig)
	assert.NoError(t, err)

	r.UpdateConfig(context.Background(), configBytes, 1)

	// Mock a failure response from the next FB
	forwardingErr := errors.New("failed to process batch")
	mockNextFB.On("PushMetrics", mock.Anything, mock.MatchedBy(func(req *fb.MetricBatchRequest) bool {
		return req.BatchId == "test-batch-id"
	})).Return(&fb.MetricBatchResponse{
		Status:       fb.StatusError,
		BatchId:      "test-batch-id",
		ErrorCode:    fb.ErrorCodeProcessingFailed,
		ErrorMessage: forwardingErr.Error(),
	}, nil)

	// Mock a successful response from the DLQ
	mockDLQ.On("PushMetrics", mock.Anything, mock.MatchedBy(func(req *fb.MetricBatchRequest) bool {
		return req.BatchId == "test-batch-id" && req.InternalLabels["fb_sender"] == "fb-rx"
	})).Return(&fb.MetricBatchResponse{
		Status:  fb.StatusSuccess,
		BatchId: "test-batch-id",
	}, nil)

	// Create a test batch
	batch := &fb.MetricBatch{
		BatchID: "test-batch-id",
		Data:    []byte(`{"resource_metrics":[{"resource":{"attributes":{"service.name":"test-service"}}}]}`),
		Format:  "otlp",
	}

	// Process the batch
	result, err := r.ProcessBatch(context.Background(), batch)
	assert.Error(t, err)
	assert.Equal(t, fb.StatusError, result.Status)
	assert.Equal(t, fb.ErrorCodeForwardingFailed, result.ErrorCode)
	assert.True(t, result.SentToDLQ)
	
	mockNextFB.AssertExpectations(t)
	mockDLQ.AssertExpectations(t)
}

func TestRX_ProcessBatch_CircuitBreakerOpen(t *testing.T) {
	r := NewRX()
	err := r.Initialize(context.Background())
	assert.NoError(t, err)

	// Configure with valid config
	validConfig := RXConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    "fb-dlq:5000",
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		Endpoints: []Endpoint{
			{
				Protocol: "otlp/grpc",
				Port:     4317,
				Enabled:  true,
			},
		},
	}

	configBytes, err := json.Marshal(validConfig)
	assert.NoError(t, err)

	r.UpdateConfig(context.Background(), configBytes, 1)

	// Replace circuit breaker with a mock that always returns open
	mockCB := new(MockCircuitBreaker)
	r.circuitBreaker = mockCB

	// Mock the circuit breaker to return ErrCircuitOpen
	mockCB.On("Execute", mock.Anything, mock.Anything).Return(resilience.ErrCircuitOpen)

	// Create a test batch
	batch := &fb.MetricBatch{
		BatchID: "test-batch-id",
		Data:    []byte(`{"resource_metrics":[{"resource":{"attributes":{"service.name":"test-service"}}}]}`),
		Format:  "otlp",
	}

	// Process the batch
	result, err := r.ProcessBatch(context.Background(), batch)
	assert.Error(t, err)
	assert.Equal(t, fb.StatusError, result.Status)
	assert.Equal(t, fb.ErrorCodeCircuitBreakerOpen, result.ErrorCode)
	assert.False(t, result.SentToDLQ) // Should not send to DLQ when circuit is open
	
	mockCB.AssertExpectations(t)
}

func TestRX_Shutdown(t *testing.T) {
	r := NewRX()
	err := r.Initialize(context.Background())
	assert.NoError(t, err)
	
	// Shutdown should succeed
	err = r.Shutdown(context.Background())
	assert.NoError(t, err)
	assert.False(t, r.Ready())
}
