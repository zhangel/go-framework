package server

import (
	"log"
	"net/http"
	"time"

	"github.com/zhangel/go-framework/declare"
	"github.com/zhangel/go-framework/lifecycle"
	"github.com/zhangel/go-framework/memory_registry"
	"github.com/zhangel/go-framework/server/internal"
	internal_http "github.com/zhangel/go-framework/server/internal/http_server"
	"github.com/zhangel/go-framework/server/internal/option"
	"github.com/zhangel/go-framework/server/internal/server"
	"github.com/zhangel/go-framework/server/internal/service"
)

type ServiceProvider service.Provider
type DefaultProviderImpl struct{ service.DefaultProviderImpl }
type Route internal_http.Route

type Server interface {
	RegisterService(srvRegister interface{}, srvHandler ServiceProvider, opt ...RegisterOption) error
	RegisterHttpService(route Route, handler http.Handler, opt ...HttpRegisterOption) error
	ServiceInfos() ([]*memory_registry.ServiceInfo, error)
	Run(option ...Option) error
}

type serverImpl struct {
	srv *server.Server
}

func init() {
	declare.Flags(internal.ServerPrefix,
		option.AddrFlag,
		declare.Flag{Name: internal.FlagHttpAddr, DefaultValue: internal.AutoSelectAddr, Description: "Bind address of the http/grpc-web server. options: address, 'auto', 'mux'"},
		declare.Flag{Name: internal.FlagRecovery, DefaultValue: true, Description: "Enable panic-recovery of the server."},
		declare.Flag{Name: internal.FlagHttpServer, DefaultValue: true, Description: "Enable http server on the same port(only unary service supported)."},
		declare.Flag{Name: internal.FlagGrpcWeb, DefaultValue: true, Description: "Enable grpc-web server on the same port."},
		declare.Flag{Name: internal.FlagGrpcWebWithWs, DefaultValue: true, Description: "Enable grpc-web server with WebSocket support on the same port."},
		declare.Flag{Name: internal.FlagCert, DefaultValue: "", Description: "Using this TLS certificate file to identify secure server."},
		declare.Flag{Name: internal.FlagKey, DefaultValue: "", Description: "Using this TLS key file to identify secure server."},
		declare.Flag{Name: internal.FlagCaCert, DefaultValue: "", Description: "Comma-separated CA bundle used to verify certificates of TLS-enabled clients."},
		declare.Flag{Name: internal.FlagInsecure, DefaultValue: true, Description: "Whether allow connection without transport layer security."},
		declare.Flag{Name: internal.FlagSelfSignCertSAN, DefaultValue: "", Description: "Use self-signed certificate with SAN specified as server side certificate."},
		declare.Flag{Name: internal.FlagClientAuthType, DefaultValue: 3, Description: `ClientAuthType declares the policy the server will follow for TLS Client Authentication. Valid values are:
- 0: NoClientCert
- 1: RequestClientCert
- 2: RequireAnyClientCert
- 3: VerifyClientCertIfGiven
- 4: RequireAndVerifyClientCert`},
		declare.Flag{Name: internal.FlagOrigName, DefaultValue: true, Description: "Whether use the original (.proto) name for fields in HTTP server."},
		declare.Flag{Name: internal.FlagEnumAsInts, DefaultValue: false, Description: "Whether render enum values as integers, as opposed to string values in HTTP server."},
		declare.Flag{Name: internal.FlagEmitDefaults, DefaultValue: true, Description: "Whether render fields with zero values in HTTP server."},
		declare.Flag{Name: internal.FlagWithPackageName, DefaultValue: false, Description: "Raw POST http path registered with package name."},
		declare.Flag{Name: internal.FlagWithGrpcGatewayCompatible, DefaultValue: false, Description: "Http handler with grpc-gateway compatible."},
		declare.Flag{Name: internal.FlagAllowedOrigins, DefaultValue: "*", Description: "CORS Allowed origins for http request."},
		declare.Flag{Name: internal.FlagAllowedRequestHeaders, DefaultValue: "*", Description: "CORS Allowed request headers for http request."},
		declare.Flag{Name: internal.FlagAllowedMethods, DefaultValue: "GET,HEAD,POST,PUT,DELETE,OPTIONS,PATCH", Description: "CORS Allowed methods for http request."},
		declare.Flag{Name: internal.FlagHttpReadTimeout, DefaultValue: 0 * time.Second, Description: "Read timeout of http request."},
		declare.Flag{Name: internal.FlagHttpWriteTimeout, DefaultValue: 0 * time.Second, Description: "Write timeout http request."},
		declare.Flag{Name: internal.FlagHttpIdleTimeout, DefaultValue: 0 * time.Second, Description: "The maximum amount of time to wait for the next request when keep-alive are enabled."},
		declare.Flag{Name: internal.FlagHttpKeepAlive, DefaultValue: true, Description: "Whether HTTP keep-alive are enabled."},
		declare.Flag{Name: internal.FlagHttpGzip, DefaultValue: false, Description: "Whether return GZip compressed http response if http request prefer gzip encoding."},
		declare.Flag{Name: internal.FlagClientAuthorities, DefaultValue: "", Description: "Comma-separated client authorities for client authentication."},
		declare.Flag{Name: internal.FlagMaxConcurrentStreams, DefaultValue: uint32(100), Description: "The number of concurrent streams to each ServerTransport."},
		declare.Flag{Name: internal.FlagIgnoreInternalInterceptors, DefaultValue: false, Description: "Disable all internal interceptors of server."},
		declare.Flag{Name: internal.FlagHealthServiceEnable, DefaultValue: true, Description: "Enable grpc health checking service."},
		declare.Flag{Name: internal.FlagMaxHttpConcurrentConnection, DefaultValue: -1, Description: "Max concurrent connections for http server. -1 means unlimit."},
		declare.Flag{Name: internal.FlagPortRange, DefaultValue: "10000-", Description: "Port range for server binding while port is auto selected."},
		declare.Flag{Name: internal.FlagCheckClientCertExpiration, DefaultValue: false, Description: "Client certificate expiration verification."},
		declare.Flag{Name: internal.FlagLegacyHttpRegistry, DefaultValue: false, Description: "Http registration with legacy registry information."},
		declare.Flag{Name: internal.FlagCorsLogger, DefaultValue: true, Description: "Enable cors log with default logger."},
		declare.Flag{Name: internal.FlagServerReflection, DefaultValue: false, Description: "Enable grpc server reflection."},
		declare.Flag{Name: internal.FlagServerReflectionToken, DefaultValue: "", Description: "Grpc server reflection authentication token."},
	)
}

func NewServer() *serverImpl {
	return &serverImpl{
		srv: &server.Server{},
	}
}

func (s *serverImpl) RegisterService(srvRegister interface{}, srvHandler ServiceProvider, opt ...RegisterOption) error {
	var o []option.RegisterOption
	for _, v := range opt {
		o = append(o, option.RegisterOption(v))
	}

	return s.srv.RegisterService(srvRegister, srvHandler, o...)
}

func (s *serverImpl) RegisterHttpService(route Route, handler http.Handler, opt ...HttpRegisterOption) error {
	var o []option.HttpRegisterOption
	for _, v := range opt {
		o = append(o, option.HttpRegisterOption(v))
	}

	return s.srv.RegisterHttpService(route, handler, o...)
}

func (s *serverImpl) ServiceInfos() ([]*memory_registry.ServiceInfo, error) {
	return s.srv.ServiceInfos()
}

func (s *serverImpl) Run(opt ...Option) error {
	if !lifecycle.LifeCycle().IsInitialized() {
		log.Fatal("[ERROR] Framework not initialized!")
	}

	var o []option.Option
	for _, v := range opt {
		o = append(o, option.Option(v))
	}

	return s.srv.Run(option.ParseOptions(o...))
}
