package registry

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/zhangel/go-framework/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/naming"
	"google.golang.org/grpc/status"
)

type MemoryDiscovery struct {
	entries      map[string]map[string]interface{}
	entriesMutex sync.RWMutex
	notifier     *Notifier
}

func NewMemoryServiceDiscovery() (*MemoryDiscovery, error) {
	return &MemoryDiscovery{
		entries:  make(map[string]map[string]interface{}),
		notifier: NewNotifier(),
	}, nil
}

func (s *MemoryDiscovery) RegisterServiceWithEntries(serviceNames []string, entries []Instance) error {
	processEntries := func() map[string][2][]Instance {
		s.entriesMutex.Lock()
		defer s.entriesMutex.Unlock()

		notifyMap := make(map[string][2][]Instance, len(serviceNames))

		for _, serviceName := range serviceNames {
			instanceMap := make(map[string]interface{}, len(entries))
			prevInstances := s.entries[serviceName]
			var newInsts []Instance
			var delInsts []Instance

			for _, entry := range entries {
				instanceMap[entry.Addr] = entry.Meta
				if prevInstances == nil {
					newInsts = append(newInsts, Instance{Addr: entry.Addr, Meta: entry.Meta})
				} else if _, ok := prevInstances[entry.Addr]; !ok {
					newInsts = append(newInsts, Instance{Addr: entry.Addr, Meta: entry.Meta})
				} else {
					delete(prevInstances, entry.Addr)
				}
			}

			for addr, meta := range prevInstances {
				delInsts = append(delInsts, Instance{Addr: addr, Meta: meta})
			}

			if len(instanceMap) == 0 {
				delete(s.entries, serviceName)
			} else {
				s.entries[serviceName] = instanceMap
			}

			notifyMap[serviceName] = [2][]Instance{newInsts, delInsts}
		}
		return notifyMap
	}

	notifyMap := processEntries()
	for serviceName, notify := range notifyMap {
		if len(notify[0]) > 0 {
			s.notifier.OnUpdate(serviceName, notify[0])
		}
		if len(notify[1]) > 0 {
			s.notifier.OnDelete(serviceName, notify[1])
		}
	}

	return nil
}

func (s *MemoryDiscovery) RegisterService(serviceName string, addr string, meta interface{}) error {
	s.entriesMutex.Lock()
	instanceMap := s.entries[serviceName]
	if instanceMap == nil {
		instanceMap = make(map[string]interface{}, 1)
		s.entries[serviceName] = instanceMap
	}
	instanceMap[addr] = meta
	s.entriesMutex.Unlock()

	s.notifier.OnUpdate(serviceName, []Instance{{Addr: addr, Meta: meta}})

	return nil
}

func (s *MemoryDiscovery) UnregisterService(serviceName string, addr string) error {
	s.entriesMutex.Lock()
	instanceMap := s.entries[serviceName]

	var dels []Instance
	if instanceMap != nil {
		if addr == "" {
			for addr, meta := range instanceMap {
				dels = append(dels, Instance{Addr: addr, Meta: meta})
			}
			delete(s.entries, serviceName)
		} else {
			meta, ok := instanceMap[addr]
			if ok {
				dels = append(dels, Instance{Addr: addr, Meta: meta})
				delete(instanceMap, addr)
			}

			if len(instanceMap) == 0 {
				delete(s.entries, serviceName)
			}
		}
	}

	s.entriesMutex.Unlock()

	s.notifier.OnDelete(serviceName, dels)
	return nil
}

func (s *MemoryDiscovery) Resolver() naming.Resolver {
	return &memoryResolver{registry: s}
}

func (s *MemoryDiscovery) ServiceInstances(serviceName string) ([]Instance, error) {
	s.entriesMutex.RLock()
	defer s.entriesMutex.RUnlock()

	if instanceMap := s.entries[serviceName]; instanceMap == nil {
		return nil, nil
	} else {
		instances := make([]Instance, 0, len(instanceMap))
		for addr, meta := range instanceMap {
			instances = append(instances, Instance{Addr: addr, Meta: meta})
		}
		return instances, nil
	}
}

func (s *MemoryDiscovery) HasService(serviceName string) (bool, error) {
	s.entriesMutex.RLock()
	defer s.entriesMutex.RUnlock()

	return len(s.entries[serviceName]) > 0, nil
}

func (s *MemoryDiscovery) HasInstance(serviceName, addr string) bool {
	s.entriesMutex.RLock()
	defer s.entriesMutex.RUnlock()

	if len(s.entries[serviceName]) == 0 {
		return false
	}

	if _, ok := s.entries[serviceName][addr]; ok {
		return true
	} else {
		return false
	}
}

func (s *MemoryDiscovery) ListServiceAddresses(serviceName string) ([]string, error) {
	s.entriesMutex.RLock()
	defer s.entriesMutex.RUnlock()

	if instanceMap := s.entries[serviceName]; instanceMap == nil {
		return nil, nil
	} else {
		addrs := make([]string, 0, len(instanceMap))
		for addr := range instanceMap {
			addrs = append(addrs, addr)
		}
		return addrs, nil
	}
}

func (s *MemoryDiscovery) ListServiceByPrefix(prefix string) (map[string][]string, error) {
	s.entriesMutex.RLock()
	defer s.entriesMutex.RUnlock()

	result := map[string][]string{}
	for serviceName, instanceMap := range s.entries {
		if len(instanceMap) == 0 {
			continue
		}

		if prefix == "" || strings.HasPrefix(serviceName, prefix) {
			addrs := make([]string, 0, len(instanceMap))
			for addr := range instanceMap {
				addrs = append(addrs, addr)
			}
			result[serviceName] = addrs
		}
	}
	return result, nil
}

func (s *MemoryDiscovery) ListAllService() ([]Entry, error) {
	s.entriesMutex.RLock()
	defer s.entriesMutex.RUnlock()

	result := make([]Entry, 0, len(s.entries))
	for serviceName, instanceMap := range s.entries {
		if len(instanceMap) == 0 {
			continue
		}

		instances := make([]Instance, 0, len(instanceMap))
		for addr, meta := range instanceMap {
			instances = append(instances, Instance{Addr: addr, Meta: meta})
		}
		result = append(result, Entry{ServiceName: serviceName, Instances: instances})
	}

	return result, nil
}

func (s *MemoryDiscovery) Info(target string) string {
	if addrs, _ := s.ListServiceAddresses(target); len(addrs) > 0 {
		return fmt.Sprintf("MemoryRegistry, %s = %v", target, addrs)
	} else {
		allService, _ := s.ListAllService()
		return fmt.Sprintf("MemoryRegistry, %s = [], registered = %+v", target, allService)
	}
}

func (s *MemoryDiscovery) WatchAllServiceChanges(watcher Watcher) (cancel func(), err error) {
	return s.notifier.Watch(watcher), nil
}

type memoryResolver struct {
	registry     *MemoryDiscovery
	resolverOpts ResolverOptions
}

func (s *memoryResolver) Resolve(target string) (naming.Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	w := newMemoryWatcher(s.registry, target, ctx, cancel, &s.resolverOpts)
	return w, nil
}

func (s *memoryResolver) WithOption(opt ...ResolverOptionFunc) error {
	for _, o := range opt {
		if err := o(&s.resolverOpts); err != nil {
			return err
		}
	}
	return nil
}

type memoryWatcher struct {
	registry     *MemoryDiscovery
	resolverOpts *ResolverOptions
	target       string
	ctx          context.Context
	cancel       context.CancelFunc

	ch          chan []*naming.Update
	cancelWatch func()

	mutex  sync.RWMutex
	closed bool
}

var ErrWatcherClosed = status.Error(codes.Unavailable, "registry: watch closed")

func newMemoryWatcher(registry *MemoryDiscovery, target string, ctx context.Context, cancel context.CancelFunc, resolverOpts *ResolverOptions) *memoryWatcher {
	return &memoryWatcher{registry: registry, target: target, ctx: ctx, cancel: cancel, resolverOpts: resolverOpts}
}

func (s *memoryWatcher) Next() ([]*naming.Update, error) {
	if s.ch == nil {
		updates, err := s.firstNext()
		addrs := make([]string, 0, len(updates))
		for _, u := range updates {
			addrs = append(addrs, u.Addr)
		}
		log.Infof("memory_registry: firstNext of %q, addrs = %+v, err = %v", s.target, addrs, err)
		return updates, err
	}

	select {
	case updates, ok := <-s.ch:
		if !ok {
			return nil, ErrWatcherClosed
		} else {
			return updates, nil
		}
	case <-s.ctx.Done():
		return nil, ErrWatcherClosed
	}
}

func (s *memoryWatcher) firstNext() ([]*naming.Update, error) {
	if s.closed {
		return nil, ErrWatcherClosed
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.closed {
		return nil, ErrWatcherClosed
	}

	if instances, err := s.registry.ServiceInstances(s.target); err != nil {
		return nil, err
	} else {
		updates := make([]*naming.Update, 0, len(instances))
		for _, instance := range instances {
			updates = append(updates, &naming.Update{
				Op:       naming.Add,
				Addr:     instance.Addr,
				Metadata: instance.Meta,
			})
		}

		s.ch = make(chan []*naming.Update, 1)
		watcher := NewWatcherWrapper(s.notifyServiceUpdate, s.notifyServiceDelete)
		s.cancelWatch, _ = s.registry.WatchAllServiceChanges(watcher)

		return updates, nil
	}
}

func (s *memoryWatcher) notifyServiceUpdate(serviceName string, instances []Instance) {
	if serviceName != s.target {
		return
	}

	if s.closed {
		return
	}

	updates := make([]*naming.Update, 0, len(instances))
	addrs := make([]string, 0, len(instances))
	for _, instance := range instances {
		updates = append(updates, &naming.Update{
			Op:       naming.Add,
			Addr:     instance.Addr,
			Metadata: instance.Meta,
		})
		addrs = append(addrs, instance.Addr)
	}

	s.mutex.RLock()
	if !s.closed {
		s.ch <- updates
	}
	s.mutex.RUnlock()
	log.Infof("memory_registry: notifyServiceUpdate of %q, addrs = %+v", s.target, addrs)
}

func (s *memoryWatcher) notifyServiceDelete(serviceName string, instances []Instance) {
	if serviceName != s.target {
		return
	}

	if s.closed {
		return
	}

	updates := make([]*naming.Update, 0, len(instances))
	addrs := make([]string, 0, len(instances))
	for _, instance := range instances {
		updates = append(updates, &naming.Update{
			Op:       naming.Delete,
			Addr:     instance.Addr,
			Metadata: instance.Meta,
		})
		addrs = append(addrs, instance.Addr)
	}

	s.mutex.RLock()
	if !s.closed {
		s.ch <- updates
	}
	s.mutex.RUnlock()
	log.Infof("memory_registry: notifyServiceDelete of %q, addrs = %+v", s.target, addrs)
}

func (s *memoryWatcher) Close() {
	if s.closed {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return
	}

	s.closed = true

	if s.cancelWatch != nil {
		s.cancelWatch()
	}

	if s.cancel != nil {
		s.cancel()
	}
}
