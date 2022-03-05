package service

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/zhangel/go-framework.git/config"
	"github.com/zhangel/go-framework.git/db"
	"github.com/zhangel/go-framework.git/log"
	"github.com/zhangel/go-framework.git/log/logger"
	"github.com/zhangel/go-framework.git/plugin"
)

type Provider interface {
	ServiceName() string
	Logger() logger.Logger
	Config() config.Config
	DefaultDB() func() (ns, alias string, err error)
	SelectDB(alias string) func() (ns, alias string, err error)
}

type providerSetter interface {
	Provider
	setServiceName(string)
	setLogger(logger.Logger)
	setConfig(config.Config)
}

type DefaultProviderImpl struct {
	serviceName string
	logger      logger.Logger
	config      config.Config
}

var (
	providerMapMutex sync.RWMutex
	providerMap      = map[string]*DefaultProviderImpl{}
)

func CreateDefaultProvider(serviceName string, configNs []string) *DefaultProviderImpl {
	providerKey := serviceName + "|" + strings.Join(configNs, "|")

	log.Infof("CreateDefaultProvider for %q, config ns = %v", serviceName, configNs)

	providerMapMutex.RLock()
	if provider, ok := providerMap[providerKey]; ok {
		providerMapMutex.RUnlock()
		return provider
	}
	providerMapMutex.RUnlock()

	providerMapMutex.Lock()
	defer providerMapMutex.Unlock()

	if provider, ok := providerMap[providerKey]; ok {
		return provider
	}

	provider := &DefaultProviderImpl{}
	provider.setServiceName(serviceName)
	provider.setConfig(config.WithNamespace(configNs...))

	if plugin.HasPlugin(db.Plugin) {
		if e := plugin.InvokePlugin(db.Plugin, provider.Config(), &db.ExtraParameter{
			Ns: serviceName,
		}); e != nil {
			log.Fatalf("CreateDefaultProvider failed, err = create db plugin failed, err= %v", e)
		}
	}

	providerMap[providerKey] = provider
	return provider
}

func InitProvider(srvHandler Provider, initializer func() (Provider, error)) error {
	if initializer == nil {
		return fmt.Errorf("init service provider failed, initializer is nil")
	}

	provider, err := initializer()
	if err != nil {
		return fmt.Errorf("init service provider failed, err = %+v", err)
	}

	setter, ok := srvHandler.(providerSetter)
	if ok && setter != nil {
		setter.setServiceName(provider.ServiceName())
		setter.setConfig(provider.Config())
		setter.setLogger(provider.Logger())
	} else {
		// Only for legacy codes compatible
		serverModuleType := reflect.TypeOf((*Provider)(nil)).Elem()
		serverModuleVal := reflect.ValueOf(srvHandler).Elem()
		field := serverModuleVal.FieldByName("ServiceProvider")
		if field.Type().Kind() == reflect.Interface && field.Type().Implements(serverModuleType) && field.IsNil() {
			field.Set(reflect.ValueOf(provider))
		}
	}
	return nil
}

func (s *DefaultProviderImpl) ServiceName() string {
	return s.serviceName
}

func (s *DefaultProviderImpl) Logger() logger.Logger {
	if s.logger == nil {
		return log.WithField("tag", s.serviceName)
	} else {
		return s.logger
	}
}

func (s *DefaultProviderImpl) Config() config.Config {
	return s.config
}

func (s *DefaultProviderImpl) DefaultDB() func() (string, string, error) {
	aliasList := db.GlobalDatabaseConfig().AliasInNamespace(s.serviceName)

	if len(aliasList) == 1 {
		return func() (string, string, error) { return s.serviceName, aliasList[0], nil }
	} else if len(aliasList) == 0 {
		return func() (string, string, error) {
			return s.serviceName, "", fmt.Errorf("no db alias found for %q", s.serviceName)
		}
	} else {
		return func() (string, string, error) {
			for _, a := range aliasList {
				if a == s.serviceName {
					return s.serviceName, a, nil
				}
			}
			return s.serviceName, "", fmt.Errorf("%q has multiple db alias %+v, use SelectDB choice one", s.serviceName, aliasList)
		}
	}
}

func (s *DefaultProviderImpl) SelectDB(alias string) func() (string, string, error) {
	aliasList := db.GlobalDatabaseConfig().AliasInNamespace(s.serviceName)

	for _, a := range aliasList {
		if a == alias {
			return func() (string, string, error) { return s.serviceName, a, nil }
		}
	}

	return func() (string, string, error) {
		return s.serviceName, "", fmt.Errorf("no alias %q found in %q %v", alias, s.serviceName, aliasList)
	}
}

func (s *DefaultProviderImpl) setServiceName(serviceName string) {
	s.serviceName = serviceName
}

func (s *DefaultProviderImpl) setLogger(logger logger.Logger) {
	s.logger = logger
}

func (s *DefaultProviderImpl) setConfig(config config.Config) {
	s.config = config
}
