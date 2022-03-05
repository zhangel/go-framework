package declare

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/zhangel/go-framework.git/di"
	"github.com/zhangel/go-framework.git/uri"
)

type PluginType struct {
	Name          string
	DefaultPlugin string
}

var (
	AllPlugins     = map[PluginType]*PluginTypeInfo{}
	PluginCreators = map[PluginType]map[string]di.DepInjector{}
	pluginOnce     sync.Once
)

type PluginTypeInfo struct {
	uriParser   *uri.SimpleUriParser
	pluginInfos []PluginInfo
}

type PluginInfo struct {
	Name           string
	Creator        interface{}
	Deprecated     bool
	ForceEnableUri bool
}

func Plugin(typ PluginType, plugin PluginInfo, flags ...Flag) {
	pluginTypeInfo, ok := AllPlugins[typ]
	if !ok {
		pluginTypeInfo = &PluginTypeInfo{}
		AllPlugins[typ] = pluginTypeInfo
	}

	pluginTypeInfo.pluginInfos = append(pluginTypeInfo.pluginInfos, plugin)

	if PluginCreators[typ] == nil {
		PluginCreators[typ] = map[string]di.DepInjector{}
	}

	depInjector := di.NewDepInjector()
	_ = depInjector.Provide(plugin.Creator, false)
	PluginCreators[typ][plugin.Name] = depInjector

	if len(flags) > 0 || plugin.ForceEnableUri {
		if len(flags) == 0 {
			if pluginTypeInfo.uriParser == nil {
				pluginTypeInfo.uriParser = uri.NewSimpleUriParser(typ.Name, "type")
			}
			pluginTypeInfo.uriParser.WithScheme(plugin.Name)
		} else {
			hasUriField := false
			for idx := range flags {
				if flags[idx].UriField != uri.UriFieldNone {
					hasUriField = true
					break
				}
			}

			for idx := range flags {
				if hasUriField {
					if pluginTypeInfo.uriParser == nil {
						pluginTypeInfo.uriParser = uri.NewSimpleUriParser(typ.Name, "type")
					}

					if flags[idx].UriField != uri.UriFieldNone && flags[idx].UriField != uri.UriFieldQuery {
						pluginTypeInfo.uriParser.WithScheme(plugin.Name).WithField(flags[idx].UriField, flags[idx].Name, flags[idx].UriFieldHandler)
					} else {
						pluginTypeInfo.uriParser.WithScheme(plugin.Name).WithQuery(flags[idx].Name)
					}

				}

				flags[idx].pluginType = typ.Name
				flags[idx].pluginName = plugin.Name
				flags[idx].pluginDeprecated = plugin.Deprecated
			}
		}
		Flags(typ.Name+"."+plugin.Name, flags...)
	}
}

func PopulatePluginFlags() {
	pluginOnce.Do(func() {
		for pluginType, plugins := range AllPlugins {
			if len(plugins.pluginInfos) == 0 {
				continue
			}

			var pluginNames []string
			defaultPlugin := ""
			for _, plugin := range plugins.pluginInfos {
				if plugin.Deprecated {
					continue
				}

				pluginNames = append(pluginNames, plugin.Name)
				if plugin.Name == pluginType.DefaultPlugin {
					defaultPlugin = pluginType.DefaultPlugin
				}
			}

			if plugins.uriParser != nil {
				if err := uri.RegisterUri(plugins.uriParser); err != nil {
					log.Fatalf("[ERROR] Register uriParser for plugin type %q failed, err = %v", pluginType.Name, err)
				}

				uris := strings.Split(plugins.uriParser.String(), "\n")
				var deprecatedPlugins []string
				for _, plugin := range plugins.pluginInfos {
					if plugin.Deprecated {
						deprecatedPlugins = append(deprecatedPlugins, strings.ToLower(plugin.Name))
					}
				}

				var availableUris []string
				for _, pluginUri := range uris {
					deprecated := false
					for _, deprecatedPlugin := range deprecatedPlugins {
						if strings.HasPrefix(pluginUri, deprecatedPlugin+":") {
							deprecated = true
							break
						}
					}

					if !deprecated {
						availableUris = append(availableUris, pluginUri)
					}
				}

				Flags("", Flag{Name: pluginType.Name, DefaultValue: "", Description: fmt.Sprintf("Uri of %s plugin instance. optionals:%s", pluginType.Name, strings.Replace("\n"+strings.Join(availableUris, "\n"), "\n", "\n- ", -1)), pluginTypeFlag: true})
			}

			sort.Strings(pluginNames)

			Flags("",
				Flag{Name: PluginTypeFlag(pluginType), DefaultValue: defaultPlugin, Description: fmt.Sprintf("Specify a %s plugin. optionals: %s.", pluginType.Name, strings.Join(pluginNames, ", ")), pluginTypeFlag: true},
			)
		}
	})
}

func PluginTypeFlag(pluginType PluginType) string {
	return pluginType.Name + ".type"
}
