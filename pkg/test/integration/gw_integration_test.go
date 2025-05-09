package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/newrelic/nrdot-internal-devlab/internal/common/schema"
	"github.com/newrelic/nrdot-internal-devlab/internal/config"
	"github.com/newrelic/nrdot-internal-devlab/pkg/fb"
	"github.com/newrelic/nrdot-internal-devlab/pkg/fb/gw"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// mockDLQServer is a mock implementation of the DLQ gRPC server
type mockDLQServer struct {
	fb.UnimplementedChainPushServiceServer
	receivedBatches []*fb.MetricBatchRequest
}

func (s *mockDLQServer) PushMetrics(ctx context.Context, req *fb.MetricBatchRequest) (*fb.MetricBatchResponse, error) {
	s.receivedBatches = append(s.receivedBatches, req)
	return &fb.MetricBatchResponse{
		Status:       fb.StatusSuccess,
		BatchId:      req.BatchId,
		ErrorCode:    "",
		ErrorMessage: "",
	}, nil
}

// setupGRPCServer creates an in-memory gRPC server for testing
func setupGRPCServer(server fb.ChainPushServiceServer) (*grpc.Server, *bufconn.Listener) {
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	fb.RegisterChainPushServiceServer(s, server)
	go func() {
		if err := s.Serve(lis); err != nil {
			panic(fmt.Sprintf("Server exited with error: %v", err))
		}
	}()
	return s, lis
}

// bufDialer returns a dialer for the in-memory gRPC server
func bufDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, s string) (net.Conn, error) {
		return lis.Dial()
	}
}

func TestGatewayIntegration_SchemaValidation(t *testing.T) {
	// Setup DLQ mock server
	dlqServer := &mockDLQServer{}
	grpcServer, lis := setupGRPCServer(dlqServer)
	defer grpcServer.Stop()

	// Create context with dialer for in-memory connection
	ctx := context.Background()
	
	// Create FB-GW instance
	gateway := gw.NewGW()
	err := gateway.Initialize(ctx)
	assert.NoError(t, err)
	
	// Connect to DLQ
	dlqAddr := "bufnet"
	conn, err := grpc.DialContext(ctx, dlqAddr,
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, err)
	defer conn.Close()
	
	// Set up DLQ client manually for testing
	gateway.SetDLQClientForTesting(fb.NewChainPushServiceClient(conn))
	
	// Create a config with schema validation enabled
	gwConfig := gw.GWConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    dlqAddr,
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		SchemaEnforce:       true,
		ExportEndpoint:      "https://metrics-api.example.com",
		PiiFields:           []string{"user.email", "user.phone"},
		EnablePiiDetection:  true,
	}
	
	configBytes, err := json.Marshal(gwConfig)
	assert.NoError(t, err)
	
	err = gateway.UpdateConfig(ctx, configBytes, 1)
	assert.NoError(t, err)
	
	// Test Case 1: Valid batch data
	validData := map[string]interface{}{
		"resource_metrics": []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": map[string]interface{}{
						"service.name": "test-service",
					},
				},
				"scope_metrics": []interface{}{
					map[string]interface{}{
						"metrics": []interface{}{
							map[string]interface{}{
								"name":  "test.metric",
								"value": 42.0,
							},
						},
					},
				},
			},
		},
		"internal_labels": map[string]interface{}{
			"environment": "test",
		},
	}
	validDataBytes, err := json.Marshal(validData)
	assert.NoError(t, err)
	
	validBatch := &fb.MetricBatch{
		BatchID: "test-valid-batch",
		Data:    validDataBytes,
		Format:  "otlp",
	}
	
	// Process the valid batch - should succeed
	result, err := gateway.ProcessBatch(ctx, validBatch)
	assert.NoError(t, err)
	assert.Equal(t, fb.StatusSuccess, result.Status)
	assert.Equal(t, 0, len(dlqServer.receivedBatches))
	
	// Test Case 2: Invalid batch data (missing required field)
	invalidData := map[string]interface{}{
		// Missing the required resource_metrics field
		"internal_labels": map[string]interface{}{
			"environment": "test",
		},
	}
	invalidDataBytes, err := json.Marshal(invalidData)
	assert.NoError(t, err)
	
	invalidBatch := &fb.MetricBatch{
		BatchID: "test-invalid-batch",
		Data:    invalidDataBytes,
		Format:  "otlp",
	}
	
	// Override the schema validator with a custom one for testing
	gateway.SetSchemaValidatorForTesting(schema.NewSimpleValidator(
		[]string{"resource_metrics"}, // required fields
		[]string{},                  // PII fields
		false,                       // PII detection disabled for simplicity
	))
	
	// Process the invalid batch - should fail and be sent to DLQ
	result, err = gateway.ProcessBatch(ctx, invalidBatch)
	assert.Error(t, err)
	assert.Equal(t, fb.StatusError, result.Status)
	assert.Equal(t, fb.ErrorCodeInvalidInput, result.ErrorCode)
	assert.True(t, result.SentToDLQ)
	
	// Verify the batch was sent to DLQ
	assert.Equal(t, 1, len(dlqServer.receivedBatches))
	assert.Equal(t, invalidBatch.BatchID, dlqServer.receivedBatches[0].BatchId)
	assert.Equal(t, string(fb.ErrorCodeInvalidInput), dlqServer.receivedBatches[0].InternalLabels["error_code"])
}

func TestGatewayIntegration_PiiDetection(t *testing.T) {
	// Setup DLQ mock server
	dlqServer := &mockDLQServer{}
	grpcServer, lis := setupGRPCServer(dlqServer)
	defer grpcServer.Stop()
	
	// Create context with dialer for in-memory connection
	ctx := context.Background()
	
	// Create FB-GW instance
	gateway := gw.NewGW()
	err := gateway.Initialize(ctx)
	assert.NoError(t, err)
	
	// Connect to DLQ
	dlqAddr := "bufnet"
	conn, err := grpc.DialContext(ctx, dlqAddr,
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, err)
	defer conn.Close()
	
	// Set up DLQ client manually for testing
	gateway.SetDLQClientForTesting(fb.NewChainPushServiceClient(conn))
	
	// Create a config with schema validation and PII detection enabled
	gwConfig := gw.GWConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000",
			DLQ:    dlqAddr,
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		SchemaEnforce:       true,
		ExportEndpoint:      "https://metrics-api.example.com",
		PiiFields:           []string{"user.email", "user.phone"},
		EnablePiiDetection:  true,
	}
	
	configBytes, err := json.Marshal(gwConfig)
	assert.NoError(t, err)
	
	err = gateway.UpdateConfig(ctx, configBytes, 1)
	assert.NoError(t, err)
	
	// Test Case: Batch with unhashed PII
	dataWithPii := map[string]interface{}{
		"resource_metrics": []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": map[string]interface{}{
						"service.name": "test-service",
						"user.email":   "test@example.com", // Unhashed PII
					},
				},
			},
		},
	}
	dataWithPiiBytes, err := json.Marshal(dataWithPii)
	assert.NoError(t, err)
	
	batchWithPii := &fb.MetricBatch{
		BatchID: "test-pii-batch",
		Data:    dataWithPiiBytes,
		Format:  "otlp",
	}
	
	// Override the schema validator with a custom one for testing
	// that specifically checks for "user.email" as a PII field
	gateway.SetSchemaValidatorForTesting(schema.NewSimpleValidator(
		[]string{"resource_metrics"}, // required fields
		[]string{"user.email"},      // PII fields to check
		true,                       // PII detection enabled
	))
	
	// Process the batch with PII - should fail and be sent to DLQ
	result, err := gateway.ProcessBatch(ctx, batchWithPii)
	assert.Error(t, err)
	assert.Equal(t, fb.StatusError, result.Status)
	assert.Contains(t, err.Error(), "PII")
	assert.True(t, result.SentToDLQ)
	
	// Verify the batch was sent to DLQ
	assert.Equal(t, 1, len(dlqServer.receivedBatches))
	assert.Equal(t, batchWithPii.BatchID, dlqServer.receivedBatches[0].BatchId)
	assert.Contains(t, dlqServer.receivedBatches[0].InternalLabels["error"], "PII")
}
