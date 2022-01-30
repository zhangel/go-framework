package declare

import (
	"github.com/zhangel/go-framework/internal/declare"
)

type PluginType declare.PluginType
type PluginInfo declare.PluginInfo

func Plugin(_type PluginType, plugin PluginInfo, flags ...Flag) {

}
