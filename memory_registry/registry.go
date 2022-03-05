package memory_registry

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/memory_registry/internal"
)

var GlobalRegistry = NewMemoryRegistry()

type _ServiceWrapper struct {
	serviceName string
	unary       map[string]*internal.UnaryWrapper
	streaming   map[string]*internal.StreamingWrapper
}

type Method interface {
	Method() string
	RequestType() reflect.Type
	ResponseType() reflect.Type
}

type UnaryMethod interface {
	Method
	Invoke(ctx context.Context, req interface{}, callOpts ...grpc.CallOption) (interface{}, error)
}

type Streaming interface {
	Method
	NewStream(ctx context.Context, callOpts ...grpc.CallOption) (grpc.ClientStream, error)
}

type MemoryRegistry struct {
	mutex           sync.RWMutex
	srvRegistryInfo map[string]*_ServiceWrapper
}

func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		mutex:           sync.RWMutex{},
		srvRegistryInfo: make(map[string]*_ServiceWrapper, 20),
	}
}

func (s *MemoryRegistry) RegisterService(register, srvHandler interface{}, opts ...Option) error {
	serviceInfo, err := GetServiceInfo(register, srvHandler)
	if err != nil {
		return err
	}

	return s.RegisterWithServiceInfo(serviceInfo, opts...)
}

func (s *MemoryRegistry) RegisterWithServiceInfo(serviceInfo *ServiceInfo, opt ...Option) error {
	opts := Options{}
	for _, o := range opt {
		if err := o(&opts); err != nil {
			return err
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	wrapper := &_ServiceWrapper{
		serviceInfo.ServiceName,
		map[string]*internal.UnaryWrapper{},
		map[string]*internal.StreamingWrapper{},
	}

	if !opts.aliasOnly {
		if _, ok := s.srvRegistryInfo[serviceInfo.ServiceName]; ok {
			return fmt.Errorf("memory_registry: MemoryRegistry found duplicate service registration for %q", serviceInfo.ServiceName)
		} else {
			s.srvRegistryInfo[serviceInfo.ServiceName] = wrapper
		}
	}

	for _, alias := range opts.aliasNames {
		if strings.TrimSpace(alias) == serviceInfo.ServiceName {
			continue
		}

		if _, ok := s.srvRegistryInfo[alias]; ok {
			return fmt.Errorf("memory_registry: MemoryRegistry found duplicate service registration for %q", alias)
		} else {
			s.srvRegistryInfo[alias] = wrapper
		}
	}

	for name, md := range serviceInfo.Methods {
		method := "/" + serviceInfo.ServiceName + "/" + name
		if unaryMethod, err := internal.CreateUnaryWrapper(method, serviceInfo.HandlerType, md, opts.unaryInt); err != nil {
			return err
		} else {
			wrapper.unary[method] = unaryMethod
		}
	}

	for name, sd := range serviceInfo.Streams {
		method := "/" + serviceInfo.ServiceName + "/" + name
		if streamer, err := internal.CreateStreamingWrapper(method, serviceInfo.HandlerType, sd, opts.streamInt); err != nil {
			return err
		} else {
			wrapper.streaming[method] = streamer
		}
	}

	log.Debugf("memory_registry: Memory register for %q", serviceInfo.ServiceName)
	return nil
}

func (s *MemoryRegistry) UnregisterWithServiceInfo(serviceInfo *ServiceInfo, alias []string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.srvRegistryInfo, serviceInfo.ServiceName)
	for _, aliasName := range alias {
		delete(s.srvRegistryInfo, aliasName)
	}
	return nil
}

func (s *MemoryRegistry) QueryServiceInfo(serviceName string) *_ServiceWrapper {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.srvRegistryInfo[serviceName]
}

func (s *MemoryRegistry) QueryUnary(srvName, method string) (UnaryMethod, error) {
	srv := s.QueryServiceInfo(srvName)
	if srv == nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("memory_registry: query_unary, service %q not found", srvName))
	}

	m, ok := srv.unary[method]
	if !ok {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("memory_registry: query_unary, method %q of service %q not found", method, srvName))
	}

	return m, nil
}

func (s *MemoryRegistry) UnaryInvoke(srvName, method string, ctx context.Context, req interface{}, callOpts ...grpc.CallOption) (interface{}, error) {
	m, err := s.QueryUnary(srvName, method)
	if err != nil {
		return nil, err
	}

	return m.Invoke(ctx, req, callOpts...)
}

func (s *MemoryRegistry) QueryStreaming(srvName, method string) (Streaming, error) {
	srv := s.QueryServiceInfo(srvName)
	if srv == nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("memory_registry: query_streaming, service %q not found", srvName))
	}

	m, ok := srv.streaming[method]
	if !ok {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("memory_registry: query_streaming, method %q of service %q not found", method, srvName))
	}

	return m, nil
}

func (s *MemoryRegistry) NewStream(srvName, method string, ctx context.Context, callOpts ...grpc.CallOption) (grpc.ClientStream, error) {
	m, err := s.QueryStreaming(srvName, method)
	if err != nil {
		return nil, err
	}

	return m.NewStream(ctx, callOpts...)
}
