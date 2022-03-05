package http

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/zhangel/go-framework.git/utils"
)

const (
	XForwardedFor  = "X-Forwarded-For"
	XForwardedHost = "X-Forwarded-Host"

	Path       = "X-FRAME-PATH"
	Method     = "X-FRAME-METHOD"
	Host       = "X-FRAME-HOST"
	Scheme     = "X-FRAME-SCHEME"
	RequestURI = "X-FRAME-URI"
)

type EstimatableHeaderMeta interface {
	HeaderMap() []map[string]string
	HeaderRegexMap() []map[string]*regexp.Regexp
}

func EstimateHeader(r *http.Request, meta interface{}, count int) error {
	headerMeta, ok := meta.(EstimatableHeaderMeta)
	if !ok {
		return fmt.Errorf("estimate header failed, meta[%t] not implement EstimatableHeaderMeta interface", meta)
	}

	if count == 1 {
		return nil
	} else {
		if len(headerMeta.HeaderMap()) == 0 && len(headerMeta.HeaderRegexMap()) == 0 {
			return nil
		}

		r.Header["Host"] = []string{r.Host}
		for _, headerStringMap := range headerMeta.HeaderMap() {
			if utils.MatchHeadersWithString(headerStringMap, r.Header) {
				return nil
			}
		}

		for _, headerRegexMap := range headerMeta.HeaderRegexMap() {
			if utils.MatchHeadersWithRegex(headerRegexMap, r.Header) {
				return nil
			}
		}

		return fmt.Errorf("estimate header failed, accepted header = %+v, accepted regex header = %+v, request header = %+v", headerMeta.HeaderMap(), headerMeta.HeaderRegexMap(), r.Header)
	}
}

type mapPair struct {
	k string
	v string
}

func adjustHeaderRuleCount(metas []interface{}) int {
	result := 0
	headerMetaSet := make(map[string]struct{})
	for _, meta := range metas {
		headerMeta, ok := meta.(EstimatableHeaderMeta)
		if !ok {
			result += 1
		}

		keyBuilder := strings.Builder{}
		var headerPairs []*mapPair
		for _, h := range headerMeta.HeaderMap() {
			for k, v := range h {
				headerPairs = append(headerPairs, &mapPair{k: k, v: v})
			}
		}
		sort.SliceStable(headerPairs, func(i, j int) bool {
			return strings.Compare(headerPairs[i].k, headerPairs[j].k) < 0
		})

		for _, headerPair := range headerPairs {
			keyBuilder.WriteString(headerPair.k)
			keyBuilder.WriteString(",")
			keyBuilder.WriteString(headerPair.v)
			keyBuilder.WriteString(",")
		}
		keyBuilder.WriteString("|")

		var headerRegexPairs []*mapPair
		for _, r := range headerMeta.HeaderRegexMap() {
			for k, v := range r {
				headerRegexPairs = append(headerRegexPairs, &mapPair{k: k, v: v.String()})
			}
		}
		sort.SliceStable(headerRegexPairs, func(i, j int) bool {
			return strings.Compare(headerRegexPairs[i].k, headerRegexPairs[j].k) < 0
		})
		for _, headerRegexPair := range headerRegexPairs {
			keyBuilder.WriteString(headerRegexPair.k)
			keyBuilder.WriteString(",")
			keyBuilder.WriteString(headerRegexPair.v)
			keyBuilder.WriteString(",")
		}

		headerMetaSet[keyBuilder.String()] = struct{}{}
	}

	return result + len(headerMetaSet)
}
