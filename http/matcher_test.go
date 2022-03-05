package http

import (
	"fmt"
	"sync"
	"testing"
)

type testingPattern struct {
	method  string
	pattern string
}
type testingMatches struct {
	method        string
	path          string
	shouldMatched bool
	meta          interface{}
}

func TestTemplateConvert(t *testing.T) {
	cases := []struct {
		in     string
		expect string
	}{
		{"/lanxin/staff/v1", "/lanxin/staff/v1"},
		{"/lanxin/staff/v1/*", "/lanxin/staff/v1/*"},
		{"/lanxin/staff/v1/**", "/lanxin/staff/v1/**"},
		{"/lanxin/staff/v1/*/staff/*", "/lanxin/staff/v1/*/staff/*"},
		{"/lanxin/staff/v1/{staff.id}/staff", "/lanxin/staff/v1/*/staff"},
		{"/lanxin/staff/v1/{org.id}/staff/{staff.id}", "/lanxin/staff/v1/*/staff/*"},
		{"/lanxin/staff/v1/{staff.id=staff/*}", "/lanxin/staff/v1/staff/*"},
		{"/lanxin/staff/v1/{staff.id=staff/*}/meta", "/lanxin/staff/v1/staff/*/meta"},
		{"/lanxin/staff/v1/{staff.id=staff/**}", "/lanxin/staff/v1/staff/**"},
		{"/lanxin/staff/v1/{staff.id=staff/**}:meta", "/lanxin/staff/v1/staff/**:meta"},
		{"/lanxin/staff/v1/{staff.id=staff/**}:meta", "/lanxin/staff/v1/staff/**:meta"},
		{"/lanxin/staff/v1/*/staff/*:meta", "/lanxin/staff/v1/*/staff/*:meta"},
		{"/lanxin/staff/v1/{org.id}/staff/{staff.id}:meta", "/lanxin/staff/v1/*/staff/*:meta"},
		{"/lanxin/staff/v1/{org.id=staff/**/meta}", "/lanxin/staff/v1/staff/**/meta"},
		{"/lanxin/staff/v1/{org.id=staff/**/meta}/test", "/lanxin/staff/v1/staff/**/meta/test"},
		{"/lanxin/staff/v1/{org.id=staff/**/meta}/**/test", "/lanxin/staff/v1/staff/**/meta/**/test"},
		{"/v1beta1/{parent=projects/*/databases/*/documents/*/**}/{collection_id}", "/v1beta1/projects/*/databases/*/documents/*/**/*"},
		{"/v1beta1/{parent=projects/*/databases/*/documents/*/**}/{collection_id}/*", "/v1beta1/projects/*/databases/*/documents/*/**/*/*"},
	}

	t.Parallel()
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			r, e := TemplateConvert(c.in)
			if r != c.expect || e != nil {
				t.Errorf("got = %s, expect = %s, err = %v", r, c.expect, e)
			}
		})
	}
}

func TestMatcher(t *testing.T) {
	cases := []struct {
		name     string
		patterns map[int]testingPattern
		matches  []testingMatches
	}{
		{
			"SimplePattern",
			map[int]testingPattern{
				1: {"GET", "/lanxin/staff/v1"},
				2: {"POST", "/lanxin/staff/v1"},
				3: {"*", "/lanxin/org/v1"},
			},
			[]testingMatches{
				{"GET", "/lanxin/staff/v1", true, 1},
				{"POST", "/lanxin/staff/v1", true, 2},
				{"DELETE", "/lanxin/staff/v1", false, nil},
				{"GET", "/lanxin/staff/v1/meta", false, nil},
				{"GET", "/lanxin/org/v1", true, 3},
				{"POST", "/lanxin/org/v1", true, 3},
				{"DELETE", "/lanxin/org/v1", true, 3},
			},
		},
		{
			"WildPattern",
			map[int]testingPattern{
				1: {"GET", "/lanxin/staff/v1/*"},
				2: {"GET", "/lanxin/org/v1/*/meta"},
				3: {"GET", "/lanxin/chat/v1/*/meta/*"},
				4: {"GET", "/lanxin/www/v1/**"},
			},
			[]testingMatches{
				{"GET", "/lanxin/staff/v1/meta", true, 1},
				{"GET", "/lanxin/staff/v1/info", true, 1},
				{"GET", "/lanxin/staff/v1/info/details", false, nil},
				{"GET", "/lanxin/org/v1/info/meta", true, 2},
				{"GET", "/lanxin/org/v1/child/meta", true, 2},
				{"GET", "/lanxin/org/v1/child/meta/details", false, nil},
				{"GET", "/lanxin/chat/v1/child/meta/details", true, 3},
				{"POST", "/lanxin/chat/v1/child/meta/details", false, nil},
				{"GET", "/lanxin/chat/v1/child/meta/details/additional", false, nil},
				{"GET", "/lanxin/www/v1/child/meta/details/additional/hahaha/blabla", true, 4},
			},
		},
		{
			"Verb",
			map[int]testingPattern{
				1: {"GET", "/lanxin/staff/v1/*:meta"},
				2: {"POST", "/lanxin/staff/v1/*:meta"},
				3: {"POST", "/lanxin/org/**:meta"},
				4: {"POST", "/lanxin/org/**/meta"},
				5: {"POST", "/lanxin/org/**/meta/**/end"},
			},
			[]testingMatches{
				{"GET", "/lanxin/staff/v1/info:meta", true, 1},
				{"GET", "/lanxin/staff/v1/child:meta", true, 1},
				{"POST", "/lanxin/staff/v1/info:meta", true, 2},
				{"POST", "/lanxin/org/v1/info:meta", true, 3},
				{"POST", "/lanxin/org/v2/info:meta", true, 3},
				{"POST", "/lanxin/org/v2/xxx/yyy/zzz/wtf:meta", true, 3},
				{"POST", "/lanxin/org/v2/info/meta", true, 4},
				{"POST", "/lanxin/org/v2/info/xxx/meta/123/4353/end", true, 5},
				{"POST", "/lanxin/org/v2/info/xxx/meta/123/4353/end1", false, nil},
			},
		},
		{
			"VariableTemplate",
			map[int]testingPattern{
				1:  {"GET", "/lanxin/staff/v1/{staff.id}"},
				2:  {"*", "/lanxin/staff/v2/{staff.id}/staff:meta"},
				3:  {"*", "/lanxin/staff/v3/{org.id}/staff/{staff.id}:meta"},
				4:  {"*", "/lanxin/staff/v4/{org.id}/{staff.id=staff/*}:meta"},
				5:  {"GET", "/lanxin/staff/v1/{org.id=staff/**/meta}/**"},
				6:  {"GET", "/lanxin/staff/v1/{org.id=staff/**/meta}/**/test"},
				7:  {"GET", "/lanxin/staff/v1/{org.id=staff/**/meta}/**/test2:dog"},
				8:  {"GET", "/v1beta1/{parent=projects/*/databases/*/documents/*/**}/{collection_id}"},
				9:  {"GET", "/v1beta1/{parent=projects/*/databases/*/documents/*/**}/{collection_id}/ok"},
				10: {"GET", "/v1beta1/{parent=projects/*/databases/*/documents/*/**}/{collection_id}/*"},
				11: {"GET", "/v1beta1/**/123"},
			},
			[]testingMatches{
				{"GET", "/lanxin/staff/v1/1.0", true, 1},
				{"GET", "/lanxin/staff/v2/1024/staff:meta", true, 2},
				{"POST", "/lanxin/staff/v2/wx/staff:meta", true, 2},
				{"POST", "/lanxin/staff/v3/hello/staff/wx:meta", true, 3},
				{"POST", "/lanxin/staff/v4/hello/staff/wx:meta", true, 4},
				{"GET", "/lanxin/staff/v1/staff/1234/5678/meta/hello/test", true, 6},
				{"GET", "/lanxin/staff/v1/staff/12345678/meta/1/2/test", true, 6},
				{"POST", "/lanxin/staff/v1/staff/1234/5678/meta/hello/test", false, nil},
				{"GET", "/lanxin/staff/v1/staff/12345678/meta/1/2/test2", true, 5},
				{"GET", "/lanxin/staff/v1/staff/1234/5678/meta/1/2/test/dog", true, 5},
				{"GET", "/lanxin/staff/v1/staff/1234/5678/meta/1/2/test2:dog", true, 7},
				{"GET", "/lanxin/staff/v1/staff/1234/5678/meta/1/2/test:dog", true, 5},
				{"GET", "/lanxin/staff/v1/staff/1234/5678/meta/1/2/test2:dog2", true, 5},
				{"GET", "/v1beta1/projects/p1/databases/d2/documents/d3/456/collectionId", true, 8},
				{"GET", "/v1beta1/projects/p1/databases/d2/documents/d3/123/456/collectionId", true, 10},
				{"GET", "/v1beta1/projects/p1/databases/d2/documents/d3/123/456/collectionId/ok", true, 9},
				{"GET", "/v1beta1/projects/p1/databases/d2/documents/d3/123/456/collectionId/ok1", true, 10},
				{"GET", "/v1beta1/12345/789", false, nil},
				{"GET", "/v1beta1/12345/789/123", true, 11},
			},
		},
		{
			"Priority",
			map[int]testingPattern{
				1: {"GET", "/a/b/c/d"},
				2: {"GET", "/*/b/c/d"},
				3: {"GET", "/a/*/c/d"},
				4: {"GET", "/a/b/*/d"},
				5: {"GET", "/a/b/c/*"},
				6: {"GET", "/a/*/*/*"},
				7: {"GET", "/a/*/*/d"},
				8: {"GET", "/a/**"},
			},
			[]testingMatches{
				{"GET", "/a/b/c/d", true, 1},
				{"GET", "/b/b/c/d", true, 2},
				{"GET", "/a/c/c/d", true, 3},
				{"GET", "/a/b/d/d", true, 4},
				{"GET", "/a/b/c/e", true, 5},
				{"GET", "/a/b/c/f", true, 5},
				{"GET", "/a/c/b/d", true, 7},
				{"GET", "/a/c/b/e", true, 6},
				{"GET", "/a/c/b/e/f", true, 8},
				{"GET", "/a/b/c/d/e", true, 8},
				{"GET", "/a/wtf/blabla", true, 8},
			},
		},
	}

	t.Parallel()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := NewPathMatcher()

			wg := sync.WaitGroup{}
			wg.Add(len(c.patterns))
			for _, p := range c.patterns {
				go func(pattern testingPattern) {
					defer wg.Done()
					if e := m.AddPattern(pattern.method, pattern.pattern, pattern.method+pattern.pattern); e != nil {
						t.Errorf("AddPattern failed, pattern = %s, e = %v", pattern.pattern, e)
					}
				}(p)
			}
			wg.Wait()

			for _, matches := range c.matches {
				ok, meta := m.Match(matches.method, matches.path)
				if ok != matches.shouldMatched {
					if matches.meta != nil {
						t.Errorf("Match failed, method = %s, path = %s, matched = %v, shouldMatched = %v, patternExpected = %s", matches.method, matches.path, ok, matches.shouldMatched, c.patterns[matches.meta.(int)].method+c.patterns[matches.meta.(int)].pattern)
					} else {
						t.Errorf("Match failed, method = %s, path = %s, matched = %v, shouldMatched = %v, patternExpected = nil", matches.method, matches.path, ok, matches.shouldMatched)
					}
				} else if ok && (len(meta) == 0 || meta[0] == nil || matches.meta == nil || meta[0] != c.patterns[matches.meta.(int)].method+c.patterns[matches.meta.(int)].pattern) {
					if matches.meta != nil {
						t.Errorf("Mismatched, method = %s, path = %s, matched = %v, shouldMatched = %v, patternExpected = %s, patternGot = %s", matches.method, matches.path, ok, matches.shouldMatched, c.patterns[matches.meta.(int)].method+c.patterns[matches.meta.(int)].pattern, meta[0])
					} else {
						t.Errorf("Mismatched, method = %s, path = %s, matched = %v, shouldMatched = %v, patternExpected = nil, patternGot = %s", matches.method, matches.path, ok, matches.shouldMatched, meta[0])
					}
				}
			}
		})
	}
}

func BenchmarkAddPattern(b *testing.B) {
	m := NewPathMatcher()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		pat := fmt.Sprintf("/lanxin/staff/v%d/{staff.id}/*", i)
		if e := m.AddPattern("GET", pat, pat); e != nil {
			b.Errorf("AddPattern failed, pattern = %s, e = %v", pat, e)
		}
	}
	b.StopTimer()
}

func BenchmarkMatcher(b *testing.B) {
	cases := []struct {
		name     string
		patterns map[int]testingPattern
		matches  []testingMatches
	}{
		{
			"VariableTemplate",
			map[int]testingPattern{
				1: {"GET", "/lanxin/staff/v1/{staff.id}"},
				2: {"*", "/lanxin/staff/v2/{staff.id}/staff:meta"},
				3: {"*", "/lanxin/staff/v3/{org.id}/staff/{staff.id}:meta"},
				4: {"*", "/lanxin/staff/v4/{org.id}/{staff.id=staff/*}:meta"},
			},
			[]testingMatches{
				{"GET", "/lanxin/staff/v1/1.0", true, 1},
				{"GET", "/lanxin/staff/v2/1024/staff:meta", true, 2},
				{"POST", "/lanxin/staff/v2/wx/staff:meta", true, 2},
				{"POST", "/lanxin/staff/v3/hello/staff/wx:meta", true, 3},
				{"POST", "/lanxin/staff/v3/hello/staff/wx/test:meta", false, nil},
				{"POST", "/lanxin/staff/v4/hello/staff/wx:meta", true, 4},
				{"GET", "/lanxin/staff/v10290/wx/staff", true, 5},
			},
		},
	}

	for _, c := range cases {
		m := NewPathMatcher()
		for _, p := range c.patterns {
			if e := m.AddPattern(p.method, p.pattern, p.method+p.pattern); e != nil {
				b.Errorf("AddPattern failed, pattern = %s, e = %v", p.pattern, e)
			}
		}

		for i := 0; i < 102400; i++ {
			pat := fmt.Sprintf("/lanxin/staff/v%d/{staff.id}/*", i)
			if e := m.AddPattern("GET", pat, "GET"+pat); e != nil {
				b.Errorf("AddPattern failed, pattern = %s, e = %v", pat, e)
			}
		}
		c.patterns[5] = testingPattern{"GET", "/lanxin/staff/v10290/{staff.id}/*"}

		for _, matches := range c.matches {
			b.Run(matches.method+matches.path, func(b *testing.B) {
				b.StartTimer()
				for i := 0; i < b.N; i++ {
					ok, meta := m.Match(matches.method, matches.path)
					if ok != matches.shouldMatched {
						if matches.meta != nil {
							b.Errorf("Match failed, method = %s, path = %s, matched = %v, shouldMatched = %v, patternExpected = %s", matches.method, matches.path, ok, matches.shouldMatched, c.patterns[matches.meta.(int)].method+c.patterns[matches.meta.(int)].pattern)
						} else {
							b.Errorf("Match failed, method = %s, path = %s, matched = %v, shouldMatched = %v, patternExpected = nil", matches.method, matches.path, ok, matches.shouldMatched)
						}
					} else if ok && (len(meta) == 0 || meta[0] == nil || matches.meta == nil || meta[0] != c.patterns[matches.meta.(int)].method+c.patterns[matches.meta.(int)].pattern) {
						if matches.meta != nil {
							b.Errorf("Mismatched, method = %s, path = %s, matched = %v, shouldMatched = %v, patternExpected = %s, patternGot = %s", matches.method, matches.path, ok, matches.shouldMatched, c.patterns[matches.meta.(int)].method+c.patterns[matches.meta.(int)].pattern, meta[0])
						} else {
							b.Errorf("Mismatched, method = %s, path = %s, matched = %v, shouldMatched = %v, patternExpected = nil, patternGot = %s", matches.method, matches.path, ok, matches.shouldMatched, meta[0])
						}
					}
				}
				b.StopTimer()
			})
		}
	}
}
