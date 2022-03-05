package http_server

import (
	"context"
	"crypto/tls"
	go_log "log"
	"net"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/mux"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/zhangel/go-framework/authentication"
	"github.com/zhangel/go-framework/credentials"
	"github.com/zhangel/go-framework/lifecycle"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/recovery"
	"github.com/zhangel/go-framework/server/internal"
	"github.com/zhangel/go-framework/server/internal/grpc_server"
	"github.com/zhangel/go-framework/server/internal/option"
	"github.com/zhangel/go-framework/server/internal/server_list"
	"github.com/zhangel/go-framework/tracing"
	"golang.org/x/net/netutil"
	"google.golang.org/grpc"
)

const horizontalLineWidth = 110

type Server struct {
	srv             *grpc.Server
	httpSrv         *http.Server
	mux             *ServerMux
	opts            *option.Options
	grpcServiceDesc grpc_server.ServiceDescList
	httpServiceDesc ServiceDescList
	registryHandler *HttpServiceRegistryHandler
	tlsServer       bool
}

func NewHttpServer(srv *grpc.Server, opts *option.Options, grpcServiceDesc grpc_server.ServiceDescList, httpServiceDesc ServiceDescList) *Server {
	corsHandler := newCorsHandler(opts.AllowedOrigins, opts.AllowedRequestHeaders, opts.AllowedMethods, opts.EnableCorsLog)

	var grpcWebServer *grpcweb.WrappedGrpcServer
	if opts.EnableGrpcWeb {
		grpcWebServer = grpcweb.WrapServer(
			srv,
			grpcweb.WithOriginFunc(corsHandler.isOriginAllowed),
			grpcweb.WithAllowedRequestHeaders(opts.AllowedRequestHeaders),
			grpcweb.WithWebsockets(opts.EnableGrpcWebWithWs),
			grpcweb.WithWebsocketOriginFunc(func(r *http.Request) bool { return opts.EnableGrpcWebWithWs }),
		)
	}

	var restApiServer *mux.Router
	var serverMux *ServerMux
	if opts.EnableHttp {
		restApiServer = mux.NewRouter()
		if len(httpServiceDesc) != 0 {
			log.Info(strings.Repeat("-", horizontalLineWidth))
			log.Info("Register HTTP handlers for http service:")
			for _, sd := range httpServiceDesc {
				if sd.Handler == nil {
					continue
				}
				log.Infof("\t %s | Handler: %s", sd.Route, runtime.FuncForPC(reflect.ValueOf(sd.Handler).Pointer()).Name())
				if err := apply(restApiServer, sd.Route, sd.Handler); err != nil {
					go_log.Fatal(err)
				}
			}
		}

		serverMux = NewServerMux(srv, grpcServiceDesc, opts)
		restApiServer.PathPrefix("/").Handler(serverMux)
	}

	handlerFunc := func(grpcServer *grpc.Server) http.Handler {
		var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if grpcWebServer != nil && (grpcWebServer.IsGrpcWebRequest(r) || grpcWebServer.IsGrpcWebSocketRequest(r)) {
				grpcWebServer.ServeHTTP(w, r)
			} else if isGrpcRequest(r) {
				grpcServer.ServeHTTP(w, r)
			} else if restApiServer != nil {
				corsHandler.ServeHTTP(w, r, restApiServer.ServeHTTP)
			} else {
				corsHandler.ServeHTTP(w, r, func(writer http.ResponseWriter, request *http.Request) {
					writer.WriteHeader(http.StatusNotFound)
				})
				return
			}
		})

		if httpAuthProvider, ok := authentication.DefaultProvider().(authentication.HttpProvider); ok {
			h = httpAuthProvider.HttpAuthInterceptor(h)
		}

		h = tracing.HttpServerReqIdHandler(h)

		if opts.EnableHttpGzip {
			h = gziphandler.GzipHandler(h)
		}

		if opts.Recovery {
			h = recovery.HttpHandler(h)
		}
		return h
	}

	if tlsConfig, err := credentials.HttpServerTlsConfig(opts.CredentialOpts...); err != nil {
		go_log.Fatal("[ERROR]", err)
		return nil
	} else {
		s := &Server{
			srv: srv,
			httpSrv: &http.Server{
				Handler:      handlerFunc(srv),
				IdleTimeout:  opts.HttpIdleTimeout,
				ReadTimeout:  opts.HttpReadTimeout,
				WriteTimeout: opts.HttpWriteTimeout,
				TLSConfig:    tlsConfig,
			},
			mux:             serverMux,
			opts:            opts,
			grpcServiceDesc: grpcServiceDesc,
			httpServiceDesc: httpServiceDesc,
			tlsServer:       tlsConfig != nil,
			registryHandler: NewHttpServiceRegistryHandler(serverMux, grpcServiceDesc, httpServiceDesc),
		}

		s.httpSrv.SetKeepAlivesEnabled(opts.HttpKeepAlive)
		return s
	}
}

func (s *Server) Serve(l net.Listener, prepareCB func(l net.Listener) func()) error {
	if s.opts.MaxHttpConcurrentConnection > 0 {
		l = netutil.LimitListener(l, s.opts.MaxHttpConcurrentConnection)
	}

	if s.opts.WithTls {
		if tlsConfig, err := credentials.HttpServerTlsConfig(s.opts.CredentialOpts...); err != nil {
			log.Fatal("[ERROR]", err)
			return nil
		} else {
			l = tls.NewListener(l, tlsConfig)
		}
	}

	if prepareCB != nil {
		defer prepareCB(l)()
	}

	server_list.AddHttpServer(l.Addr().String())
	defer server_list.RemoveHttpServer(l.Addr().String())

	return s.httpSrv.Serve(l)
}

func (s *Server) Prepare(l net.Listener) func() {
	lifecycle.LifeCycle().HookFinalize(func(ctx context.Context) {
		_ = s.httpSrv.Shutdown(ctx)
	}, lifecycle.WithName("Http server shutdown"), lifecycle.WithPriority(lifecycle.PriorityHigher))

	defer s.displayInfo(l.Addr().String())

	s.registryHandler.Prepare(l.Addr().String(), s.opts)
	finalizer := s.httpServiceHook()

	if err := s.registryHandler.RegisterServices(); err != nil {
		log.Fatal(err)
	}

	lifecycle.LifeCycle().HookFinalize(func(context.Context) {
		s.registryHandler.UnregisterServices()
	}, lifecycle.WithName("Http service unregister"), lifecycle.WithPriority(lifecycle.PriorityHigher))

	return finalizer
}

func (s *Server) httpServiceHook() func() {
	startHookFunc := func(sd *ServiceDesc, wg *sync.WaitGroup) {
		defer func() {
			wg.Done()
		}()

		if sd.RegisterOption.ServiceStartHook == nil {
			return
		}

		if err := internal.InvokeWithTimeout(func() error {
			log.Infof("Run http-service start-hook for %q ...", sd.Route)
			return sd.RegisterOption.ServiceStartHook(sd.Handler)
		}, sd.RegisterOption.ServiceStartHookOpts.Timeout); err != nil {
			log.Fatalf("Run http-service start-hook for %q failed, err = %v", sd.Route, err)
		} else {
			log.Infof("Run http-service start-hook for %q done", sd.Route)
		}
	}

	stopHookFunc := func(sd *ServiceDesc, wg *sync.WaitGroup) {
		defer wg.Done()

		if sd.RegisterOption.ServiceStopHook == nil {
			return
		}

		if err := internal.InvokeWithTimeout(func() error {
			log.Infof("Run http-service stop-hook for %q ...", sd.Route)
			sd.RegisterOption.ServiceStopHook(sd.Handler)
			return nil
		}, sd.RegisterOption.ServiceStopHookOpts.Timeout); err != nil {
			log.Fatalf("Run http-service stop-hook for %q failed, err = %v", sd.Route, err)
		} else {
			log.Infof("Run http-service stop-hook for %q done", sd.Route)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(s.httpServiceDesc))
	for _, sd := range s.httpServiceDesc {
		go startHookFunc(sd, &wg)
	}
	wg.Wait()

	finalizerCh := make(chan struct{}, 1)
	lifecycle.LifeCycle().HookFinalize(func(ctx context.Context) {
		<-finalizerCh
	}, lifecycle.WithName("Http-service stop hooks"))

	return func() {
		var wg sync.WaitGroup
		wg.Add(len(s.httpServiceDesc))
		for _, sd := range s.httpServiceDesc {
			go stopHookFunc(sd, &wg)
		}
		wg.Wait()
		finalizerCh <- struct{}{}
	}
}

func (s *Server) displayInfo(addr string) {
	var httpServerType []string
	if s.opts.EnableHttp {
		httpServerType = append(httpServerType, "http")
	}
	if s.opts.EnableGrpcWeb {
		httpServerType = append(httpServerType, "grpc-web")

		if s.opts.EnableGrpcWebWithWs {
			httpServerType = append(httpServerType, "grpc-web-ws")
		}
	}

	if s.opts.WithTls {
		log.Infof("Start %s server with tls: %s", strings.Join(httpServerType, "/"), addr)
	} else {
		log.Infof("Start %s server: %s", strings.Join(httpServerType, "/"), addr)
	}
}

func isGrpcRequest(r *http.Request) bool {
	return r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc")
}
