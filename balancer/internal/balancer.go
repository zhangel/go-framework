package internal

import (
	"context"
	"errors"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/naming"
	"google.golang.org/grpc/status"

	"github.com/zhangel/go-framework/balancer/picker"
	"github.com/zhangel/go-framework/log"
)

type Provider func() picker.Builder

var (
	errBalancerClosed     = errors.New("balancer: balancer is closed")
	errPickerBuilderIsNil = errors.New("balancer: PickerBuilder is nil")
	errNoAddressAvailable = status.Errorf(codes.Unavailable, "there is no address available")
)

type _Balancer struct {
	mu     sync.Mutex
	r      naming.Resolver
	w      naming.Watcher
	addrs  []picker.AddressInfo
	addrCh chan []grpc.Address
	waitCh chan struct{}
	done   bool

	pickerBuilder picker.Builder
	picker        picker.Picker
}

func NewBalancerWithPicker(r naming.Resolver, pickerBuilder picker.Builder) (*_Balancer, error) {
	if pickerBuilder == nil {
		return nil, errPickerBuilderIsNil
	}
	return &_Balancer{r: r, pickerBuilder: pickerBuilder}, nil
}

func (s *_Balancer) watchAddrUpdates() error {
	updates, err := s.w.Next()
	if err != nil {
		log.Warnf("Balancer: the naming watcher stops working due to %v.", err)
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, update := range updates {
		addr := grpc.Address{
			Addr:     update.Addr,
			Metadata: update.Metadata,
		}
		switch update.Op {
		case naming.Add:
			var exist bool
			for _, v := range s.addrs {
				if addr == v.Address() {
					exist = true
					break
				}
			}
			if exist {
				continue
			}
			s.addrs = append(s.addrs, &AddressInfo{addr: addr})
		case naming.Delete:
			for i, v := range s.addrs {
				if addr == v.Address() {
					copy(s.addrs[i:], s.addrs[i+1:])
					s.addrs = s.addrs[:len(s.addrs)-1]
					break
				}
			}
		default:
			log.Error("Balancer: Unknown update.Op ", update.Op)
		}
	}

	open := make([]grpc.Address, len(s.addrs))
	for i, v := range s.addrs {
		open[i] = v.Address()
	}
	if s.done {
		return grpc.ErrClientConnClosing
	}
	select {
	case <-s.addrCh:
	default:
	}

	s.addrCh <- open
	s.picker = s.pickerBuilder.BuildPicker(s.addrs)

	return nil
}

func (s *_Balancer) StartInternal(target string) (<-chan error, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return nil, grpc.ErrClientConnClosing
	}

	if s.r == nil {
		s.addrs = append(s.addrs, &AddressInfo{addr: grpc.Address{Addr: target}})
		s.picker = s.pickerBuilder.BuildPicker(s.addrs)
		return nil, nil
	}

	w, err := s.r.Resolve(target)
	if err != nil {
		return nil, err
	}
	s.w = w
	s.addrCh = make(chan []grpc.Address, 1)
	s.picker = s.pickerBuilder.BuildPicker([]picker.AddressInfo{})

	errCh := make(chan error, 1)
	go func() {
		for {
			if err := s.watchAddrUpdates(); err != nil {
				errCh <- err
				close(errCh)
				return
			}
		}
	}()

	return errCh, nil
}

func (s *_Balancer) Start(target string, _ grpc.BalancerConfig) error {
	_, err := s.StartInternal(target)
	return err
}

func (s *_Balancer) Up(addr grpc.Address) func(error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var cnt int
	for _, a := range s.addrs {
		if a.Address() == addr {
			if a.Connected() {
				return nil
			}
			a.(*AddressInfo).connected = true
			s.picker.StateChanged(a)
		}
		if a.Connected() {
			cnt++
		}
	}

	if cnt == 1 && s.waitCh != nil {
		close(s.waitCh)
		s.waitCh = nil
	}

	return func(err error) {
		s.down(addr, err)
	}
}

func (s *_Balancer) down(addr grpc.Address, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, a := range s.addrs {
		if addr == a.Address() {
			a.(*AddressInfo).connected = false
			s.picker.StateChanged(a)
			break
		}
	}
}

func (s *_Balancer) Get(ctx context.Context, opts grpc.BalancerGetOptions) (addr grpc.Address, put func(), err error) {
	var ch chan struct{}
	s.mu.Lock()

	if s.done {
		s.mu.Unlock()
		err = grpc.ErrClientConnClosing
		return
	}

	addrInfo, e := s.picker.Pick(ctx)
	if addrInfo == nil || e != nil {
		s.mu.Unlock()
		err = errNoAddressAvailable
		return
	}

	if addrInfo.Connected() {
		addr = addrInfo.Address()
		s.mu.Unlock()
		return
	}

	if !opts.BlockingWait {
		if len(s.addrs) == 0 {
			s.mu.Unlock()
			err = errNoAddressAvailable
			return
		}

		addr = addrInfo.Address()
		s.mu.Unlock()
		return
	}

	if s.waitCh == nil {
		ch = make(chan struct{})
		s.waitCh = ch
	} else {
		ch = s.waitCh
	}
	s.mu.Unlock()
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return
		case <-ch:
			s.mu.Lock()
			if s.done {
				s.mu.Unlock()
				err = grpc.ErrClientConnClosing
				return
			}

			addrInfo, e := s.picker.Pick(ctx)
			if addrInfo == nil || e != nil {
				s.mu.Unlock()
				err = errNoAddressAvailable
				return
			}

			if addrInfo.Connected() {
				addr = addrInfo.Address()
				s.mu.Unlock()
				return
			}

			if s.waitCh == nil {
				ch = make(chan struct{})
				s.waitCh = ch
			} else {
				ch = s.waitCh
			}
			s.mu.Unlock()
		}
	}
}

func (s *_Balancer) Notify() <-chan []grpc.Address {
	return s.addrCh
}

func (s *_Balancer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return errBalancerClosed
	}
	s.done = true
	if s.w != nil {
		s.w.Close()
	}
	if s.waitCh != nil {
		close(s.waitCh)
		s.waitCh = nil
	}
	if s.addrCh != nil {
		close(s.addrCh)
	}
	return nil
}
