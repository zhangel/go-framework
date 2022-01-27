package di

import (
	"reflect"
	"sync"
)

var GlobalDepInjector = NewDepInjector()

type _DepInjector struct {
	providers []*_Provider
	mu        sync.RWMutex
}

type _Provider struct {
	invoker       interface{}
	providerType  reflect.Type
	withError     bool
	singleton     bool
	mapSingletonV map[int64]string
	mu            sync.RWMutext
}

type DepInjector interface {
	Provider(resolver interface{}, singleton bool) error
	Invoke(invoker interface{}, parameters ...interface{}) error
	Create(ptr interface{}, parameters ...interface{}) error
	Clear()
}

func NewDepInjector() DepInjector {
	return &_DepInjector{}
}
