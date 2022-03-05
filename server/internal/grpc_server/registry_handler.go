package grpc_server

import "github.com/zhangel/go-framework/registry"

type RegistryHandlerImpl struct {
	initialRegistry bool

	registerClosure   func() error
	unregisterClosure func() error
}

func DefaultRegistryHandler() *RegistryHandlerImpl {
	return &RegistryHandlerImpl{
		initialRegistry: true,
	}
}

func WithInitialRegistry(enable bool) func(handler *RegistryHandlerImpl) {
	return func(handler *RegistryHandlerImpl) {
		handler.initialRegistry = enable
	}
}

func (s *RegistryHandlerImpl) PrepareRegister(register func() error, unregister func() error) {
	s.registerClosure = register
	s.unregisterClosure = unregister
}

func (s RegistryHandlerImpl) Register() error {
	if s.registerClosure == nil {
		return nil
	} else {
		return s.registerClosure()
	}
}

func (s RegistryHandlerImpl) Unregister() error {
	if s.unregisterClosure == nil {
		return nil
	} else {
		return s.unregisterClosure()
	}
}

func (s RegistryHandlerImpl) IsInitialRegister() bool {
	return s.initialRegistry
}

type GrpcServiceRegistryHandler struct {
	sd ServiceDescList
}

func NewGrpcServiceRegistryHandler(sd ServiceDescList) *GrpcServiceRegistryHandler {
	return &GrpcServiceRegistryHandler{sd: sd}
}

func (s *GrpcServiceRegistryHandler) Prepare(r registry.Registry, addr string, withTls bool) {
	prepare := func(sd *ServiceDesc) {
		if handler, ok := sd.RegisterOption.RegistryHandler.(interface {
			PrepareRegister(register func() error, unregister func() error)
		}); ok {
			handler.PrepareRegister(func() error {
				return sd.registerService(r, addr, withTls)
			}, func() error {
				return sd.unregisterService(r, addr)
			})
		}
	}

	for _, sd := range s.sd {
		if sd.RegisterOption.BindAddrCallback != nil {
			sd.RegisterOption.BindAddrCallback(0, addr)
		}
		prepare(sd)
	}
}

func (s *GrpcServiceRegistryHandler) RegisterServices() error {
	for _, sd := range s.sd {
		if sd.RegisterOption.RegistryHandler.IsInitialRegister() {
			if err := sd.RegisterOption.RegistryHandler.Register(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *GrpcServiceRegistryHandler) UnregisterServices() {
	for _, sd := range s.sd {
		_ = sd.RegisterOption.RegistryHandler.Unregister()
	}
}
