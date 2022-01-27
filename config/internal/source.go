package internal

import (
	//	"fmt"
	"sync"
)

type Source interface {
	Sync() (map[string]string, error)
}

type SyncWrapper struct {
	source Source
	cache  sync.Map
	prefix []string
}

func (s *SyncWrapper) Sync() (map[string]string, error) {
	result := make(map[string]string, 0)
	return result, nil
}
