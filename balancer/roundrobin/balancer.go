package roundrobin

import (
	"context"
	"fmt"

	"github.com/zhangel/go-framework/balancer"
	"github.com/zhangel/go-framework/balancer/picker"
)

type options struct {
	isAddressAvailable func(addr picker.AddressInfo) bool
}

type Option func(opts *options)

func WithAddressFilter(isAddressAvailable func(addr picker.AddressInfo) bool) Option {
	return func(opts *options) {
		opts.isAddressAvailable = isAddressAvailable
	}
}

type _RoundRobinPicker struct {
	opts  *options
	addrs []picker.AddressInfo
	next  int
}

func NewProvider(opt ...Option) balancer.Provider {
	return func() picker.Builder {
		return picker.BuilderFunc(func(addrs []picker.AddressInfo) picker.Picker {
			opts := &options{}
			for _, o := range opt {
				o(opts)
			}

			return &_RoundRobinPicker{
				opts:  opts,
				addrs: addrs,
			}
		})
	}
}

func (s *_RoundRobinPicker) Pick(ctx context.Context) (picker.AddressInfo, error) {
	if len(s.addrs) == 0 {
		return nil, fmt.Errorf("no avaliable address")
	}

	if s.opts.isAddressAvailable != nil {
		cur := s.next
		for {
			addr := s.addrs[s.next]
			s.next = (s.next + 1) % len(s.addrs)
			if s.opts.isAddressAvailable(addr) {
				return addr, nil
			}

			if s.next == cur {
				return nil, fmt.Errorf("no available address")
			}
		}
	} else {
		addr := s.addrs[s.next]
		s.next = (s.next + 1) % len(s.addrs)
		return addr, nil
	}
}

func (s *_RoundRobinPicker) StateChanged(addr picker.AddressInfo) {
}
