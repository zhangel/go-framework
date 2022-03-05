package grpc_server

import (
	"fmt"
	"strings"

	"google.golang.org/grpc"

	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/http"
	"github.com/zhangel/go-framework/lifecycle"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/memory_registry"
	"github.com/zhangel/go-framework/registry"
	"github.com/zhangel/go-framework/server/internal/option"
	"github.com/zhangel/go-framework/server/internal/service"
)

type ServiceDesc struct {
	ServiceInfo    *memory_registry.ServiceInfo
	Register       interface{}
	Handler        service.Provider
	RegisterOption option.RegisterOptions
}

type ServiceDescList []*ServiceDesc

func (s *ServiceDescList) RegisterServiceHandler(srvRegister interface{}, srvHandler service.Provider, opt ...option.RegisterOption) error {
	opts := &option.RegisterOptions{}
	for _, o := range opt {
		if err := o(opts); err != nil {
			return err
		}
	}

	serviceInfo, err := memory_registry.GetServiceInfo(srvRegister, srvHandler)
	if err != nil {
		log.Fatal(err)
	}

	if opts.RegistryHandler == nil {
		opts.RegistryHandler = DefaultRegistryHandler()
	}

	sd := &ServiceDesc{
		ServiceInfo:    serviceInfo,
		Register:       srvRegister,
		Handler:        srvHandler,
		RegisterOption: *opts,
	}
	*s = append(*s, sd)

	initProvider := func(serviceInfo *memory_registry.ServiceInfo) {
		if opts.ServiceProviderInitializer == nil {
			opts.ServiceProviderInitializer = func() (service.Provider, error) {
				ns := config.StringList("config.ns")
				if len(ns) == 0 || len(ns) == 1 && len(strings.TrimSpace(ns[0])) == 0 {
					return service.CreateDefaultProvider(serviceInfo.ServiceName, []string{serviceInfo.ServiceName}), nil
				} else {
					for i := 0; i < len(ns); i++ {
						ns[i] = strings.TrimSuffix(ns[i], ".") + "." + serviceInfo.ServiceName
					}

					return service.CreateDefaultProvider(serviceInfo.ServiceName, ns), nil
				}
			}
		}

		if err := service.InitProvider(srvHandler, opts.ServiceProviderInitializer); err != nil {
			log.Fatal(err)
		}
	}

	registerService := func() {
		initProvider(serviceInfo)
		if err := sd.MemoryRegister(option.ParseOptions()); err != nil {
			log.Fatal(err)
		}
	}

	if !lifecycle.LifeCycle().IsInitialized() {
		lifecycle.LifeCycle().HookInitialize(registerService, lifecycle.WithName(fmt.Sprintf("Register service %+v", srvHandler)))
	} else {
		registerService()
	}

	return nil
}

func (s ServiceDescList) GrpcRegister(srv *grpc.Server) error {
	for _, sd := range s {
		if err := memory_registry.RegisterService(srv, sd.Register, sd.Handler); err != nil {
			return err
		}
	}
	return nil
}

func (s ServiceDescList) MemoryRegister(opts *option.Options) error {
	for _, sd := range s {
		if err := sd.MemoryRegister(opts); err != nil {
			return err
		}
	}
	return nil
}

func (s ServiceDescList) MemoryUnregister() error {
	for _, sd := range s {
		if err := memory_registry.GlobalRegistry.UnregisterWithServiceInfo(sd.ServiceInfo, sd.RegisterOption.Alias); err != nil {
			return err
		}
	}
	return nil
}

func (s ServiceDescList) HttpRules() []*registry.HttpRule {
	rule := &registry.HttpRule{}
	for _, sd := range s {
		if len(sd.RegisterOption.Alias) == 0 {
			rule.HttpPatterns = append(rule.HttpPatterns, http.RegistryWithPath([]string{"*"}, fmt.Sprintf("/%s/*", sd.ServiceInfo.ServiceName))...)
		} else {
			for _, alias := range sd.RegisterOption.Alias {
				rule.HttpPatterns = append(rule.HttpPatterns, http.RegistryWithPath([]string{"*"}, fmt.Sprintf("/%s/*", alias))...)
			}
		}
	}
	return []*registry.HttpRule{rule}
}

func (s *ServiceDesc) MemoryRegister(opts *option.Options) error {
	return memory_registry.GlobalRegistry.RegisterWithServiceInfo(
		s.ServiceInfo,
		memory_registry.WithAliasNames(s.RegisterOption.Alias...),
		memory_registry.WithUnaryInterceptor(PrepareUnaryInterceptors(opts, true)),
		memory_registry.WithStreamInterceptor(PrepareStreamInterceptors(opts, true)),
		memory_registry.WithAliasOnly(s.RegisterOption.AliasOnly),
	)
}

func (s *ServiceDesc) registerService(r registry.Registry, addr string, withTls bool) error {
	if r == nil || registry.IsNoopRegistry(r) || option.IsPipeAddress(addr) {
		return nil
	}

	metaJson, err := registry.MarshalRegistryMeta(registry.Meta{TlsServer: withTls, Type: registry.RegistryType_Grpc})
	if err != nil {
		return err
	}

	serviceList, err := r.ListAllService()
	if err != nil {
		return err
	}

	serviceNames := s.RegisterOption.Alias
	if len(serviceNames) == 0 {
		serviceNames = append(serviceNames, s.ServiceInfo.ServiceName)
	}

	for _, serviceName := range serviceNames {
		var oldEntry *registry.Entry
		for _, entry := range serviceList {
			if entry.ServiceName == serviceName {
				oldEntry = &entry
				break
			}
		}

		if oldEntry != nil {
			for _, instance := range oldEntry.Instances {
				jsonMeta, ok := instance.Meta.(string)
				if !ok {
					continue
				}

				if meta, err := registry.UnmarshalRegistryMeta(jsonMeta); err != nil {
					continue
				} else if meta.TlsServer != withTls {
					return fmt.Errorf("try to start %q with different tls config, conflict server addr = %s, tls = %v", serviceName, instance.Addr, meta.TlsServer)
				}
			}
		}

		log.Infof("registry_handler: register service, serviceName = %s, addr = %s", serviceName, addr)
		if err := r.RegisterService(serviceName, addr, metaJson); err != nil {
			return err
		}
	}

	return nil
}

func (s *ServiceDesc) unregisterService(r registry.Registry, addr string) error {
	if r == nil || registry.IsNoopRegistry(r) || option.IsPipeAddress(addr) {
		return nil
	}

	serviceNames := s.RegisterOption.Alias
	if len(serviceNames) == 0 {
		serviceNames = append(serviceNames, s.ServiceInfo.ServiceName)
	}

	log.Infof("registry_handler: unregister service, serviceName = %+v, addr = %s", serviceNames, addr)
	for _, serviceName := range serviceNames {
		_ = r.UnregisterService(serviceName, addr)
	}

	return nil
}
