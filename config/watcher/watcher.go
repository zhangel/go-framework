package watcher

import "strings"

type Watcher interface {
	OnUpdate(map[string]string)
	OnDelete([]string)
}

type SourceWatcher interface {
	Watcher
	OnSync(map[string]string)
}

type Helper struct {
	keyToWatch  string
	watcherFunc func(v string, deleted bool)
}

func NewHelper(keyToWatch string, watcher func(v string, deleted bool)) *Helper {
	return &Helper{
		keyToWatch:  strings.ToLower(keyToWatch),
		watcherFunc: watcher,
	}
}

func (s *Helper) OnUpdate(val map[string]string) {
	if v, ok := val[strings.ToLower(s.keyToWatch)]; ok {
		s.watcherFunc(v, false)
	}
}

func (s *Helper) OnDelete(val []string) {
	for _, k := range val {
		if strings.ToLower(k) != s.keyToWatch {
			continue
		}
		s.watcherFunc("", true)
		return
	}
}
