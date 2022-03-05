package internal

import (
	"log"
	"strings"
	"sync"
)

const initializeCap = 100

type PrefixTree struct {
	root          *PrefixTreeNode
	separator     string
	prefixCache   map[string][]string
	prefixCacheMu sync.Mutex
	m             map[string]*PrefixTreeNode
	mu            sync.RWMutex
}

type PrefixTreeNode struct {
	meta     interface{}
	children map[string]*PrefixTreeNode
}

func NewPrefixTree(separator string) *PrefixTree {
	if len(separator) > 1 {
		log.Fatalf("[ERROR] Only single-char separator supported.")
	}

	return &PrefixTree{
		newNode(),
		separator,
		make(map[string][]string, initializeCap),
		sync.Mutex{},
		make(map[string]*PrefixTreeNode, initializeCap),
		sync.RWMutex{},
	}
}

func newNode() *PrefixTreeNode {
	return &PrefixTreeNode{nil, map[string]*PrefixTreeNode{}}
}

func (s *PrefixTree) prefix(key string) []string {
	s.prefixCacheMu.Lock()
	defer s.prefixCacheMu.Unlock()

	if prefix, ok := s.prefixCache[key]; ok {
		return prefix
	} else {
		prefix := strings.Split(key, s.separator)
		s.prefixCache[key] = prefix
		return prefix
	}
}

func (s *PrefixTree) Put(key string, meta interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var node = s.root
	for _, prefix := range s.prefix(key) {
		if len(prefix) == 0 {
			return
		}

		child, ok := node.children[prefix]
		if !ok {
			child = newNode()
			node.children[prefix] = child
		}
		node = child
	}
	node.meta = meta
	s.m[key] = node
}

func (s *PrefixTree) node(key string) *PrefixTreeNode {
	var node = s.root
	for _, prefix := range s.prefix(key) {
		if len(prefix) == 0 {
			return nil
		}

		child, ok := node.children[prefix]
		node = child
		if !ok || node == nil {
			break
		}
	}

	return node
}

func (s *PrefixTree) Get(key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if node, ok := s.m[key]; !ok {
		return nil
	} else {
		return node.meta
	}
}

func (s *PrefixTree) Del(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node := s.node(key)
	if node != nil {
		node.meta = nil
	}
	delete(s.m, key)
}

func (s *PrefixTree) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	for k, _ := range s.m {
		keys = append(keys, k)
	}
	return keys
}

func (s *PrefixTree) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.root = newNode()
	s.prefixCacheMu.Lock()
	s.prefixCache = make(map[string][]string, initializeCap)
	s.prefixCacheMu.Unlock()
	s.m = make(map[string]*PrefixTreeNode, initializeCap)
}

func (s *PrefixTree) SearchByPrefix(prefix string) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := map[string]interface{}{}
	var travel func(name string, node *PrefixTreeNode)
	travel = func(name string, node *PrefixTreeNode) {
		if node == nil {
			return
		}

		if len(name) != 0 && node.meta != nil {
			result[name] = node.meta
		}

		for k, node := range node.children {
			if len(name) != 0 {
				travel(name+s.separator+k, node)
			} else {
				travel(k, node)
			}
		}
	}

	if len(prefix) == 0 {
		travel(prefix, s.root)
	} else {
		travel(prefix, s.node(prefix))
	}

	return result
}
