package registry

import (
	"sync"
)

type Notifier struct {
	watchers []Watcher
	mu       sync.RWMutex
}

func NewNotifier(watchers ...Watcher) *Notifier {
	return &Notifier{watchers: watchers, mu: sync.RWMutex{}}
}

func (s *Notifier) Watch(watcher Watcher) (cancel func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.watchers = append(s.watchers, watcher)
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		var remains []Watcher
		for _, w := range s.watchers {
			if w == watcher {
				continue
			}
			remains = append(remains, w)
		}
		s.watchers = remains
	}
}

func (s *Notifier) _Watchers() []Watcher {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Watcher, len(s.watchers))
	copy(result, s.watchers)

	return result
}

func (s *Notifier) OnUpdate(serviceName string, instances []Instance) {
	for _, w := range s._Watchers() {
		w.OnUpdate(serviceName, instances)
	}
}

func (s *Notifier) OnDelete(serviceName string, instances []Instance) {
	for _, w := range s._Watchers() {
		w.OnDelete(serviceName, instances)
	}
}
