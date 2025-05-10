package fb

import (
	"context"

	"google.golang.org/grpc"
)

// ChainPushServiceClient is the client API for ChainPushService.
type ChainPushServiceClient interface {
	// PushMetrics pushes a batch of metrics to the next FB in the chain
	PushMetrics(ctx context.Context, in *MetricBatchRequest, opts ...grpc.CallOption) (*MetricBatchResponse, error)
}

// MetricBatchRequest contains a batch of metrics to be processed
type MetricBatchRequest struct {
	// Unique identifier for this batch
	BatchId string `json:"batch_id"`
	
	// The serialized metric batch data
	Data []byte `json:"data"`
	
	// Format of the data (e.g., "otlp", "prometheus")
	Format string `json:"format"`
	
	// Whether this is a replay from DLQ
	Replay bool `json:"replay"`
	
	// Configuration generation applied to this batch
	ConfigGeneration int64 `json:"config_generation"`
	
	// Metadata for processing
	Metadata map[string]string `json:"metadata"`
	
	// Internal labels for pipeline processing
	InternalLabels map[string]string `json:"internal_labels"`
}

// MetricBatchResponse contains the result of processing a metric batch
type MetricBatchResponse struct {
	// Status of the operation
	Status Status `json:"status"`
	
	// Error message, if any
	ErrorMessage string `json:"error_message,omitempty"`
	
	// Error code, if any
	ErrorCode string `json:"error_code,omitempty"`
	
	// Batch ID echo
	BatchId string `json:"batch_id"`
}

// chainPushServiceClient is an implementation of ChainPushServiceClient.
type chainPushServiceClient struct {
	cc *grpc.ClientConn
}

// NewChainPushServiceClient creates a new ChainPushService client
func NewChainPushServiceClient(cc *grpc.ClientConn) ChainPushServiceClient {
	return &chainPushServiceClient{cc}
}

// PushMetrics pushes a batch of metrics to the next FB in the chain
func (c *chainPushServiceClient) PushMetrics(ctx context.Context, in *MetricBatchRequest, opts ...grpc.CallOption) (*MetricBatchResponse, error) {
	out := new(MetricBatchResponse)
	err := c.cc.Invoke(ctx, "/nrdot.api.v1.ChainPushService/PushMetrics", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}
// The ChainPushServiceServer interface is defined in grpc_service.go

// MockChainPushServiceClient is a mock implementation of ChainPushServiceClient for testing
type MockChainPushServiceClient struct {
	PushMetricsFunc func(ctx context.Context, in *MetricBatchRequest, opts ...grpc.CallOption) (*MetricBatchResponse, error)
}

// PushMetrics mocks the PushMetrics method
func (m *MockChainPushServiceClient) PushMetrics(ctx context.Context, in *MetricBatchRequest, opts ...grpc.CallOption) (*MetricBatchResponse, error) {
	if m.PushMetricsFunc != nil {
		return m.PushMetricsFunc(ctx, in, opts...)
	}
	return &MetricBatchResponse{
		Status:  StatusSuccess,
		BatchId: in.BatchId,
	}, nil
}

// MockChainPushServiceServer is a mock implementation of ChainPushServiceServer for testing
type MockChainPushServiceServer struct {
	PushMetricsFunc func(ctx context.Context, in *MetricBatchRequest) (*MetricBatchResponse, error)
}

// PushMetrics mocks the PushMetrics method
func (m *MockChainPushServiceServer) PushMetrics(ctx context.Context, in *MetricBatchRequest) (*MetricBatchResponse, error) {
	if m.PushMetricsFunc != nil {
		return m.PushMetricsFunc(ctx, in)
	}
	return &MetricBatchResponse{
		Status:  StatusSuccess,
		BatchId: in.BatchId,
	}, nil
}
