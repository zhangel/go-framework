package rate_limit

import (
	"strconv"
	"sync"
	"time"

	"github.com/zhangel/go-framework/control"
	"github.com/zhangel/go-framework/log"

	"github.com/juju/ratelimit"
)

type rateLimiterType int

const (
	requestRateLimiter rateLimiterType = iota
	sendMsgRateLimiter
	recvMsgRateLimiter
)

var (
	once    sync.Once
	manager *rateLimiterManager
)

type rateLimiterManager struct {
	rateLimitMu  sync.RWMutex
	rateLimiters map[string]*rateLimiterWrapper
}

type rateLimiterWrapper struct {
	name        string
	mu          sync.RWMutex
	rateLimiter *ratelimit.Bucket
	policy      *control.RateLimitPolicy
}

func rateLimiterManger() *rateLimiterManager {
	once.Do(func() {
		manager = &rateLimiterManager{
			rateLimiters: map[string]*rateLimiterWrapper{},
		}
	})

	return manager
}

func newRateLimiterWrapper(name string, policy *control.RateLimitPolicy, requestPerUnit func(*control.RateLimitPolicy) int) *rateLimiterWrapper {
	log.Infof("rate_limiter: initialize rate limiter policy for %q, policy = %+v", name, policy)

	return &rateLimiterWrapper{
		name:        name,
		rateLimiter: ratelimit.NewBucket(policy.Unit/time.Duration(requestPerUnit(policy)), int64(requestPerUnit(policy))),
		policy:      policy,
	}
}

func (s *rateLimiterWrapper) UpdatePolicy(policy *control.RateLimitPolicy, requestPerUnit func(*control.RateLimitPolicy) int) {
	s.mu.RLock()
	if policy.Version == s.policy.Version {
		s.mu.RUnlock()
		return
	}

	if policy.Enabled == s.policy.Enabled &&
		policy.MaxRequestPerUnit == s.policy.MaxRequestPerUnit &&
		policy.MaxSendMsgPerUnit == s.policy.MaxSendMsgPerUnit &&
		policy.MaxRecvMsgPerUnit == s.policy.MaxRecvMsgPerUnit &&
		policy.Unit == s.policy.Unit {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = policy
	s.rateLimiter = ratelimit.NewBucket(policy.Unit/time.Duration(requestPerUnit(policy)), int64(requestPerUnit(policy)))

	log.Infof("rate_limiter: update rate limiter policy for %q, policy = %+v", s.name, policy)
}

func (s *rateLimiterManager) RateLimiterWithPolicy(typ rateLimiterType, name string, policy *control.RateLimitPolicy, requestPerUnit func(*control.RateLimitPolicy) int) *ratelimit.Bucket {
	s.rateLimitMu.RLock()
	if l, ok := s.rateLimiters[name+strconv.Itoa(int(typ))]; ok {
		s.rateLimitMu.RUnlock()
		l.UpdatePolicy(policy, requestPerUnit)
		return l.rateLimiter
	}
	s.rateLimitMu.RUnlock()

	s.rateLimitMu.Lock()
	if l, ok := s.rateLimiters[name+strconv.Itoa(int(typ))]; ok {
		l.UpdatePolicy(policy, requestPerUnit)
		s.rateLimitMu.Unlock()
		return l.rateLimiter
	}
	defer s.rateLimitMu.Unlock()

	wrapper := newRateLimiterWrapper(name, policy, requestPerUnit)
	s.rateLimiters[name+strconv.Itoa(int(typ))] = wrapper
	return wrapper.rateLimiter
}
