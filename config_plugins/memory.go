package config_plugins

import (
	"strings"
	"sync"

	"github.com/zhangel/go-framework.git/config/watcher"
	"github.com/zhangel/go-framework.git/config_plugins/internal"
)

type MemoryConfigSource struct {
	mu       sync.RWMutex
	val      map[string]interface{}
	notifier *watcher.Notifier
}

func NewMemoryConfigSource(val map[string]interface{}) *MemoryConfigSource {
	return &MemoryConfigSource{
		val:      val,
		notifier: watcher.NewNotifier(),
	}
}

func (s *MemoryConfigSource) Sync() (map[string]string, error) {
	config := make(map[string]string, len(s.val))
	internal.Walk("", s.val, &config)
	return config, nil
}

func (s *MemoryConfigSource) Watch(watcher watcher.SourceWatcher) (cancel func()) {
	return s.notifier.Watch(watcher)
}

func (s *MemoryConfigSource) AppendPrefix(_ []string) error {
	return nil
}

func (s *MemoryConfigSource) Close() error {
	return nil
}

func (s *MemoryConfigSource) Get(k string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	k = strings.ToLower(k)
	v, ok := s.val[k]
	return v, ok
}

func (s *MemoryConfigSource) Put(k string, v interface{}) {
	k = strings.ToLower(k)

	s.mu.Lock()
	s.val[k] = v
	s.mu.Unlock()

	updateNotify := make(map[string]string)
	internal.Walk("", map[string]interface{}{k: v}, &updateNotify)
	s.notifier.OnUpdate(updateNotify)
}

func (s *MemoryConfigSource) Del(k string) {
	k = strings.ToLower(k)

	s.mu.Lock()
	delete(s.val, k)
	s.mu.Unlock()

	deleteNotify := []string{k}
	s.notifier.OnDelete(deleteNotify)
}

func (s *MemoryConfigSource) Replace(val map[string]interface{}) {
	s.mu.Lock()
	s.val = val
	s.mu.Unlock()

	s.mu.RLock()
	config := make(map[string]string, len(s.val))
	internal.Walk("", s.val, &config)
	s.mu.RUnlock()

	s.notifier.OnSync(config)
}
