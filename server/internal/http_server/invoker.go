package http_server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	grpc_gateway "github.com/zhangel/go-framework/3rdparty/grpc-gateway"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/memory_registry"
)

var errEmptyResponse = errors.New("empty response")

type noInProcPeerCallOpt struct {
	grpc.EmptyCallOption
}

func (s *noInProcPeerCallOpt) OverridePeerContext(ctx context.Context) context.Context {
	return ctx
}

type InvokerWrapper interface {
	memory_registry.Method
	Invoke(ctx context.Context, req interface{}, respHandler func(resp interface{}) (interface{}, error), grpcGatewayCompatible bool, marshaler *runtime.JSONPb, w http.ResponseWriter)
}

func NewUnaryInvoker(method memory_registry.UnaryMethod) InvokerWrapper {
	return UnaryInvoker{method}
}

func NewServerStreamingInvoker(streaming memory_registry.Streaming) InvokerWrapper {
	return ServerStreamingInvoker{streaming}
}

type UnaryInvoker struct {
	memory_registry.UnaryMethod
}

func (s UnaryInvoker) Invoke(
	ctx context.Context,
	req interface{},
	respHandler func(resp interface{}) (interface{}, error),
	grpcGatewayCompatible bool,
	marshaler *runtime.JSONPb,
	w http.ResponseWriter,
) {
	respHeader := metadata.New(nil)
	respTrailer := metadata.New(nil)

	httpCode := func() int {
		if httpCodeHeader := respHeader.Get(XHttpCodeHeader); len(httpCodeHeader) > 0 {
			if code, err := strconv.Atoi(httpCodeHeader[0]); err == nil {
				delete(respHeader, "x-http-code")
				return code
			}
		}
		return 0
	}

	if resp, err := s.UnaryMethod.Invoke(ctx, req, grpc.Header(&respHeader), grpc.Trailer(&respTrailer), &noInProcPeerCallOpt{}); err != nil {
		httpErrorHandler(httpCode(), w, marshaler, err)
	} else {
		resp, err := respHandler(resp)
		if err != nil {
			httpErrorHandler(httpCode(), w, marshaler, err)
			return
		}

		w.Header().Set("Content-Type", marshaler.ContentType())

		httpResp, err := marshaler.Marshal(resp)
		if err != nil {
			httpErrorHandler(httpCode(), w, marshaler, status.Errorf(codes.Internal, "http unary invoker marshal grpc response failed, err = %v", err))
			return
		}

		if code := httpCode(); code != 0 {
			w.WriteHeader(code)
		}

		writeResponseHeader(w, respHeader, grpcGatewayCompatible)
		writeResponseTrailer(w, respTrailer)

		_, err = w.Write(httpResp)
		if err != nil {
			log.Infof("Failed to write http response, err = %v", err)
		}
	}
}

type ServerStreamingInvoker struct {
	memory_registry.Streaming
}

func (s ServerStreamingInvoker) Invoke(
	ctx context.Context,
	req interface{},
	respHandler func(resp interface{}) (interface{}, error),
	grpcGatewayCompatible bool,
	marshaler *runtime.JSONPb,
	w http.ResponseWriter,
) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Infof("Flush not supported in %T", w)
		httpErrorHandler(0, w, marshaler, status.Errorf(codes.Internal, "flush not supported in http response writer"))
		return
	}

	streamer, err := s.Streaming.NewStream(ctx, &noInProcPeerCallOpt{})
	if err != nil {
		httpErrorHandler(0, w, marshaler, status.Errorf(codes.Internal, "http streaming invoker NewStream failed, err = %v", err))
		return
	}

	if err := streamer.SendMsg(req); err != nil {
		httpErrorHandler(0, w, marshaler, err)
		return
	}

	if err := streamer.CloseSend(); err != nil {
		httpErrorHandler(0, w, marshaler, status.Errorf(codes.Internal, "http streaming invoker CloseSend failed, err = %v", err))
		return
	}

	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", marshaler.ContentType())

	delimiter := []byte("\n")

	var wroteHeader, wroteCustomCode bool
	for {
		pbResp := reflect.Indirect(reflect.New(s.ResponseType().Elem())).Addr().Interface()
		err := streamer.RecvMsg(pbResp)
		if err == io.EOF {
			return
		}

		if !wroteHeader {
			if respHeader, err := streamer.Header(); err == nil {
				if httpCodeHeader := respHeader.Get(XHttpCodeHeader); len(httpCodeHeader) > 0 {
					if code, err := strconv.Atoi(httpCodeHeader[0]); err == nil {
						delete(respHeader, "x-http-code")
						w.WriteHeader(code)
						wroteCustomCode = true
					}
				}
				writeResponseHeader(w, respHeader, grpcGatewayCompatible)
			}
		}

		if err != nil {
			handleStreamErrorResponse(wroteHeader, wroteCustomCode, marshaler, w, err)
			return
		}

		pbResp, err = respHandler(pbResp)
		if err != nil {
			handleStreamErrorResponse(wroteHeader, wroteCustomCode, marshaler, w, err)
			return
		}

		httpResp, err := marshaler.Marshal(streamChunk(pbResp.(proto.Message)))
		if err != nil {
			httpErrorHandler(0, w, marshaler, status.Errorf(codes.Internal, "http streaming invoker marshal grpc response failed, err = %v", err))
			return
		}

		wroteHeader = true
		_, err = w.Write(httpResp)
		if err != nil {
			log.Infof("Failed to write http response, err = %v", err)
		}

		_, err = w.Write(delimiter)
		if err != nil {
			log.Infof("Failed to write http response, err = %v", err)
		}

		flusher.Flush()
	}
}

func handleStreamErrorResponse(wroteHeader, wroteCustomCode bool, marshaler *runtime.JSONPb, w http.ResponseWriter, err error) {
	streamErr := streamError(err)
	if !wroteHeader && !wroteCustomCode {
		w.WriteHeader(int(streamErr.HttpCode))
	}

	if !wroteCustomCode {
		buf, err := marshaler.Marshal(errorChunk(streamErr))
		if err != nil {
			log.Infof("Failed to marshal an error: %v", err)
			return
		}
		if _, err := w.Write(buf); err != nil {
			log.Infof("Failed to notify error to client: %v", err)
			return
		}
	}
}

func streamError(err error) *grpc_gateway.StreamError {
	grpcCode := codes.Unknown
	grpcMessage := err.Error()
	var grpcDetails []*any.Any

	if s, ok := status.FromError(err); ok {
		grpcCode = s.Code()
		grpcMessage = s.Message()
		grpcDetails = s.Proto().GetDetails()
	}

	httpCode := httpStatusFromCode(grpcCode)
	return &grpc_gateway.StreamError{
		GrpcCode:   int32(grpcCode),
		HttpCode:   int32(httpCode),
		Message:    grpcMessage,
		HttpStatus: http.StatusText(httpCode),
		Details:    grpcDetails,
	}
}

func streamChunk(result proto.Message) map[string]proto.Message {
	if result == nil {
		return errorChunk(streamError(errEmptyResponse))
	}
	return map[string]proto.Message{"result": result}
}

func errorChunk(err *grpc_gateway.StreamError) map[string]proto.Message {
	return map[string]proto.Message{"error": err}
}
