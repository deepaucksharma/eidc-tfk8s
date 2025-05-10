package gw

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"eidc-tfk8s/internal/common/schema"
	"eidc-tfk8s/internal/config"
	"eidc-tfk8s/pkg/fb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSchemaValidator is a mock schema validator for testing
type MockSchemaValidator struct {
	mock.Mock
}

func (m *MockSchemaValidator) Validate(data interface{}) *schema.ValidationResult {
	args := m.Called(data)
	return args.Get(0).(*schema.ValidationResult)
}

// MockDLQClient is a mock DLQ client for testing
type MockDLQClient struct {
	mock.Mock
}

func (m *MockDLQClient) PushMetrics(ctx context.Context, in *fb.MetricBatchRequest, opts ...interface{}) (*fb.MetricBatchResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*fb.MetricBatchResponse), args.Error(1)
}

func TestGW_Initialize(t *testing.T) {
	g := NewGW()
	err := g.Initialize(context.Background())
	assert.NoError(t, err)
	assert.True(t, g.Ready())
}

func TestGW_UpdateConfig(t *testing.T) {
	g := NewGW()
	err := g.Initialize(context.Background())
	assert.NoError(t, err)

	// Test with valid config
	validConfig := GWConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    "fb-dlq:5000",
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		SchemaEnforce:   true,
		ExportEndpoint:  "https://metrics-api.example.com",
		PiiFields:       []string{"user.email", "user.phone"},
		EnablePiiDetection: true,
	}

	configBytes, err := json.Marshal(validConfig)
	assert.NoError(t, err)

	err = g.UpdateConfig(context.Background(), configBytes, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), g.GetConfigGeneration())

	// Test with invalid config (missing export endpoint)
	invalidConfig := GWConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    "fb-dlq:5000",
		},
		SchemaEnforce:   true,
		ExportEndpoint:  "", // Invalid - empty export endpoint
		PiiFields:       []string{"user.email", "user.phone"},
		EnablePiiDetection: true,
	}

	configBytes, err = json.Marshal(invalidConfig)
	assert.NoError(t, err)

	err = g.UpdateConfig(context.Background(), configBytes, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "export endpoint not configured")
}

func TestGW_ProcessBatch_ValidData(t *testing.T) {
	g := NewGW()
	err := g.Initialize(context.Background())
	assert.NoError(t, err)

	// Setup mock schema validator
	mockValidator := new(MockSchemaValidator)
	g.schemaValidator = mockValidator

	// Configure with valid config
	validConfig := GWConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    "fb-dlq:5000",
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		SchemaEnforce:   true,
		ExportEndpoint:  "https://metrics-api.example.com",
		PiiFields:       []string{"user.email", "user.phone"},
		EnablePiiDetection: true,
	}

	configBytes, err := json.Marshal(validConfig)
	assert.NoError(t, err)

	g.UpdateConfig(context.Background(), configBytes, 1)

	// Valid batch data
	validData := map[string]interface{}{
		"resource_metrics": []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": map[string]interface{}{
						"service.name": "test-service",
					},
				},
			},
		},
	}

	validDataBytes, err := json.Marshal(validData)
	assert.NoError(t, err)

	batch := &fb.MetricBatch{
		BatchID: "test-batch-id",
		Data:    validDataBytes,
		Format:  "otlp",
	}

	// Expect validation to succeed
	mockValidator.On("Validate", mock.Anything).Return(&schema.ValidationResult{
		Valid: true,
	})

	// Process the batch
	result, err := g.ProcessBatch(context.Background(), batch)
	assert.NoError(t, err)
	assert.Equal(t, fb.StatusSuccess, result.Status)
	mockValidator.AssertExpectations(t)
}

func TestGW_ProcessBatch_InvalidData(t *testing.T) {
	g := NewGW()
	err := g.Initialize(context.Background())
	assert.NoError(t, err)

	// Setup mock schema validator
	mockValidator := new(MockSchemaValidator)
	g.schemaValidator = mockValidator

	// Setup mock DLQ client
	mockDLQClient := new(MockDLQClient)
	g.dlqClient = mockDLQClient

	// Configure with valid config
	validConfig := GWConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    "fb-dlq:5000",
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		SchemaEnforce:   true,
		ExportEndpoint:  "https://metrics-api.example.com",
		PiiFields:       []string{"user.email", "user.phone"},
		EnablePiiDetection: true,
	}

	configBytes, err := json.Marshal(validConfig)
	assert.NoError(t, err)

	g.UpdateConfig(context.Background(), configBytes, 1)

	// Invalid batch data (PII not hashed)
	invalidData := map[string]interface{}{
		"resource_metrics": []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": map[string]interface{}{
						"user.email": "test@example.com", // unhashed PII
					},
				},
			},
		},
	}

	invalidDataBytes, err := json.Marshal(invalidData)
	assert.NoError(t, err)

	batch := &fb.MetricBatch{
		BatchID: "test-batch-id",
		Data:    invalidDataBytes,
		Format:  "otlp",
	}

	// Expect validation to fail with PII error
	validationErr := errors.New("PII field detected without hashing")
	mockValidator.On("Validate", mock.Anything).Return(&schema.ValidationResult{
		Valid: false,
		Error: validationErr,
		Path:  "user.email",
	})

	// Expect DLQ push to succeed
	mockDLQClient.On("PushMetrics", mock.Anything, mock.Anything).Return(&fb.MetricBatchResponse{
		Status: fb.StatusSuccess,
	}, nil)

	// Process the batch
	result, err := g.ProcessBatch(context.Background(), batch)
	assert.Error(t, err)
	assert.Equal(t, validationErr.Error(), err.Error())
	assert.Equal(t, fb.StatusError, result.Status)
	assert.Equal(t, fb.ErrorCodeInvalidInput, result.ErrorCode)
	assert.True(t, result.SentToDLQ)

	mockValidator.AssertExpectations(t)
	mockDLQClient.AssertExpectations(t)
}

func TestGW_ProcessBatch_DLQFailure(t *testing.T) {
	g := NewGW()
	err := g.Initialize(context.Background())
	assert.NoError(t, err)

	// Setup mock schema validator
	mockValidator := new(MockSchemaValidator)
	g.schemaValidator = mockValidator

	// Setup mock DLQ client
	mockDLQClient := new(MockDLQClient)
	g.dlqClient = mockDLQClient

	// Configure with valid config
	validConfig := GWConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    "fb-dlq:5000",
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		SchemaEnforce:   true,
		ExportEndpoint:  "https://metrics-api.example.com",
		PiiFields:       []string{"user.email", "user.phone"},
		EnablePiiDetection: true,
	}

	configBytes, err := json.Marshal(validConfig)
	assert.NoError(t, err)

	g.UpdateConfig(context.Background(), configBytes, 1)

	// Invalid batch data
	invalidData := map[string]interface{}{
		"resource_metrics": []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": map[string]interface{}{
						"user.email": "test@example.com", // unhashed PII
					},
				},
			},
		},
	}

	invalidDataBytes, err := json.Marshal(invalidData)
	assert.NoError(t, err)

	batch := &fb.MetricBatch{
		BatchID: "test-batch-id",
		Data:    invalidDataBytes,
		Format:  "otlp",
	}

	// Expect validation to fail
	validationErr := errors.New("PII field detected without hashing")
	mockValidator.On("Validate", mock.Anything).Return(&schema.ValidationResult{
		Valid: false,
		Error: validationErr,
		Path:  "user.email",
	})

	// Expect DLQ push to fail
	dlqErr := errors.New("DLQ service unavailable")
	mockDLQClient.On("PushMetrics", mock.Anything, mock.Anything).Return(nil, dlqErr)

	// Process the batch
	result, err := g.ProcessBatch(context.Background(), batch)
	assert.Error(t, err)
	assert.Equal(t, fb.StatusError, result.Status)
	assert.Equal(t, fb.ErrorCodeDLQSendFailed, result.ErrorCode)
	assert.False(t, result.SentToDLQ)

	mockValidator.AssertExpectations(t)
	mockDLQClient.AssertExpectations(t)
}

func TestGW_Shutdown(t *testing.T) {
	g := NewGW()
	err := g.Initialize(context.Background())
	assert.NoError(t, err)
	
	// Mock a connection that should be closed
	mockDLQClient := new(MockDLQClient)
	g.dlqClient = mockDLQClient
	
	// Shutdown should succeed
	err = g.Shutdown(context.Background())
	assert.NoError(t, err)
	assert.False(t, g.Ready())
}

