package config

import (
	"sync"
	"time"

	"github.com/zhangel/go-framework.git/config/internal"
	"github.com/zhangel/go-framework.git/config/watcher"
)

type Source internal.Source
type Config internal.Config

var Plugin = internal.ConfigPlugin
var (
	defaultConfig Config
	mutex         sync.RWMutex
)

func GlobalConfig() Config {
	mutex.RLock()
	defer mutex.RUnlock()
	return defaultConfig
}

func NewConfig(ns []string, sources ...Source) (Config, error) {
	var internalSources []internal.Source
	for _, source := range sources {
		if source == nil {
			continue
		}
		internalSources = append(internalSources, source)
	}
	return internal.NewConfig(ns, internalSources...)
}

func SetGlobalConfig(config Config) {
	mutex.Lock()
	defer mutex.Unlock()

	defaultConfig = config
}

func WithNamespace(ns ...string) Config {
	return GlobalConfig().WithNamespace(ns...)
}

func WithPrefix(prefix string) Config {
	return GlobalConfig().WithPrefix(prefix)
}

func WithPassword(password ...string) Config {
	return GlobalConfig().WithPassword(password...)
}

func Watch(watcher watcher.Watcher) (cancel func()) {
	return GlobalConfig().Watch(watcher)
}

func Bool(key string) bool {
	return GlobalConfig().Bool(key)
}

func Int(key string) int {
	return GlobalConfig().Int(key)
}

func Uint(key string) uint {
	return GlobalConfig().Uint(key)
}

func Int64(key string) int64 {
	return GlobalConfig().Int64(key)
}

func Uint64(key string) uint64 {
	return GlobalConfig().Uint64(key)
}

func Float64(key string) float64 {
	return GlobalConfig().Float64(key)
}

func Duration(key string) time.Duration {
	return GlobalConfig().Duration(key)
}

func Bytes(key string) []byte {
	return GlobalConfig().Bytes(key)
}

func String(key string) string {
	return GlobalConfig().String(key)
}

func Int64List(key string) []int64 {
	return GlobalConfig().Int64List(key)
}

func Uint64List(key string) []uint64 {
	return GlobalConfig().Uint64List(key)
}

func StringList(key string) []string {
	return GlobalConfig().StringList(key)
}

func StringMap(key string) map[string]string {
	return GlobalConfig().StringMap(key)
}

func GetByPrefix(prefix string) map[string]string {
	return GlobalConfig().GetByPrefix(prefix)
}

func Has(key string) bool {
	return GlobalConfig().Has(key)
}
