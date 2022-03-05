package stream

import (
	"context"
	"errors"
	"io"
	"reflect"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type payloadT byte

var ErrIllegalHeaderWrite = errors.New("transport: the stream is done or WriteHeader was already called")

const (
	typeEOF payloadT = iota
	typeErr
	typeData
)

const queueDepth = 16

type payload struct {
	t payloadT
	e error
	d interface{}
}

type headerWrapper struct {
	sent  bool
	md    metadata.MD
	mutex sync.Mutex
	ch    chan struct{}
}

func (s *headerWrapper) Header() metadata.MD {
	<-s.ch
	return s.md
}

func (s *headerWrapper) SetHeader(md metadata.MD) error {
	if s.sent {
		return ErrIllegalHeaderWrite
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.sent {
		return ErrIllegalHeaderWrite
	}

	if len(md) == 0 {
		return nil
	}

	s.md = metadata.Join(s.md, md)
	return nil
}

func (s *headerWrapper) Send() {
	if s.sent {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.sent {
		return
	}

	s.sent = true
	close(s.ch)
}

type trailerWrapper struct {
	md    metadata.MD
	mutex sync.RWMutex
}

func (s *trailerWrapper) Trailer() metadata.MD {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.md
}

func (s *trailerWrapper) SetTrailer(md metadata.MD) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(md) == 0 {
		return
	}

	s.md = metadata.Join(s.md, md)
}

type GrpcStream struct {
	method       string
	clientCtx    context.Context
	serverCtx    context.Context
	c2s          chan *payload
	s2c          chan *payload
	srvClosed    uint32
	srvCloseCtx  context.Context
	srvCloseFunc func()

	clientStream *_GrpcClientStream
	serverStream *_GrpcServerStream
}

func NewGrpcStream(ctx context.Context, method string) *GrpcStream {
	var serverCtx context.Context
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		serverCtx = metadata.NewIncomingContext(ctx, md)
		serverCtx = metadata.NewOutgoingContext(serverCtx, make(metadata.MD))
	} else {
		serverCtx = metadata.NewIncomingContext(ctx, make(metadata.MD))
	}

	closeCtx, closeFn := context.WithCancel(context.Background())

	stream := &GrpcStream{
		method:       method,
		clientCtx:    ctx,
		serverCtx:    serverCtx,
		c2s:          make(chan *payload, queueDepth),
		s2c:          make(chan *payload, queueDepth),
		srvClosed:    0,
		srvCloseCtx:  closeCtx,
		srvCloseFunc: closeFn,
	}

	header := &headerWrapper{ch: make(chan struct{})}
	trailer := &trailerWrapper{}

	stream.clientStream = &_GrpcClientStream{
		p:       stream,
		s:       &_StreamInternal{cancelCtx: stream.clientCtx, sendCh: stream.c2s, recvCh: stream.s2c, closeCtx: closeCtx},
		header:  header,
		trailer: trailer,
	}

	stream.serverStream = &_GrpcServerStream{
		p:       stream,
		s:       &_StreamInternal{cancelCtx: stream.clientCtx, sendCh: stream.s2c, recvCh: stream.c2s, closeCtx: closeCtx},
		header:  header,
		trailer: trailer,
	}
	stream.serverCtx = grpc.NewContextWithServerTransportStream(stream.serverCtx, &_GrpcServerTransportStream{stream.serverStream})

	return stream
}

func (s *GrpcStream) Method() string {
	return s.method
}

func (s *GrpcStream) ClientStream() grpc.ClientStream {
	return s.clientStream
}

func (s *GrpcStream) ServerStream() grpc.ServerStream {
	return s.serverStream
}

func (s *GrpcStream) WriteStatus(err error) {
	if atomic.LoadUint32(&s.srvClosed) == 1 {
		return
	}

	_ = s.serverStream.SendHeader(metadata.New(map[string]string{}))
	if err != nil {
		_ = s.serverStream.s.Send(&payload{e: err, t: typeErr})
	}

	s.srvCloseFunc()
	s.serverStream.s.Close()
	atomic.StoreUint32(&s.srvClosed, 1)
}

func (s *GrpcStream) isSrvClosed() bool {
	return atomic.LoadUint32(&s.srvClosed) == 1
}

type _StreamInternal struct {
	mutex     sync.RWMutex
	closed    bool
	cancelCtx context.Context
	sendCh    chan *payload
	recvCh    chan *payload
	closeCtx  context.Context
}

func (s *_StreamInternal) Send(m *payload) error {
	if s.closed {
		return status.Error(codes.Internal, "grpc_stream: the stream is done or WriteHeader was already called")
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.closed {
		return status.Error(codes.Internal, "grpc_stream: the stream is done or WriteHeader was already called")
	}

	select {
	case <-s.cancelCtx.Done():
		return status.Error(codes.Internal, "grpc_stream: the stream is done or WriteHeader was already called")
	default:
	}

	select {
	case <-s.cancelCtx.Done():
		return status.Error(codes.Internal, "grpc_stream: the stream is done or WriteHeader was already called")
	case s.sendCh <- m:
		return nil
	case <-s.closeCtx.Done():
		return status.Error(codes.Internal, "grpc_stream: the stream is done or WriteHeader was already called")
	}
}

func (s *_StreamInternal) Recv() (*payload, error) {
	select {
	case v := <-s.recvCh:
		if v == nil {
			return nil, io.EOF
		}

		switch v.t {
		case typeData:
			return v, nil
		case typeErr:
			return nil, v.e
		case typeEOF:
			return nil, io.EOF
		default:
			return nil, status.Errorf(codes.Internal, "grpc_stream: invalid payload type %v", v.t)
		}
	case <-s.cancelCtx.Done():
		select {
		case v := <-s.recvCh:
			if v == nil {
				return nil, io.EOF
			}

			if v.t == typeEOF {
				return nil, io.EOF
			}
		default:
		}

		return nil, status.FromContextError(context.Canceled).Err()
	}
}

func (s *_StreamInternal) Close() error {
	if s.closed {
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.closed {
		return nil
	}

	s.closed = true
	close(s.sendCh)
	return nil
}

type _GrpcClientStream struct {
	p       *GrpcStream
	s       *_StreamInternal
	header  *headerWrapper
	trailer *trailerWrapper

	onceClose sync.Once
}

func (s *_GrpcClientStream) Header() (metadata.MD, error) {
	header := s.header.Header()
	return header, nil
}

func (s *_GrpcClientStream) Trailer() metadata.MD {
	return s.trailer.Trailer()
}

func (s *_GrpcClientStream) CloseSend() error {
	var err error
	s.onceClose.Do(func() {
		err = s.s.Close()
	})
	return err
}

func (s *_GrpcClientStream) Context() context.Context {
	return s.p.clientCtx
}

func (s *_GrpcClientStream) SendMsg(m interface{}) error {
	if s.p.isSrvClosed() {
		return io.EOF
	}
	return s.s.Send(&payload{d: m, t: typeData})
}

func (s *_GrpcClientStream) RecvMsg(m interface{}) error {
	for {
		payload, err := s.s.Recv()
		if err != nil {
			return err
		}

		switch payload.t {
		case typeData:
			reflect.ValueOf(m).Elem().Set(reflect.ValueOf(payload.d).Elem())
			return nil
		default:
			return status.Errorf(codes.Internal, "grpc_stream: invalid payload type %v", payload.t)
		}
	}
}

type _GrpcServerStream struct {
	p       *GrpcStream
	s       *_StreamInternal
	header  *headerWrapper
	trailer *trailerWrapper
}

func (s *_GrpcServerStream) SetHeader(md metadata.MD) error {
	if md.Len() == 0 {
		return nil
	}

	if s.p.isSrvClosed() {
		return ErrIllegalHeaderWrite
	}

	return s.header.SetHeader(md)
}

func (s *_GrpcServerStream) SendHeader(md metadata.MD) error {
	if s.p.isSrvClosed() {
		return ErrIllegalHeaderWrite
	}

	if err := s.header.SetHeader(md); err != nil {
		return err
	}

	s.header.Send()
	return nil
}

func (s *_GrpcServerStream) SetTrailer(md metadata.MD) {
	s.trailer.SetTrailer(md)
}

func (s *_GrpcServerStream) Context() context.Context {
	return s.p.serverCtx
}

func (s *_GrpcServerStream) SendMsg(m interface{}) error {
	_ = s.SendHeader(metadata.New(map[string]string{}))
	return s.s.Send(&payload{d: m, t: typeData})
}

func (s *_GrpcServerStream) RecvMsg(m interface{}) error {
	payload, err := s.s.Recv()
	if err != nil {
		return err
	}

	switch payload.t {
	case typeData:
		reflect.ValueOf(m).Elem().Set(reflect.ValueOf(payload.d).Elem())
		return nil
	default:
		return status.Errorf(codes.Internal, "grpc_stream: invalid payload type %v", payload.t)
	}
}

type _GrpcServerTransportStream struct {
	ss *_GrpcServerStream
}

func (s *_GrpcServerTransportStream) Method() string {
	return s.ss.p.Method()
}

func (s *_GrpcServerTransportStream) SetHeader(md metadata.MD) error {
	return s.ss.SetHeader(md)
}

func (s *_GrpcServerTransportStream) SendHeader(md metadata.MD) error {
	return s.ss.SendHeader(md)
}

func (s *_GrpcServerTransportStream) SetTrailer(md metadata.MD) error {
	s.ss.SetTrailer(md)
	return nil
}
