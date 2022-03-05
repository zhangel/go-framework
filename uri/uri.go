package uri

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

type UriFieldType byte

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

var (
	uriRegisterMap = make(map[string]UriParser)
	mu             sync.RWMutex
)

type UriField struct {
	typ UriFieldType
	v   string
	q   map[string][]string
}

func (s *UriField) Type() UriFieldType {
	return s.typ
}

func (s *UriField) Value() string {
	return s.v
}

func (s *UriField) Query() map[string][]string {
	if s.typ == UriFieldQuery {
		return s.q
	} else {
		return map[string][]string{}
	}
}

type UriParser interface {
	fmt.Stringer
	Key() string
	ParseField(field UriField, result map[string]string) error
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

func ParseUri(k, v string) (map[string]string, error) {
	mu.RLock()
	parser, ok := uriRegisterMap[k]
	if !ok {
		mu.RUnlock()
		return nil, fmt.Errorf("parseURI: no parser for URI %s found", k)
	}
	mu.RUnlock()

	if !strings.Contains(v, ":") {
		v += ":"
	}

	uri, err := url.ParseRequestURI(v)
	if err != nil {
		return nil, fmt.Errorf("parseURI: parse uri %q failed, err = %v", v, err)
	}

	result := make(map[string]string)
	parseField := func(typ UriFieldType, val string, query map[string][]string, result map[string]string) error {
		if err := parser.ParseField(UriField{typ: typ, v: val, q: query}, result); err != nil {
			if typ == UriFieldQuery {
				return fmt.Errorf("parseURIField: parse %q failed, err = %v", v, err)
			} else {
				return fmt.Errorf("parseURIField: parse %q failed, err = %v", v, err)
			}
		}
		return nil
	}

	if err := parseField(UriFieldScheme, uri.Scheme, nil, result); err != nil {
		return nil, err
	}

	if uri.User != nil {
		if err := parseField(UriFieldUsername, uri.User.Username(), nil, result); err != nil {
			return nil, err
		}

		if password, ok := uri.User.Password(); ok {
			if err := parseField(UriFieldPassword, password, nil, result); err != nil {
				return nil, err
			}
		}
	}

	if uri.Host != "" {
		if err := parseField(UriFieldHost, uri.Host, nil, result); err != nil {
			return nil, err
		}
	}

	if uri.Hostname() != "" {
		if err := parseField(UriFieldHostname, uri.Hostname(), nil, result); err != nil {
			return nil, err
		}
	}

	if uri.Port() != "" {
		if err := parseField(UriFieldPort, uri.Port(), nil, result); err != nil {
			return nil, err
		}
	}

	if uri.Path != "" {
		if err := parseField(UriFieldPath, uri.Path, nil, result); err != nil {
			return nil, err
		}
	}

	if len(uri.Query()) != 0 {
		if err := parseField(UriFieldQuery, "", uri.Query(), result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func IsUriRegistered(k string) bool {
	mu.RLock()
	defer mu.RUnlock()

	_, ok := uriRegisterMap[k]
	return ok
}

type fieldInfo struct {
	name         string
	valueHandler func(string) string
}

type SimpleUriParser struct {
	key      string
	selector string
	schemes  map[string]struct{}

	mapping map[string]map[UriFieldType]*fieldInfo
	query   map[string]map[string]struct{}
}

type _SimpleUriParserScope struct {
	*SimpleUriParser
	scheme string
}

func NewSimpleUriParser(key, selector string) *SimpleUriParser {
	p := &SimpleUriParser{
		key:      key,
		selector: selector,
		schemes:  make(map[string]struct{}),
		mapping:  make(map[string]map[UriFieldType]*fieldInfo),
		query:    make(map[string]map[string]struct{}),
	}

	return p
}

func (s *SimpleUriParser) WithScheme(scheme string) *_SimpleUriParserScope {
	s.schemes[scheme] = struct{}{}
	return &_SimpleUriParserScope{
		SimpleUriParser: s,
		scheme:          scheme,
	}
}

func (s *_SimpleUriParserScope) WithField(typ UriFieldType, to string, valueHandler func(string) string) *_SimpleUriParserScope {
	m, ok := s.SimpleUriParser.mapping[s.scheme]
	if !ok {
		m = make(map[UriFieldType]*fieldInfo)
		s.SimpleUriParser.mapping[s.scheme] = m
	}

	if valueHandler == nil {
		// 大部分逻辑对Path的处理都需要省略'/'，作为默认逻辑处理，如果不希望使用该逻辑，自行定义Handler覆盖
		if typ == UriFieldPath {
			valueHandler = FilePathHandler
		} else {
			valueHandler = func(v string) string { return v }
		}
	}

	m[typ] = &fieldInfo{to, valueHandler}
	return s
}

func (s *_SimpleUriParserScope) WithQuery(name string) *_SimpleUriParserScope {
	q, ok := s.SimpleUriParser.query[s.scheme]
	if !ok {
		q = make(map[string]struct{})
		s.SimpleUriParser.query[s.scheme] = q
	}

	q[name] = struct{}{}
	return s
}

func (s *SimpleUriParser) Key() string {
	return s.key
}

func (s *SimpleUriParser) ParseField(field UriField, result map[string]string) error {
	if field.typ == UriFieldScheme {
		_, ok := s.schemes[field.v]
		if !ok {
			return fmt.Errorf("no scheme %q registered", field.v)
		}
		result[s.key+"."+s.selector] = field.v
	} else if field.typ != UriFieldQuery {
		scheme := result[s.key+"."+s.selector]
		if f, ok := s.mapping[scheme][field.typ]; ok {
			result[s.key+"."+scheme+"."+f.name] = f.valueHandler(field.v)
		}
	} else {
		scheme := result[s.key+"."+s.selector]
		for k, v := range field.q {
			k = strings.ToLower(k)
			result[s.key+"."+scheme+"."+k] = strings.Join(v, ",")
		}
	}

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

func FilePathHandler(path string) string {
	if path == "" {
		return ""
	}

	if path[0] == '/' {
		return path[1:]
	} else {
		return path
	}
}
