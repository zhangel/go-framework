package server

import (
	"context"
	"net/http"

	"github.com/zhangel/go-framework.git/server/internal"
	"github.com/zhangel/go-framework.git/server/internal/grpc_server"
	"github.com/zhangel/go-framework.git/server/internal/http_server"
	"github.com/zhangel/go-framework.git/server/internal/server_list"
	"github.com/zhangel/go-framework.git/server/internal/service"
	"google.golang.org/grpc/metadata"
)

var (
	DefaultServer = NewServer()
)

const StandardIncomingHeaderPrefix = http_server.StandardIncomingHeaderPrefix
const StandardOutgoingHeaderPrefix = http_server.StandardOutgoingHeaderPrefix

type RegistryHandler internal.RegistryHandler

func RegisterService(srvRegister interface{}, srvHandler ServiceProvider, opt ...RegisterOption) error {
	return DefaultServer.RegisterService(srvRegister, srvHandler, opt...)
}

func RegisterHttpService(route Route, handler http.Handler, opt ...HttpRegisterOption) error {
	return DefaultServer.RegisterHttpService(route, handler, opt...)
}

func Run(option ...Option) error {
	return DefaultServer.Run(option...)
}

func CreateDefaultProvider(serviceName string, configNs []string) ServiceProvider {
	return service.CreateDefaultProvider(serviceName, configNs)
}

func InitServiceProvider(srvHandler ServiceProvider, initializer func() (ServiceProvider, error)) error {
	inner := func() (service.Provider, error) {
		return initializer()
	}
	return service.InitProvider(srvHandler, inner)
}

func GrpcServerList() []string {
	return server_list.GrpcServerList()
}

func HttpServerList() []string {
	return server_list.HttpServerList()
}

func WithInitialRegistry(enable bool) func(handler *grpc_server.RegistryHandlerImpl) {
	return grpc_server.WithInitialRegistry(enable)
}

func NewRegistryHandler(opts ...func(*grpc_server.RegistryHandlerImpl)) RegistryHandler {
	handler := grpc_server.DefaultRegistryHandler()
	for _, o := range opts {
		o(handler)
	}
	return handler
}

func IsHttpInvoke(ctx context.Context) bool {
	isHttpInvoke, ok := ctx.Value(http_server.HttpInvokeCtx).(bool)
	return ok && isHttpInvoke
}

func HttpHeaderFromMetadata(md metadata.MD, key string) ([]string, bool) {
	if len(md) == 0 {
		return nil, false
	}

	v := md.Get(StandardIncomingHeaderPrefix + key)
	return v, len(v) != 0
}

func HttpHeaderToMetadata(md metadata.MD, key string, val []string, append bool) metadata.MD {
	if md == nil {
		md = metadata.New(nil)
	}

	if append {
		md.Append(StandardOutgoingHeaderPrefix+key, val...)
	} else {
		md.Set(StandardOutgoingHeaderPrefix+key, val...)
	}
	return md
}
