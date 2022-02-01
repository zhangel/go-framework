package watcher

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

		var canceled []Watcher
		for _, w := range s.watchers {
			if w == watcher {
				continue
			}
			canceled = append(canceled, w)
		}
		s.watchers = canceled
	}
}

func (s *Notifier) _Watchers() []Watcher {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Watcher, len(s.watchers))
	copy(result, s.watchers)

	return result
}

func (s *Notifier) OnUpdate(notify map[string]string) {
	if len(notify) == 0 {
		return
	}

	for _, w := range s._Watchers() {
		w.OnUpdate(notify)
	}
}

func (s *Notifier) OnDelete(notify []string) {
	if len(notify) == 0 {
		return
	}

	for _, w := range s._Watchers() {
		w.OnDelete(notify)
	}
}

func (s *Notifier) OnSync(notify map[string]string) {
	if len(notify) == 0 {
		return
	}

	for _, w := range s._Watchers() {
		if sw, ok := w.(SourceWatcher); ok {
			sw.OnSync(notify)
		}
	}
}

func (s *Notifier) Size() int {
	return len(s._Watchers())
}
