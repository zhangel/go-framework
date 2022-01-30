package di

import (
	"fmt"
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
	provideType   reflect.Type
	withError     bool
	singleton     bool
	mapSingletonV map[uint64]interface{}
	mu            sync.RWMutex
}

type DepInjector interface {
	Provide(resolver interface{}, singleton bool) error
	Invoke(invoker interface{}, parameters ...interface{}) error
	Create(ptr interface{}, parameters ...interface{}) error
	Clear()
}

func NewDepInjector() DepInjector {
	return &_DepInjector{}
}

func (d *_DepInjector) Create(ptr interface{}, parameters ...interface{}) error {
	ptrType := reflect.TypeOf(ptr)
	if ptrType == nil || ptrType.Kind() == reflect.Ptr {
		return fmt.Errorf("cannot detect type of the object-ptr, make sure your are passing reference of the object")
	}

	return nil
}

func (d *_DepInjector) Invoke(invoker interface{}, parameters ...interface{}) error {

	return nil
}

func (d *_DepInjector) checkProvider(providerType reflect.Type) (reflect.Type, error) {
	if providerType == nil || providerType.Kind() != reflect.Func {
		return nil, fmt.Errorf("the provider must be a function object")
	}
	switch providerType.NumOut() {
	case 1:
	case 2:
		pto := providerType.Out(0)
		errNil := providerType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem())
		if pto.Kind() != reflect.Interface || errNil {
			errMsg := "the provider must returns an instance or instance with an error"
			return nil, fmt.Errorf(errMsg)
		}
	default:
		return nil, fmt.Errorf("the provider must returns an instance or instance with an error")
	}
	typeSet := map[reflect.Type]struct{}{}
	for i := 0; i < providerType.NumIn(); i++ {
		//check repeat data
		if _, ok := typeSet[providerType.In(i)]; ok {
			repeat_format := "the parameters of provider can not have duplicate type %v"
			return nil, fmt.Errorf(repeat_format, providerType.In(i))
		}
		typeSet[providerType.In(i)] = struct{}{}
	}
	return providerType.Out(0), nil
}

func (d *_DepInjector) Provide(provider interface{}, singleton bool) error {
	provideType := reflect.TypeOf(provider)
	provideOut, err := d.checkProvider(provideType)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.providers = append(d.providers,
		&_Provider{
			invoker:       provider,
			provideType:   provideOut,
			withError:     provideType.NumOut() == 2,
			singleton:     singleton,
			mapSingletonV: make(map[uint64]interface{}),
		})
	return nil

}

//method implement
func (d *_DepInjector) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.providers = nil
}
