package option

import (
	"context"
	"net"
	"time"

	"github.com/zhangel/go-framework.git/async"
	"github.com/zhangel/go-framework.git/balancer"
	"github.com/zhangel/go-framework.git/credentials"
	"github.com/zhangel/go-framework.git/registry"
	"github.com/zhangel/go-framework.git/tracing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type DialOption func(*DialOptions) error
type PoolIdentity func() string
type PoolIdentityProvider func(PoolIdentity) string
type CallOptionHook func(context.Context, []grpc.CallOption, *CallOptions) []grpc.CallOption

type DialOptions struct {
	Codec                  grpc.Codec
	BalancerBuilder        balancer.Builder
	Registry               registry.Registry
	DialTimeout            time.Duration
	CredentialOpts         []credentials.ClientOptionFunc
	PoolSize               int
	DialFunc               func(ctx context.Context, addr string) (net.Conn, error)
	CredentialFromRegistry bool
	ReadBufferSize         int
	WriteBufferSize        int
	KeepAliveParameters    *keepalive.ClientParameters
}

type CallOptions struct {
	DialOptTag                 []string
	UseInProcDial              bool
	Target                     string
	ServiceName                string
	Tracer                     tracing.Tracer
	UnaryInt                   []grpc.UnaryClientInterceptor
	StreamInt                  []grpc.StreamClientInterceptor
	TargetTransformer          *registry.TargetTransformer
	IgnoreInternalInterceptors bool
	AsyncOpt                   *AsyncCallOpt
	DialOptTagAsPoolId         bool

	CallOpt        []grpc.CallOption
	CallOptFromCtx []grpc.CallOption
	Handler        interface{}
	poolIdentity   PoolIdentity
}

func (s *CallOptions) SetPoolIdentityProvider(provider PoolIdentityProvider) {
	prevGenerator := s.poolIdentity
	s.poolIdentity = func() string {
		return provider(prevGenerator)
	}
}

func (s *CallOptions) PoolIdentity() string {
	return s.poolIdentity()
}

type DialOptTagOpt struct {
	grpc.EmptyCallOption
	DialOptTag string
}

type DialOptTagAsPoolIdOpt struct {
	grpc.EmptyCallOption
	Enable bool
}

type UseInProcDialOpt struct {
	grpc.EmptyCallOption
	UseInProcDial bool
}

type TargetCallOpt struct {
	grpc.EmptyCallOption
	Target string
}

type TracerCallOpt struct {
	grpc.EmptyCallOption
	Tracer tracing.Tracer
}

type UnaryInterceptorCallOpt struct {
	grpc.EmptyCallOption
	UnaryInt grpc.UnaryClientInterceptor
}

type StreamInterceptorCallOpt struct {
	grpc.EmptyCallOption
	StreamInt grpc.StreamClientInterceptor
}

type AsyncCallOpt struct {
	grpc.EmptyCallOption
	async.Options
}

type IgnoreInternalInterceptorsCallOpt struct {
	grpc.EmptyCallOption
	Ignore bool
}
