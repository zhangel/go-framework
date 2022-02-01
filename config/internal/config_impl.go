package internal

import (
	"github.com/zhangel/go-framework/config/watcher"
	"strings"
	"sync"
)

type ValueNode struct {
	level int
	value string
}

type ConfigImpl struct {
	source   []*SourceWrapper
	notifier *watcher.Notifier
	cache    *PrefixTree
	closed   uint32
	mu       sync.RWMutex
	cancel   []func()
}

func (c *ConfigImpl) withPrefix(key, prefix string) string {
	return prefix + key
}

func (c *ConfigImpl) Get(key, prefix string, raw bool) (string, int) {
	key = strings.ToLower(key)
	if raw {
		raw, hasRaw := c.cache.Get(key).(ValueNode)
		if !hasRaw || !c.source[raw.level].IgnoreNamespace() {
			return "", 1
		} else {
			return raw.value, raw.level
		}
	}
	v, hasV := c.cache.Get(c.withPrefix(key, prefix)).(ValueNode)
	if !hasV {
		return "", -1
	} else {
		return v.value, v.level
	}
}
