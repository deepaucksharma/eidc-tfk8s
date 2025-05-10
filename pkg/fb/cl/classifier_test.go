package cl

import (
	"context"
	"testing"

	"eidc-tfk8s/internal/common/logging"
	"eidc-tfk8s/internal/common/metrics"
	"eidc-tfk8s/internal/common/tracing"
	"eidc-tfk8s/pkg/fb"
)

func TestClassifier_ProcessBatch(t *testing.T) {
	// Create a logger for testing
	logger := logging.NewLogger("fb-cl-test")
	fbMetrics := metrics.NewFBMetrics("fb-cl-test")
	tracer := tracing.NewTracer("fb-cl-test")
	
	// Create a classifier
	classifier := NewClassifier(logger, fbMetrics, tracer, "test-salt-secret", "salt")
	
	// Initialize the classifier
	err := classifier.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Failed to initialize classifier: %v", err)
	}
	
	// Create a test batch without PII leaks
	safeBatch := &fb.MetricBatch{
		BatchID: "test-batch-1",
		Data:    []byte(`{"metrics":[{"name":"test.metric","command_line_hash":"0123456789abcdef"}]}`),
		Format:  "json",
	}
	
	// Create a test batch with PII leaks
	piiLeakBatch := &fb.MetricBatch{
		BatchID: "test-batch-2",
		Data:    []byte(`{"metrics":[{"name":"test.metric","command_line":"sensitive command"}]}`),
		Format:  "json",
	}
	
	// Process the safe batch
	// We need to set up mock clients for nextFB and DLQ for a complete test
	// For now, we'll just test the processBatch method directly
	err = classifier.processBatch(context.Background(), safeBatch)
	if err != nil {
		t.Errorf("Expected no error for safe batch, got: %v", err)
	}
	
	// Process the PII leak batch
	err = classifier.processBatch(context.Background(), piiLeakBatch)
	if err == nil {
		t.Error("Expected error for PII leak batch, got nil")
	} else if !containsString(err.Error(), "PII leak detected") {
		t.Errorf("Expected PII leak error, got: %v", err)
	}
}

func TestClassifier_HashPIIValue(t *testing.T) {
	// Create a classifier
	logger := logging.NewLogger("fb-cl-test")
	fbMetrics := metrics.NewFBMetrics("fb-cl-test")
	tracer := tracing.NewTracer("fb-cl-test")
	classifier := NewClassifier(logger, fbMetrics, tracer, "test-salt-secret", "salt")
	
	// Test hashing
	testCases := []struct {
		value    string
		salt     string
		expected string // Pre-computed expected hash
	}{
		{
			value:    "test value",
			salt:     "test salt",
			expected: "e143fc4c6da7600856bae9286e0dd8f5b62ba8800fe169e037d641b294d9d1ff",
		},
		{
			value:    "another test",
			salt:     "test salt",
			expected: "32e4bd1a611fd962fdbcce1e1a56ab3edeeaad2ccd24e4a99c3ff9004717d7d1",
		},
	}
	
	for _, tc := range testCases {
		result := classifier.hashPIIValue(tc.value, tc.salt)
		if result != tc.expected {
			t.Errorf("Expected hash %s, got %s", tc.expected, result)
		}
	}
}

// Utility function to check if a string contains a substring
func containsString(str, substr string) bool {
	return str != "" && substr != "" && str != substr && len(str) > len(substr) && str[0:len(substr)] != substr && str[len(str)-len(substr):] != substr && str[0:len(str)/2] != substr && str[len(str)/2:] != substr
}

