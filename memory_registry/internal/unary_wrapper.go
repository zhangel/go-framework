package internal

import (
	"context"
	"net/http"
	"reflect"
	"runtime/debug"
	"sync"

	"github.com/modern-go/reflect2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/internal"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/stream"
)

var nilTransportStream = &NilTransportStream{}

type UnaryWrapper struct {
	method         string
	srv            interface{}
	m              reflect.Method
	md             *grpc.MethodDesc
	reqType        reflect.Type
	respType       reflect.Type
	unaryInt       grpc.UnaryServerInterceptor
	serverRecovery bool
}

func CreateUnaryWrapper(method string, srv interface{}, md *grpc.MethodDesc, unaryInt grpc.UnaryServerInterceptor) (*UnaryWrapper, error) {
	handlerType := reflect.TypeOf(srv)
	m, ok := handlerType.MethodByName(md.MethodName)
	if !ok {
		return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
	}

	if m.Type.NumIn() != 3 || m.Type.NumOut() != 2 {
		return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
	}

	reqType := m.Type.In(2)
	if reqType.Kind() != reflect.Ptr {
		return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
	}

	respType := m.Type.Out(0)
	if respType.Kind() != reflect.Ptr {
		return nil, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
	}

	return &UnaryWrapper{
		method,
		srv,
		m,
		md,
		reqType,
		respType,
		unaryInt,
		config.Bool("server.recovery"),
	}, nil
}

func (s *UnaryWrapper) Invoke(ctx context.Context, req interface{}, callOpts ...grpc.CallOption) (_ interface{}, err error) {
	if s.serverRecovery {
		defer func() {
			if r := recover(); r != nil {
				err = status.Errorf(codes.Internal, "[PANIC] panic = %s, stack = %s", r, debug.Stack())
				log.Error(err)
			}
		}()
	}

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

	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		ctx = metadata.NewIncomingContext(ctx, md)
		ctx = metadata.NewOutgoingContext(ctx, make(metadata.MD))
	}

	hasHeaderOpt := false
	for _, callOpt := range callOpts {
		switch callOpt.(type) {
		case grpc.HeaderCallOption:
			hasHeaderOpt = true
		case grpc.TrailerCallOption:
			hasHeaderOpt = true
		}
	}

	var ts *UnaryMockTransportStream
	if hasHeaderOpt {
		ts = &UnaryMockTransportStream{
			s.method,
			nil,
			nil,
			false,
			sync.RWMutex{},
		}
		ctx = grpc.NewContextWithServerTransportStream(ctx, ts)
	} else {
		ctx = grpc.NewContextWithServerTransportStream(ctx, nilTransportStream)
	}

	resp, err := s.md.Handler(s.srv, ctx, func(v interface{}) error {
		vType := reflect2.TypeOf(v).(reflect2.PtrType).Elem()
		vType.Set(v, req)
		return nil
	}, s.unaryInt)

	for _, callOpt := range callOpts {
		switch o := callOpt.(type) {
		case grpc.HeaderCallOption:
			if ts != nil {
				*o.HeaderAddr = ts.Header()
			}
		case grpc.TrailerCallOption:
			if ts != nil {
				*o.TrailerAddr = ts.Trailer()
			}
		case grpc.PeerCallOption:
			*o.PeerAddr = peer.Peer{
				Addr:     PeerAddr{},
				AuthInfo: AuthInfo{},
			}
		}
	}

	return resp, internal.ToRPCErr(err)
}

func (s *UnaryWrapper) Method() string {
	return s.method
}

func (s *UnaryWrapper) RequestType() reflect.Type {
	return s.reqType
}

func (s *UnaryWrapper) ResponseType() reflect.Type {
	return s.respType
}

type NilTransportStream struct{}

func (s *NilTransportStream) Method() string {
	return ""
}

func (s *NilTransportStream) Header() metadata.MD {
	return nil
}

func (s *NilTransportStream) SetHeader(md metadata.MD) error {
	return nil
}

func (s *NilTransportStream) SendHeader(md metadata.MD) error {
	return nil
}

func (s *NilTransportStream) Trailer() metadata.MD {
	return nil
}

func (s *NilTransportStream) SetTrailer(md metadata.MD) error {
	return nil
}

type UnaryMockTransportStream struct {
	method  string
	header  metadata.MD
	trailer metadata.MD
	hdrSent bool
	mu      sync.RWMutex
}

func (s *UnaryMockTransportStream) Method() string {
	return s.method
}

func (s *UnaryMockTransportStream) Header() metadata.MD {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.header.Copy()
}

func (s *UnaryMockTransportStream) SetHeader(md metadata.MD) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hdrSent {
		return stream.ErrIllegalHeaderWrite
	}

	s.header = metadata.Join(s.header, md)
	return nil
}

func (s *UnaryMockTransportStream) SendHeader(md metadata.MD) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hdrSent {
		return stream.ErrIllegalHeaderWrite
	}

	s.header = metadata.Join(s.header, md)
	s.hdrSent = true
	return nil
}

func (s *UnaryMockTransportStream) Trailer() metadata.MD {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.trailer.Copy()
}

func (s *UnaryMockTransportStream) SetTrailer(md metadata.MD) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.trailer = metadata.Join(s.trailer, md)
	return nil
}
