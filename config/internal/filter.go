package internal

import (
	"strings"
	"sync"

	"github.com/zhangel/go-framework/lib"
)

type Filter struct {
	source    DataSource
	prefix    string
	ns        []string
	password  [][16]byte
	decrypted sync.Map
	prefixMap map[string]string
}

func NewFilter(source DataSource, prefix string, ns []string, password [][16]byte) *Filter {
	nsHelper := &Filter{
		source:    source,
		prefix:    prefix,
		password:  password,
		prefixMap: make(map[string]string, len(ns)+1),
	}

	hasEmptyNs := false
	for _, n := range ns {
		if n == "" {
			hasEmptyNs = true
			break
		}
	}

	if !hasEmptyNs {
		nsHelper.ns = append([]string{""}, ns...)
	} else {
		nsHelper.ns = ns
	}

	for _, ns := range nsHelper.ns {
		prefix := ""

		if nsHelper.prefix != "" {
			prefix = nsHelper.prefix + "."
		}

		if ns != "" {
			prefix = ns + "." + prefix
		}

		nsHelper.prefixMap[ns] = prefix
	}

	return nsHelper
}

func (s *Filter) Get(k string) (string, bool) {
	rv, rvIdx := s.source.Get(k, "", true)
	v, idx := "", -1

	ns := s.namespace()
	for i := len(ns) - 1; i >= 0; i-- {
		if v1, idx1 := s.source.Get(k, s.getPrefix(ns[i]), false); idx1 > idx {
			v, idx = v1, idx1
		}
	}

	if rvIdx == -1 && idx == -1 {
		return "", false
	}

	if rvIdx > idx {
		return s.decrypt(rv), true
	} else {
		return s.decrypt(v), true
	}
}

func (s *Filter) GetByPrefix(prefix string) map[string]string {
	return s.source.GetByPrefix(prefix)
}

func (s *Filter) FilterUpdate(notify map[string]string) map[string]string {
	result := map[string]string{}

	ns := s.namespace()
	for k, v := range notify {
		for i := len(ns) - 1; i >= 0; i-- {
			if strings.HasPrefix(k, s.getPrefix(ns[i])) {
				k = strings.TrimPrefix(k, s.getPrefix(ns[i]))
				dec_v := s.decrypt(v)
				if val, ok := s.Get(k); !ok || val == dec_v {
					result[k] = dec_v
				}
				break
			}
		}
	}

	return result
}

func (s *Filter) FilterDelete(notify []string) (map[string]string, []string) {
	updateNotify := map[string]string{}
	var deleteNotify []string

	ns := s.namespace()
	for _, k := range notify {
		for i := len(ns) - 1; i >= 0; i-- {
			if strings.HasPrefix(k, s.getPrefix(ns[i])) {
				k = strings.TrimPrefix(k, s.getPrefix(ns[i]))
				if val, ok := s.Get(k); ok {
					updateNotify[k] = val
				} else {
					deleteNotify = append(deleteNotify, k)
				}
				break
			}
		}
	}

	return updateNotify, deleteNotify
}

func (s *Filter) getPrefix(ns string) string {
	return s.prefixMap[ns]
}

func (s *Filter) namespace() []string {
	return s.ns
}

func (s *Filter) decrypt(val string) string {
	if !strings.HasPrefix(val, lib.EncryptedPrefix) {
		return val
	}

	if val, ok := s.decrypted.Load(val); ok {
		return val.(string)
	}

	for _, p := range append(s.password, lib.K) {
		if v, err := lib.Decrypt(p, val); err == nil {
			s.decrypted.Store(val, v)
			return v
		}
	}

	s.decrypted.Store(val, val)
	return val
}
