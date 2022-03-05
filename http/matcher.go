package http

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var variablePattern = regexp.MustCompile("(?U){(.+)}")

type EstimatableMetaPriority interface {
	Priority() int
}

type PathMatcher struct {
	root *PathSegment
	mu   sync.RWMutex
}

type PathSegment struct {
	meta     []interface{}
	children map[string]*PathSegment
}

func newPathSegment() *PathSegment {
	return &PathSegment{nil, map[string]*PathSegment{}}
}

func NewPathMatcher() *PathMatcher {
	return &PathMatcher{newPathSegment(), sync.RWMutex{}}
}

func (s *PathMatcher) AddPattern(method, pattern string, meta interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pattern, err := TemplateConvert(pattern)
	if err != nil {
		return err
	}

	components, err := s.components(method, pattern)
	if err != nil {
		return err
	}

	var node = s.root
	for _, p := range components {
		if len(p) == 0 {
			return fmt.Errorf("invalid pattern, component is empty string")
		}

		child, ok := node.children[p]
		if !ok {
			child = newPathSegment()
			node.children[p] = child
		}

		node = child
	}

	node.meta = append(node.meta, meta)
	return nil
}

// Deprecated: Use MatchWithMeta instead
func (s *PathMatcher) Match(method, path string) (bool, []interface{}) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if components, err := s.components(method, path); err != nil {
		return false, nil
	} else if result, ok := s.matchInternal(components, s.root); !ok {
		return false, nil
	} else {
		return true, result
	}
}

func (s *PathMatcher) MatchWithMeta(r *http.Request, estimateMeta func(r *http.Request, meta interface{}, count int) error) ([]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if components, err := s.components(r.Method, r.URL.Path); err != nil {
		return nil, err
	} else {
		if metas, ok := s.matchInternal(components, s.root); !ok {
			return nil, fmt.Errorf("no path pattern matched for %s%s", r.Method, r.URL.Path)
		} else {
			headerRuleCount := len(metas)
			if headerRuleCount > 1 {
				headerRuleCount = adjustHeaderRuleCount(metas)
			}
			result := make([]interface{}, 0, len(metas))

			var errs []error
			for _, meta := range metas {
				if err := estimateMeta(r, meta, headerRuleCount); err == nil {
					result = append(result, meta)
				} else {
					errs = append(errs, err)
				}
			}

			if len(result) > 0 {
				sort.SliceStable(result, func(i, j int) bool {
					metaI, okI := result[i].(EstimatableMetaPriority)
					metaJ, okJ := result[i].(EstimatableMetaPriority)

					if okI != okJ {
						return okI
					} else if !okI {
						return false
					}

					return metaI.Priority() < metaJ.Priority()
				})
				return result, nil
			} else {
				return nil, fmt.Errorf("%+v", errs)
			}
		}
	}
}

func (s *PathMatcher) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.root = newPathSegment()
}

func (s *PathMatcher) matchInternal(components []string, seg *PathSegment) ([]interface{}, bool) {
	if len(components) == 0 {
		if seg.meta != nil {
			return seg.meta, true
		} else {
			return nil, false
		}
	}

	component := components[0]
	if len(component) == 0 {
		return nil, false
	}

	node := seg.children[component]
	if node != nil {
		if meta, ok := s.matchInternal(components[1:], node); ok {
			return meta, true
		}
	}

	node = seg.children["*"]
	if node != nil {
		if meta, ok := s.matchInternal(components[1:], node); ok {
			return meta, true
		}
	}

	node = seg.children["**"]
	if node == nil {
		return nil, false
	}

	if len(node.children) > 0 {
		for i := 0; i < len(components); i++ {
			if _, ok := node.children[components[i]]; ok {
				if meta, ok := s.matchInternal(components[i:], node); ok {
					return meta, ok
				}
			} else if _, ok := node.children["*"]; ok {
				if meta, ok := s.matchInternal(components[i+1:], node); ok {
					return meta, ok
				}
			}
		}
	}

	if node.meta != nil {
		return node.meta, true
	} else {
		return nil, false
	}
}

func (s *PathMatcher) components(method, path string) ([]string, error) {
	components := strings.Split(strings.ToLower(path[1:]), "/")
	result := make([]string, 0, len(components)+2)
	if method != "" {
		result = append(result, strings.ToLower(method))
	}
	result = append(result, components...)
	components = result

	var verb string
	l := len(components)
	if idx := strings.LastIndex(components[l-1], ":"); idx == 0 {
		return nil, fmt.Errorf("invalide path format, found standalone verb segment")
	} else if idx > 0 {
		c := components[l-1]
		components[l-1], verb = c[:idx], c[idx+1:]
	}

	if verb != "" {
		components = append(components, ":"+verb)
	}

	return components, nil
}

func TemplateConvert(patterns string) (string, error) {
	idx := variablePattern.FindAllStringSubmatchIndex(patterns, -1)
	if idx == nil {
		return patterns, nil
	}
	result := strings.Builder{}
	offset := 0
	for _, idxPairs := range idx {
		if len(idxPairs) != 4 {
			continue
		}

		result.WriteString(patterns[offset:idxPairs[0]])
		if v := strings.Split(patterns[idxPairs[2]:idxPairs[3]], "="); len(v) == 2 {
			result.WriteString(v[1])
		} else if len(v) == 1 {
			result.WriteString("*")
		} else {
			return "", fmt.Errorf("invalid variable template format")
		}
		offset = idxPairs[1]
	}
	result.WriteString(patterns[offset:])
	patterns = result.String()
	return patterns, nil
}

func EstimateMetaChain(estimates ...func(*http.Request, interface{}, int) error) func(*http.Request, interface{}, int) error {
	return func(r *http.Request, meta interface{}, count int) error {
		for _, e := range estimates {
			if err := e(r, meta, count); err != nil {
				return err
			}
		}
		return nil
	}
}
