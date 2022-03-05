package declare

import "github.com/zhangel/go-framework/internal/declare"

type PluginType declare.PluginType
type PluginInfo declare.PluginInfo

type PluginInfoWithFlags struct {
	Name           string
	Creator        interface{}
	Deprecated     bool
	ForceEnableUri bool
	Flags          []Flag
}

var (
	pluginInfoMap = map[PluginType]map[string]*PluginInfoWithFlags{}
)

func Plugin(typ PluginType, plugin PluginInfo, flags ...Flag) {
	var internalFlags []declare.Flag
	for _, f := range flags {
		internalFlags = append(internalFlags, declare.Flag(f))
	}
	declare.Plugin(declare.PluginType(typ), declare.PluginInfo(plugin), internalFlags...)

	m, ok := pluginInfoMap[typ]
	if !ok {
		m = map[string]*PluginInfoWithFlags{}
		pluginInfoMap[typ] = m
	}

	m[plugin.Name] = &PluginInfoWithFlags{
		Name:           plugin.Name,
		Creator:        plugin.Creator,
		Deprecated:     plugin.Deprecated,
		ForceEnableUri: plugin.ForceEnableUri,
		Flags:          flags,
	}
}

func PluginInfoByType(typ PluginType) map[string]*PluginInfoWithFlags {
	result := map[string]*PluginInfoWithFlags{}
	m, ok := pluginInfoMap[typ]
	if ok {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
