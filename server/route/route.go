package route

import (
	"github.com/gorilla/mux"

	"github.com/zhangel/go-framework.git/server"
	"github.com/zhangel/go-framework.git/server/internal/http_server"
)

func Path(tpl string) server.Route {
	return http_server.NewRouteOpt().Path(tpl)
}

func Headers(pairs ...string) server.Route {
	return http_server.NewRouteOpt().Headers(pairs...)
}

func HeadersRegexp(pairs ...string) server.Route {
	return http_server.NewRouteOpt().HeadersRegexp(pairs...)
}

func Host(tpl string) server.Route {
	return http_server.NewRouteOpt().Host(tpl)
}

func Methods(methods ...string) server.Route {
	return http_server.NewRouteOpt().Methods(methods...)
}

func PathPrefix(tpl string) server.Route {
	return http_server.NewRouteOpt().PathPrefix(tpl)
}

func Queries(pairs ...string) server.Route {
	return http_server.NewRouteOpt().Queries(pairs...)
}

func Schemes(schemes ...string) server.Route {
	return http_server.NewRouteOpt().Schemes(schemes...)
}

func MatcherFunc(f mux.MatcherFunc) server.Route {
	return http_server.NewRouteOpt().MatcherFunc(f)
}
