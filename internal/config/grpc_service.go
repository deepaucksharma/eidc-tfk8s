// ConfigServiceClient is the client API for ConfigService.
type ConfigServiceClient interface {
	// GetConfig gets the latest configuration
	GetConfig(ctx context.Context, in *ConfigRequest, opts ...grpc.CallOption) (*ConfigResponse, error)
	
	// StreamConfig streams configuration updates
	StreamConfig(ctx context.Context, in *ConfigRequest, opts ...grpc.CallOption) (ConfigService_StreamConfigClient, error)
	
	// AckConfig acknowledges a configuration update
	AckConfig(ctx context.Context, in *ConfigAckRequest, opts ...grpc.CallOption) (*ConfigAckResponse, error)
}

// ConfigService_StreamConfigClient is the client API for ConfigService_StreamConfig.
type ConfigService_StreamConfigClient interface {
	Recv() (*ConfigResponse, error)
	grpc.ClientStream
}

// ConfigRequest is a request for configuration
type ConfigRequest struct {
	// Function block name
	FbName string `json:"fb_name"`
	
	// Instance ID
	InstanceId string `json:"instance_id"`
	
	// Last known configuration generation
	LastKnownGeneration int64 `json:"last_known_generation"`
}

// ConfigResponse is a response containing configuration
type ConfigResponse struct {
	// Configuration bytes
	Config []byte `json:"config"`
	
	// Generation number
	Generation int64 `json:"generation"`
	
	// Whether the configuration requires a restart
	RequiresRestart bool `json:"requires_restart"`
}

// ConfigAckRequest is a request to acknowledge a configuration update
type ConfigAckRequest struct {
	// Function block name
	FbName string `json:"fb_name"`
	
	// Instance ID
	InstanceId string `json:"instance_id"`
	
	// Generation number
	Generation int64 `json:"generation"`
	
	// Whether the config was successfully applied
	Success bool `json:"success"`
	
	// Error message, if any
	ErrorMessage string `json:"error_message,omitempty"`
}

// ConfigAckResponse is a response to a config acknowledgement
type ConfigAckResponse struct {
	// Success
	Success bool `json:"success"`
}

// NewConfigServiceClient creates a new config service client
func NewConfigServiceClient(cc *grpc.ClientConn) ConfigServiceClient {
	return &configServiceClient{cc}
}

// configServiceClient is an implementation of ConfigServiceClient
type configServiceClient struct {
	cc *grpc.ClientConn
}

// GetConfig gets the latest configuration
func (c *configServiceClient) GetConfig(ctx context.Context, in *ConfigRequest, opts ...grpc.CallOption) (*ConfigResponse, error) {
	out := new(ConfigResponse)
	err := c.cc.Invoke(ctx, "/nrdot.api.v1.ConfigService/GetConfig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StreamConfig streams configuration updates
func (c *configServiceClient) StreamConfig(ctx context.Context, in *ConfigRequest, opts ...grpc.CallOption) (ConfigService_StreamConfigClient, error) {
	stream, err := c.cc.NewStream(ctx, &_ConfigService_serviceDesc.Streams[0], "/nrdot.api.v1.ConfigService/StreamConfig", opts...)
	if err != nil {
		return nil, err
	}
	x := &configServiceStreamConfigClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// configServiceStreamConfigClient is the client API for the StreamConfig method
type configServiceStreamConfigClient struct {
	grpc.ClientStream
}

// Recv receives a ConfigResponse
func (x *configServiceStreamConfigClient) Recv() (*ConfigResponse, error) {
	m := new(ConfigResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// AckConfig acknowledges a configuration update
func (c *configServiceClient) AckConfig(ctx context.Context, in *ConfigAckRequest, opts ...grpc.CallOption) (*ConfigAckResponse, error) {
	out := new(ConfigAckResponse)
	err := c.cc.Invoke(ctx, "/nrdot.api.v1.ConfigService/AckConfig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ConfigServiceServer is the server API for ConfigService.
type ConfigServiceServer interface {
	// GetConfig gets the latest configuration
	GetConfig(context.Context, *ConfigRequest) (*ConfigResponse, error)
	
	// StreamConfig streams configuration updates
	StreamConfig(*ConfigRequest, ConfigService_StreamConfigServer) error
	
	// AckConfig acknowledges a configuration update
	AckConfig(context.Context, *ConfigAckRequest) (*ConfigAckResponse, error)
}

// ConfigService_StreamConfigServer is the server API for ConfigService_StreamConfig.
type ConfigService_StreamConfigServer interface {
	Send(*ConfigResponse) error
	grpc.ServerStream
}

// UnimplementedConfigServiceServer can be embedded to have forward compatible implementations.
type UnimplementedConfigServiceServer struct {
}

// GetConfig implements ConfigServiceServer.GetConfig
func (*UnimplementedConfigServiceServer) GetConfig(context.Context, *ConfigRequest) (*ConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetConfig not implemented")
}

// StreamConfig implements ConfigServiceServer.StreamConfig
func (*UnimplementedConfigServiceServer) StreamConfig(*ConfigRequest, ConfigService_StreamConfigServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamConfig not implemented")
}

// AckConfig implements ConfigServiceServer.AckConfig
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
	in := new(ConfigRequest)
	if err := stream.RecvMsg(in); err != nil {
		return err
	}
	return srv.(ConfigServiceServer).StreamConfig(in, &configServiceStreamConfigServer{stream})
}

// configServiceStreamConfigServer is the server API for the StreamConfig method
type configServiceStreamConfigServer struct {
	grpc.ServerStream
}

// Send sends a ConfigResponse
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