package authentication

import (
	"log"
	"net/http"

	"github.com/zhangel/go-framework/declare"
	"github.com/zhangel/go-framework/plugin"
	"google.golang.org/grpc"
)

var (
	Plugin = declare.PluginType{Name: "auth"}

	defaultProvider Provider
	noop            noopAuthentication
)

type Provider interface {
	UnaryAuthInterceptor() grpc.UnaryServerInterceptor
	StreamAuthInterceptor() grpc.StreamServerInterceptor
}

type HttpProvider interface {
	HttpAuthInterceptor(handler http.Handler) http.Handler
}

func DefaultProvider() Provider {
	if defaultProvider != nil {
		return defaultProvider
	}

	err := plugin.CreatePlugin(Plugin, &defaultProvider)
	if err != nil {
		log.Fatalf("[ERROR] Create registry plugin failed, err = %s.\n", err)
	}

	if defaultProvider == nil {
		defaultProvider = noop
	}

	return defaultProvider
}
