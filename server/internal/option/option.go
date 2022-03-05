package option

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/zhangel/go-framework.git/authentication"
	"github.com/zhangel/go-framework.git/credentials"
	"github.com/zhangel/go-framework.git/healthy"
	"github.com/zhangel/go-framework.git/hooks"
	"github.com/zhangel/go-framework.git/registry"
	server_internal "github.com/zhangel/go-framework.git/server/internal"
	"github.com/zhangel/go-framework.git/server/internal/service"
	"github.com/zhangel/go-framework.git/tracing"
)

type Options struct {
	Addr                        string
	HttpAddr                    string
	MultiplexAddr               bool
	Registry                    registry.Registry
	Tracer                      tracing.Tracer
	UnaryInterceptors           []grpc.UnaryServerInterceptor
	StreamInterceptors          []grpc.StreamServerInterceptor
	UnaryHookInterceptors       []hooks.UnaryHookInterceptor
	StreamHookInterceptors      []hooks.StreamHookInterceptor
	CredentialOpts              []credentials.ServerOptionFunc
	AuthProvider                authentication.Provider
	Recovery                    bool
	EnableHttp                  bool
	EnableHttpGzip              bool
	OrigName                    bool
	EnumAsInts                  bool
	EmitDefaults                bool
	HttpReadTimeout             time.Duration
	HttpWriteTimeout            time.Duration
	HttpIdleTimeout             time.Duration
	HttpKeepAlive               bool
	GrpcGatewayCompatible       bool
	WithPackageName             bool
	EnableGrpcWeb               bool
	EnableGrpcWebWithWs         bool
	AllowedOrigins              []string
	AllowedRequestHeaders       []string
	AllowedMethods              []string
	EnableCorsLog               bool
	MaxConcurrentStreams        uint32
	IgnoreInternalInterceptors  bool
	MaxRecvMsgSize              int
	MaxSendMsgSize              int
	WithTls                     bool
	MaxHttpConcurrentConnection int
	KeepAliveParams             *keepalive.ServerParameters
	KeepAliveEnforcementPolicy  *keepalive.EnforcementPolicy
	HealthService               healthy.HealthService
	OptionsHook                 func(opts *Options)
	MinPort                     int
	MaxPort                     int
	LegacyHttpRegistry          bool
	GrpcServerReflection        bool
	GrpcServerReflectionAuth    []func(ctx context.Context) error
}

type Option func(*Options) error

func WithRegistry(reg registry.Registry) Option {
	return func(opts *Options) error {
		opts.Registry = reg
		return nil
	}
}

func WithTracer(tracer tracing.Tracer) Option {
	return func(opts *Options) error {
		opts.Tracer = tracer
		return nil
	}
}

func WithUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) Option {
	return func(opts *Options) error {
		if interceptor == nil {
			return errors.New("interceptor is nil")
		}

		opts.UnaryInterceptors = append(opts.UnaryInterceptors, interceptor)
		return nil
	}
}

func WithStreamInterceptor(interceptor grpc.StreamServerInterceptor) Option {
	return func(opts *Options) error {
		if interceptor == nil {
			return errors.New("interceptor is nil")
		}

		opts.StreamInterceptors = append(opts.StreamInterceptors, interceptor)
		return nil
	}
}

func WithUnaryHookInterceptor(interceptor ...hooks.UnaryHookInterceptor) Option {
	return func(opts *Options) error {
		if interceptor == nil {
			return errors.New("interceptor is nil")
		}

		opts.UnaryHookInterceptors = append(opts.UnaryHookInterceptors, interceptor...)
		return nil
	}
}

func WithStreamHookInterceptor(interceptor ...hooks.StreamHookInterceptor) Option {
	return func(opts *Options) error {
		if interceptor == nil {
			return errors.New("interceptor is nil")
		}

		opts.StreamHookInterceptors = append(opts.StreamHookInterceptors, interceptor...)
		return nil
	}
}

func WithRecovery(enable bool) Option {
	return func(opts *Options) error {
		opts.Recovery = enable
		return nil
	}
}

func WithAuthentication(authProvider authentication.Provider) Option {
	return func(opts *Options) error {
		opts.AuthProvider = authProvider
		return nil
	}
}

func WithEnableHttpServer(enable bool) Option {
	return func(opts *Options) error {
		opts.EnableHttp = enable
		return nil
	}
}

func WithEnableHttpGzip(enable bool) Option {
	return func(opts *Options) error {
		opts.EnableHttpGzip = enable
		return nil
	}
}

func WithHttpMaxConcurrentConnection(maxConn int) Option {
	return func(opts *Options) error {
		if maxConn != -1 && maxConn <= 0 {
			return fmt.Errorf("invalid max concurrent connections")
		}
		opts.MaxHttpConcurrentConnection = maxConn
		return nil
	}
}

func WithEnableGrpcWeb(enable bool) Option {
	return func(opts *Options) error {
		opts.EnableGrpcWeb = enable
		return nil
	}
}

func WithEnableGrpcWebWithWs(enable bool) Option {
	return func(opts *Options) error {
		opts.EnableGrpcWebWithWs = enable
		return nil
	}
}

func WithCredentialsOption(opt ...credentials.ServerOptionFunc) Option {
	return func(opts *Options) error {
		opts.CredentialOpts = append(opts.CredentialOpts, opt...)
		return nil
	}
}

func WithJsonOrigNameOption(origName bool) Option {
	return func(opts *Options) error {
		opts.OrigName = origName
		return nil
	}
}

func WithJsonEnumAsIntsOption(enumAsInts bool) Option {
	return func(opts *Options) error {
		opts.EnumAsInts = enumAsInts
		return nil
	}
}

func WithJsonEmitDefaultsOption(emitDefaults bool) Option {
	return func(opts *Options) error {
		opts.EmitDefaults = emitDefaults
		return nil
	}
}

func WithHttpReadTimeout(tmo time.Duration) Option {
	return func(opts *Options) error {
		opts.HttpReadTimeout = tmo
		return nil
	}
}

func WithHttpWriteTimeout(tmo time.Duration) Option {
	return func(opts *Options) error {
		opts.HttpWriteTimeout = tmo
		return nil
	}
}

func WithHttpIdleTimeout(tmo time.Duration) Option {
	return func(opts *Options) error {
		opts.HttpIdleTimeout = tmo
		return nil
	}
}

func WithHttpKeepAlive(keepAlive bool) Option {
	return func(opts *Options) error {
		opts.HttpKeepAlive = keepAlive
		return nil
	}
}

func WithPackageName(enable bool) Option {
	return func(opts *Options) error {
		opts.WithPackageName = enable
		return nil
	}
}

func WithGrpcGatewayCompatible(enable bool) Option {
	return func(opts *Options) error {
		opts.GrpcGatewayCompatible = enable
		return nil
	}
}

func WithAllowedOrigins(allowedOrigins []string) Option {
	return func(opts *Options) error {
		opts.AllowedOrigins = allowedOrigins
		return nil
	}
}

func WithAllowedRequestHeaders(allowedRequestHeaders []string) Option {
	return func(opts *Options) error {
		opts.AllowedRequestHeaders = allowedRequestHeaders
		return nil
	}
}

func WithAllowedMethods(allowedMethods []string) Option {
	return func(opts *Options) error {
		opts.AllowedMethods = allowedMethods
		return nil
	}
}

func WithCorsLogger(enable bool) Option {
	return func(opts *Options) error {
		opts.EnableCorsLog = enable
		return nil
	}
}

func WithMaxConcurrentStreams(maxConcurrentStreams uint32) Option {
	return func(opts *Options) error {
		opts.MaxConcurrentStreams = maxConcurrentStreams
		return nil
	}
}

func WithIgnoreInternalInterceptors(ignore bool) Option {
	return func(opts *Options) error {
		opts.IgnoreInternalInterceptors = ignore
		return nil
	}
}

func WithMaxRecvMsgSize(maxRecvMsgSize int) Option {
	return func(opts *Options) error {
		opts.MaxRecvMsgSize = maxRecvMsgSize
		return nil
	}
}

func WithMaxSendMsgSize(maxSendMsgSize int) Option {
	return func(opts *Options) error {
		opts.MaxSendMsgSize = maxSendMsgSize
		return nil
	}
}

func WithKeepAliveParams(params keepalive.ServerParameters) Option {
	return func(opts *Options) error {
		opts.KeepAliveParams = &params
		return nil
	}
}

func WithKeepAliveEnforcementPolicy(policy keepalive.EnforcementPolicy) Option {
	return func(opts *Options) error {
		opts.KeepAliveEnforcementPolicy = &policy
		return nil
	}
}

func WithHealthService(healthService healthy.HealthService) Option {
	return func(opts *Options) error {
		opts.HealthService = healthService
		return nil
	}
}

func WithOptionsHook(hook func(opts *Options)) Option {
	return func(opts *Options) error {
		opts.OptionsHook = hook
		return nil
	}
}

func WithPortRange(portRange string) Option {
	return func(opts *Options) error {
		opts.MinPort = -1
		opts.MaxPort = -1

		portRanges := strings.Split(portRange, "-")
		if len(portRanges) >= 1 {
			if minPort, err := strconv.Atoi(portRanges[0]); err != nil {
				return fmt.Errorf("port range invalid, err = %v", err)
			} else {
				opts.MinPort = minPort
			}
		}

		if len(portRanges) == 2 {
			if maxPort, err := strconv.Atoi(portRanges[1]); err == nil {
				opts.MaxPort = maxPort
			}
		}
		return nil
	}
}

func WithLegacyHttpRegistry(legacyHttpRegistry bool) Option {
	return func(opts *Options) error {
		opts.LegacyHttpRegistry = legacyHttpRegistry
		return nil
	}
}

func WithGrpcServerReflection(enable bool) Option {
	return func(opts *Options) error {
		opts.GrpcServerReflection = enable
		return nil
	}
}

func WithGrpcServerReflectionAuth(authHandler func(ctx context.Context) error) Option {
	return func(opts *Options) error {
		if authHandler == nil {
			return nil
		}
		opts.GrpcServerReflectionAuth = append(opts.GrpcServerReflectionAuth, authHandler)
		return nil
	}
}

type ParameterHandler func(httpReq *http.Request, values []string, grpcReq proto.Message) (proto.Message, bool)

type RegisterOptions struct {
	Alias                      []string
	RegistryHandler            server_internal.RegistryHandler
	ServiceStartHook           func(provider service.Provider) error
	ServiceStartHookOpts       *ServiceHookOptions
	ServiceStopHook            func(provider service.Provider)
	ServiceStopHookOpts        *ServiceHookOptions
	ServiceProviderInitializer func() (service.Provider, error)
	AliasOnly                  bool
	BindAddrCallback           func(typ int, addr string)
	HttpParameterHandler       map[string]ParameterHandler
}

type RegisterOption func(*RegisterOptions) error

func WithAlias(alias ...string) RegisterOption {
	return func(opts *RegisterOptions) error {
		opts.Alias = make([]string, 0, len(alias))
		for _, a := range alias {
			opts.Alias = append(opts.Alias, strings.TrimSpace(a))
		}
		return nil
	}
}

func WithRegistryHandler(handler server_internal.RegistryHandler) RegisterOption {
	return func(opts *RegisterOptions) error {
		opts.RegistryHandler = handler
		return nil
	}
}

func WithServiceStartHook(serviceStartHook func(provider service.Provider) error, opt ...ServiceHookOption) RegisterOption {
	return func(opts *RegisterOptions) error {
		opts.ServiceStartHook = serviceStartHook
		opts.ServiceStartHookOpts = defaultServiceHookOption()
		for _, o := range opt {
			if err := o(opts.ServiceStartHookOpts); err != nil {
				return err
			}
		}
		return nil
	}
}

func WithServiceStopHook(serviceStopHook func(provider service.Provider), opt ...ServiceHookOption) RegisterOption {
	return func(opts *RegisterOptions) error {
		opts.ServiceStopHook = serviceStopHook
		opts.ServiceStopHookOpts = defaultServiceHookOption()
		for _, o := range opt {
			if err := o(opts.ServiceStopHookOpts); err != nil {
				return err
			}
		}
		return nil
	}
}

func WithServiceProviderInitializer(initializer func() (service.Provider, error)) RegisterOption {
	return func(opts *RegisterOptions) error {
		opts.ServiceProviderInitializer = initializer
		return nil
	}
}

func WithAliasOnlyMemoryRegistry(aliasOnly bool) RegisterOption {
	return func(opts *RegisterOptions) error {
		opts.AliasOnly = aliasOnly
		return nil
	}
}

func WithBindAddrCallback(cb func(typ int, addr string)) RegisterOption {
	return func(opts *RegisterOptions) error {
		opts.BindAddrCallback = cb
		return nil
	}
}

func WithGrpcHttpParameterHandler(parameterName string, handler ParameterHandler) RegisterOption {
	return func(opts *RegisterOptions) error {
		if opts.HttpParameterHandler == nil {
			opts.HttpParameterHandler = map[string]ParameterHandler{}
		}
		opts.HttpParameterHandler[parameterName] = handler
		return nil
	}
}

type HttpRegisterOptions struct {
	ServiceStartHook     func(handler http.Handler) error
	ServiceStartHookOpts *ServiceHookOptions
	ServiceStopHook      func(handler http.Handler)
	ServiceStopHookOpts  *ServiceHookOptions
	BindAddrCallback     func(string)
}

type HttpRegisterOption func(*HttpRegisterOptions) error

func WithHttpServiceStartHook(serviceStartHook func(handler http.Handler) error, opt ...ServiceHookOption) HttpRegisterOption {
	return func(opts *HttpRegisterOptions) error {
		opts.ServiceStartHook = serviceStartHook
		opts.ServiceStartHookOpts = defaultServiceHookOption()
		for _, o := range opt {
			if err := o(opts.ServiceStartHookOpts); err != nil {
				return err
			}
		}
		return nil
	}
}

func WithHttpServiceStopHook(serviceStopHook func(handler http.Handler), opt ...ServiceHookOption) HttpRegisterOption {
	return func(opts *HttpRegisterOptions) error {
		opts.ServiceStopHook = serviceStopHook
		opts.ServiceStopHookOpts = defaultServiceHookOption()
		for _, o := range opt {
			if err := o(opts.ServiceStopHookOpts); err != nil {
				return err
			}
		}
		return nil
	}
}

func WithHttpBindAddrCallback(cb func(addr string)) HttpRegisterOption {
	return func(opts *HttpRegisterOptions) error {
		opts.BindAddrCallback = cb
		return nil
	}
}

type ServiceHookOptions struct {
	Timeout time.Duration
}

type ServiceHookOption func(options *ServiceHookOptions) error

func defaultServiceHookOption() *ServiceHookOptions {
	return &ServiceHookOptions{
		Timeout: 1 * time.Minute,
	}
}

func WithServiceHookTimeout(timeout time.Duration) ServiceHookOption {
	return func(opts *ServiceHookOptions) error {
		opts.Timeout = timeout
		return nil
	}
}
