package plugin

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/config/plugin"
	"github.com/zhangel/go-framework/config_plugins"
	"github.com/zhangel/go-framework/declare"
	"github.com/zhangel/go-framework/di"
	declare_internal "github.com/zhangel/go-framework/internal/declare"
	"github.com/zhangel/go-framework/uri"
	"github.com/zhangel/go-framework/utils"
)

func PluginInfoByType(pluginType declare.PluginType) map[string]*declare.PluginInfoWithFlags {
	return declare.PluginInfoByType(pluginType)
}

func desensitization(uri string, args map[string]string) (string, map[string]string) {
	uriResult := uri
	argsResult := args

	if uri, err := url.ParseRequestURI(uri); err == nil && uri != nil && uri.User != nil {
		uriResult = strings.Replace(uriResult, uri.User.Username(), strings.Repeat("*", len(uri.User.Username())), -1)
		for k, v := range args {
			if v == uri.User.Username() {
				argsResult[k] = strings.Repeat("*", len(uri.User.Username()))
			}
		}
		if password, ok := uri.User.Password(); ok {
			uriResult = strings.Replace(uriResult, password, strings.Repeat("*", len(password)), -1)
			for k, v := range args {
				if v == password {
					argsResult[k] = strings.Repeat("*", len(password))
				}
			}
		}
	}

	for k, v := range args {
		if declare_internal.IsSensitive(k) {
			argsResult[k] = strings.Repeat("*", len(v))
			uriResult = strings.Replace(uriResult, v, strings.Repeat("*", len(v)), -1)
		}
	}
	return uriResult, argsResult
}

func CreatePlugin(pluginType declare.PluginType, ptrPlugin interface{}, extra ...interface{}) error {
	return di.GlobalDepInjector.Invoke(func(cfg plugin.Config) error {
		if hasUriPlugin, err := enumUriPluginConfig(pluginType, cfg, false, func(uriConfig config.Config, uri string, args map[string]string) error {
			desensitiveUri, desensitiveArgs := desensitization(uri, args)
			log.Printf("Create %q plugin with uri string %q, args = %v\n", pluginType.Name, desensitiveUri, desensitiveArgs)
			return CreatePlugin(pluginType, ptrPlugin, replaceConfig(extra, cfg, uriConfig)...)
		}); hasUriPlugin {
			return err
		}

		if typ := cfg.String(declare_internal.PluginTypeFlag(declare_internal.PluginType(pluginType))); typ == "" {
			return nil
		} else {
			return CreatePluginWithName(pluginType, typ, ptrPlugin, extra...)
		}
	}, extra...)
}

func CreatePluginWithUri(pluginType declare.PluginType, uri string, ptrPlugin interface{}) error {
	if hasUri, err := parsePluginUris(pluginType, []string{uri}, func(uriConfig config.Config, uri string, args map[string]string) error {
		desensitiveUri, desensitiveArgs := desensitization(uri, args)
		log.Printf("Create %q plugin with uri string %q, args = %v\n", pluginType.Name, desensitiveUri, desensitiveArgs)
		return CreatePlugin(pluginType, ptrPlugin, replaceConfig(nil, nil, uriConfig)...)
	}); err != nil {
		return err
	} else if !hasUri {
		return fmt.Errorf("no plugin type %q registered for pluginUri %s", pluginType.Name, uri)
	} else {
		return nil
	}
}

func CreatePluginWithName(pluginType declare.PluginType, pluginName string, ptrPlugin interface{}, extra ...interface{}) error {
	return di.GlobalDepInjector.Invoke(func(config plugin.Config) error {
		defer utils.TimeoutGuard(30*time.Second, fmt.Sprintf("Create plugin [%s]", pluginType.Name+"."+pluginName))()

		if creator, err := pluginCreator(pluginType, pluginName); err != nil {
			return err
		} else {
			return creator.Create(ptrPlugin, replaceConfig(extra, config, ConfigForPluginInit(config, pluginType, pluginName))...)
		}
	}, extra...)
}

func InvokePlugin(pluginType declare.PluginType, extra ...interface{}) error {
	return di.GlobalDepInjector.Invoke(func(cfg plugin.Config) error {
		if hasUriPlugin, err := enumUriPluginConfig(pluginType, cfg, true, func(uriConfig config.Config, uri string, args map[string]string) error {
			desensitiveUri, desensitiveArgs := desensitization(uri, args)
			log.Printf("Invoke %q plugin with uri string %q, args = %v\n", pluginType.Name, desensitiveUri, desensitiveArgs)
			return InvokePlugin(pluginType, replaceConfig(extra, cfg, uriConfig)...)
		}); hasUriPlugin {
			return err
		}

		if typ := cfg.String(declare_internal.PluginTypeFlag(declare_internal.PluginType(pluginType))); typ == "" {
			return nil
		} else {
			return InvokePluginWithName(pluginType, typ, extra...)
		}
	}, extra...)
}

func InvokePluginWithUri(pluginType declare.PluginType, uri string) error {
	if hasUri, err := parsePluginUris(pluginType, []string{uri}, func(uriConfig config.Config, uri string, args map[string]string) error {
		desensitiveUri, desensitiveArgs := desensitization(uri, args)
		log.Printf("Invoke %q plugin with uri string %q, args = %v\n", pluginType.Name, desensitiveUri, desensitiveArgs)
		return InvokePlugin(pluginType, replaceConfig(nil, nil, uriConfig)...)
	}); err != nil {
		return err
	} else if !hasUri {
		return fmt.Errorf("no plugin type %q registered for pluginUri %s", pluginType.Name, uri)
	} else {
		return nil
	}
}

func InvokePluginWithName(pluginType declare.PluginType, pluginName string, extra ...interface{}) error {
	return di.GlobalDepInjector.Invoke(func(config plugin.Config) error {
		defer utils.TimeoutGuard(30*time.Second, fmt.Sprintf("Invoke plugin [%s]", pluginType.Name+"."+pluginName))()

		creator, err := pluginCreator(pluginType, pluginName)
		if err != nil {
			return err
		}

		invoker := func(e error) {
			err = e
		}

		if e := creator.Invoke(invoker, replaceConfig(extra, config, ConfigForPluginInit(config, pluginType, pluginName))...); e != nil {
			return e
		} else {
			return err
		}
	}, extra...)
}

func ConfigForPluginInit(cfg config.Config, pluginType declare.PluginType, pluginName string) config.Config {
	return cfg.WithPrefix(pluginType.Name + "." + pluginName)
}

func pluginCreator(pluginType declare.PluginType, pluginName string) (di.DepInjector, error) {
	if creators, ok := declare_internal.PluginCreators[declare_internal.PluginType(pluginType)]; !ok {
		return nil, fmt.Errorf("'%s.%s' plugin not found", pluginType.Name, pluginName)
	} else if creator, ok := creators[pluginName]; !ok {
		return nil, fmt.Errorf("'%s.%s' plugin not found", pluginType.Name, pluginName)
	} else {
		return creator, nil
	}
}

func enumUriPluginConfig(pluginType declare.PluginType, cfg config.Config, multipleUri bool, uriCallback func(uriConfig config.Config, uri string, args map[string]string) error) (bool, error) {
	var pluginUris []string
	if multipleUri {
		pluginUris = strings.Split(cfg.String(pluginType.Name), ";")
	} else {
		pluginUris = []string{cfg.String(pluginType.Name)}
	}

	return parsePluginUris(pluginType, pluginUris, uriCallback)
}

func parsePluginUris(pluginType declare.PluginType, pluginUris []string, uriCallback func(uriConfig config.Config, uri string, args map[string]string) error) (bool, error) {
	hasUriPlugin := false
	for _, pluginUri := range pluginUris {
		if pluginUri != "" && uri.IsUriRegistered(pluginType.Name) {
			hasUriPlugin = true
			if p, err := uri.ParseUri(pluginType.Name, pluginUri); err != nil {
				return hasUriPlugin, fmt.Errorf("parse %q plugin URI %q failed, err = %v", pluginType.Name, pluginUri, err)
			} else if uriConfig, err := UriConfig(pluginType.Name, p); err != nil {
				return hasUriPlugin, err
			} else if err := uriCallback(uriConfig, pluginUri, p); err != nil {
				return hasUriPlugin, err
			}
		} else if pluginUri != "" {
			log.Printf("No plugin type %q registered for pluginUri %s", pluginType.Name, pluginUri)
		}
	}
	return hasUriPlugin, nil
}

func UriConfig(pluginType string, p map[string]string) (config.Config, error) {
	uriParams := make(map[string]interface{}, len(p))
	for k, v := range p {
		uriParams[k] = v
	}
	uriParams[pluginType] = ""

	uriConfig, err := config.NewConfig([]string{""}, config_plugins.NewCmdDefaultConfigSource(), config_plugins.NewEnvConfigSource(), config_plugins.NewCmdConfigSource(), config_plugins.NewMemoryConfigSource(uriParams))
	if err != nil {
		return nil, fmt.Errorf("create URI info config failed, err = %v", err)
	}

	return uriConfig, nil
}

func HasPlugin(pluginType declare.PluginType) bool {
	return len(declare.PluginInfoByType(pluginType)) > 0
}

func replaceConfig(extra []interface{}, old, new config.Config) []interface{} {
	args := make([]interface{}, 0, len(extra))
	for _, e := range extra {
		if e == old {
			args = append(args, new)
		} else {
			args = append(args, e)
		}
	}
	if len(args) == 0 {
		args = append(args, new)
	}
	return args
}
