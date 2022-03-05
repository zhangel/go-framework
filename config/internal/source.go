package internal

import (
	"github.com/zhangel/go-framework.git/config/watcher"
	"strings"
	"sync"
	"sync/atomic"
)

type Source interface {
	Sync() (map[string]string, error)
	Watch(watcher.SourceWatcher) (cancel func())
	AppendPrefix(prefix []string) error
	Close() error
}

type SyncWrapper struct {
	source Source
	cache  sync.Map
	prefix []string
}

type SourceWrapper struct {
	source       Source
	notifier     *watcher.Notifier
	cache        sync.Map
	prefix       []string
	synchronized uint32
	mu           sync.RWMutex
	cancel       func()
}

type SourceIgnoreNamespace interface {
	IgnoreNamespace()
}

func (s *SyncWrapper) Sync() (map[string]string, error) {
	result := make(map[string]string, 0)
	return result, nil
}

/*
func (s *SourceWrapper) Get(key string) (string, bool) {
	val, ok := s.cache.Load(key)
	if ok {
		return val.(string), true
	} else {
		return "", false
	}
}
*/

func NewSourceWrapper(source Source) (*SourceWrapper, error) {
	wrapper := &SourceWrapper{
		source:   source,
		notifier: watcher.NewNotifier(),
	}

	wrapper.cancel = source.Watch(wrapper)
	return wrapper, nil
}

func (s *SourceWrapper) Sync() (map[string]string, error) {
	result := make(map[string]string)
	if atomic.LoadUint32(&s.synchronized) == 1 {
		s.cache.Range(func(k, v interface{}) bool {
			result[k.(string)] = v.(string)
			return true
		})
		return result, nil
	}

	if val, err := s.source.Sync(); err != nil {
		return nil, err
	} else {
		for k, v := range val {
			k = strings.ToLower(k)
			s.cache.Store(k, v)
			result[k] = v
		}
		return result, nil
	}
}

func (s *SourceWrapper) Watch(watcher watcher.SourceWatcher) (cancel func()) {
	return s.notifier.Watch(watcher)
}

func (s *SourceWrapper) AppendPrefix(prefix []string) error {
	var filtered []string
Out:
	for _, p := range prefix {
		p = strings.TrimSpace(p)

		s.mu.RLock()
		for _, cache := range s.prefix {
			if strings.HasPrefix(p, cache) {
				s.mu.RUnlock()
				continue Out
			}
		}

		filtered = append(filtered, p)
		s.mu.RUnlock()
	}

	if len(filtered) == 0 {
		return nil
	}

	if err := s.source.AppendPrefix(filtered); err != nil {
		return err
	} else {
		s.mu.Lock()
		s.prefix = append(s.prefix, filtered...)
		s.mu.Unlock()
		return nil
	}
}

func (s *SourceWrapper) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return s.source.Close()
}

func (s *SourceWrapper) Get(key string) (string, bool) {
	val, ok := s.cache.Load(key)
	if ok {
		return val.(string), true
	} else {
		return "", false
	}
}

func (s *SourceWrapper) OnUpdate(val map[string]string) {
	for k, v := range val {
		s.cache.Store(k, v)
	}

	s.notifier.OnUpdate(val)
}

func (s *SourceWrapper) OnDelete(val []string) {
	for _, k := range val {
		s.cache.Delete(k)
	}

	s.notifier.OnDelete(val)
}

func (s *SourceWrapper) OnSync(val map[string]string) {
	for k, v := range val {
		s.cache.Store(k, v)
	}

	s.notifier.OnSync(val)
}

func (s *SourceWrapper) IgnoreNamespace() bool {
	_, ok := s.source.(SourceIgnoreNamespace)
	return ok
}
