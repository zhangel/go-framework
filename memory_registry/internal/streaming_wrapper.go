package internal

import (
	"context"
	"net/http"
	"reflect"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/internal"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/stream"
)

type StreamingWrapper struct {
	method         string
	srv            interface{}
	sd             *grpc.StreamDesc
	reqType        reflect.Type
	respType       reflect.Type
	streamInt      grpc.StreamServerInterceptor
	serverRecovery bool
}

func CreateStreamingWrapper(method string, srv interface{}, sd *grpc.StreamDesc, streamInt grpc.StreamServerInterceptor) (*StreamingWrapper, error) {
	handlerType := reflect.TypeOf(srv)
	m, ok := handlerType.MethodByName(sd.StreamName)
	if !ok {
		return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
	}

	var reqType, respType reflect.Type
	if sd.ServerStreams && !sd.ClientStreams {
		if m.Type.NumIn() != 3 || m.Type.NumOut() != 1 {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		reqType = m.Type.In(1)
		if reqType.Kind() != reflect.Ptr {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		streamerType := m.Type.In(2)
		if !streamerType.Implements(reflect.TypeOf((*grpc.ServerStream)(nil)).Elem()) {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		sendMethodT, ok := streamerType.MethodByName("Send")
		if !ok {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		respType = sendMethodT.Type.In(0)
		if reqType.Kind() != reflect.Ptr {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}
	} else if sd.ClientStreams && !sd.ServerStreams {
		if m.Type.NumIn() != 2 || m.Type.NumOut() != 1 {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		streamerType := m.Type.In(1)
		if !streamerType.Implements(reflect.TypeOf((*grpc.ServerStream)(nil)).Elem()) {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		recvMethodT, ok := streamerType.MethodByName("Recv")
		if !ok {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		reqType = recvMethodT.Type.Out(0)
		if reqType.Kind() != reflect.Ptr {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		sendAndCloseMethodT, ok := streamerType.MethodByName("SendAndClose")
		if !ok {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		respType = sendAndCloseMethodT.Type.In(0)
		if reqType.Kind() != reflect.Ptr {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}
	} else if sd.ServerStreams && sd.ClientStreams {
		if m.Type.NumIn() != 2 || m.Type.NumOut() != 1 {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		streamerType := m.Type.In(1)
		if !streamerType.Implements(reflect.TypeOf((*grpc.ServerStream)(nil)).Elem()) {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		recvMethodT, ok := streamerType.MethodByName("Recv")
		if !ok {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		reqType = recvMethodT.Type.Out(0)
		if reqType.Kind() != reflect.Ptr {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		sendAndCloseMethodT, ok := streamerType.MethodByName("Send")
		if !ok {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}

		respType = sendAndCloseMethodT.Type.In(0)
		if reqType.Kind() != reflect.Ptr {
			return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
		}
	}

	return &StreamingWrapper{
		method,
		srv,
		sd,
		reqType,
		respType,
		streamInt,
		config.Bool("server.recovery"),
	}, nil
}

func (s *StreamingWrapper) NewStream(ctx context.Context, callOpts ...grpc.CallOption) (grpc.ClientStream, error) {
	peerCtxOverride := false

	for _, o := range callOpts {
		if peerCtxOpt, ok := o.(interface {
			OverridePeerContext(context.Context) context.Context
		}); ok {
			ctx = peerCtxOpt.OverridePeerContext(ctx)
			peerCtxOverride = true
		}
	}

	if !peerCtxOverride {
		ctx = peer.NewContext(ctx, &peer.Peer{
			Addr:     PeerAddr{},
			AuthInfo: AuthInfo{},
		})
	}

	grpcStream := stream.NewGrpcStream(ctx, s.method)
	go func() {
		if s.serverRecovery {
			defer func() {
				if r := recover(); r != nil {
					err := status.Errorf(codes.Internal, "[PANIC] panic = %s, stack = %s", r, debug.Stack())
					log.Error(err)
					grpcStream.WriteStatus(internal.ToRPCErr(err))
				}
			}()
		}

		var err error
		if s.streamInt != nil {
			err = s.streamInt(s.srv, grpcStream.ServerStream(), &grpc.StreamServerInfo{
				FullMethod:     s.method,
				IsClientStream: s.sd.ClientStreams,
				IsServerStream: s.sd.ServerStreams,
			}, s.sd.Handler)
		} else {
			err = s.sd.Handler(s.srv, grpcStream.ServerStream())
		}

		grpcStream.WriteStatus(internal.ToRPCErr(err))

		for _, callOpt := range callOpts {
			switch o := callOpt.(type) {
			case grpc.HeaderCallOption:
				header, _ := grpcStream.ClientStream().Header()
				*o.HeaderAddr = header
			case grpc.TrailerCallOption:
				*o.TrailerAddr = grpcStream.ClientStream().Trailer()
			case grpc.PeerCallOption:
				*o.PeerAddr = peer.Peer{
					Addr:     PeerAddr{},
					AuthInfo: AuthInfo{},
				}
			}
		}

		_ = grpcStream.ClientStream().CloseSend()
	}()
	return grpcStream.ClientStream(), nil
}

func (s *StreamingWrapper) Method() string {
	return s.method
}

func (s *StreamingWrapper) RequestType() reflect.Type {
	return s.reqType
}

func (s *StreamingWrapper) ResponseType() reflect.Type {
	return s.respType
}
