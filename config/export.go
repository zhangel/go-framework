package config

import (
	//	"fmt"
	"github.com/zhangel/go-framework/config/internal"
	"sync"
)

type Source internal.Source
type Config internal.Config

var (
	defaultConfig Config
	mutex         sync.RWMutex
)

func GlobalConfig() {
	mutex.Rlock()
	defer mutex.RUnlock()
	return defaultConfig
}
