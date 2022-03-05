package grpc_server

import (
	"context"
	"net"
	"sync"

	"google.golang.org/grpc/reflection"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/zhangel/go-framework/circuit_breaker"
	"github.com/zhangel/go-framework/hooks"
	"github.com/zhangel/go-framework/lifecycle"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/prometheus"
	"github.com/zhangel/go-framework/rate_limit"
	"github.com/zhangel/go-framework/server/internal"
	"github.com/zhangel/go-framework/server/internal/option"
	"github.com/zhangel/go-framework/server/internal/server_list"
	"github.com/zhangel/go-framework/timeout"
	"github.com/zhangel/go-framework/utils"
)

type Server struct {
	srv             *grpc.Server
	opts            *option.Options
	serviceDesc     ServiceDescList
	registryHandler *GrpcServiceRegistryHandler
}

func NewGrpcServer(srv *grpc.Server, opts *option.Options, serviceDesc ServiceDescList) *Server {
	if err := serviceDesc.GrpcRegister(srv); err != nil {
		log.Fatal(err)
	}

	return &Server{
		srv:             srv,
		opts:            opts,
		serviceDesc:     serviceDesc,
		registryHandler: NewGrpcServiceRegistryHandler(serviceDesc),
	}
}

func (s *Server) Serve(l net.Listener, prepareCB func(l net.Listener) func()) error {
	if prepareCB != nil {
		defer prepareCB(l)()
	}

	server_list.AddGrpcServer(l.Addr().String())
	defer server_list.RemoveGrpcServer(l.Addr().String())
	return s.srv.Serve(l)
}

func (s *Server) Prepare(l net.Listener) func() {
	lifecycle.LifeCycle().HookFinalize(func(context.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Infof("Graceful stop failed, err = %s", r)
			}
		}()
		s.srv.GracefulStop()
	}, lifecycle.WithName("Grpc server graceful stop"), lifecycle.WithPriority(lifecycle.PriorityHigher))

	if s.opts.HealthService != nil {
		grpc_health_v1.RegisterHealthServer(s.srv, s.opts.HealthService)
		log.Infof("Grpc health server enabled")
	}

	if s.opts.GrpcServerReflection {
		reflection.Register(s.srv)
	}

	prometheus.Register(s.srv)

	unaryHookInterceptors := s.opts.UnaryHookInterceptors
	unaryHookInterceptors = append(unaryHookInterceptors,
		rate_limit.UnaryServerHookInterceptor(),
		circuit_breaker.UnaryServerHookInterceptor(),
		timeout.UnaryServerHookInterceptor(),
	)

	streamHookInterceptors := s.opts.StreamHookInterceptors
	streamHookInterceptors = append(streamHookInterceptors,
		rate_limit.StreamServerHookInterceptor(),
		circuit_breaker.StreamServerHookInterceptor(),
		timeout.StreamServerHookInterceptor(),
	)

	if len(streamHookInterceptors) > 0 || len(unaryHookInterceptors) > 0 {
		if err := hooks.HookServer(s.srv, hooks.HookWithUnaryInterceptors(unaryHookInterceptors...), hooks.HookWithStreamInterceptors(streamHookInterceptors...)); err != nil {
			log.Fatal(err)
		}
	}

	s.registryHandler.Prepare(s.opts.Registry, l.Addr().String(), s.opts.WithTls)

	finalizer := s.grpcServiceHook(s.opts)

	//if err := s.registryHandler.RegisterServices(); err != nil {
	//    log.Fatal(err)
	//}

	//lifecycle.LifeCycle().HookFinalize(func(context.Context) {
	//    s.registryHandler.UnregisterServices()
	//}, lifecycle.WithName("Grpc service unregister"), lifecycle.WithPriority(lifecycle.PriorityHigher))
	//

	if s.opts.WithTls {
		log.Infof("Start grpc server with tls: %s", l.Addr())
	} else {
		log.Infof("Start grpc server: %s", l.Addr())
	}

	return finalizer
}

func (s *Server) grpcServiceHook(opts *option.Options) func() {
	startHookFunc := func(sd *ServiceDesc, wg *sync.WaitGroup) {
		defer func() {
			if err := sd.MemoryRegister(opts); err != nil {
				log.Fatal(err)
			}

			if sd.RegisterOption.RegistryHandler != nil {
				lifecycle.LifeCycle().HookFinalize(func(context.Context) {
					sd.RegisterOption.RegistryHandler.Unregister()
				}, lifecycle.WithName("Grpc service unregister"), lifecycle.WithPriority(lifecycle.PriorityHigher))

				if sd.RegisterOption.RegistryHandler.IsInitialRegister() {
					if err := sd.RegisterOption.RegistryHandler.Register(); err != nil {
						log.Fatal(err)
					}
				}
			}

			wg.Done()
		}()

		if sd.RegisterOption.ServiceStartHook == nil {
			return
		}

		if err := internal.InvokeWithTimeout(func() error {
			log.Infof("Run service start-hook for %q ...", sd.ServiceInfo.ServiceName)
			return sd.RegisterOption.ServiceStartHook(sd.Handler)
		}, sd.RegisterOption.ServiceStartHookOpts.Timeout); err != nil {
			log.Info(utils.FullCallStack())
			log.Fatalf("Run service start-hook for %q failed, err = %v", sd.ServiceInfo.ServiceName, err)
		} else {
			log.Infof("Run service start-hook for %q done", sd.ServiceInfo.ServiceName)
		}
	}

	stopHookFunc := func(sd *ServiceDesc, wg *sync.WaitGroup) {
		defer wg.Done()

		if sd.RegisterOption.ServiceStopHook == nil {
			return
		}

		if err := internal.InvokeWithTimeout(func() error {
			log.Infof("Run service stop-hook for %q ...", sd.ServiceInfo.ServiceName)
			sd.RegisterOption.ServiceStopHook(sd.Handler)
			return nil
		}, sd.RegisterOption.ServiceStopHookOpts.Timeout); err != nil {
			log.Fatalf("Run service stop-hook for %q failed, err = %v", sd.ServiceInfo.ServiceName, err)
		} else {
			log.Infof("Run service stop-hook for %q done", sd.ServiceInfo.ServiceName)
		}
	}

	if err := s.serviceDesc.MemoryUnregister(); err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(len(s.serviceDesc))
	for _, sd := range s.serviceDesc {
		go startHookFunc(sd, &wg)
	}
	wg.Wait()

	finalizerCh := make(chan struct{}, 1)
	lifecycle.LifeCycle().HookFinalize(func(ctx context.Context) {
		<-finalizerCh
	}, lifecycle.WithName("Service stop hooks"))

	return func() {
		var wg sync.WaitGroup
		wg.Add(len(s.serviceDesc))
		for _, sd := range s.serviceDesc {
			go stopHookFunc(sd, &wg)
		}
		wg.Wait()
		finalizerCh <- struct{}{}
	}
}
