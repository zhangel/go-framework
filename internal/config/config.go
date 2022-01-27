package config

import (
	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/di"
)

func PrepareConfigs(beforeConfigPrepareHook []func(),
	configPrepare func(config.Config) (config.Config, error),
	configSources []config.Source, compactUsage bool,
	flagsToShow, flagsToHide []string) {
	_ = di.GlobalDepInjector.Provide(func() config.Config {
		return config.GlobalConfig()
	}, false)
}
