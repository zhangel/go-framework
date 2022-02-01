package internal

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/zhangel/go-framework/config/watcher"
	"strings"
	"sync"
)

type ValueGetter interface {
	Get(key string) (string, bool)
	GetByPrefix(prefix string) map[string]string
}

type DataSource interface {
	Get(key, prefix string, raw bool) (string, int)
	GetByPrefix(prefix string) map[string]string
	AppendPrefix(prefix []string) error
	Watch(watcher watcher.Watcher) (cancel func())
	Close() error
}

type View struct {
	Value
	filter   *Filter
	source   DataSource
	prefix   string
	ns       []string
	password [][16]byte
	notifier *watcher.Notifier
	once     sync.Once

	mu        sync.RWMutex
	viewCache map[string]*View
	cancel    func()
}

func NewConfigView(source DataSource, prefix string, ns []string, password [][16]byte) *View {
	lowerNs := make([]string, len(ns))
	for i, n := range ns {
		lowerNs[i] = strings.ToLower(n)
	}

	if len(lowerNs) != 0 {
		_ = source.AppendPrefix(lowerNs)
	}

	filter := NewFilter(source, prefix, lowerNs, password)
	return &View{
		Value:     NewValue(filter),
		filter:    filter,
		source:    source,
		prefix:    prefix,
		ns:        lowerNs,
		notifier:  watcher.NewNotifier(),
		viewCache: map[string]*View{},
	}
}

func (s *View) WithNamespace(ns ...string) Config {
	lowerNs := make([]string, len(ns))
	for i, n := range ns {
		lowerNs[i] = strings.ToLower(n)
	}

	newNs := append(s.ns, lowerNs...)
	id := viewId(s.prefix, newNs, s.password)
	s.mu.RLock()
	if view, ok := s.viewCache[id]; ok {
		s.mu.RUnlock()
		return view
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if view, ok := s.viewCache[id]; ok {
		return view
	}

	if len(lowerNs) != 0 {
		_ = s.source.AppendPrefix(lowerNs)
	}

	view := NewConfigView(s.source, s.prefix, newNs, s.password)
	s.viewCache[id] = view
	return view
}

func (s *View) WithPrefix(prefix string) Config {
	prefix = strings.ToLower(prefix)

	id := viewId(prefix, s.ns, s.password)
	s.mu.RLock()
	if view, ok := s.viewCache[id]; ok {
		s.mu.RUnlock()
		return view
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if view, ok := s.viewCache[id]; ok {
		return view
	}

	view := NewConfigView(s.source, prefix, s.ns, s.password)
	s.viewCache[id] = view
	return view
}

func (s *View) WithPassword(password ...string) Config {
	newPassword := s.password
	for _, p := range password {
		if p == "" {
			continue
		}
		newPassword = append(newPassword, md5.Sum([]byte(p)))
	}

	id := viewId(s.prefix, s.ns, newPassword)
	s.mu.RLock()
	if view, ok := s.viewCache[id]; ok {
		s.mu.RUnlock()
		return view
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if view, ok := s.viewCache[id]; ok {
		return view
	}

	view := NewConfigView(s.source, s.prefix, s.ns, newPassword)
	s.viewCache[id] = view
	return view
}

func (s *View) Watch(watcher watcher.Watcher) (cancel func()) {
	s.once.Do(func() { s.cancel = s.source.Watch(s) })
	return s.notifier.Watch(watcher)
}

func (s *View) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return s.source.Close()
}

func (s *View) OnUpdate(val map[string]string) {
	if s.notifier.Size() == 0 {
		return
	}
	result := s.filter.FilterUpdate(val)
	if len(result) > 0 {
		s.notifier.OnUpdate(result)
	}
}

func (s *View) OnDelete(val []string) {
	if s.notifier.Size() == 0 {
		return
	}

	updateNotify, deleteNotify := s.filter.FilterDelete(val)
	if len(updateNotify) > 0 {
		s.notifier.OnUpdate(updateNotify)
	}
	if len(deleteNotify) > 0 {
		s.notifier.OnDelete(deleteNotify)
	}
}

func viewId(prefix string, ns []string, password [][16]byte) string {
	passwordIdBuilder := strings.Builder{}
	for _, p := range password {
		passwordIdBuilder.WriteString(hex.EncodeToString(p[:]))
	}
	return prefix + ":" + strings.Join(ns, "|") + passwordIdBuilder.String()
}
