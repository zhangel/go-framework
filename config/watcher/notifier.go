package watcher

import (
	"sync"
)

type Notifier struct {
	watchers []Watcher
	mu       sync.RWMutex
}
