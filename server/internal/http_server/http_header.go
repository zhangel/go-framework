package http_server

import (
	"context"
	"fmt"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc/metadata"

	framework_http "github.com/zhangel/go-framework/http"
)

const (
	StandardIncomingHeaderPrefix = "x-frame-incoming"
	StandardOutgoingHeaderPrefix = "x-frame-outgoing"
)

func requestCtxWithHeader(ctx context.Context, r *http.Request, grpcGatewayCompatible bool) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}

	md.Set(framework_http.Path, r.URL.Path)
	md.Set(framework_http.Method, r.Method)
	md.Set(framework_http.Host, r.URL.Host)
	md.Set(framework_http.RequestURI, r.RequestURI)
	if r.URL.Scheme != "" {
		md.Set(framework_http.Scheme, r.URL.Scheme)
	} else if r.TLS == nil {
		md.Set(framework_http.Scheme, "http://")
	} else {
		md.Set(framework_http.Scheme, "https://")
	}

	for k, v := range r.Header {
		if k, ok := requestHeaderFilter(k, grpcGatewayCompatible); ok {
			md.Set(k, v...)
		}
	}

	if h := r.Header.Get(framework_http.XForwardedFor); h == "" {
		md.Append(framework_http.XForwardedFor, r.RemoteAddr)
	}

	if h := r.Header.Get(framework_http.XForwardedHost); h == "" {
		md.Append(framework_http.XForwardedHost, r.Host)
	}

	return metadata.NewIncomingContext(ctx, md)
}

func requestHeaderFilter(key string, grpcGatewayCompatible bool) (string, bool) {
	key = textproto.CanonicalMIMEHeaderKey(key)

	if grpcGatewayCompatible {
		if isPermanentHTTPHeader(key) {
			key = runtime.MetadataPrefix + key
		} else if strings.HasPrefix(key, runtime.MetadataHeaderPrefix) {
			key = key[len(runtime.MetadataHeaderPrefix):]
		} else {
			return "", false
		}
	}

	if isHttp2Header(key) {
		key = StandardIncomingHeaderPrefix + key
	}

	return key, true
}

func isHttp2Header(hdr string) bool {
	switch strings.ToLower(hdr) {
	case
		":uri",
		":host",
		":authority",
		":path",
		"accept",
		"accept-charset",
		"accept-encoding",
		"accept-language",
		"accept-ranges",
		"age",
		"access-control-allow-origin",
		"allow",
		"authorization",
		"cache-control",
		"content-disposition",
		"content-encoding",
		"content-language",
		"content-length",
		"content-location",
		"content-range",
		"content-type",
		"cookie",
		"date",
		"etag",
		"expect",
		"expires",
		"from",
		"host",
		"if-match",
		"if-modified-since",
		"if-none-match",
		"if-unmodified-since",
		"last-modified",
		"link",
		"location",
		"max-forwards",
		"proxy-authenticate",
		"proxy-authorization",
		"range",
		"referer",
		"refresh",
		"retry-after",
		"server",
		"set-cookie",
		"strict-transport-security",
		"trailer",
		"transfer-encoding",
		"user-agent",
		"vary",
		"via",
		"www-authenticate":
		return true
	}
	return false
}

func isPermanentHTTPHeader(hdr string) bool {
	switch hdr {
	case
		"Accept",
		"Accept-Charset",
		"Accept-Language",
		"Accept-Ranges",
		"Authorization",
		"Cache-Control",
		"Content-Type",
		"Cookie",
		"Date",
		"Expect",
		"From",
		"Host",
		"If-Match",
		"If-Modified-Since",
		"If-None-Match",
		"If-Schedule-Tag-Match",
		"If-Unmodified-Since",
		"Max-Forwards",
		"Origin",
		"Pragma",
		"Referer",
		"User-Agent",
		"Via",
		"Warning":
		return true
	}
	return false
}

func writeResponseHeader(w http.ResponseWriter, header metadata.MD, grpcGatewayCompatible bool) {
	for k, v := range header {
		k = responseHeaderKey(k, grpcGatewayCompatible)
		if isHttp2Header(k) {
			continue
		}

		k = strings.TrimPrefix(k, StandardOutgoingHeaderPrefix)
		for _, v := range v {
			w.Header().Add(k, v)
		}
	}
}

func responseHeaderKey(key string, grpcGatewayCompatible bool) string {
	if grpcGatewayCompatible {
		key = fmt.Sprintf("%s%s", runtime.MetadataHeaderPrefix, key)
	}

	return key
}

func writeResponseTrailer(w http.ResponseWriter, trailer metadata.MD) {
	for k := range trailer {
		tKey := textproto.CanonicalMIMEHeaderKey(fmt.Sprintf("%s%s", runtime.MetadataTrailerPrefix, k))
		if isHttp2Header(tKey) {
			continue
		}

		w.Header().Add("Trailer", tKey)
	}

	for k, vs := range trailer {
		tKey := fmt.Sprintf("%s%s", runtime.MetadataTrailerPrefix, k)

		if isHttp2Header(k) {
			continue
		}

		for _, v := range vs {
			w.Header().Add(tKey, v)
		}
	}
}
