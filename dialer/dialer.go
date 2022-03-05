package dialer

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc/metadata"

	"google.golang.org/grpc"

	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/connection"
	"github.com/zhangel/go-framework/declare"
	"github.com/zhangel/go-framework/dialer/internal"
	"github.com/zhangel/go-framework/dialer/internal/dialer"
	"github.com/zhangel/go-framework/dialer/internal/memory_dialer"
	"github.com/zhangel/go-framework/dialer/internal/option"
	"github.com/zhangel/go-framework/dialer/internal/pool"
	"github.com/zhangel/go-framework/interceptor"
	root_internal "github.com/zhangel/go-framework/internal"
	"github.com/zhangel/go-framework/lifecycle"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/memory_registry"
)

const targetMeta = "x-grpc-target"

type callOptCtx struct{}

var (
	cc         = CreateClientConnection()
	ClientConn = cc
)

type DialFunc func(ctx context.Context, callOpts CallOptions) (*grpc.ClientConn, error)
type MetadataByCallOpt interface {
	SetMetadata(md *metadata.MD)
}

func init() {
	declare.Flags(internal.DialerPrefix,
		declare.Flag{Name: internal.FlagConnTimeout, DefaultValue: 1 * time.Second, Description: "Dialer connect timeout."},
		declare.Flag{Name: internal.FlagDialInMemory, DefaultValue: true, Description: "Use in-memory dial."},
		declare.Flag{Name: internal.FlagInMemoryMaxConcurrent, DefaultValue: -1, Description: "Max concurrent stream of in-memory grpc invoke.", Deprecated: true},
		declare.Flag{Name: internal.FlagIgnoreProxyEnv, DefaultValue: false, Description: "Ignore http(s)_proxy environment variable."},
		declare.Flag{Name: internal.FlagCert, DefaultValue: "", Description: "Using this TLS certificate file to identify secure client."},
		declare.Flag{Name: internal.FlagKey, DefaultValue: "", Description: "Using this TLS key file to identify secure client."},
		declare.Flag{Name: internal.FlagCaCert, DefaultValue: "", Description: "Comma-separated CA bundle used to verify certificates of TLS-enabled secure servers."},
		declare.Flag{Name: internal.FlagInsecure, DefaultValue: true, Description: "Whether disables transport security for client connection."},
		declare.Flag{Name: internal.FlagInsecureSkipVerify, DefaultValue: false, Description: "Whether client verifies the server's certificate chain and host name. In this mode, TLS is susceptible to man-in-the-middle attacks."},
		declare.Flag{Name: internal.FlagUseServerCert, DefaultValue: false, Description: "Whether use server certificate as client certificate for client-authentication(mTLS)."},
		declare.Flag{Name: internal.FlagIgnoreInternalInterceptors, DefaultValue: false, Description: "Disable all internal interceptors of client."},
	)

	lifecycle.LifeCycle().HookInitialize(func() {
		if config.Bool(internal.FlagIgnoreProxyEnv) {
			_ = os.Setenv("no_proxy", "*")
		}
	}, lifecycle.WithName("Set no_proxy for dialer"))

	lifecycle.LifeCycle().HookFinalize(func(context.Context) { pool.Clear() }, lifecycle.WithName("Clear dialer pool"))
}

func callOptsToOutgoingContext(ctx context.Context, target string, opts []grpc.CallOption) context.Context {
	md := metadata.New(nil)
	for _, opt := range opts {
		if mdSet, ok := opt.(MetadataByCallOpt); ok {
			mdSet.SetMetadata(&md)
		}
	}

	for k, vs := range md {
		for _, v := range vs {
			ctx = metadata.AppendToOutgoingContext(ctx, k, v)
		}
	}

	return metadata.AppendToOutgoingContext(ctx, targetMeta, target)
}

func CreateCustomClientConnectionWithCallOptHook(dialFunc DialFunc, callOptHook CallOptionHook, presetCallOpt ...grpc.CallOption) *grpc.ClientConn {
	cc, _ := connection.MakeConnection("",
		interceptor.ChainUnaryClient(joinUnaryCallOpts(presetCallOpt...), unaryConnectionWrapper(dialFunc, callOptHook)),
		interceptor.ChainStreamClient(joinStreamCallOpts(presetCallOpt...), streamConnectionWrapper(dialFunc, callOptHook)),
	)
	return cc
}

func CreateCustomClientConnection(dialFunc DialFunc, presetCallOpt ...grpc.CallOption) *grpc.ClientConn {
	return CreateCustomClientConnectionWithCallOptHook(dialFunc, nil, presetCallOpt...)
}

func CreateClientConnectionWithCallOptHook(callOptHook CallOptionHook, presetCallOpt ...grpc.CallOption) *grpc.ClientConn {
	return CreateCustomClientConnectionWithCallOptHook(dial, callOptHook, presetCallOpt...)
}

func CreateClientConnection(presetCallOpt ...grpc.CallOption) *grpc.ClientConn {
	return CreateClientConnectionWithCallOptHook(nil, presetCallOpt...)
}

func dial(dialCtx context.Context, callOpts CallOptions) (*grpc.ClientConn, error) {
	cc, err := memory_dialer.Dial(dialCtx, callOpts)
	if err == nil && cc != nil {
		return cc, nil
	}

	if err != nil {
		if _, ok := err.(memory_registry.ErrNotFound); !ok {
			log.Errorf("In-memory_dialer dial failed, err = %+v", err)
		} else {
			log.Debugf("In-memory_dialer dial ignored, no in memory registry found for %q", callOpts.Target)
		}
	}

	// COMMENT: 很黑，但是没有更好的办法，DA内部是通过微服务框架RPC进程内调用的，对于用户传递的类似Target、Async等参数，DA是不会继续传递的，会导致这些CallOption失去作用，
	// 因此，微服务框架会将这些参数保存在Ctx中，在最终调用时带上这些参数，但是由于Target参数在DA内部调用过程中，可能与最初用户传递的Target不同（如用户传递IP，但DA内部必须用
	// 进程内注册的服务名进行调用），因此，只能将Ctx仅在最初一次进行设置，在进行真正的远程Dial时再取出进行设置
	if callOpts.Handler != nil {
		if handler, ok := callOpts.Handler.(func(o []grpc.CallOption) ([]grpc.CallOption, CallOptions, error)); ok {
			_, callOpts, err = handler(append(callOpts.CallOptFromCtx, callOpts.CallOpt...))
			if err != nil {
				return nil, err
			}
		}
	}

	if p, err := pool.WithPool(dialer.Dial, callOpts); err != nil {
		return nil, err
	} else {
		cc, err := p.Dial(dialCtx)
		if err != nil {
			return nil, err
		} else {
			return cc, nil
		}
	}
}

func dialCh(dialFunc DialFunc, callOpts CallOptions) chan interface{} {
	ch := make(chan interface{}, 1)

	go func(ch chan interface{}) {
		// 进行Dial时底层会设置context的timeout，因此这里直接使用Background
		cc, err := dialFunc(context.Background(), callOpts)
		if err == nil {
			ch <- cc
		} else {
			ch <- err
		}
	}(ch)

	return ch
}

func joinCallOpts(callOpts ...[]grpc.CallOption) []grpc.CallOption {
	var result []grpc.CallOption
	for idx := range callOpts {
		result = joinTwoCallOpts(result, callOpts[idx])
	}

	return result
}

func joinTwoCallOpts(lhs, rhs []grpc.CallOption) []grpc.CallOption {
Out:
	for _, r := range rhs {
		for _, l := range lhs {
			if l == r {
				continue Out
			}
		}
		lhs = append(lhs, r)
	}
	return lhs
}

func joinUnaryCallOpts(presetCallOpt ...grpc.CallOption) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpt ...grpc.CallOption) error {
		callOptFromCtx := CallOptionFromContext(ctx)
		if callOpt != nil {
			ctx = context.WithValue(ctx, callOptCtx{}, joinTwoCallOpts(callOptFromCtx, callOpt))
		}

		err := invoker(ctx, method, req, reply, cc, joinCallOpts(presetCallOpt, callOpt)...)
		return err
	}
}

func joinStreamCallOpts(presetCallOpt ...grpc.CallOption) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, callOpt ...grpc.CallOption) (grpc.ClientStream, error) {
		callOptFromCtx := CallOptionFromContext(ctx)
		if callOpt != nil {
			ctx = context.WithValue(ctx, callOptCtx{}, joinTwoCallOpts(callOptFromCtx, callOpt))
		}

		stream, err := streamer(ctx, desc, cc, method, joinCallOpts(presetCallOpt, callOpt)...)
		return stream, err
	}
}

func unaryConnectionWrapper(dialFunc DialFunc, callOptHook CallOptionHook) func(ctx context.Context, method string, req, reply interface{}, _ *grpc.ClientConn, _ grpc.UnaryInvoker, callOpt ...grpc.CallOption) (err error) {
	return func(ctx context.Context, method string, req, reply interface{}, _ *grpc.ClientConn, _ grpc.UnaryInvoker, callOpt ...grpc.CallOption) (err error) {
		if !lifecycle.LifeCycle().IsInitialized() {
			return fmt.Errorf("framework not initialized")
		}

		defer func() {
			if err != nil {
				err = root_internal.ToRPCErr(err)
			}
		}()

		callOptsHandler := func(o []grpc.CallOption) ([]grpc.CallOption, CallOptions, error) {
			return option.PrepareCallOptions(ctx, method, o, func(ctx context.Context, callOpt []grpc.CallOption, callOpts *option.CallOptions) []grpc.CallOption {
				if callOptHook == nil {
					return callOpt
				}
				return callOptHook(ctx, callOpt, callOpts)
			})
		}

		callOpt, callOpts, err := callOptsHandler(callOpt)
		if err != nil {
			return err
		}
		callOpts.Handler = callOptsHandler
		callOpts.CallOptFromCtx = CallOptionFromContext(ctx)

		select {
		case v := <-dialCh(dialFunc, callOpts):
			switch v := v.(type) {
			case error:
				return v
			case *grpc.ClientConn:
				return interceptor.ChainUnaryClient(option.PrepareUnaryInterceptor(callOpts, callOpts.IgnoreInternalInterceptors || connection.IsMemoryConnection(v))...)(ctx, method, req, reply, v,
					func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, _ ...grpc.CallOption) error {
						co := make([]grpc.CallOption, 0, len(callOpt))
						for _, o := range callOpt {
							if o == nil {
								continue
							}
							co = append(co, o)
						}
						if !connection.IsMemoryConnection(v) {
							ctx = callOptsToOutgoingContext(ctx, callOpts.Target, co)
						}
						return cc.Invoke(ctx, method, req, reply, co...)
					}, callOpt...)
			default:
				return fmt.Errorf("invalid dial status in CreateClientConnection, v = %+v", v)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func streamConnectionWrapper(dialFunc DialFunc, callOptHook CallOptionHook) func(ctx context.Context, desc *grpc.StreamDesc, _ *grpc.ClientConn, method string, _ grpc.Streamer, callOpt ...grpc.CallOption) (cc grpc.ClientStream, err error) {
	return func(ctx context.Context, desc *grpc.StreamDesc, _ *grpc.ClientConn, method string, _ grpc.Streamer, callOpt ...grpc.CallOption) (cc grpc.ClientStream, err error) {
		if !lifecycle.LifeCycle().IsInitialized() {
			return nil, fmt.Errorf("framework not initialized")
		}

		callOptsHandler := func(o []grpc.CallOption) ([]grpc.CallOption, CallOptions, error) {
			return option.PrepareCallOptions(ctx, method, o, func(ctx context.Context, callOpt []grpc.CallOption, callOpts *option.CallOptions) []grpc.CallOption {
				if callOptHook == nil {
					return callOpt
				}
				return callOptHook(ctx, callOpt, callOpts)
			})
		}

		callOpt, callOpts, err := callOptsHandler(callOpt)
		if err != nil {
			return nil, err
		}
		callOpts.Handler = callOptsHandler
		callOpts.CallOptFromCtx = CallOptionFromContext(ctx)

		select {
		case v := <-dialCh(dialFunc, callOpts):
			switch v := v.(type) {
			case error:
				return nil, v
			case *grpc.ClientConn:
				return interceptor.ChainStreamClient(option.PrepareStreamInterceptor(callOpts, callOpts.IgnoreInternalInterceptors || connection.IsMemoryConnection(v))...)(ctx, desc, v, method,
					func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, _ ...grpc.CallOption) (grpc.ClientStream, error) {
						co := make([]grpc.CallOption, 0, len(callOpt))
						for _, o := range callOpt {
							if o == nil {
								continue
							}
							co = append(co, o)
						}
						if !connection.IsMemoryConnection(v) {
							ctx = callOptsToOutgoingContext(ctx, callOpts.Target, co)
						}
						return cc.NewStream(ctx, desc, method, co...)
					}, callOpt...)
			default:
				return nil, fmt.Errorf("invalid dial status in CreateClientConnection, v = %+v", v)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func CallOptionFromContext(ctx context.Context) []grpc.CallOption {
	if callOpt, ok := ctx.Value(callOptCtx{}).([]grpc.CallOption); ok {
		return callOpt
	} else {
		return nil
	}
}

func ClearPool() {
	pool.Clear()
}
