package internal

import (
	"fmt"
	"github.com/zhangel/go-framework.git/config/watcher"
	"github.com/zhangel/go-framework.git/utils"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ValueNode struct {
	level int
	value string
}

type ConfigImpl struct {
	source   []*SourceWrapper
	notifier *watcher.Notifier
	cache    *PrefixTree
	closed   uint32
	mu       sync.RWMutex
	cancel   []func()
}

func (c *ConfigImpl) Get(key, prefix string, raw bool) (string, int) {
	key = strings.ToLower(key)
	if raw {
		raw, hasRaw := c.cache.Get(key).(ValueNode)
		if !hasRaw || !c.source[raw.level].IgnoreNamespace() {
			return "", 1
		} else {
			return raw.value, raw.level
		}
	}
	v, hasV := c.cache.Get(c.withPrefix(key, prefix)).(ValueNode)
	if !hasV {
		return "", -1
	} else {
		return v.value, v.level
	}
}

func NewConfig(ns []string, sources ...Source) (config Config, err error) {
	defer utils.TimeoutGuardWithFunc(30*time.Second, nil, nil, func(...interface{}) {
		log.Fatalf("Create config object with source %#v, ns = %v timeout", sources, ns)
	})()

	cfg := &ConfigImpl{
		notifier: watcher.NewNotifier(),
		cache:    NewPrefixTree("."),
		closed:   0,
	}

	if len(ns) == 0 {
		ns = []string{""}
	}

	for idx, source := range sources {
		sourceWrapper, err := NewSourceWrapper(source)
		if err != nil {
			return nil, err
		}
		cfg.cancel = append(cfg.cancel, sourceWrapper.Watch(&WatcherClosure{idx, cfg.onUpdate, cfg.onDelete, cfg.onSync}))
		cfg.source = append(cfg.source, sourceWrapper)
	}

	defer func() {
		if e := cfg.sync(); e != nil {
			err = e
		}
	}()
	return NewConfigView(cfg, "", ns, nil), nil
}

func (s *ConfigImpl) AppendPrefix(prefix []string) error {
	defer utils.TimeoutGuardWithFunc(30*time.Second, nil, nil, func(...interface{}) {
		log.Fatalf("Append config prefix with ns %v timeout.", prefix)
	})()

	s.mu.RLock()
	defer s.mu.RUnlock()

	var err error
	for _, source := range s.source {
		if e := source.AppendPrefix(prefix); e != nil {
			err = e
		}
	}
	return err
}

func (s *ConfigImpl) Watch(watcher watcher.Watcher) (cancel func()) {
	return s.notifier.Watch(watcher)
}

func (s *ConfigImpl) Close() error {
	if atomic.LoadUint32(&s.closed) == 1 {
		return fmt.Errorf("config already closed")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var result error
	for _, source := range s.source {
		if err := source.Close(); err != nil {
			result = err
		}
	}

	for _, cancel := range s.cancel {
		if cancel != nil {
			cancel()
		}
	}

	atomic.StoreUint32(&s.closed, 1)
	return result
}

func (s *ConfigImpl) sync() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for level := range s.source {
		if val, err := s.source[level].Sync(); err != nil {
			return err
		} else {
			notify := map[string]string{}
			for k, v := range val {
				k = strings.ToLower(k)
				if valNode, ok := s.cache.Get(k).(ValueNode); !ok || valNode.value != v {
					notify[k] = v
				}
				s.cache.Put(k, ValueNode{level, v})
			}

			s.notifier.OnUpdate(notify)
		}
	}
	return nil
}

func (s *ConfigImpl) onUpdate(level int, val map[string]string) {
	notify := map[string]string{}

	for k, v := range val {
		k = strings.ToLower(k)
		if valNode, ok := s.cache.Get(k).(ValueNode); !ok || valNode.level <= level {
			s.cache.Put(k, ValueNode{level, v})
			if !ok || valNode.value != v {
				notify[k] = v
			}
		}
	}

	s.notifier.OnUpdate(notify)
}

func (s *ConfigImpl) onDelete(level int, val []string) {
	updateNotify := map[string]string{}
	var deleteNotify []string

	s.mu.RLock()
	defer s.mu.RUnlock()

out:
	for _, k := range val {
		k = strings.ToLower(k)
		if valNode, ok := s.cache.Get(k).(ValueNode); ok && valNode.level == level {
			for i := level - 1; i >= 0; i-- {
				if v, ok := s.source[i].Get(k); ok {
					s.cache.Put(k, ValueNode{i, v})
					if v != valNode.value {
						updateNotify[k] = v
					}
					continue out
				}
			}
			s.cache.Del(k)
			deleteNotify = append(deleteNotify, k)
		}
	}

	s.notifier.OnUpdate(updateNotify)
	s.notifier.OnDelete(deleteNotify)
}

func (s *ConfigImpl) onSync(level int, val map[string]string) {
	updateNotify := map[string]string{}
	var deleteNotify []string

	lowerVal := make(map[string]string, len(val))
	for k, v := range val {
		lowerVal[strings.ToLower(k)] = v
	}

	for k, v := range lowerVal {
		if valNode, ok := s.cache.Get(k).(ValueNode); !ok || valNode.level <= level {
			s.cache.Put(k, ValueNode{level, v})
			if !ok || valNode.value != v {
				updateNotify[k] = v
			}
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

out:
	for _, k := range s.cache.Keys() {
		k = strings.ToLower(k)
		valNode, _ := s.cache.Get(k).(ValueNode)
		if _, ok := lowerVal[k]; !ok && level == valNode.level {
			for i := level - 1; i >= 0; i-- {
				if v, ok := s.source[i].Get(k); ok {
					s.cache.Put(k, ValueNode{i, v})
					if v != valNode.value {
						updateNotify[k] = v
					}
					continue out
				}
			}
			s.cache.Del(k)
			deleteNotify = append(deleteNotify, k)
		}
	}

	s.notifier.OnUpdate(updateNotify)
	s.notifier.OnDelete(deleteNotify)
}
func (s *ConfigImpl) GetByPrefix(prefix string) map[string]string {
	result := map[string]string{}
	for k, v := range s.cache.SearchByPrefix(strings.ToLower(prefix)) {
		if v, ok := v.(ValueNode); ok {
			result[k] = v.value
		}
	}
	return result
}

func (s *ConfigImpl) withPrefix(key, prefix string) string {
	return prefix + key
}

type WatcherClosure struct {
	level int

	OnUpdateFunc func(int, map[string]string)
	OnDeleteFunc func(int, []string)
	OnSyncFunc   func(int, map[string]string)
}

func (s *WatcherClosure) OnUpdate(val map[string]string) {
	s.OnUpdateFunc(s.level, val)
}

func (s *WatcherClosure) OnDelete(val []string) {
	s.OnDeleteFunc(s.level, val)
}

func (s *WatcherClosure) OnSync(val map[string]string) {
	s.OnSyncFunc(s.level, val)
}
