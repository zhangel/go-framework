package config
import (
	"sync"
	""
)


type ConfigImple struct {
	source   []*SourceWrapper
	notifier *watcher.Notifier
	cache    PrefixTree
	closed   uint32
	mu       sync.RWMutex
	cancel   []func()
}
