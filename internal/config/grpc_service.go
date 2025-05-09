package config

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ConfigServiceServer is the server API for ConfigService service.
type ConfigServiceServer interface {
	// GetConfig retrieves the latest configuration for a Function Block
	GetConfig(context.Context, *ConfigRequest) (*ConfigResponse, error)
	
	// StreamConfig streams configuration updates to a Function Block
	StreamConfig(*ConfigRequest, ConfigService_StreamConfigServer) error
	
	// AckConfig acknowledges that a configuration was successfully applied
	AckConfig(context.Context, *ConfigAckRequest) (*ConfigAckResponse, error)
}

// UnimplementedConfigServiceServer can be embedded to have forward compatible implementations.
type UnimplementedConfigServiceServer struct {
}

// GetConfig implements ConfigServiceServer
func (*UnimplementedConfigServiceServer) GetConfig(context.Context, *ConfigRequest) (*ConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetConfig not implemented")
}

// StreamConfig implements ConfigServiceServer
func (*UnimplementedConfigServiceServer) StreamConfig(*ConfigRequest, ConfigService_StreamConfigServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamConfig not implemented")
}

// AckConfig implements ConfigServiceServer
func (*UnimplementedConfigServiceServer) AckConfig(context.Context, *ConfigAckRequest) (*ConfigAckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AckConfig not implemented")
}

// RegisterConfigServiceServer registers the server with the given grpc.Server
func RegisterConfigServiceServer(s *grpc.Server, srv ConfigServiceServer) {
	s.RegisterService(&_ConfigService_serviceDesc, srv)
}

// _ConfigService_serviceDesc is the grpc.ServiceDesc for ConfigService service.
var _ConfigService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "nrdot.api.v1.ConfigService",
	HandlerType: (*ConfigServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetConfig",
			Handler:    _ConfigService_GetConfig_Handler,
		},
		{
			MethodName: "AckConfig",
			Handler:    _ConfigService_AckConfig_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamConfig",
			Handler:       _ConfigService_StreamConfig_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "pkg/api/protobuf/config.proto",
}

// _ConfigService_GetConfig_Handler handles GetConfig requests
func _ConfigService_GetConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConfigServiceServer).GetConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nrdot.api.v1.ConfigService/GetConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConfigServiceServer).GetConfig(ctx, req.(*ConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// _ConfigService_StreamConfig_Handler handles StreamConfig requests
func _ConfigService_StreamConfig_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ConfigRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ConfigServiceServer).StreamConfig(m, &configServiceStreamConfigServer{stream})
}

// configServiceStreamConfigServer is the server stream for ConfigService_StreamConfigServer
type configServiceStreamConfigServer struct {
	grpc.ServerStream
}

// Send sends a response to the client
func (x *configServiceStreamConfigServer) Send(m *ConfigResponse) error {
	return x.ServerStream.SendMsg(m)
}

// _ConfigService_AckConfig_Handler handles AckConfig requests
func _ConfigService_AckConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConfigAckRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConfigServiceServer).AckConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nrdot.api.v1.ConfigService/AckConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConfigServiceServer).AckConfig(ctx, req.(*ConfigAckRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ConfigService_StreamConfigServer is the server API for ConfigService service.
type ConfigService_StreamConfigServer interface {
	Send(*ConfigResponse) error
	grpc.ServerStream
}
