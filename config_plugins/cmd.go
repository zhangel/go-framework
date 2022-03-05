package config_plugins

import (
	"flag"

	"github.com/zhangel/go-framework.git/config/watcher"
	"github.com/zhangel/go-framework.git/internal/declare"
)

type CmdConfigSource struct{}

func NewCmdConfigSource() *CmdConfigSource {
	return &CmdConfigSource{}
}

func (s *CmdConfigSource) Sync() (map[string]string, error) {
	val := map[string]string{}
	declare.FrameworkFlagSet.Visit(func(f *flag.Flag) {
		if f.Name == declare.FlagOverwrite {
			if overwriteFlags, ok := f.Value.(*declare.MapFlags); ok {
				for k, v := range *overwriteFlags {
					val[k] = v
				}
			}
		} else {
			val[f.Name] = f.Value.String()
		}
	})

	return val, nil
}

func (s *CmdConfigSource) Watch(watcher.SourceWatcher) (cancel func()) {
	return func() {}
}

func (s *CmdConfigSource) AppendPrefix(_ []string) error {
	return nil
}

func (s *CmdConfigSource) Close() error {
	return nil
}

func (s *CmdConfigSource) IgnoreNamespace() {}
