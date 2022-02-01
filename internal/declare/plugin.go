package declare

import (
	"fmt"
	"github.com/zhangel/go-framework/uri"
	"log"
	"sort"
	"strings"
	"sync"
)

var (
	pluginOnce sync.Once
	AllPlugins = map[PluginType]*PluginTypeInfo{}
)

type PluginType struct {
	Name          string
	DefaultPlugin string
}

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

func PopulatePluginFlags() {
	pluginOnce.Do(func() {
		for pluginType, plugins := range AllPlugins {
			fmt.Printf("pluginType=%+v\n", pluginType)
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
					log.Fatalf("[ERROR] Register uriParser for plugin type %q failed, err = %v",
						pluginType.Name, err)
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
				Flags("",
					Flag{Name: pluginType.Name, DefaultValue: "", Description: fmt.Sprintf("U    ri of %s plugin instance. optionals:%s", pluginType.Name, strings.Replace("\n"+strings.Join(availableUris, "\n"), "\n", "\n- ", -1)), pluginTypeFlag: true})

			}
			sort.Strings(pluginNames)

			Flags("",
				Flag{Name: PluginTypeFlag(pluginType),
					DefaultValue: defaultPlugin,
					Description:  fmt.Sprintf("Specify a %s plugin. optionals: %s.", pluginType.Name, strings.Join(pluginNames, ", ")), pluginTypeFlag: true},
			)

		}
	})
}

func PluginTypeFlag(pluginType PluginType) string {
	return pluginType.Name + ".type"
}
