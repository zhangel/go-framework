package registry

import (
	"context"
	"log"
	"sync"

	"github.com/zhangel/go-framework.git/lifecycle"

	"google.golang.org/grpc/naming"

	"github.com/zhangel/go-framework.git/declare"
	"github.com/zhangel/go-framework.git/plugin"
)

var (
	Plugin = declare.PluginType{Name: "registry"}

	defaultRegistry Registry
	noop            = &_NoopRegistry{}
	mutex           sync.RWMutex
)

type Entry struct {
	ServiceName string
	Instances   []Instance
}

type Instance struct {
	Addr string
	Meta interface{}
}

type Registry interface {
	RegisterService(serviceName string, addr string, meta interface{}) error
	UnregisterService(serviceName string, addr string) error
	Resolver() naming.Resolver
	HasService(serviceName string) (bool, error)
	ListServiceAddresses(serviceName string) ([]string, error)
	ListServiceByPrefix(prefix string) (map[string][]string, error)
	ListAllService() ([]Entry, error)
	WatchAllServiceChanges(watcher Watcher) (cancel func(), err error)
}

type NamespaceConverter interface {
	WithNamespace(serviceName string) string
	WithoutNamespace(servicePath string) string
}

type Info interface {
	Info(target string) string
}

type Closer interface {
	Close() error
}

type TargetTransformer func(target string) string

type ResolverOption interface {
	WithOption(opt ...ResolverOptionFunc) error
}

type ResolverOptions struct {
	filter []func(serviceName string, instance Instance) bool
}
type ResolverOptionFunc func(*ResolverOptions) error

func init() {
	lifecycle.LifeCycle().HookFinalize(func(context.Context) {
		mutex.RLock()
		defer mutex.RUnlock()

		if registry, ok := defaultRegistry.(Closer); ok && registry != nil {
			_ = registry.Close()
		}
	}, lifecycle.WithName("Close default registry"), lifecycle.WithPriority(lifecycle.PriorityLowest))
}

func ResolverWithFilter(filter func(serviceName string, instance Instance) bool) func(*ResolverOptions) error {
	return func(opts *ResolverOptions) error {
		opts.filter = append(opts.filter, filter)
		return nil
	}
}

func DefaultRegistry() Registry {
	mutex.RLock()
	if defaultRegistry != nil {
		mutex.RUnlock()
		return defaultRegistry
	}
	mutex.RUnlock()

	mutex.Lock()
	defer mutex.Unlock()

	if defaultRegistry != nil {
		return defaultRegistry
	}

	err := plugin.CreatePlugin(Plugin, &defaultRegistry)
	if err != nil {
		log.Fatalf("[ERROR] Create registry plugin failed, err = %s.\n", err)
	}

	if defaultRegistry == nil {
		defaultRegistry = noop
	}

	return defaultRegistry
}

func ServicePath(registry Registry, target string) string {
	if nr, ok := registry.(NamespaceConverter); !ok {
		return target
	} else {
		return nr.WithNamespace(target)
	}
}

func Target(registry Registry, servicePath string) string {
	if nr, ok := registry.(NamespaceConverter); !ok {
		return servicePath
	} else {
		return nr.WithoutNamespace(servicePath)
	}
}

func IsNoopRegistry(registry Registry) bool {
	_, ok := registry.(interface {
		Noop()
	})
	return ok
}
