// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             (unknown)
// source: api/v2/pi.proto

package v2

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// PiServiceClient is the client API for PiService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type PiServiceClient interface {
	GetDigit(ctx context.Context, in *GetDigitRequest, opts ...grpc.CallOption) (*GetDigitResponse, error)
}

type piServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewPiServiceClient(cc grpc.ClientConnInterface) PiServiceClient {
	return &piServiceClient{cc}
}

func (c *piServiceClient) GetDigit(ctx context.Context, in *GetDigitRequest, opts ...grpc.CallOption) (*GetDigitResponse, error) {
	out := new(GetDigitResponse)
	err := c.cc.Invoke(ctx, "/api.v2.PiService/GetDigit", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PiServiceServer is the server API for PiService service.
// All implementations must embed UnimplementedPiServiceServer
// for forward compatibility
type PiServiceServer interface {
	GetDigit(context.Context, *GetDigitRequest) (*GetDigitResponse, error)
	mustEmbedUnimplementedPiServiceServer()
}

// UnimplementedPiServiceServer must be embedded to have forward compatible implementations.
type UnimplementedPiServiceServer struct {
}

func (UnimplementedPiServiceServer) GetDigit(context.Context, *GetDigitRequest) (*GetDigitResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDigit not implemented")
}
func (UnimplementedPiServiceServer) mustEmbedUnimplementedPiServiceServer() {}

// UnsafePiServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to PiServiceServer will
// result in compilation errors.
type UnsafePiServiceServer interface {
	mustEmbedUnimplementedPiServiceServer()
}

func RegisterPiServiceServer(s grpc.ServiceRegistrar, srv PiServiceServer) {
	s.RegisterService(&PiService_ServiceDesc, srv)
}

func _PiService_GetDigit_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetDigitRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PiServiceServer).GetDigit(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.v2.PiService/GetDigit",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PiServiceServer).GetDigit(ctx, req.(*GetDigitRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// PiService_ServiceDesc is the grpc.ServiceDesc for PiService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var PiService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "api.v2.PiService",
	HandlerType: (*PiServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetDigit",
			Handler:    _PiService_GetDigit_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/v2/pi.proto",
}