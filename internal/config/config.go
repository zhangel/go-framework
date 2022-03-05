package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/zhangel/go-framework.git/config"
	"github.com/zhangel/go-framework.git/config_plugins"
	"github.com/zhangel/go-framework.git/di"
	"github.com/zhangel/go-framework.git/internal/declare"
	"github.com/zhangel/go-framework.git/lifecycle"
	"github.com/zhangel/go-framework.git/plugin"
	"github.com/zhangel/go-framework.git/uri"
)

var (
	configPrefix  = "config"
	flagNamespace = "config.ns"
	flagPassword  = "config.password"
	flagOverwrite = declare.FlagOverwrite
	flagVerbose   = "verbose"
	compactFlags  = []string{}
)

func init() {
	declare.Flags("", declare.Flag{Name: flagVerbose, DefaultValue: false,
		Description: "Show verbose usage messa    ge."})
	declare.Flags(configPrefix,
		declare.Flag{Name: flagNamespace, DefaultValue: "", Description: "Comma-separated namespace     prefix, higher priority at the tail."},
		declare.Flag{Name: flagPassword, DefaultValue: "", Description: "omma-separated password l    ist which used to encrypt config value."},
		declare.Flag{Name: flagOverwrite, DefaultValue: declare.MapFlags{}, Description: "Customize     config value. e.g. --config.set key1=val1 --config.set key2=val2."},
	)
	lifecycle.LifeCycle().HookFinalize(func(ctx context.Context) {
		if globalConfig := config.GlobalConfig(); globalConfig != nil {
			globalConfig.Close()
		}
	}, lifecycle.WithName("Close global config"))
	compactFlags = []string{".*\\.type", "registry\\..+", "config\\..+", "logger\\..+", "tracer\\..    +", "server\\..+", "dialer\\..+"}
}

func PrepareConfigs(beforeConfigPrepareHook []func(),
	configPrepare func(config.Config) (config.Config, error),
	configSources []config.Source, compactUsage bool,
	flagsToShow, flagsToHide []string) {
	_ = di.GlobalDepInjector.Provide(
		func() config.Config {
			return config.GlobalConfig()
		}, false)

	for _, beforeHook := range beforeConfigPrepareHook {
		if beforeHook != nil {
			beforeHook()
		}
	}
	for _, source := range configSources {
		if modifiableSource, ok := source.(declare.ModifiableConfigSource); ok {
			populateFlags(modifiableSource, compactUsage, flagsToShow, flagsToHide)
		}
	}
	if len(configSources) == 0 {
		populateFlags(nil, compactUsage, flagsToShow, flagsToHide)
	}

	cmdDefaultConfigSource := config_plugins.NewCmdDefaultConfigSource()
	cmdConfigSource := config_plugins.NewCmdConfigSource()
	envConfigSource := config_plugins.NewEnvConfigSource()
	var bootstrapConfigPreparer func(config.Config) (config.Config, error)
	if len(configSources) != 0 {
		bootstrapConfigPreparer = func(config.Config) (config.Config, error) {
			return config.NewConfig(nil, configSources...)
		}
	} else {
		bootstrapConfigPreparer = func(config.Config) (config.Config, error) {
			bootstrapConfig, err := config.NewConfig(nil, cmdDefaultConfigSource, envConfigSource, cmdConfigSource)
			if err != nil {
				return nil, err
			}

			var sourcePlugins []config.Source

			configUri := bootstrapConfig.String(config.Plugin.Name)
			if configUri == "" {
				var sourcePlugin config.Source
				err = plugin.CreatePlugin(config.Plugin, &sourcePlugin, bootstrapConfig)
				if err != nil {
					return nil, err
				}
				sourcePlugins = append(sourcePlugins, sourcePlugin)
			} else {
				uris := strings.Split(configUri, ";")
				for _, u := range uris {
					if u != "" && uri.IsUriRegistered(config.Plugin.Name) {
						if p, err := uri.ParseUri(config.Plugin.Name, u); err != nil {
							return nil, fmt.Errorf("parse %q plugin URI %q failed, err = %v", config.Plugin.Name, u, err)
						} else if uriConfig, err := plugin.UriConfig(config.Plugin.Name, p); err != nil {
							return nil, err
						} else {
							var sourcePlugin config.Source
							if err := plugin.CreatePluginWithName(config.Plugin, uriConfig.String(declare.PluginTypeFlag(declare.PluginType(config.Plugin))), &sourcePlugin, uriConfig); err != nil {
								return nil, err
							}
							sourcePlugins = append(sourcePlugins, sourcePlugin)
						}
					}
				}
			}

			configSources := []config.Source{cmdDefaultConfigSource}
			configSources = append(configSources, sourcePlugins...)
			configSources = append(configSources, envConfigSource)
			configSources = append(configSources, cmdConfigSource)

			return config.NewConfig(bootstrapConfig.StringList(flagNamespace), configSources...)
		}
	}

	bootstrapConfig, err := bootstrapConfigPreparer(nil)
	if err != nil {
		log.Fatalf("[ERROR] Create bootstreap config failed, err = %+v", err)
	}

	if bootstrapConfig.Bool(flagVerbose) {
		declare.ShowUsage(nil, nil, true)
		os.Exit(2)
	}

	bootstrapPassword := bootstrapConfig.StringList(flagPassword)
	if len(bootstrapPassword) != 0 {
		bootstrapConfig = bootstrapConfig.WithPassword(bootstrapPassword...)
	}

	if configPreparer == nil {
		configPreparer = func(cfg config.Config) (config.Config, error) {
			return bootstrapConfig, nil
		}
	}

	defaultConfig, err := configPreparer(bootstrapConfig)
	if err != nil {
		log.Fatalf("[ERROR] Create config failed, err = %+v", err)
	}

	password := defaultConfig.StringList(flagPassword)
	if len(password) != 0 || len(bootstrapPassword) != 0 {
		defaultConfig = defaultConfig.WithPassword(append(bootstrapPassword, password...)...)
	}

	config.SetGlobalConfig(defaultConfig)

}

func populateFlags(source declare.ModifiableConfigSource, compactUsage bool,
	flagsToShow, flagsToHide []string) {
	declare.PopulatePluginFlags()
	if compactUsage {
		declare.PopulateAllFlags(source, compactFlags, nil)
	} else {
		declare.PopulateAllFlags(source, flagsToShow, flagsToHide)
	}
}
