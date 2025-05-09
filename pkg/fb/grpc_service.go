package fb

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ChainPushServiceServer is the server API for ChainPushService.
type ChainPushServiceServer interface {
	// PushMetrics processes a batch of metrics
	PushMetrics(context.Context, *MetricBatchRequest) (*MetricBatchResponse, error)
}

// UnimplementedChainPushServiceServer can be embedded to have forward compatible implementations.
type UnimplementedChainPushServiceServer struct {
}

// PushMetrics implements ChainPushServiceServer
func (*UnimplementedChainPushServiceServer) PushMetrics(context.Context, *MetricBatchRequest) (*MetricBatchResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PushMetrics not implemented")
}

// RegisterChainPushServiceServer registers the server with the given grpc.Server
func RegisterChainPushServiceServer(s *grpc.Server, srv ChainPushServiceServer) {
	s.RegisterService(&_ChainPushService_serviceDesc, srv)
}

// _ChainPushService_serviceDesc is the grpc.ServiceDesc for ChainPushService service.
var _ChainPushService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "nrdot.api.v1.ChainPushService",
	HandlerType: (*ChainPushServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "PushMetrics",
			Handler:    _ChainPushService_PushMetrics_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "pkg/api/protobuf/chain.proto",
}

// _ChainPushService_PushMetrics_Handler handles PushMetrics requests
func _ChainPushService_PushMetrics_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MetricBatchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ChainPushServiceServer).PushMetrics(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nrdot.api.v1.ChainPushService/PushMetrics",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ChainPushServiceServer).PushMetrics(ctx, req.(*MetricBatchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ChainPushServiceHandler implements ChainPushServiceServer by delegating to a FunctionBlock
type ChainPushServiceHandler struct {
	fb FunctionBlock
}

// NewChainPushServiceHandler creates a new ChainPushServiceHandler
func NewChainPushServiceHandler(fb FunctionBlock) *ChainPushServiceHandler {
	return &ChainPushServiceHandler{fb: fb}
}

// PushMetrics implements ChainPushServiceServer.PushMetrics
func (h *ChainPushServiceHandler) PushMetrics(ctx context.Context, req *MetricBatchRequest) (*MetricBatchResponse, error) {
	// Convert request to MetricBatch
	batch := &MetricBatch{
		BatchID:          req.BatchId,
		Data:             req.Data,
		Format:           req.Format,
		Replay:           req.Replay,
		ConfigGeneration: req.ConfigGeneration,
		Metadata:         req.Metadata,
		InternalLabels:   req.InternalLabels,
	}

	// Process the batch
	result, err := h.fb.ProcessBatch(ctx, batch)
	if err != nil {
		// Return error response with status from result
		return &MetricBatchResponse{
			Status:       result.Status,
			ErrorMessage: result.ErrorMessage,
			ErrorCode:    string(result.ErrorCode),
			BatchId:      req.BatchId,
		}, nil
	}

	// Return success response
	return &MetricBatchResponse{
		Status:  result.Status,
		BatchId: req.BatchId,
	}, nil
}
