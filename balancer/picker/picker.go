package picker

import (
	"context"
	"sync"
)

type Picker interface {
	Pick(ctx context.Context) (AddressInfo, error)
	StateChanged(addr AddressInfo)
}

type Builder interface {
	BuildPicker(addrs []AddressInfo) Picker
}

type BuilderFunc func(addrs []AddressInfo) Picker

func (s BuilderFunc) BuildPicker(addrs []AddressInfo) Picker {
	return s(addrs)
}

type ConcurrentPicker struct {
	picker Picker
	mutex  sync.Mutex
}

func NewConcurrentPicker(picker Picker) Picker {
	return &ConcurrentPicker{picker: picker}
}

func (s *ConcurrentPicker) Pick(ctx context.Context) (AddressInfo, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.picker.Pick(ctx)
}

func (s *ConcurrentPicker) StateChanged(addr AddressInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.picker.StateChanged(addr)
}
