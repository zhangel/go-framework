package watcher

type Watcher interface {
	OnUpdate(map[string]string)
	OnDelete([]string)
}

type SourceWatcher interface {
	Watcher
	Onsync(map[string]string)
}
