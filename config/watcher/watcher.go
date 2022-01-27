package watcher

type Watcher interface {
	OnUpdate(map[string]string)
	OnDelete([]string)
}
