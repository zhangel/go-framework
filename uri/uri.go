package uri

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

var (
	uriRegisterMap = make(map[string]UriParser)
	mu             sync.RWMutex
)

const (
	UriFieldNone UriFieldType = iota
	UriFieldScheme
	UriFieldUsername
	UriFieldPassword
	UriFieldHost
	UriFieldHostname
	UriFieldPort
	UriFieldPath
	UriFieldQuery
)

type UriFieldType byte

type UriField struct {
	typ UriFieldType
	v   string
	q   map[string][]string
}

type SimpleUriParser struct {
	key      string
	selector string
	schemes  map[string]struct{}
	mapping  map[string]map[UriFieldType]*fieldInfo
	query    map[string]map[string]struct{}
}

type fieldInfo struct {
	name         string
	valueHandler func(string) string
}

type UriParser interface {
	fmt.Stringer
	Key() string
	ParseField(field UriField, result map[string]string) error
}

func (s *SimpleUriParser) Key() string {
	return s.key
}

func (s *SimpleUriParser) ParseField(field UriField, result map[string]string) error {
	return nil
}

func (s *SimpleUriParser) String() string {
	fieldName := func(scheme string, typ UriFieldType) string {
		if typ == UriFieldScheme {
			return scheme
		} else {
			if n, ok := s.mapping[scheme][typ]; ok {
				if s.mapping[scheme][UriFieldHost] == nil && typ == UriFieldPath {
					return fmt.Sprintf("/{%s.%s.%s}", s.key, scheme, n.name)
				} else {
					return fmt.Sprintf("{%s.%s.%s}", s.key, scheme, n.name)
				}
			} else {
				return ""
			}
		}
	}
	queries := func(scheme string) string {
		query := make([]string, 0, len(s.query))
		if _, ok := s.query[scheme]; ok {
			for q := range s.query[scheme] {
				query = append(query, fmt.Sprintf("%s={%s.%s.%s}", q, s.key, scheme, q))
			}
		}
		return strings.Join(query, "&")
	}
	result := make([]string, 0, len(s.schemes))
	for scheme := range s.schemes {
		uri := url.URL{
			Scheme:   fieldName(scheme, UriFieldScheme),
			Host:     fieldName(scheme, UriFieldHost),
			Path:     fieldName(scheme, UriFieldPath),
			RawQuery: queries(scheme),
		}

		if username := fieldName(scheme, UriFieldUsername); username != "" {
			if password := fieldName(scheme, UriFieldPassword); password != "" {
				uri.User = url.UserPassword(username, password)
			} else {
				uri.User = url.User(username)
			}
		}
		if uri.Host == "" && uri.Path == "" && uri.RawQuery == "" && uri.User == nil {
			result = append(result, fmt.Sprintf("%s://", uri.Scheme))
		} else {
			x, _ := url.QueryUnescape(uri.String())
			result = append(result, x)
		}
	}
	return strings.Join(result, "\n")
}

func RegisterUri(parser UriParser) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := uriRegisterMap[parser.Key()]; ok {
		return fmt.Errorf("registerURI: parser of URI %s has been registered", parser.Key())
	}
	uriRegisterMap[parser.Key()] = parser
	return nil
}
