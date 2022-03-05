package internal

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/xhit/go-str2duration/v2"
)

type ValueImpl struct {
	ValueGetter
}

func NewValue(getter ValueGetter) *ValueImpl {
	return &ValueImpl{getter}
}

func (s *ValueImpl) Bool(key string) bool {
	v := strings.ToLower(s.String(key))
	return v == "true" || v == "1"
}

func (s *ValueImpl) Int(key string) int {
	v := s.String(key)
	if n, err := strconv.ParseInt(v, 10, 0); err == nil {
		return int(n)
	} else {
		return 0
	}
}

func (s *ValueImpl) Uint(key string) uint {
	v := s.String(key)
	if n, err := strconv.ParseUint(v, 10, 0); err == nil {
		return uint(n)
	} else {
		return 0
	}
}

func (s *ValueImpl) Int64(key string) int64 {
	v := s.String(key)
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		return n
	} else {
		return 0
	}
}

func (s *ValueImpl) Uint64(key string) uint64 {
	v := s.String(key)
	if n, err := strconv.ParseUint(v, 10, 64); err == nil {
		return n
	} else {
		return 0
	}
}

func (s *ValueImpl) Float64(key string) float64 {
	v := s.String(key)
	if n, err := strconv.ParseFloat(v, 64); err == nil {
		return n
	} else {
		return 0
	}
}

func (s *ValueImpl) Duration(key string) time.Duration {
	if v := s.String(key); len(v) > 0 {
		if duration, err := str2duration.ParseDuration(v); err != nil {
			log.Printf("parse duration config %q = %q failed, err = %v", key, v, err)
			return 0
		} else {
			return duration
		}
	}

	return 0
}

func (s *ValueImpl) Bytes(key string) []byte {
	if v, ok := s.Get(key); ok {
		return []byte(v)
	} else {
		return []byte{}
	}
}

func (s *ValueImpl) String(key string) string {
	if v, ok := s.Get(key); ok {
		return v
	} else {
		return ""
	}
}

func (s *ValueImpl) IntList(key string) []int {
	var v []int
	for _, sv := range s.StringList(key) {
		if n, err := strconv.ParseInt(sv, 10, 0); err == nil {
			v = append(v, int(n))
		}
	}
	return v
}

func (s *ValueImpl) UintList(key string) []uint {
	var v []uint
	for _, sv := range s.StringList(key) {
		if n, err := strconv.ParseUint(sv, 10, 0); err == nil {
			v = append(v, uint(n))
		}
	}
	return v
}

func (s *ValueImpl) Int64List(key string) []int64 {
	var v []int64
	for _, sv := range s.StringList(key) {
		if n, err := strconv.ParseInt(sv, 10, 64); err == nil {
			v = append(v, n)
		}
	}
	return v
}

func (s *ValueImpl) Uint64List(key string) []uint64 {
	var v []uint64
	for _, sv := range s.StringList(key) {
		if n, err := strconv.ParseUint(sv, 10, 64); err == nil {
			v = append(v, n)
		}
	}
	return v
}

func (s *ValueImpl) StringList(key string) []string {
	var v []string
	for _, sv := range strings.Split(s.String(key), ",") {
		sv = strings.TrimSpace(sv)
		if len(sv) == 0 {
			continue
		}
		v = append(v, sv)
	}
	return v
}

func (s *ValueImpl) StringMap(key string) map[string]string {
	var v map[string]string
	if json.Unmarshal([]byte(s.String(key)), &v) == nil {
		return v
	} else {
		return map[string]string{}
	}
}

func (s *ValueImpl) Has(key string) bool {
	_, ok := s.Get(key)
	return ok
}
