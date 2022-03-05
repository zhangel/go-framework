package http_server

import (
	"net/http"

	http2 "github.com/zhangel/go-framework/http"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/registry"
	"github.com/zhangel/go-framework/server/internal/option"
	"github.com/zhangel/go-framework/utils"
)

type ServiceDesc struct {
	Route          Route
	Handler        http.Handler
	RegisterOption option.HttpRegisterOptions
}

type ServiceDescList []*ServiceDesc

func (s *ServiceDescList) RegisterServiceHandler(route Route, handler http.Handler, opt ...option.HttpRegisterOption) error {
	opts := &option.HttpRegisterOptions{}
	for _, o := range opt {
		if err := o(opts); err != nil {
			return err
		}
	}

	*s = append(*s, &ServiceDesc{
		Route:          route,
		Handler:        handler,
		RegisterOption: *opts,
	})

	return nil
}

func (s ServiceDescList) HttpRules() []*registry.HttpRule {
	var rules []*registry.HttpRule
	for _, sd := range s {
		if rule := sd.HttpRule(); rule != nil {
			rules = append(rules, rule)
		}
	}
	return rules
}

func (s *ServiceDesc) HttpRule() *registry.HttpRule {
	if s.Handler == nil {
		return nil
	}

	rule := &registry.HttpRule{}

	// TODO: 暂时不支持mux风格的正则，只进行全匹配和通配
	if s.Route.GetPath() != "" {
		rule.HttpPatterns = http2.RegistryWithPath(s.Route.GetMethods(), s.Route.GetPath())
	}

	// TODO: 暂时不支持mux风格的正则，只进行全匹配
	if s.Route.GetHost() != "" {
		hostHeaderMap := map[string]string{"Host": s.Route.GetHost()}
		rule.HeaderMap = append(rule.HeaderMap, hostHeaderMap)
	}

	if len(s.Route.GetHeaders()) != 0 {
		headerMap, err := utils.MapFromPairsToString(s.Route.GetHeaders()...)
		if err != nil {
			log.Fatalf("[ERROR], register http service failed, err = %v\n", err)
		}
		rule.HeaderMap = append(rule.HeaderMap, headerMap)
	}

	if len(s.Route.GetHeadersRegexp()) != 0 {
		headersRegexpMap, err := utils.MapFromPairsToString(s.Route.GetHeadersRegexp()...)
		if err != nil {
			log.Fatalf("[ERROR], register http service failed, err = %v\n", err)
		}
		rule.HeaderRegexMap = append(rule.HeaderRegexMap, headersRegexpMap)
	}

	return rule
}
