package dialer

import (
	"context"
	"errors"
	"net"
	"time"

	"google.golang.org/grpc/keepalive"

	"github.com/zhangel/go-framework.git/async"

	"github.com/zhangel/go-framework.git/balancer"
	"github.com/zhangel/go-framework.git/credentials"
	"github.com/zhangel/go-framework.git/dialer/internal/option"
	"github.com/zhangel/go-framework.git/registry"
	"github.com/zhangel/go-framework.git/tracing"
	"google.golang.org/grpc"
)

type DialOption option.DialOption
type DialOptions *option.DialOptions
type CallOptions *option.CallOptions
type PoolIdentity func() string
type PoolIdentityProvider func(PoolIdentity) string
type CallOptionHook func(context.Context, []grpc.CallOption, CallOptions) []grpc.CallOption

func DialWithCodec(codec grpc.Codec) DialOption {
	return func(opts *option.DialOptions) error {
		opts.Codec = codec
		return nil
	}
}

func DialWithBalancer(provider balancer.Provider) DialOption {
	return func(opts *option.DialOptions) error {
		if provider == nil {
			opts.BalancerBuilder = balancer.DefaultBalancerBuilder()
		} else {
			builder, err := balancer.BuilderWithProvider(provider)
			if err != nil {
				return err
			}
			opts.BalancerBuilder = builder
		}
		return nil
	}
}

func DialWithRegistry(registry registry.Registry) DialOption {
	return func(opts *option.DialOptions) error {
		opts.Registry = registry
		return nil
	}
}

func DialWithTimeout(timeout time.Duration) DialOption {
	return func(opts *option.DialOptions) error {
		opts.DialTimeout = timeout
		return nil
	}
}

func DialWithCredentialsOption(opt ...credentials.ClientOptionFunc) DialOption {
	return func(opts *option.DialOptions) error {
		opts.CredentialOpts = append(opts.CredentialOpts, opt...)
		return nil
	}
}

func DialWithPoolSize(size int) DialOption {
	return func(opts *option.DialOptions) error {
		if size < 1 {
			return errors.New("pool size must larger than 0")
		}
		opts.PoolSize = size
		return nil
	}
}

func DialWithCustomDialer(dialFunc func(ctx context.Context, addr string) (net.Conn, error)) DialOption {
	return func(opts *option.DialOptions) error {
		opts.DialFunc = dialFunc
		return nil
	}
}

func DialWithCredentialsFromRegistry(enable bool) DialOption {
	return func(opts *option.DialOptions) error {
		opts.CredentialFromRegistry = enable
		return nil
	}
}

func DialWithReadBufferSize(s int) DialOption {
	return func(opts *option.DialOptions) error {
		opts.ReadBufferSize = s
		return nil
	}
}

func DialWithWriteBufferSize(s int) DialOption {
	return func(opts *option.DialOptions) error {
		opts.WriteBufferSize = s
		return nil
	}
}

func DialWithKeepAliveParams(kp keepalive.ClientParameters) DialOption {
	return func(opts *option.DialOptions) error {
		opts.KeepAliveParameters = &kp
		return nil
	}
}

func WithDialOptTag(dialOptTag string) grpc.CallOption {
	return &option.DialOptTagOpt{DialOptTag: dialOptTag}
}

func WithUseDialOptTagAsPoolId(enable bool) grpc.CallOption {
	return &option.DialOptTagAsPoolIdOpt{Enable: enable}
}

func WithUseInProcDial(useInProcDial bool) grpc.CallOption {
	return &option.UseInProcDialOpt{UseInProcDial: useInProcDial}
}

func WithTarget(target string) grpc.CallOption {
	return &option.TargetCallOpt{Target: target}
}

func WithTracer(tracer tracing.Tracer) grpc.CallOption {
	return &option.TracerCallOpt{Tracer: tracer}
}

func WithAsync(opts ...async.Option) grpc.CallOption {
	opt := &option.AsyncCallOpt{}
	for _, o := range opts {
		o(&opt.Options)
	}
	return opt
}

func WithUnaryInterceptor(unaryInt grpc.UnaryClientInterceptor) grpc.CallOption {
	return &option.UnaryInterceptorCallOpt{UnaryInt: unaryInt}
}

func WithStreamInterceptor(streamInt grpc.StreamClientInterceptor) grpc.CallOption {
	return &option.StreamInterceptorCallOpt{StreamInt: streamInt}
}

func WithIgnoreInternalInterceptors(ignore bool) grpc.CallOption {
	return &option.IgnoreInternalInterceptorsCallOpt{Ignore: ignore}
}

func RegisterDialOption(dialOptTag string, opt ...DialOption) error {
	dialOpt := make([]option.DialOption, 0, len(opt))
	for _, o := range opt {
		dialOpt = append(dialOpt, option.DialOption(o))
	}
	return option.RegisterDialOption(dialOptTag, dialOpt...)
}

func IsDialOptionRegistered(dialOptTag string) bool {
	return option.IsDialOptionRegistered(dialOptTag)
}

func QueryDialOption(dialOptTag string) ([]DialOption, bool) {
	dialOpt, ok := option.QueryDialOption(dialOptTag)
	if !ok {
		return nil, false
	}
	result := make([]DialOption, 0, len(dialOpt))
	for _, o := range dialOpt {
		result = append(result, DialOption(o))
	}
	return result, true
}

// Deprecated: please use QueryDialOption.  May be removed in a future release.
func DialOptionForService(serviceName string) ([]DialOption, bool) {
	return QueryDialOption(serviceName)
}

func SetPoolIdentityProvider(callOpts CallOptions, provider PoolIdentityProvider) {
	((*option.CallOptions)(callOpts)).SetPoolIdentityProvider(func(generator option.PoolIdentity) string {
		return provider(PoolIdentity(generator))
	})
}
