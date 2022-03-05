package tracing

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/zhangel/go-framework.git/3rdparty/grpc-opentracing/go/otgrpc"
	"github.com/zhangel/go-framework.git/interceptor"

	"google.golang.org/grpc/metadata"

	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"github.com/zhangel/go-framework.git/declare"
	"github.com/zhangel/go-framework.git/lifecycle"
	"github.com/zhangel/go-framework.git/log"
	"github.com/zhangel/go-framework.git/log/fields"
	"github.com/zhangel/go-framework.git/plugin"
)

var (
	Plugin = declare.PluginType{Name: "tracer"}

	defaultTracer Tracer
	mutex         = new(sync.RWMutex)
	targetMeta    = "x-grpc-target"
)

type TracerIdExtractor interface {
	TraceIdFromCtx(ctx context.Context) (tracerId string, ok bool)
}

type Tracer interface {
	opentracing.Tracer
	Close() error
}

type TracerTagContext struct{}

func init() {
	lifecycle.LifeCycle().HookInitialize(func() {
		log.RegisterContextFieldsProvider(func(ctx context.Context) fields.Fields {
			f := fields.Fields{}
			if DefaultTracer() == nil {
				return f
			}

			if tracer, ok := DefaultTracer().(TracerIdExtractor); !ok {
				return f
			} else if traceId, ok := tracer.TraceIdFromCtx(ctx); ok {
				f = append(f, fields.Field{K: "trace_id", V: traceId})
				return f
			}

			return f
		})

		err := plugin.CreatePlugin(Plugin, &defaultTracer)
		if err != nil {
			log.Fatalf("[ERROR] Create tracer plugin failed, err = %s.\n", err)
		}
	}, lifecycle.WithName("TraceId initializer"))

	lifecycle.LifeCycle().HookFinalize(func(context.Context) {
		if DefaultTracer() != nil {
			_ = DefaultTracer().Close()
		}
	}, lifecycle.WithName("Close default tracer"))
}

func DefaultTracer() Tracer {
	return defaultTracer
}

type streamWrapper struct {
	ctx context.Context
	ss  grpc.ServerStream
}

func (s *streamWrapper) SetHeader(md metadata.MD) error {
	return s.ss.SetHeader(md)
}

func (s *streamWrapper) SendHeader(md metadata.MD) error {
	return s.ss.SendHeader(md)
}

func (s *streamWrapper) SetTrailer(md metadata.MD) {
	s.ss.SetTrailer(md)
}

func (s *streamWrapper) Context() context.Context {
	return s.ctx
}

func (s *streamWrapper) SendMsg(m interface{}) error {
	return s.ss.SendMsg(m)
}

func (s *streamWrapper) RecvMsg(m interface{}) error {
	return s.ss.RecvMsg(m)
}

func serviceAliasName(ctx context.Context, fullMethod string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return fullMethod
	}

	targetMd := md.Get(targetMeta)
	if len(targetMd) == 0 {
		return fullMethod
	}

	target := targetMd[len(targetMd)-1]

	path := strings.Split(fullMethod, "/")
	if len(path) != 3 {
		return fullMethod
	}

	return fmt.Sprintf("/%s/%s", target, path[2])
}

func tracerTagContext(ctx context.Context) context.Context {
	if _, ok := ctx.Value(TracerTagContext{}).(map[string]interface{}); ok {
		return ctx
	} else {
		return context.WithValue(ctx, TracerTagContext{}, map[string]interface{}{})
	}
}

func setAdditionalUnarySpanTag(ctx context.Context, methodName string, inMemory bool) {
	if inMemory {
		SetSpanTag(ctx, "in_memory_invoke", true)
	}
	SetSpanTag(ctx, "type", "unary")
	SetSpanTag(ctx, "alias", serviceAliasName(ctx, methodName))
}

func setAdditionalStreamSpanTag(ctx context.Context, methodName string, serverStream, clientStream bool, inMemory bool) {
	if inMemory {
		SetSpanTag(ctx, "in_memory_invoke", true)
	}

	if serverStream && clientStream {
		SetSpanTag(ctx, "type", "bidi_stream")
	} else if serverStream {
		SetSpanTag(ctx, "type", "server_stream")
	} else {
		SetSpanTag(ctx, "type", "client_stream")
	}

	SetSpanTag(ctx, "alias", serviceAliasName(ctx, methodName))
}

func spanTagDecorator(ctx context.Context, span opentracing.Span, method string, req, resp interface{}, grpcError error) {
	if tags, ok := ctx.Value(TracerTagContext{}).(map[string]interface{}); !ok {
		return
	} else {
		mutex.RLock()
		defer mutex.RUnlock()

		if len(tags) == 0 {
			return
		}

		for k, v := range tags {
			span.SetTag(k, v)
		}
	}
}

func tracerTagUnaryClientInterceptor(inMemory bool) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = tracerTagContext(ctx)
		setAdditionalUnarySpanTag(ctx, method, inMemory)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func tracerTagServerClientInterceptor(inMemory bool) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = tracerTagContext(ctx)
		setAdditionalStreamSpanTag(ctx, method, desc.ServerStreams, desc.ClientStreams, inMemory)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

func tracerTagUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx = tracerTagContext(ctx)
		setAdditionalUnarySpanTag(ctx, info.FullMethod, false)
		return handler(ctx, req)
	}
}

func tracerTagStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := tracerTagContext(ss.Context())
		setAdditionalStreamSpanTag(ctx, info.FullMethod, info.IsServerStream, info.IsClientStream, false)
		return handler(srv, &streamWrapper{ctx: ctx, ss: ss})
	}
}

func OpenTracingClientInterceptor(tracer Tracer, inMemory bool) grpc.UnaryClientInterceptor {
	return interceptor.ChainUnaryClient(
		tracerTagUnaryClientInterceptor(inMemory),
		otgrpc.OpenTracingClientInterceptor(tracer, otgrpc.SpanDecorator(spanTagDecorator)),
	)
}

func OpenTracingStreamClientInterceptor(tracer Tracer, inMemory bool) grpc.StreamClientInterceptor {
	return interceptor.ChainStreamClient(
		tracerTagServerClientInterceptor(inMemory),
		otgrpc.OpenTracingStreamClientInterceptor(tracer, otgrpc.SpanDecorator(spanTagDecorator)),
	)
}

func OpenTracingServerInterceptor(tracer Tracer) grpc.UnaryServerInterceptor {
	return interceptor.ChainUnaryServer(
		tracerTagUnaryServerInterceptor(),
		otgrpc.OpenTracingServerInterceptor(tracer, otgrpc.SpanDecorator(spanTagDecorator)),
	)
}

func OpenTracingStreamServerInterceptor(tracer Tracer) grpc.StreamServerInterceptor {
	return interceptor.ChainStreamServer(
		tracerTagStreamServerInterceptor(),
		otgrpc.OpenTracingStreamServerInterceptor(tracer, otgrpc.SpanDecorator(spanTagDecorator)),
	)
}

func SetSpanTag(ctx context.Context, k string, v interface{}) {
	if tags, ok := ctx.Value(TracerTagContext{}).(map[string]interface{}); ok {
		mutex.Lock()
		defer mutex.Unlock()
		tags[k] = v
	}
}
