package config

import (
	"github.com/zhangel/go-framework/config/internal"
	"sync"
)

type Source internal.Source
type Config internal.Config

var (
	defaultConfig Config
	mutex         sync.RWMutex
)

func GlobalConfig() Config {
	mutex.RLock()
	defer mutex.RUnlock()
	return defaultConfig
}

func Int(key string) int {
	return GlobalConfig().Int(key)
}
