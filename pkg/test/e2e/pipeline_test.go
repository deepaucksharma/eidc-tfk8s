package e2e

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
	"github.com/newrelic/nrdot-internal-devlab/pkg/fb/rx"
	"github.com/newrelic/nrdot-internal-devlab/pkg/fb/gw"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// MockChainServer implements the ChainPushService for testing
type MockChainServer struct {
	fb.UnimplementedChainPushServiceServer
	receivedBatches []*fb.MetricBatchRequest
	processingLogic func(*fb.MetricBatchRequest) error
}

func (s *MockChainServer) PushMetrics(ctx context.Context, req *fb.MetricBatchRequest) (*fb.MetricBatchResponse, error) {
	s.receivedBatches = append(s.receivedBatches, req)
	
	// If processing logic is provided, execute it
	if s.processingLogic != nil {
		if err := s.processingLogic(req); err != nil {
			return &fb.MetricBatchResponse{
				Status:       fb.StatusError,
				BatchId:      req.BatchId,
				ErrorCode:    fb.ErrorCodeProcessingFailed,
				ErrorMessage: err.Error(),
			}, nil
		}
	}
	
	return &fb.MetricBatchResponse{
		Status:  fb.StatusSuccess,
		BatchId: req.BatchId,
	}, nil
}

// setupGRPCServer creates an in-memory gRPC server for testing
func setupGRPCServer(server fb.ChainPushServiceServer) (*grpc.Server, *bufconn.Listener, string) {
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	fb.RegisterChainPushServiceServer(s, server)
	
	addr := fmt.Sprintf("bufnet-%p", s) // Generate unique address
	
	go func() {
		if err := s.Serve(lis); err != nil {
			panic(fmt.Sprintf("Server exited with error: %v", err))
		}
	}()
	
	return s, lis, addr
}

// bufDialer returns a dialer for the in-memory gRPC server
func bufDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, s string) (net.Conn, error) {
		return lis.Dial()
	}
}

// Test a simple pipeline with RX -> GW -> DLQ
func TestSimplePipeline_RXGW(t *testing.T) {
	ctx := context.Background()
	
	// Create mock servers for the next FB (GW) and DLQ
	gwServer := &MockChainServer{}
	gwGrpcServer, gwLis, gwAddr := setupGRPCServer(gwServer)
	defer gwGrpcServer.Stop()
	
	dlqServer := &MockChainServer{}
	dlqGrpcServer, dlqLis, dlqAddr := setupGRPCServer(dlqServer)
	defer dlqGrpcServer.Stop()
	
	// Create connectors for the mock servers
	gwDialer := bufDialer(gwLis)
	dlqDialer := bufDialer(dlqLis)
	
	// Create RX Function Block
	rxFB := rx.NewRX()
	err := rxFB.Initialize(ctx)
	assert.NoError(t, err)
	
	// Create a client connection to the GW mock server
	gwConn, err := grpc.DialContext(ctx, gwAddr,
		grpc.WithContextDialer(gwDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, err)
	defer gwConn.Close()
	
	// Create a client connection to the DLQ mock server
	dlqConn, err := grpc.DialContext(ctx, dlqAddr,
		grpc.WithContextDialer(dlqDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, err)
	defer dlqConn.Close()
	
	// Set clients manually for testing
	rxFB.SetNextFBClientForTesting(fb.NewChainPushServiceClient(gwConn))
	rxFB.SetDLQClientForTesting(fb.NewChainPushServiceClient(dlqConn))
	
	// Configure RX
	rxConfig := rx.RXConfig{
		Common: config.FBConfig{
			NextFB: gwAddr,
			DLQ:    dlqAddr,
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		Endpoints: []rx.Endpoint{
			{
				Protocol: "otlp/grpc",
				Port:     4317,
				Enabled:  true,
			},
		},
	}
	
	configBytes, err := json.Marshal(rxConfig)
	assert.NoError(t, err)
	
	err = rxFB.UpdateConfig(ctx, configBytes, 1)
	assert.NoError(t, err)
	
	// Create a test batch
	testBatch := &fb.MetricBatch{
		BatchID: "test-batch-001",
		Data:    []byte(`{"resource_metrics":[{"resource":{"attributes":{"service.name":"test-service"}}}]}`),
		Format:  "otlp",
		ConfigGeneration: 1,
		InternalLabels: map[string]string{
			"environment": "test",
		},
	}
	
	// Process the batch through RX
	result, err := rxFB.ProcessBatch(ctx, testBatch)
	assert.NoError(t, err)
	assert.Equal(t, fb.StatusSuccess, result.Status)
	
	// Verify the batch was forwarded to GW
	assert.Equal(t, 1, len(gwServer.receivedBatches))
	assert.Equal(t, testBatch.BatchID, gwServer.receivedBatches[0].BatchId)
	assert.Equal(t, testBatch.Format, gwServer.receivedBatches[0].Format)
	
	// Verify nothing was sent to DLQ
	assert.Equal(t, 0, len(dlqServer.receivedBatches))
}

// Test the error handling with schema validation in GW
func TestPipeline_SchemaValidation(t *testing.T) {
	ctx := context.Background()
	
	// Create mock servers for next FB and DLQ
	dlqServer := &MockChainServer{}
	dlqGrpcServer, dlqLis, dlqAddr := setupGRPCServer(dlqServer)
	defer dlqGrpcServer.Stop()
	
	// Create RX and GW function blocks
	rxFB := rx.NewRX()
	gwFB := gw.NewGW()
	
	err := rxFB.Initialize(ctx)
	assert.NoError(t, err)
	
	err = gwFB.Initialize(ctx)
	assert.NoError(t, err)
	
	// Create a client connection to the DLQ mock server
	dlqDialer := bufDialer(dlqLis)
	dlqConn, err := grpc.DialContext(ctx, dlqAddr,
		grpc.WithContextDialer(dlqDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, err)
	defer dlqConn.Close()
	
	// Setup RX server
	rxServer := &MockChainServer{
		processingLogic: func(req *fb.MetricBatchRequest) error {
			// Just forward to GW
			_, err := gwFB.ProcessBatch(ctx, &fb.MetricBatch{
				BatchID:          req.BatchId,
				Data:             req.Data,
				Format:           req.Format,
				ConfigGeneration: req.ConfigGeneration,
				InternalLabels:   req.InternalLabels,
				Metadata:         req.Metadata,
			})
			return err
		},
	}
	rxGrpcServer, rxLis, rxAddr := setupGRPCServer(rxServer)
	defer rxGrpcServer.Stop()
	
	// Set DLQ client for GW
	gwFB.SetDLQClientForTesting(fb.NewChainPushServiceClient(dlqConn))
	
	// Configure GW with schema validation
	gwConfig := gw.GWConfig{
		Common: config.FBConfig{
			NextFB: "fb-next:5000", // Not used in this test
			DLQ:    dlqAddr,
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,
				OpenStateSeconds:         5,
				HalfOpenRequestThreshold: 3,
			},
		},
		SchemaEnforce:       true,
		ExportEndpoint:      "https://metrics-api.example.com",
		PiiFields:           []string{"user.email"},
		EnablePiiDetection:  true,
	}
	
	configBytes, err := json.Marshal(gwConfig)
	assert.NoError(t, err)
	
	err = gwFB.UpdateConfig(ctx, configBytes, 1)
	assert.NoError(t, err)
	
	// Override the schema validator with a simple one
	gwFB.SetSchemaValidatorForTesting(schema.NewSimpleValidator(
		[]string{"resource_metrics"}, // Required fields
		[]string{"user.email"},       // PII fields
		true,                         // Enable PII detection
	))
	
	// Create a client to send batches
	rxDialer := bufDialer(rxLis)
	client, err := grpc.DialContext(ctx, rxAddr,
		grpc.WithContextDialer(rxDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, err)
	defer client.Close()
	
	chainClient := fb.NewChainPushServiceClient(client)
	
	// Test Case 1: Valid batch passes through
	validReq := &fb.MetricBatchRequest{
		BatchId: "valid-batch",
		Data:    []byte(`{"resource_metrics":[{"resource":{"attributes":{"service.name":"test-service"}}}]}`),
		Format:  "otlp",
		ConfigGeneration: 1,
	}
	
	res, err := chainClient.PushMetrics(ctx, validReq)
	assert.NoError(t, err)
	assert.Equal(t, fb.StatusSuccess, res.Status)
	
	// Test Case 2: Invalid batch (missing required field) gets rejected and sent to DLQ
	invalidReq := &fb.MetricBatchRequest{
		BatchId: "invalid-batch",
		Data:    []byte(`{"wrong_field":[{"resource":{"attributes":{"service.name":"test-service"}}}]}`),
		Format:  "otlp",
		ConfigGeneration: 1,
	}
	
	res, err = chainClient.PushMetrics(ctx, invalidReq)
	assert.NoError(t, err)
	assert.Equal(t, fb.StatusError, res.Status)
	
	// Verify the invalid batch was sent to DLQ
	assert.GreaterOrEqual(t, len(dlqServer.receivedBatches), 1)
	
	foundBatch := false
	for _, batch := range dlqServer.receivedBatches {
		if batch.BatchId == "invalid-batch" {
			foundBatch = true
			break
		}
	}
	assert.True(t, foundBatch, "Invalid batch should have been sent to DLQ")
}

// Test circuit breaker functionality
func TestPipeline_CircuitBreaker(t *testing.T) {
	ctx := context.Background()
	
	// Create mock servers
	failingServer := &MockChainServer{
		processingLogic: func(req *fb.MetricBatchRequest) error {
			// Always fail
			return fmt.Errorf("service unavailable")
		},
	}
	failingGrpcServer, failingLis, failingAddr := setupGRPCServer(failingServer)
	defer failingGrpcServer.Stop()
	
	dlqServer := &MockChainServer{}
	dlqGrpcServer, dlqLis, dlqAddr := setupGRPCServer(dlqServer)
	defer dlqGrpcServer.Stop()
	
	// Create RX function block
	rxFB := rx.NewRX()
	err := rxFB.Initialize(ctx)
	assert.NoError(t, err)
	
	// Create connections to mock servers
	failingDialer := bufDialer(failingLis)
	failingConn, err := grpc.DialContext(ctx, failingAddr,
		grpc.WithContextDialer(failingDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, err)
	defer failingConn.Close()
	
	dlqDialer := bufDialer(dlqLis)
	dlqConn, err := grpc.DialContext(ctx, dlqAddr,
		grpc.WithContextDialer(dlqDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, err)
	defer dlqConn.Close()
	
	// Set clients manually for testing
	rxFB.SetNextFBClientForTesting(fb.NewChainPushServiceClient(failingConn))
	rxFB.SetDLQClientForTesting(fb.NewChainPushServiceClient(dlqConn))
	
	// Configure RX with quick circuit breaker
	rxConfig := rx.RXConfig{
		Common: config.FBConfig{
			NextFB: failingAddr,
			DLQ:    dlqAddr,
			CircuitBreaker: config.CircuitBreakerConfig{
				ErrorThresholdPercentage: 50,   // Open after 50% errors
				OpenStateSeconds:         1,    // Open for 1 second
				HalfOpenRequestThreshold: 1,    // Try 1 request in half-open state
			},
		},
		Endpoints: []rx.Endpoint{
			{
				Protocol: "otlp/grpc",
				Port:     4317,
				Enabled:  true,
			},
		},
	}
	
	configBytes, err := json.Marshal(rxConfig)
	assert.NoError(t, err)
	
	err = rxFB.UpdateConfig(ctx, configBytes, 1)
	assert.NoError(t, err)
	
	// Create test batches
	createBatch := func(id string) *fb.MetricBatch {
		return &fb.MetricBatch{
			BatchID: id,
			Data:    []byte(`{"resource_metrics":[{"resource":{"attributes":{"service.name":"test-service"}}}]}`),
			Format:  "otlp",
			ConfigGeneration: 1,
		}
	}
	
	// Send first batch - should fail but not trip the circuit breaker yet
	result, err := rxFB.ProcessBatch(ctx, createBatch("batch-1"))
	assert.Error(t, err)
	assert.Equal(t, fb.StatusError, result.Status)
	assert.True(t, result.SentToDLQ)
	
	// Send second batch - should fail and trip the circuit breaker
	result, err = rxFB.ProcessBatch(ctx, createBatch("batch-2"))
	assert.Error(t, err)
	assert.Equal(t, fb.StatusError, result.Status)
	assert.True(t, result.SentToDLQ)
	
	// Send third batch - circuit should be open
	result, err = rxFB.ProcessBatch(ctx, createBatch("batch-3"))
	assert.Error(t, err)
	assert.Equal(t, fb.StatusError, result.Status)
	assert.Equal(t, fb.ErrorCodeCircuitBreakerOpen, result.ErrorCode)
	assert.False(t, result.SentToDLQ) // With open circuit, should not try to send to DLQ
	
	// Wait for circuit breaker to go to half-open state
	time.Sleep(1100 * time.Millisecond)
	
	// Send fourth batch - should attempt and fail again
	result, err = rxFB.ProcessBatch(ctx, createBatch("batch-4"))
	assert.Error(t, err)
	assert.Equal(t, fb.StatusError, result.Status)
	assert.True(t, result.SentToDLQ)
	
	// Circuit should be open again
	result, err = rxFB.ProcessBatch(ctx, createBatch("batch-5"))
	assert.Error(t, err)
	assert.Equal(t, fb.StatusError, result.Status)
	assert.Equal(t, fb.ErrorCodeCircuitBreakerOpen, result.ErrorCode)
	
	// Verify DLQ received the expected batches
	assert.Equal(t, 3, len(dlqServer.receivedBatches))
	expectedBatchIDs := map[string]bool{
		"batch-1": true,
		"batch-2": true,
		"batch-4": true,
	}
	
	for _, batch := range dlqServer.receivedBatches {
		assert.True(t, expectedBatchIDs[batch.BatchId], fmt.Sprintf("Unexpected batch in DLQ: %s", batch.BatchId))
	}
}
