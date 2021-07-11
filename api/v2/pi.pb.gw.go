// Code generated by protoc-gen-grpc-gateway. DO NOT EDIT.
// source: api/v2/pi.proto

/*
Package v2 is a reverse proxy.

It translates gRPC into RESTful JSON APIs.
*/
package v2

import (
	"context"
	"io"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/v2/utilities"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Suppress "imported and not used" errors
var _ codes.Code
var _ io.Reader
var _ status.Status
var _ = runtime.String
var _ = utilities.NewDoubleArray
var _ = metadata.Join

func request_PiService_GetDigit_0(ctx context.Context, marshaler runtime.Marshaler, client PiServiceClient, req *http.Request, pathParams map[string]string) (proto.Message, runtime.ServerMetadata, error) {
	var protoReq GetDigitRequest
	var metadata runtime.ServerMetadata

	var (
		val string
		ok  bool
		err error
		_   = err
	)

	val, ok = pathParams["index"]
	if !ok {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "missing parameter %s", "index")
	}

	protoReq.Index, err = runtime.Uint64(val)
	if err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "type mismatch, parameter: %s, error: %v", "index", err)
	}

	msg, err := client.GetDigit(ctx, &protoReq, grpc.Header(&metadata.HeaderMD), grpc.Trailer(&metadata.TrailerMD))
	return msg, metadata, err

}

func local_request_PiService_GetDigit_0(ctx context.Context, marshaler runtime.Marshaler, server PiServiceServer, req *http.Request, pathParams map[string]string) (proto.Message, runtime.ServerMetadata, error) {
	var protoReq GetDigitRequest
	var metadata runtime.ServerMetadata

	var (
		val string
		ok  bool
		err error
		_   = err
	)

	val, ok = pathParams["index"]
	if !ok {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "missing parameter %s", "index")
	}

	protoReq.Index, err = runtime.Uint64(val)
	if err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "type mismatch, parameter: %s, error: %v", "index", err)
	}

	msg, err := server.GetDigit(ctx, &protoReq)
	return msg, metadata, err

}

// RegisterPiServiceHandlerServer registers the http handlers for service PiService to "mux".
// UnaryRPC     :call PiServiceServer directly.
// StreamingRPC :currently unsupported pending https://github.com/grpc/grpc-go/issues/906.
// Note that using this registration option will cause many gRPC library features to stop working. Consider using RegisterPiServiceHandlerFromEndpoint instead.
func RegisterPiServiceHandlerServer(ctx context.Context, mux *runtime.ServeMux, server PiServiceServer) error {

	mux.Handle("GET", pattern_PiService_GetDigit_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		var stream runtime.ServerTransportStream
		ctx = grpc.NewContextWithServerTransportStream(ctx, &stream)
		inboundMarshaler, outboundMarshaler := runtime.MarshalerForRequest(mux, req)
		rctx, err := runtime.AnnotateIncomingContext(ctx, mux, req, "/api.v2.PiService/GetDigit", runtime.WithHTTPPathPattern("/api/v2/digit/{index}"))
		if err != nil {
			runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
			return
		}
		resp, md, err := local_request_PiService_GetDigit_0(rctx, inboundMarshaler, server, req, pathParams)
		md.HeaderMD, md.TrailerMD = metadata.Join(md.HeaderMD, stream.Header()), metadata.Join(md.TrailerMD, stream.Trailer())
		ctx = runtime.NewServerMetadataContext(ctx, md)
		if err != nil {
			runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
			return
		}

		forward_PiService_GetDigit_0(ctx, mux, outboundMarshaler, w, req, resp, mux.GetForwardResponseOptions()...)

	})

	return nil
}

// RegisterPiServiceHandlerFromEndpoint is same as RegisterPiServiceHandler but
// automatically dials to "endpoint" and closes the connection when "ctx" gets done.
func RegisterPiServiceHandlerFromEndpoint(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error) {
	conn, err := grpc.Dial(endpoint, opts...)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if cerr := conn.Close(); cerr != nil {
				grpclog.Infof("Failed to close conn to %s: %v", endpoint, cerr)
			}
			return
		}
		go func() {
			<-ctx.Done()
			if cerr := conn.Close(); cerr != nil {
				grpclog.Infof("Failed to close conn to %s: %v", endpoint, cerr)
			}
		}()
	}()

	return RegisterPiServiceHandler(ctx, mux, conn)
}

// RegisterPiServiceHandler registers the http handlers for service PiService to "mux".
// The handlers forward requests to the grpc endpoint over "conn".
func RegisterPiServiceHandler(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
	return RegisterPiServiceHandlerClient(ctx, mux, NewPiServiceClient(conn))
}

// RegisterPiServiceHandlerClient registers the http handlers for service PiService
// to "mux". The handlers forward requests to the grpc endpoint over the given implementation of "PiServiceClient".
// Note: the gRPC framework executes interceptors within the gRPC handler. If the passed in "PiServiceClient"
// doesn't go through the normal gRPC flow (creating a gRPC client etc.) then it will be up to the passed in
// "PiServiceClient" to call the correct interceptors.
func RegisterPiServiceHandlerClient(ctx context.Context, mux *runtime.ServeMux, client PiServiceClient) error {

	mux.Handle("GET", pattern_PiService_GetDigit_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		inboundMarshaler, outboundMarshaler := runtime.MarshalerForRequest(mux, req)
		rctx, err := runtime.AnnotateContext(ctx, mux, req, "/api.v2.PiService/GetDigit", runtime.WithHTTPPathPattern("/api/v2/digit/{index}"))
		if err != nil {
			runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
			return
		}
		resp, md, err := request_PiService_GetDigit_0(rctx, inboundMarshaler, client, req, pathParams)
		ctx = runtime.NewServerMetadataContext(ctx, md)
		if err != nil {
			runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
			return
		}

		forward_PiService_GetDigit_0(ctx, mux, outboundMarshaler, w, req, resp, mux.GetForwardResponseOptions()...)

	})

	return nil
}

var (
	pattern_PiService_GetDigit_0 = runtime.MustPattern(runtime.NewPattern(1, []int{2, 0, 2, 1, 2, 2, 1, 0, 4, 1, 5, 3}, []string{"api", "v2", "digit", "index"}, ""))
)

var (
	forward_PiService_GetDigit_0 = runtime.ForwardResponseMessage
)
