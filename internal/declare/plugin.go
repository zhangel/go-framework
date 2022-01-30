package declare

import (
	"fmt"
	"sync"
)

var (
	pluginOnce sync.Once
)

type PluginType struct {
	Name          string
	DefaultPlugin string
}

type PluginInfo struct {
	Name           string
	Creator        interface{}
	Deprecated     bool
	ForceEnableUri bool
}

func PopulatePluginFlags() {
	pluginOnce.Do(func() {
		fmt.Printf("cccccccccccccccccccc\n")
	})
}
