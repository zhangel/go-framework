package http_server

import (
	go_log "log"

	"github.com/zhangel/go-framework.git/http"
	"github.com/zhangel/go-framework.git/registry"
	"github.com/zhangel/go-framework.git/server/internal/grpc_server"
	"github.com/zhangel/go-framework.git/server/internal/option"
)

type HttpServiceRegistryHandler struct {
	mux               *ServerMux
	grpcServiceDesc   grpc_server.ServiceDescList
	httpServiceDesc   ServiceDescList
	grpcWebEndpoint   []string
	registerClosure   func() error
	unregisterClosure func()
}

func NewHttpServiceRegistryHandler(mux *ServerMux, grpcServiceDesc grpc_server.ServiceDescList, httpServiceDesc ServiceDescList) *HttpServiceRegistryHandler {
	return &HttpServiceRegistryHandler{
		mux:             mux,
		grpcServiceDesc: grpcServiceDesc,
		httpServiceDesc: httpServiceDesc,
	}
}

func (s *HttpServiceRegistryHandler) Prepare(addr string, opts *option.Options) {
	var rules []*registry.HttpRule
	if opts.EnableHttp {
		rules = append(rules, s.mux.HttpRules()...)
		rules = append(rules, s.httpServiceDesc.HttpRules()...)
	}

	if opts.EnableGrpcWeb {
		rules = append(rules, s.grpcServiceDesc.HttpRules()...)
	}

	for _, sd := range s.httpServiceDesc {
		if sd.RegisterOption.BindAddrCallback != nil {
			sd.RegisterOption.BindAddrCallback(addr)
		}
	}

	for _, sd := range s.grpcServiceDesc {
		if sd.RegisterOption.BindAddrCallback != nil {
			sd.RegisterOption.BindAddrCallback(1, addr)
		}
	}

	if len(rules) == 0 || opts.Registry == nil || registry.IsNoopRegistry(opts.Registry) || option.IsPipeAddress(addr) {
		return
	}

	registryMeta := registry.Meta{
		Type:      registry.RegistryType_Http,
		TlsServer: opts.WithTls,
		HttpRules: rules,
	}

	if opts.LegacyHttpRegistry {
		for _, rule := range rules {
			registryMeta.HttpPatterns = append(registryMeta.HttpPatterns, rule.HttpPatterns...)
		}
	}

	metaJson, err := registry.MarshalRegistryMeta(registryMeta)
	if err != nil {
		go_log.Fatalf("[ERROR] Register http service failed, err = %v\n", err)
	}

	var grpcServiceEndpoints []string
	if opts.EnableGrpcWeb {
		for _, sd := range s.grpcServiceDesc {
			if len(sd.RegisterOption.Alias) == 0 {
				grpcServiceEndpoints = append(grpcServiceEndpoints, sd.ServiceInfo.ServiceName+".http.endpoint")
			} else {
				for _, alias := range sd.RegisterOption.Alias {
					grpcServiceEndpoints = append(grpcServiceEndpoints, alias+".http.endpoint")
				}
			}
		}
	}

	s.registerClosure = func() error {
		for _, e := range grpcServiceEndpoints {
			if httpMetaJson, err := registry.MarshalRegistryMeta(registry.Meta{TlsServer: opts.WithTls, Type: registry.RegistryType_Http_Endpoint}); err != nil {
				return err
			} else if err := opts.Registry.RegisterService(e, addr, httpMetaJson); err != nil {
				return err
			}
		}
		return opts.Registry.RegisterService(http.RegistryPrefix, addr, metaJson)
	}

	s.unregisterClosure = func() {
		for _, e := range grpcServiceEndpoints {
			_ = opts.Registry.UnregisterService(e, addr)
		}
		_ = opts.Registry.UnregisterService(http.RegistryPrefix, addr)
	}
}

func (s *HttpServiceRegistryHandler) RegisterServices() error {
	if s.registerClosure != nil {
		return s.registerClosure()
	} else {
		return nil
	}
}

func (s *HttpServiceRegistryHandler) UnregisterServices() {
	if s.unregisterClosure != nil {
		s.unregisterClosure()
	}
}
