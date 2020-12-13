// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package api

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion7

// ApiClient is the client API for Api service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ApiClient interface {
	Run(ctx context.Context, in *ContainerRequest, opts ...grpc.CallOption) (*ContainerResponse, error)
}

type apiClient struct {
	cc grpc.ClientConnInterface
}

func NewApiClient(cc grpc.ClientConnInterface) ApiClient {
	return &apiClient{cc}
}

func (c *apiClient) Run(ctx context.Context, in *ContainerRequest, opts ...grpc.CallOption) (*ContainerResponse, error) {
	out := new(ContainerResponse)
	err := c.cc.Invoke(ctx, "/api.Api/Run", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ApiServer is the server API for Api service.
// All implementations must embed UnimplementedApiServer
// for forward compatibility
type ApiServer interface {
	Run(context.Context, *ContainerRequest) (*ContainerResponse, error)
	mustEmbedUnimplementedApiServer()
}

// UnimplementedApiServer must be embedded to have forward compatible implementations.
type UnimplementedApiServer struct {
}

func (UnimplementedApiServer) Run(context.Context, *ContainerRequest) (*ContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Run not implemented")
}
func (UnimplementedApiServer) mustEmbedUnimplementedApiServer() {}

// UnsafeApiServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ApiServer will
// result in compilation errors.
type UnsafeApiServer interface {
	mustEmbedUnimplementedApiServer()
}

func RegisterApiServer(s grpc.ServiceRegistrar, srv ApiServer) {
	s.RegisterService(&_Api_serviceDesc, srv)
}

func _Api_Run_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ContainerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApiServer).Run(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Api/Run",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApiServer).Run(ctx, req.(*ContainerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Api_serviceDesc = grpc.ServiceDesc{
	ServiceName: "api.Api",
	HandlerType: (*ApiServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Run",
			Handler:    _Api_Run_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/api.proto",
}

// ContainerStreamerClient is the client API for ContainerStreamer service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ContainerStreamerClient interface {
	Communicate(ctx context.Context, opts ...grpc.CallOption) (ContainerStreamer_CommunicateClient, error)
}

type containerStreamerClient struct {
	cc grpc.ClientConnInterface
}

func NewContainerStreamerClient(cc grpc.ClientConnInterface) ContainerStreamerClient {
	return &containerStreamerClient{cc}
}

func (c *containerStreamerClient) Communicate(ctx context.Context, opts ...grpc.CallOption) (ContainerStreamer_CommunicateClient, error) {
	stream, err := c.cc.NewStream(ctx, &_ContainerStreamer_serviceDesc.Streams[0], "/api.ContainerStreamer/Communicate", opts...)
	if err != nil {
		return nil, err
	}
	x := &containerStreamerCommunicateClient{stream}
	return x, nil
}

type ContainerStreamer_CommunicateClient interface {
	Send(*ContainerMessage) error
	Recv() (*ContainerMessage, error)
	grpc.ClientStream
}

type containerStreamerCommunicateClient struct {
	grpc.ClientStream
}

func (x *containerStreamerCommunicateClient) Send(m *ContainerMessage) error {
	return x.ClientStream.SendMsg(m)
}

func (x *containerStreamerCommunicateClient) Recv() (*ContainerMessage, error) {
	m := new(ContainerMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ContainerStreamerServer is the server API for ContainerStreamer service.
// All implementations must embed UnimplementedContainerStreamerServer
// for forward compatibility
type ContainerStreamerServer interface {
	Communicate(ContainerStreamer_CommunicateServer) error
	mustEmbedUnimplementedContainerStreamerServer()
}

// UnimplementedContainerStreamerServer must be embedded to have forward compatible implementations.
type UnimplementedContainerStreamerServer struct {
}

func (UnimplementedContainerStreamerServer) Communicate(ContainerStreamer_CommunicateServer) error {
	return status.Errorf(codes.Unimplemented, "method Communicate not implemented")
}
func (UnimplementedContainerStreamerServer) mustEmbedUnimplementedContainerStreamerServer() {}

// UnsafeContainerStreamerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ContainerStreamerServer will
// result in compilation errors.
type UnsafeContainerStreamerServer interface {
	mustEmbedUnimplementedContainerStreamerServer()
}

func RegisterContainerStreamerServer(s grpc.ServiceRegistrar, srv ContainerStreamerServer) {
	s.RegisterService(&_ContainerStreamer_serviceDesc, srv)
}

func _ContainerStreamer_Communicate_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(ContainerStreamerServer).Communicate(&containerStreamerCommunicateServer{stream})
}

type ContainerStreamer_CommunicateServer interface {
	Send(*ContainerMessage) error
	Recv() (*ContainerMessage, error)
	grpc.ServerStream
}

type containerStreamerCommunicateServer struct {
	grpc.ServerStream
}

func (x *containerStreamerCommunicateServer) Send(m *ContainerMessage) error {
	return x.ServerStream.SendMsg(m)
}

func (x *containerStreamerCommunicateServer) Recv() (*ContainerMessage, error) {
	m := new(ContainerMessage)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _ContainerStreamer_serviceDesc = grpc.ServiceDesc{
	ServiceName: "api.ContainerStreamer",
	HandlerType: (*ContainerStreamerServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Communicate",
			Handler:       _ContainerStreamer_Communicate_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "api/api.proto",
}