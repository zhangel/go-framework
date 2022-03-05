package pool

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"

	"github.com/zhangel/go-framework/dialer/internal/option"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/registry"
)

var (
	pools map[string]*ccPool = map[string]*ccPool{}
	mutex sync.RWMutex

	redialInterval = 5 * time.Minute
	nilCC          = (*grpc.ClientConn)(nil)
)

type DialerFn func(ctx context.Context, target string, dialOpts *option.DialOptions) (*grpc.ClientConn, error)

type ccPool struct {
	id        string
	optionTag []string
	target    string
	dialer    DialerFn
	dialOpts  *option.DialOptions
	capacity  uint32
	idx       uint32
	ccs       []atomic.Value
	lastFail  []time.Time
	group     singleflight.Group
}

func WithPool(dialer DialerFn, callOpts *option.CallOptions) (pool *ccPool, err error) {
	defer func() {
		*callOpts.TargetTransformer = func(target string) string {
			return registry.Target(pool.dialOpts.Registry, target)
		}
	}()

	mutex.RLock()
	poolId := callOpts.PoolIdentity()
	if pool, ok := pools[poolId]; ok {
		mutex.RUnlock()
		return pool, nil
	}
	mutex.RUnlock()

	mutex.Lock()
	defer mutex.Unlock()

	if pool, ok := pools[poolId]; ok {
		return pool, nil
	}

	dialOpts, err := option.PrepareDialOption(callOpts)
	if err != nil {
		return nil, err
	}

	pool = &ccPool{
		id:        poolId,
		target:    callOpts.Target,
		optionTag: callOpts.DialOptTag,
		dialer:    dialer,
		dialOpts:  dialOpts,
		capacity:  uint32(dialOpts.PoolSize),
		idx:       0,
		ccs:       make([]atomic.Value, dialOpts.PoolSize),
		lastFail:  make([]time.Time, dialOpts.PoolSize),
	}
	pools[poolId] = pool
	return pool, nil
}

func Clear() {
	mutex.Lock()
	defer mutex.Unlock()

	for _, pool := range pools {
		if pool != nil {
			pool.Clear()
		}
	}
	pools = map[string]*ccPool{}
}

func (s *ccPool) Dial(ctx context.Context) (cc *grpc.ClientConn, err error) {
	idx := atomic.AddUint32(&s.idx, 1) % s.capacity

	var waitContext context.Context
	var cancel context.CancelFunc

	dialTimeout := 30 * time.Second
	if s.dialOpts.DialTimeout > 0 {
		dialTimeout = s.dialOpts.DialTimeout
	}
	waitContext, cancel = context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	cc, ok := s.ccs[idx].Load().(*grpc.ClientConn)
	if ok && cc != nil && cc != nilCC {
		if cc.GetState() == connectivity.Ready {
			if s.dialOpts.DialFunc != nil {
				log.Infof("ccPool:Dial, reuse cc from pool, poolId = %s, dialOptTags = %v, dialFunc = %+v, target = %s, connState = %v", s.id, s.optionTag, s.dialOpts.DialFunc, s.target, cc.GetState())
			} else {
				log.Debugf("ccPool:Dial, reuse cc from pool, poolId = %s, dialOptTags = %v, target = %s, connState = %v", s.id, s.optionTag, s.target, cc.GetState())
			}
			return cc, nil
		}
	}

	result, err, _ := s.group.Do(strconv.Itoa(int(idx)), func() (result interface{}, err error) {
		defer func() {
			if err != nil && s.lastFail[idx].IsZero() {
				s.lastFail[idx] = time.Now()
			}
		}()

		beginTime := time.Now()

		cc, ok := s.ccs[idx].Load().(*grpc.ClientConn)
		if ok && cc != nil && cc != nilCC {
			if err := s.checkConnState(waitContext, cc, beginTime); err == nil {
				if s.dialOpts.DialFunc != nil {
					log.Infof("ccPool:Dial, reuse cc from pool, poolId = %s, dialOptTags = %v, dialFunc = %+v, target = %s, connState = %v", s.id, s.optionTag, s.dialOpts.DialFunc, s.target, cc.GetState())
				} else {
					log.Debugf("ccPool:Dial, reuse cc from pool, poolId = %s, dialOptTags = %v, target = %s, connState = %v", s.id, s.optionTag, s.target, cc.GetState())
				}
				return cc, nil
			} else {
				if s.lastFail[idx].Add(redialInterval).Before(time.Now()) {
					log.Debugf("ccPool:Dial, cc from pool unavailable, close and redial. poolId = %s, dialOptTags = %v, target = %s, connState = %v", s.id, s.optionTag, s.target, cc.GetState())
					if err := cc.Close(); err != nil {
						log.Errorf("ccPool:Dial, close unavailable cc failed, poolId = %s, dialOptTags = %v, target = %s, connState = %v, err = %+v", s.id, s.optionTag, s.target, cc.GetState(), err)
					}
					s.ccs[idx].Store(nilCC)
				} else {
					log.Debugf("ccPool:Dial, cc from pool unavailable, last failure at %q, redial interval = %v. poolId = %s, dialOptTags = %v, target = %s, connState = %v",
						s.lastFail[idx].Format("2006-01-02 15:04:05"), redialInterval, s.id, s.optionTag, s.target, cc.GetState())
					return nil, err
				}
			}
		}

		s.lastFail[idx] = time.Time{}
		newCC, err := s.dialer(waitContext, s.target, s.dialOpts)
		if err != nil {
			return nil, err
		} else if newCC == nil {
			return nil, status.New(codes.Internal, "Dial returns nil cc.").Err()
		} else {
			s.ccs[idx].Store(newCC)
			if err := s.checkConnState(waitContext, newCC, beginTime); err != nil {
				return nil, err
			} else {
				log.Debugf("ccPool:Dial, create cc, target = %s, poolId = %s, dialOptTags = %v, dialOpts = %#v", s.target, s.id, s.optionTag, s.dialOpts)
				return newCC, nil
			}
		}
	})

	if err != nil {
		return nil, err
	} else {
		return result.(*grpc.ClientConn), nil
	}
}

func (s *ccPool) Clear() {
	for i := 0; i < len(s.ccs); i++ {
		if cc, ok := s.ccs[i].Load().(*grpc.ClientConn); ok && cc != nil {
			_ = cc.Close()
		}
	}
}

func (s *ccPool) checkConnState(ctx context.Context, cc *grpc.ClientConn, beginTime time.Time) error {
	if cc == nil {
		return status.New(codes.Internal, "checkConnState: cc == nil").Err()
	}

	for {
		state := cc.GetState()
		switch state {
		case connectivity.Ready:
			return nil
		case connectivity.Idle:
			fallthrough
		case connectivity.TransientFailure:
			fallthrough
		case connectivity.Connecting:
			if err := s.waitConnStateChange(ctx, cc, state, beginTime); err == nil {
				continue
			} else {
				return err
			}
		case connectivity.Shutdown:
			return status.New(codes.Unavailable, "checkConnState: cc.State == connectivity.Shutdown").Err()
		default:
			return status.New(codes.Internal, "checkConnState: invalid cc.State").Err()
		}
	}
}

func (s *ccPool) waitConnStateChange(ctx context.Context, cc *grpc.ClientConn, state connectivity.State, beginTime time.Time) error {
	if cc.WaitForStateChange(ctx, state) {
		return nil
	}
	connTime := time.Since(beginTime)

	registryInfo := func(r registry.Registry) string {
		if r == nil {
			return "<nil>"
		} else if info, ok := r.(registry.Info); ok {
			return info.Info(s.target)
		} else {
			return fmt.Sprintf("%v", reflect.TypeOf(s.dialOpts.Registry))
		}
	}

	dialerString := "direct"
	if s.dialOpts.DialFunc != nil {
		paths := strings.Split(runtime.FuncForPC(reflect.ValueOf(s.dialOpts.DialFunc).Pointer()).Name(), "/")
		if len(paths) > 0 {
			dialerString = paths[len(paths)-1]
		}
		dialerSeg := strings.Split(dialerString, ".")
		if len(dialerSeg) > 0 && strings.HasPrefix(dialerSeg[len(dialerSeg)-1], "func") {
			dialerString = strings.Join(dialerSeg[:len(dialerSeg)-1], ".")
		}
	}

	var addrString string
	if s.dialOpts != nil && s.dialOpts.Registry != nil && s.dialOpts.Registry.Resolver() != nil {
		if addrs, err := s.dialOpts.Registry.ListServiceAddresses(s.target); err == nil && len(addrs) > 0 {
			addrString = fmt.Sprintf("%v", addrs)
		} else {
			addrString = "[]"
		}
	} else {
		addrString = fmt.Sprintf("[%s:%q]", dialerString, s.target)
	}

	discoveryTime := time.Since(beginTime) - connTime
	return status.Newf(codes.DeadlineExceeded, "conn-stat:%s, target-name:%s, poolId = %s, dialOptTags = %v, dto = %v, addrs:%s, naming-discovery-type:%s, conn-waittime:%s, naming-discovery-time:%s, ctx-error:%v",
		state,
		s.target,
		s.id,
		s.dialOpts,
		s.dialOpts.DialTimeout,
		addrString,
		registryInfo(s.dialOpts.Registry),
		connTime,
		discoveryTime,
		ctx.Err()).Err()
}
