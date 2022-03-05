package http_server

import (
	"strings"
	"time"

	"github.com/zhangel/go-framework/log"

	"github.com/rs/cors"
)

type CORSLogger struct {
	enableLog bool
}

func (s CORSLogger) Printf(format string, args ...interface{}) {
	if s.enableLog {
		log.Infof(format, args...)
	}
}

type wildcard struct {
	prefix string
	suffix string
}

func (w wildcard) match(s string) bool {
	return len(s) >= len(w.prefix)+len(w.suffix) && strings.HasPrefix(s, w.prefix) && strings.HasSuffix(s, w.suffix)
}

type CorsHandler struct {
	*cors.Cors
	allowedOrigins    []string
	allowedWOrigins   []wildcard
	allowedOriginsAll bool
}

func newCorsHandler(allowedOrigins, allowedRequestHeaders, allowedMethods []string, enableLog bool) *CorsHandler {
	corsHandler := &CorsHandler{
		Cors: cors.New(cors.Options{
			AllowedOrigins:   allowedOrigins,
			AllowedHeaders:   allowedRequestHeaders,
			AllowedMethods:   allowedMethods,
			ExposedHeaders:   nil,
			AllowCredentials: true,
			MaxAge:           int(10 * time.Minute / time.Second),
			Debug:            false,
		}),
	}
	corsHandler.Log = CORSLogger{enableLog}

	if len(allowedOrigins) == 0 {
		corsHandler.allowedOriginsAll = true
	} else {
		corsHandler.allowedOrigins = []string{}
		corsHandler.allowedWOrigins = []wildcard{}
		for _, origin := range allowedOrigins {
			origin = strings.ToLower(origin)
			if origin == "*" {
				corsHandler.allowedOriginsAll = true
				corsHandler.allowedOrigins = nil
				corsHandler.allowedWOrigins = nil
				break
			} else if i := strings.IndexByte(origin, '*'); i >= 0 {
				w := wildcard{origin[0:i], origin[i+1:]}
				corsHandler.allowedWOrigins = append(corsHandler.allowedWOrigins, w)
			} else {
				corsHandler.allowedOrigins = append(corsHandler.allowedOrigins, origin)
			}
		}
	}
	return corsHandler
}

func (c *CorsHandler) isOriginAllowed(origin string) bool {
	if c.allowedOriginsAll {
		return true
	}
	origin = strings.ToLower(origin)
	for _, o := range c.allowedOrigins {
		if o == origin {
			return true
		}
	}
	for _, w := range c.allowedWOrigins {
		if w.match(origin) {
			return true
		}
	}
	return false
}
