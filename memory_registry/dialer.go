package memory_registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/zhangel/go-framework/connection"
	"github.com/zhangel/go-framework/interceptor"
	"github.com/zhangel/go-framework/log"

	"github.com/modern-go/reflect2"
	"google.golang.org/grpc"
)

type MemoryDialer struct {
	mutex             sync.Mutex
	registry          *MemoryRegistry
	clientConnections sync.Map
	opts              *DialOptions
}

type ErrNotFound struct {
	target string
}

func (s ErrNotFound) Error() string {
	return fmt.Sprintf("memory_dialer: no connection of target %q found in memory registry", s.target)
}

func NewMemoryDialer(registry *MemoryRegistry, opt ...DialOption) *MemoryDialer {
	opts := &DialOptions{}

	for _, o := range opt {
		if err := o(opts); err != nil {
			log.Errorf("memory_dialer: parse options failed, err = %v", err)
		}
	}

	return &MemoryDialer{
		mutex:    sync.Mutex{},
		registry: registry,
		opts:     opts,
	}
}

func (s *MemoryDialer) Dial(target string) (*grpc.ClientConn, error) {
	cc, ok := s.clientConnections.Load(target)
	if ok {
		return cc.(*grpc.ClientConn), nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	cc, ok = s.clientConnections.Load(target)
	if ok {
		return cc.(*grpc.ClientConn), nil
	}

	if cc, err := s.makeConnection(target); err != nil || cc == nil {
		if err == nil {
			return nil, ErrNotFound{target}
		} else {
			return nil, err
		}
	} else {
		log.Debugf("Use memory dial, target = %s", target)
		s.clientConnections.Store(target, cc)
		return cc, nil
	}
}

func (s *MemoryDialer) makeConnection(target string) (cc *grpc.ClientConn, err error) {
	memoryRegistry := s.registry.QueryServiceInfo(target)
	if memoryRegistry == nil {
		return nil, nil
	}

	unaryInt := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, _ grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
		unaryInvoker := func() chan error {
			ch := make(chan error, 1)

			go func() {
				if resp, err := s.registry.UnaryInvoke(cc.Target(), method, ctx, req, callOpts...); err != nil {
					ch <- err
				} else {
					vType := reflect2.TypeOf(reply).(reflect2.PtrType).Elem()
					vType.Set(reply, resp)
					ch <- nil
				}
			}()

			return ch
		}

		select {
		case v := <-unaryInvoker():
			switch v := v.(type) {
			case error:
				return v
			case nil:
				return nil
			default:
				return fmt.Errorf("memory-invoker returns unknown response, resp = %+v", v)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if s.opts.unaryInt != nil {
		unaryInt = interceptor.ChainUnaryClient(s.opts.unaryInt, unaryInt)
	}

	streamInt := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, _ grpc.Streamer, callOpts ...grpc.CallOption) (grpc.ClientStream, error) {
		return s.registry.NewStream(cc.Target(), method, ctx, callOpts...)
	}
	if s.opts.streamInt != nil {
		streamInt = interceptor.ChainStreamClient(s.opts.streamInt, streamInt)
	}

	return connection.MakeConnection(target, unaryInt, streamInt)
}
