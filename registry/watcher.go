package registry

type Watcher interface {
	OnUpdate(serviceName string, instances []Instance)
	OnDelete(serviceName string, instances []Instance)
}

type WatcherWrapper struct {
	onUpdate func(serviceName string, instances []Instance)
	onDelete func(serviceName string, instances []Instance)
}

func NewWatcherWrapper(onUpdate func(serviceName string, instances []Instance), onDelete func(serviceName string, instances []Instance)) Watcher {
	return &WatcherWrapper{onUpdate: onUpdate, onDelete: onDelete}
}

func (s *WatcherWrapper) OnUpdate(serviceName string, instances []Instance) {
	if s.onUpdate == nil {
		return
	}

	s.onUpdate(serviceName, instances)
}

func (s *WatcherWrapper) OnDelete(serviceName string, instances []Instance) {
	if s.onDelete == nil {
		return
	}

	s.onDelete(serviceName, instances)
}
