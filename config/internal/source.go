package internal

import (
	//	"fmt"
	"github.com/zhangel/go-framework/config/watcher"
	"sync"
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

func (s *SourceWrapper) Get(key string) (string, bool) {
	val, ok := s.cache.Load(key)
	if ok {
		return val.(string), true
	} else {
		return "", false
	}
}

func (s *SourceWrapper) IgnoreNamespace() bool {
	_, ok := s.source.(SourceIgnoreNamespace)
	return ok
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
