package http_server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type Route interface {
	fmt.Stringer

	Path(path string) Route
	GetPath() string

	Headers(pairs ...string) Route
	GetHeaders() []string

	HeadersRegexp(pairs ...string) Route
	GetHeadersRegexp() []string

	Host(tpl string) Route
	GetHost() string

	Methods(methods ...string) Route
	GetMethods() []string

	PathPrefix(pathPrefix string) Route
	GetPathPrefix() string

	Queries(pairs ...string) Route
	GetQueries() []string

	Schemes(schemes ...string) Route
	GetSchemes() []string

	MatcherFunc(f mux.MatcherFunc) Route
	GetMatcherFunc() mux.MatcherFunc
}

type RouteImpl struct {
	path          string
	headers       []string
	headersRegexp []string
	hostTpl       string
	methods       []string
	pathPrefix    string
	queries       []string
	schemes       []string
	matcher       mux.MatcherFunc
}

func NewRouteOpt() Route {
	return &RouteImpl{}
}

func (s *RouteImpl) Path(tpl string) Route {
	s.path = tpl
	return s
}

func (s *RouteImpl) GetPath() string {
	return s.path
}

func (s *RouteImpl) Headers(pairs ...string) Route {
	s.headers = pairs
	return s
}

func (s *RouteImpl) GetHeaders() []string {
	return s.headers
}

func (s *RouteImpl) HeadersRegexp(pairs ...string) Route {
	s.headersRegexp = pairs
	return s
}

func (s *RouteImpl) GetHeadersRegexp() []string {
	return s.headersRegexp
}

func (s *RouteImpl) Host(tpl string) Route {
	s.hostTpl = tpl
	return s
}

func (s *RouteImpl) GetHost() string {
	return s.hostTpl
}

func (s *RouteImpl) Methods(methods ...string) Route {
	s.methods = methods
	return s
}

func (s *RouteImpl) GetMethods() []string {
	return s.methods
}

func (s *RouteImpl) PathPrefix(tpl string) Route {
	s.pathPrefix = tpl
	return s
}

func (s *RouteImpl) GetPathPrefix() string {
	return s.pathPrefix
}

func (s *RouteImpl) Queries(pairs ...string) Route {
	s.queries = pairs
	return s
}

func (s *RouteImpl) GetQueries() []string {
	return s.queries
}

func (s *RouteImpl) Schemes(schemes ...string) Route {
	s.schemes = schemes
	return s
}

func (s *RouteImpl) GetSchemes() []string {
	return s.schemes
}

func (s *RouteImpl) MatcherFunc(f mux.MatcherFunc) Route {
	s.matcher = f
	return s
}

func (s *RouteImpl) GetMatcherFunc() mux.MatcherFunc {
	return s.matcher
}

func (s *RouteImpl) String() string {
	methods := "*"
	if len(s.methods) > 0 {
		methods = strings.Join(s.methods, ",")
	}

	path := s.path
	if path == "" {
		if s.pathPrefix != "" {
			path = s.pathPrefix + "*"
		} else {
			path = "*"
		}
	}

	return fmt.Sprintf("%s Path: %s", methods, path)
}

func apply(router *mux.Router, route Route, handler http.Handler) error {
	if router == nil {
		return fmt.Errorf("apply http route rules failed, router is nil")
	}

	r := router.NewRoute()
	if route.GetPath() != "" {
		r = r.Path(route.GetPath())
		if r.GetError() != nil {
			return r.GetError()
		}
	}

	if len(route.GetHeaders()) > 0 {
		r = r.Headers(route.GetHeaders()...)
		if r.GetError() != nil {
			return r.GetError()
		}
	}

	if len(route.GetHeadersRegexp()) > 0 {
		r = r.HeadersRegexp(route.GetHeadersRegexp()...)
		if r.GetError() != nil {
			return r.GetError()
		}
	}

	if route.GetHost() != "" {
		r = r.Host(route.GetHost())
		if r.GetError() != nil {
			return r.GetError()
		}
	}

	if len(route.GetMethods()) > 0 {
		r = r.Methods(route.GetMethods()...)
		if r.GetError() != nil {
			return r.GetError()
		}
	}

	if route.GetPathPrefix() != "" {
		r = r.PathPrefix(route.GetPathPrefix())
		if r.GetError() != nil {
			return r.GetError()
		}
	}

	if len(route.GetQueries()) > 0 {
		r = r.Queries(route.GetQueries()...)
		if r.GetError() != nil {
			return r.GetError()
		}
	}

	if len(route.GetSchemes()) > 0 {
		r = r.Schemes(route.GetSchemes()...)
		if r.GetError() != nil {
			return r.GetError()
		}
	}

	if route.GetMatcherFunc() != nil {
		r = r.MatcherFunc(route.GetMatcherFunc())
		if r.GetError() != nil {
			return r.GetError()
		}
	}

	r.Handler(handler)
	return nil
}
