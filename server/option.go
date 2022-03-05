package server

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/zhangel/go-framework.git/authentication"
	"github.com/zhangel/go-framework.git/credentials"
	"github.com/zhangel/go-framework.git/healthy"
	"github.com/zhangel/go-framework.git/hooks"
	"github.com/zhangel/go-framework.git/registry"
	"github.com/zhangel/go-framework.git/server/internal"
	"github.com/zhangel/go-framework.git/server/internal/option"
	"github.com/zhangel/go-framework.git/server/internal/service"
	"github.com/zhangel/go-framework.git/tracing"
)

const FlagCert = internal.FlagCert
const FlagKey = internal.FlagKey

const AutoSelectAddr = internal.AutoSelectAddr
const MultiplexServerAddr = internal.MultiplexServerAddr

const (
	GrpcAddr = iota
	HttpAddr
)

type Option option.Option
type Options *option.Options

func WithAddr(addr string) Option {
	return Option(option.WithAddr(addr))
}

func WithHttpAddr(addr string) Option {
	return Option(option.WithHttpAddr(addr))
}

func WithRegistry(reg registry.Registry) Option {
	return Option(option.WithRegistry(reg))
}

func WithTracer(tracer tracing.Tracer) Option {
	return Option(option.WithTracer(tracer))
}

func WithRecovery(enable bool) Option {
	return Option(option.WithRecovery(enable))
}

func WithAuthentication(authProvider authentication.Provider) Option {
	return Option(option.WithAuthentication(authProvider))
}

func WithEnableHttpServer(enable bool) Option {
	return Option(option.WithEnableHttpServer(enable))
}

func WithEnableHttpGzip(enable bool) Option {
	return Option(option.WithEnableHttpGzip(enable))
}

func WithHttpMaxConcurrentConnection(maxConn int) Option {
	return Option(option.WithHttpMaxConcurrentConnection(maxConn))
}

func WithUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) Option {
	return Option(option.WithUnaryInterceptor(interceptor))
}

func WithStreamInterceptor(interceptor grpc.StreamServerInterceptor) Option {
	return Option(option.WithStreamInterceptor(interceptor))
}

func WithUnaryHookInterceptor(interceptor ...hooks.UnaryHookInterceptor) Option {
	return Option(option.WithUnaryHookInterceptor(interceptor...))
}

func WithStreamHookInterceptor(interceptor ...hooks.StreamHookInterceptor) Option {
	return Option(option.WithStreamHookInterceptor(interceptor...))
}

func WithEnableGrpcWeb(enable bool) Option {
	return Option(option.WithEnableGrpcWeb(enable))
}

func WithEnableGrpcWebWithWs(enable bool) Option {
	return Option(option.WithEnableGrpcWebWithWs(enable))
}

func WithCredentialsOption(opt ...credentials.ServerOptionFunc) Option {
	return Option(option.WithCredentialsOption(opt...))
}

func WithJsonOrigNameOption(origName bool) Option {
	return Option(option.WithJsonOrigNameOption(origName))
}

func WithJsonEnumAsIntsOption(enumAsInts bool) Option {
	return Option(option.WithJsonEnumAsIntsOption(enumAsInts))
}

func WithJsonEmitDefaultsOption(emitDefaults bool) Option {
	return Option(option.WithJsonEmitDefaultsOption(emitDefaults))
}

func WithHttpReadTimeout(tmo time.Duration) Option {
	return Option(option.WithHttpReadTimeout(tmo))
}

func WithHttpWriteTimeout(tmo time.Duration) Option {
	return Option(option.WithHttpWriteTimeout(tmo))
}

func WithHttpIdleTimeout(tmo time.Duration) Option {
	return Option(option.WithHttpIdleTimeout(tmo))
}

func WithHttpKeepAlive(keepAlive bool) Option {
	return Option(option.WithHttpKeepAlive(keepAlive))
}

func WithPackageName(enable bool) Option {
	return Option(option.WithPackageName(enable))
}

func WithGrpcGatewayCompatible(enable bool) Option {
	return Option(option.WithGrpcGatewayCompatible(enable))
}

func WithAllowedOrigins(allowedOrigins []string) Option {
	return Option(option.WithAllowedOrigins(allowedOrigins))
}

func WithAllowedRequestHeaders(allowedRequestHeaders []string) Option {
	return Option(option.WithAllowedRequestHeaders(allowedRequestHeaders))
}

func WithAllowedMethods(allowedMethods []string) Option {
	return Option(option.WithAllowedMethods(allowedMethods))
}

func WithCorsLogger(enable bool) Option {
	return Option(option.WithCorsLogger(enable))
}

func WithMaxConcurrentStreams(maxConcurrentStreams uint32) Option {
	return Option(option.WithMaxConcurrentStreams(maxConcurrentStreams))
}

func WithIgnoreInternalInterceptors(ignore bool) Option {
	return Option(option.WithIgnoreInternalInterceptors(ignore))
}

func WithMaxRecvMsgSize(maxRecvMsgSize int) Option {
	return Option(option.WithMaxRecvMsgSize(maxRecvMsgSize))
}

func WithMaxSendMsgSize(maxRecvMsgSize int) Option {
	return Option(option.WithMaxSendMsgSize(maxRecvMsgSize))
}

func WithKeepAliveParams(params keepalive.ServerParameters) Option {
	return Option(option.WithKeepAliveParams(params))
}

func WithKeepAliveEnforcementPolicy(policy keepalive.EnforcementPolicy) Option {
	return Option(option.WithKeepAliveEnforcementPolicy(policy))
}

func WithHealthService(healthService healthy.HealthService) Option {
	return Option(option.WithHealthService(healthService))
}

func WithOptionsHook(hook func(opts Options)) Option {
	return Option(option.WithOptionsHook(func(opts *option.Options) {
		hook(opts)
	}))
}

func WithLegacyHttpRegistry(legacyHttpRegistry bool) Option {
	return Option(option.WithLegacyHttpRegistry(legacyHttpRegistry))
}

func WithGrpcServerReflection(enable bool) Option {
	return Option(option.WithGrpcServerReflection(enable))
}

func WithGrpcServerReflectionAuth(authHandler func(ctx context.Context) error) Option {
	return Option(option.WithGrpcServerReflectionAuth(authHandler))
}

type RegisterOption option.RegisterOption

func WithAlias(alias ...string) RegisterOption {
	return RegisterOption(option.WithAlias(alias...))
}

func WithRegistryHandler(handler internal.RegistryHandler) RegisterOption {
	return RegisterOption(option.WithRegistryHandler(handler))
}

func WithServiceStartHook(serviceStartHook func(provider ServiceProvider) error) RegisterOption {
	return RegisterOption(option.WithServiceStartHook(func(provider service.Provider) error {
		return serviceStartHook(provider)
	}))
}

func WithServiceStopHook(serviceStopHook func(provider ServiceProvider)) RegisterOption {
	return RegisterOption(option.WithServiceStopHook(func(provider service.Provider) {
		serviceStopHook(provider)
	}))
}

func WithServiceProviderInitializer(initializer func() (ServiceProvider, error)) RegisterOption {
	return RegisterOption(option.WithServiceProviderInitializer(func() (service.Provider, error) {
		return initializer()
	}))
}

func WithAliasOnlyMemoryRegistry(aliasOnly bool) RegisterOption {
	return RegisterOption(option.WithAliasOnlyMemoryRegistry(aliasOnly))
}

func WithBindAddrCallback(cb func(typ int, addr string)) RegisterOption {
	return RegisterOption(option.WithBindAddrCallback(cb))
}

func WithGrpcHttpParameterHandler(parameterName string, handler option.ParameterHandler) RegisterOption {
	return RegisterOption(option.WithGrpcHttpParameterHandler(parameterName, handler))
}

type HttpRegisterOption option.HttpRegisterOption

func WithHttpServiceStartHook(serviceStartHook func(handler http.Handler) error) HttpRegisterOption {
	return HttpRegisterOption(option.WithHttpServiceStartHook(func(handler http.Handler) error {
		return serviceStartHook(handler)
	}))
}

func WithHttpServiceStopHook(serviceStopHook func(handler http.Handler)) HttpRegisterOption {
	return HttpRegisterOption(option.WithHttpServiceStopHook(func(handler http.Handler) {
		serviceStopHook(handler)
	}))
}

func WithHttpBindAddrCallback(cb func(string)) HttpRegisterOption {
	return HttpRegisterOption(option.WithHttpBindAddrCallback(cb))
}
