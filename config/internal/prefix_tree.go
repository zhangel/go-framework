package internal

import (
	"sync"
)

type PrefixTree struct {
	root          *PrefixTreeNode
	separator     string
	prefixCache   map[string][]string
	prefixCacheMu sync.Mutex
	mu            sync.RWMutex
	m             map[string]*PrefixTreeNode
}

type PrefixTreeNode struct {
	meta     interface{}
	children map[string]*PrefixTreeNode
}

func (p *PrefixTree) Get(key string) interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if node, ok := p.m[key]; !ok {
		return nil
	} else {
		return node.meta
	}
}
