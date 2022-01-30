package config

import (
	"fmt"
	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/di"
	"github.com/zhangel/go-framework/internal/declare"
)

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
			populateFlags(source, compactUsage, flagsToShow, flagsToHide)
		}
	}
	fmt.Printf("debug\n")
}

func populateFlags(source declare.ModifiableConfigSource, compactUsage bool,
	flagsToShow, flagsToHide []string) {
	declare.PopulatePluginFlags()
}
