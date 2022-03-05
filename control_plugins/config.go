package control_plugins

import (
	"strings"
	"sync"
	"time"

	"github.com/iancoleman/strcase"
	cmap "github.com/orcaman/concurrent-map"
	"google.golang.org/grpc/codes"

	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/control"
	"github.com/zhangel/go-framework/declare"
	"github.com/zhangel/go-framework/server"
)

const (
	configUpdateInterval = 10 * time.Second

	enableFlag = "enable"

	circuitBreakerPolicyPrefix           = "circuit_breaker."
	circuitBreakerEnable                 = circuitBreakerPolicyPrefix + enableFlag
	circuitBreakerTimeout                = circuitBreakerPolicyPrefix + "timeout"
	circuitBreakerMaxConcurrentRequests  = circuitBreakerPolicyPrefix + "max_concurrent_requests"
	circuitBreakerRequestVolumeThreshold = circuitBreakerPolicyPrefix + "request_volume_threshold"
	circuitBreakerSleepWindow            = circuitBreakerPolicyPrefix + "sleep_window"
	circuitBreakerErrorPercentThreshold  = circuitBreakerPolicyPrefix + "error_percent_threshold"
	circuitBreakerFailureCodes           = circuitBreakerPolicyPrefix + "failure_status_codes"
	circuitBreakerFailureStatus          = circuitBreakerPolicyPrefix + "failure_status"
	circuitBreakerRollingDurationInSec   = circuitBreakerPolicyPrefix + "rolling_duration"

	rateLimitPolicyPrefix        = "rate_limit."
	rateLimitEnable              = rateLimitPolicyPrefix + enableFlag
	rateLimitMaxRequestPerSecond = rateLimitPolicyPrefix + "max_request_per_unit"
	rateLimitMaxSendMsgPerSecond = rateLimitPolicyPrefix + "max_send_msg_per_unit"
	rateLimitMaxRecvMsgPerSecond = rateLimitPolicyPrefix + "max_recv_msg_per_unit"
	rateLimitUnit                = rateLimitPolicyPrefix + "unit"

	clientSideTimeoutPolicyPrefix = "client_timeout."
	clientSideTimeoutEnable       = clientSideTimeoutPolicyPrefix + enableFlag
	clientSideConnTimeout         = clientSideTimeoutPolicyPrefix + "conn_timeout"
	clientSideRequestTimeout      = clientSideTimeoutPolicyPrefix + "request_timeout"

	serverSideTimeoutPolicyPrefix = "server_timeout."
	serverSideTimeoutEnable       = serverSideTimeoutPolicyPrefix + enableFlag
	serverSideRequestTimeout      = serverSideTimeoutPolicyPrefix + "request_timeout"

	retryPolicyPrefix       = "retry."
	retryEnable             = retryPolicyPrefix + enableFlag
	retryNumberOfRetries    = retryPolicyPrefix + "num_of_retries"
	retryPerTryTimeout      = retryPolicyPrefix + "per_try_timeout"
	retryRetriableStatus    = retryPolicyPrefix + "retriable_status"
	retryRetriableCodes     = retryPolicyPrefix + "retriable_status_codes"
	retryOverlap            = retryPolicyPrefix + "overlap"
	retryBackoffDuration    = retryPolicyPrefix + "backoff_duration"
	retryBackoffJitter      = retryPolicyPrefix + "backoff_jitter"
	retryBackoffExponential = retryPolicyPrefix + "backoff_exponential"
)

var (
	statusMap = map[string]int{}
)

func init() {
	declare.Plugin(control.Plugin, declare.PluginInfo{Name: "config", Creator: NewConfigControlFactory})

	fillStatusMap := func(code codes.Code) {
		statusMap[strings.ToLower(code.String())] = int(code)
		statusMap[strings.ToUpper(code.String())] = int(code)
		statusMap[strcase.ToSnake(code.String())] = int(code)
		statusMap[strcase.ToScreamingSnake(code.String())] = int(code)
		statusMap[strcase.ToKebab(code.String())] = int(code)
		statusMap[strcase.ToScreamingKebab(code.String())] = int(code)
		statusMap[strcase.ToLowerCamel(code.String())] = int(code)
	}

	fillStatusMap(codes.OK)
	fillStatusMap(codes.Canceled)
	fillStatusMap(codes.Unknown)
	fillStatusMap(codes.InvalidArgument)
	fillStatusMap(codes.DeadlineExceeded)
	fillStatusMap(codes.NotFound)
	fillStatusMap(codes.AlreadyExists)
	fillStatusMap(codes.PermissionDenied)
	fillStatusMap(codes.ResourceExhausted)
	fillStatusMap(codes.FailedPrecondition)
	fillStatusMap(codes.Aborted)
	fillStatusMap(codes.OutOfRange)
	fillStatusMap(codes.Unimplemented)
	fillStatusMap(codes.Internal)
	fillStatusMap(codes.Unavailable)
	fillStatusMap(codes.DataLoss)
	fillStatusMap(codes.Unauthenticated)
}

type configControlFactory struct {
	mu                  sync.RWMutex
	serverControlPlanes map[string]control.ServerControlPlane
	clientControlPlanes map[string]control.ClientControlPlane
}

func NewConfigControlFactory() control.Factory {
	return &configControlFactory{
		serverControlPlanes: map[string]control.ServerControlPlane{},
		clientControlPlanes: map[string]control.ClientControlPlane{},
	}
}

func (s *configControlFactory) ClientControlPlane(service string) (control.ClientControlPlane, error) {
	s.mu.RLock()
	if p, ok := s.clientControlPlanes[service]; ok {
		s.mu.RUnlock()
		return p, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	if p, ok := s.clientControlPlanes[service]; ok {
		s.mu.Unlock()
		return p, nil
	}

	defer s.mu.Unlock()
	controlPlane := &clientConfigControl{
		cfg:             config.WithPrefix(service),
		timeoutPolicies: cmap.New(),
		retryPolicies:   cmap.New(),
	}
	s.clientControlPlanes[service] = controlPlane
	return controlPlane, nil
}

type clientConfigControl struct {
	cfg             config.Config
	timeoutPolicies cmap.ConcurrentMap
	retryPolicies   cmap.ConcurrentMap
}

func (s *clientConfigControl) TimeoutPolicy(_, methodName string) *control.ClientTimeoutPolicy {
	policyFromConfig := func() *control.ClientTimeoutPolicy {
		policy := &control.ClientTimeoutPolicy{
			TimeoutPolicy: control.TimeoutPolicy{
				Policy: control.Policy{
					Version: time.Now().UnixNano(),
				},
			},
		}

		withMethod := func(key string) string {
			return methodName + "." + key
		}

		defer s.timeoutPolicies.Set(methodName, policy)

		if !s.cfg.Bool(withMethod(clientSideTimeoutEnable)) && !s.cfg.Bool(clientSideTimeoutEnable) {
			return policy
		} else {
			policy.Enabled = true
		}

		if s.cfg.Has(withMethod(clientSideRequestTimeout)) {
			policy.Timeout = s.cfg.Duration(withMethod(clientSideRequestTimeout))
		} else if s.cfg.Has(clientSideRequestTimeout) {
			policy.Timeout = s.cfg.Duration(clientSideRequestTimeout)
		}

		if s.cfg.Has(withMethod(clientSideConnTimeout)) {
			policy.ConnTimeout = s.cfg.Duration(withMethod(clientSideConnTimeout))
		} else if s.cfg.Has(clientSideConnTimeout) {
			policy.ConnTimeout = s.cfg.Duration(clientSideConnTimeout)
		}

		return policy
	}

	if p, ok := s.timeoutPolicies.Get(methodName); ok && p != nil {
		policy := p.(*control.ClientTimeoutPolicy)
		if time.Now().Sub(time.Unix(0, policy.Version)) > configUpdateInterval {
			return policyFromConfig()
		} else {
			return policy
		}
	} else {
		return policyFromConfig()
	}
}

func (s *clientConfigControl) RetryPolicy(_, methodName string) *control.RetryPolicy {
	policyFromConfig := func() *control.RetryPolicy {
		policy := &control.RetryPolicy{
			Policy: control.Policy{
				Version: time.Now().UnixNano(),
			},
		}

		withMethod := func(key string) string {
			return methodName + "." + key
		}

		defer s.retryPolicies.Set(methodName, policy)

		if !s.cfg.Bool(withMethod(retryEnable)) && !s.cfg.Bool(retryEnable) {
			return policy
		} else {
			policy.Enabled = true
		}

		if s.cfg.Has(withMethod(retryNumberOfRetries)) {
			policy.NumOfRetries = s.cfg.Uint(withMethod(retryNumberOfRetries))
		} else if s.cfg.Has(retryNumberOfRetries) {
			policy.NumOfRetries = s.cfg.Uint(retryNumberOfRetries)
		}

		if s.cfg.Has(withMethod(retryPerTryTimeout)) {
			policy.PerTryTimeout = s.cfg.Duration(withMethod(retryPerTryTimeout))
		} else if s.cfg.Has(retryPerTryTimeout) {
			policy.PerTryTimeout = s.cfg.Duration(retryPerTryTimeout)
		}

		c := map[int]struct{}{}
		if s.cfg.Has(withMethod(retryRetriableCodes)) {
			for _, code := range s.cfg.IntList(withMethod(retryRetriableCodes)) {
				c[code] = struct{}{}
			}
		} else if s.cfg.Has(retryRetriableCodes) {
			for _, code := range s.cfg.IntList(retryRetriableCodes) {
				c[code] = struct{}{}
			}
		}

		if s.cfg.Has(withMethod(retryRetriableStatus)) {
			for _, status := range s.cfg.StringList(withMethod(retryRetriableStatus)) {
				if code, ok := statusMap[status]; ok {
					c[code] = struct{}{}
				}
			}
		} else if s.cfg.Has(retryRetriableStatus) {
			for _, status := range s.cfg.StringList(retryRetriableStatus) {
				if code, ok := statusMap[status]; ok {
					c[code] = struct{}{}
				}
			}
		}

		if len(c) > 0 {
			policy.RetriableStatusCodes = make([]int, 0, len(c))
			for code := range c {
				policy.RetriableStatusCodes = append(policy.RetriableStatusCodes, code)
			}
		}

		if s.cfg.Has(withMethod(retryOverlap)) {
			policy.Overlap = s.cfg.Bool(withMethod(retryOverlap))
		} else if s.cfg.Has(retryOverlap) {
			policy.Overlap = s.cfg.Bool(retryOverlap)
		}

		if s.cfg.Has(withMethod(retryBackoffDuration)) {
			policy.BackoffDuration = s.cfg.Duration(withMethod(retryBackoffDuration))
		} else if s.cfg.Has(retryBackoffDuration) {
			policy.BackoffDuration = s.cfg.Duration(retryBackoffDuration)
		}

		if s.cfg.Has(withMethod(retryBackoffJitter)) {
			policy.BackoffJitter = s.cfg.Float64(withMethod(retryBackoffJitter))
		} else if s.cfg.Has(retryBackoffJitter) {
			policy.BackoffJitter = s.cfg.Float64(retryBackoffJitter)
		} else {
			policy.BackoffJitter = 0.2
		}

		if s.cfg.Has(withMethod(retryBackoffExponential)) {
			policy.BackoffExponential = s.cfg.Bool(withMethod(retryBackoffExponential))
		} else if s.cfg.Has(retryBackoffExponential) {
			policy.BackoffExponential = s.cfg.Bool(retryBackoffExponential)
		} else {
			policy.BackoffExponential = true
		}

		return policy
	}

	if p, ok := s.retryPolicies.Get(methodName); ok && p != nil {
		policy := p.(*control.RetryPolicy)
		if time.Now().Sub(time.Unix(0, policy.Version)) > configUpdateInterval {
			return policyFromConfig()
		} else {
			return policy
		}
	} else {
		return policyFromConfig()
	}
}

func (s *configControlFactory) ServerControlPlane(srv interface{}, service string) (control.ServerControlPlane, error) {
	provider, ok := srv.(server.ServiceProvider)
	var cfg config.Config
	if ok && provider != nil && provider.Config() != nil {
		service = provider.ServiceName()
		cfg = provider.Config()
	} else {
		cfg = config.WithPrefix(service)
	}

	s.mu.RLock()
	if p, ok := s.serverControlPlanes[service]; ok {
		s.mu.RUnlock()
		return p, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	if p, ok := s.serverControlPlanes[service]; ok {
		s.mu.Unlock()
		return p, nil
	}

	defer s.mu.Unlock()
	controlPlane := &serverConfigControl{
		cfg:                    cfg,
		circuitBreakerPolicies: cmap.New(),
		rateLimitPolicies:      cmap.New(),
		timeoutPolicies:        cmap.New(),
	}
	s.serverControlPlanes[service] = controlPlane
	return controlPlane, nil
}

type serverConfigControl struct {
	cfg                    config.Config
	circuitBreakerPolicies cmap.ConcurrentMap
	rateLimitPolicies      cmap.ConcurrentMap
	timeoutPolicies        cmap.ConcurrentMap
}

func (s *serverConfigControl) TimeoutPolicy(_, methodName string) *control.ServerTimeoutPolicy {
	policyFromConfig := func() *control.ServerTimeoutPolicy {
		policy := &control.ServerTimeoutPolicy{
			TimeoutPolicy: control.TimeoutPolicy{
				Policy: control.Policy{
					Version: time.Now().UnixNano(),
				},
			},
		}

		withMethod := func(key string) string {
			return methodName + "." + key
		}

		defer s.timeoutPolicies.Set(methodName, policy)

		if !s.cfg.Bool(withMethod(serverSideTimeoutEnable)) && !s.cfg.Bool(serverSideTimeoutEnable) {
			return policy
		} else {
			policy.Enabled = true
		}

		if s.cfg.Has(withMethod(serverSideRequestTimeout)) {
			policy.Timeout = s.cfg.Duration(withMethod(serverSideRequestTimeout))
		} else if s.cfg.Has(serverSideRequestTimeout) {
			policy.Timeout = s.cfg.Duration(serverSideRequestTimeout)
		}

		return policy
	}

	if p, ok := s.timeoutPolicies.Get(methodName); ok && p != nil {
		policy := p.(*control.ServerTimeoutPolicy)
		if time.Now().Sub(time.Unix(0, policy.Version)) > configUpdateInterval {
			return policyFromConfig()
		} else {
			return policy
		}
	} else {
		return policyFromConfig()
	}
}

func (s *serverConfigControl) CircuitBreakerPolicy(_, methodName string) *control.CircuitBreakerPolicy {
	policyFromConfig := func() *control.CircuitBreakerPolicy {
		policy := &control.CircuitBreakerPolicy{
			Policy: control.Policy{
				Version: time.Now().UnixNano(),
			},
		}

		withMethod := func(key string) string {
			return methodName + "." + key
		}

		defer s.circuitBreakerPolicies.Set(methodName, policy)

		if !s.cfg.Bool(withMethod(circuitBreakerEnable)) && !s.cfg.Bool(circuitBreakerEnable) {
			return policy
		} else {
			policy.Enabled = true
		}

		if s.cfg.Has(withMethod(circuitBreakerTimeout)) {
			policy.Timeout = s.cfg.Duration(withMethod(circuitBreakerTimeout))
		} else if s.cfg.Has(circuitBreakerTimeout) {
			policy.Timeout = s.cfg.Duration(circuitBreakerTimeout)
		}

		if s.cfg.Has(withMethod(circuitBreakerMaxConcurrentRequests)) {
			policy.MaxConcurrentRequests = s.cfg.Int(withMethod(circuitBreakerMaxConcurrentRequests))
		} else if s.cfg.Has(circuitBreakerMaxConcurrentRequests) {
			policy.MaxConcurrentRequests = s.cfg.Int(circuitBreakerMaxConcurrentRequests)
		} else {
			policy.MaxConcurrentRequests = -1
		}

		if s.cfg.Has(withMethod(circuitBreakerRequestVolumeThreshold)) {
			policy.RequestVolumeThreshold = s.cfg.Int(withMethod(circuitBreakerRequestVolumeThreshold))
		} else if s.cfg.Has(circuitBreakerRequestVolumeThreshold) {
			policy.RequestVolumeThreshold = s.cfg.Int(circuitBreakerRequestVolumeThreshold)
		}

		if s.cfg.Has(withMethod(circuitBreakerSleepWindow)) {
			policy.SleepWindow = s.cfg.Duration(withMethod(circuitBreakerSleepWindow))
		} else if s.cfg.Has(circuitBreakerSleepWindow) {
			policy.SleepWindow = s.cfg.Duration(circuitBreakerSleepWindow)
		}

		if s.cfg.Has(withMethod(circuitBreakerErrorPercentThreshold)) {
			policy.ErrorPercentThreshold = s.cfg.Int(withMethod(circuitBreakerErrorPercentThreshold))
		} else if s.cfg.Has(circuitBreakerErrorPercentThreshold) {
			policy.ErrorPercentThreshold = s.cfg.Int(circuitBreakerErrorPercentThreshold)
		}

		if s.cfg.Has(withMethod(circuitBreakerRollingDurationInSec)) {
			policy.RollingDurationInSec = s.cfg.Int(withMethod(circuitBreakerRollingDurationInSec))
		} else if s.cfg.Has(circuitBreakerRollingDurationInSec) {
			policy.RollingDurationInSec = s.cfg.Int(circuitBreakerRollingDurationInSec)
		}

		c := map[int]struct{}{}
		if s.cfg.Has(withMethod(circuitBreakerFailureCodes)) {
			for _, code := range s.cfg.IntList(withMethod(circuitBreakerFailureCodes)) {
				c[code] = struct{}{}
			}
		} else if s.cfg.Has(circuitBreakerFailureCodes) {
			for _, code := range s.cfg.IntList(circuitBreakerFailureCodes) {
				c[code] = struct{}{}
			}
		}

		if s.cfg.Has(withMethod(circuitBreakerFailureStatus)) {
			for _, status := range s.cfg.StringList(withMethod(circuitBreakerFailureStatus)) {
				if code, ok := statusMap[status]; ok {
					c[code] = struct{}{}
				}
			}
		} else if s.cfg.Has(circuitBreakerFailureStatus) {
			for _, status := range s.cfg.StringList(circuitBreakerFailureStatus) {
				if code, ok := statusMap[status]; ok {
					c[code] = struct{}{}
				}
			}
		}

		if len(c) > 0 {
			policy.FailureStatusCode = make([]int, 0, len(c))
			for code := range c {
				policy.FailureStatusCode = append(policy.FailureStatusCode, code)
			}
		}

		return policy
	}

	if p, ok := s.circuitBreakerPolicies.Get(methodName); ok && p != nil {
		policy := p.(*control.CircuitBreakerPolicy)
		if time.Now().Sub(time.Unix(0, policy.Version)) > configUpdateInterval {
			return policyFromConfig()
		} else {
			return policy
		}
	} else {
		return policyFromConfig()
	}
}

func (s *serverConfigControl) RateLimitPolicy(_, methodName string) *control.RateLimitPolicy {
	policyFromConfig := func() *control.RateLimitPolicy {
		policy := &control.RateLimitPolicy{
			Policy: control.Policy{
				Version: time.Now().UnixNano(),
			},
		}

		withMethod := func(key string) string {
			return methodName + "." + key
		}

		defer s.rateLimitPolicies.Set(methodName, policy)

		if !s.cfg.Bool(withMethod(rateLimitEnable)) && !s.cfg.Bool(rateLimitEnable) {
			return policy
		} else {
			policy.Enabled = true
		}

		if s.cfg.Has(withMethod(rateLimitMaxRequestPerSecond)) {
			policy.MaxRequestPerUnit = s.cfg.Int(withMethod(rateLimitMaxRequestPerSecond))
		} else if s.cfg.Has(rateLimitMaxRequestPerSecond) {
			policy.MaxRequestPerUnit = s.cfg.Int(rateLimitMaxRequestPerSecond)
		} else {
			policy.MaxRequestPerUnit = -1
		}

		if s.cfg.Has(withMethod(rateLimitMaxSendMsgPerSecond)) {
			policy.MaxSendMsgPerUnit = s.cfg.Int(withMethod(rateLimitMaxSendMsgPerSecond))
		} else if s.cfg.Has(rateLimitMaxSendMsgPerSecond) {
			policy.MaxSendMsgPerUnit = s.cfg.Int(rateLimitMaxSendMsgPerSecond)
		} else {
			policy.MaxSendMsgPerUnit = -1
		}

		if s.cfg.Has(withMethod(rateLimitMaxRecvMsgPerSecond)) {
			policy.MaxRecvMsgPerUnit = s.cfg.Int(withMethod(rateLimitMaxRecvMsgPerSecond))
		} else if s.cfg.Has(rateLimitMaxRecvMsgPerSecond) {
			policy.MaxRecvMsgPerUnit = s.cfg.Int(rateLimitMaxRecvMsgPerSecond)
		} else {
			policy.MaxRecvMsgPerUnit = -1
		}

		if s.cfg.Has(withMethod(rateLimitUnit)) {
			policy.Unit = s.cfg.Duration(withMethod(rateLimitUnit))
		} else if s.cfg.Has(rateLimitUnit) {
			policy.Unit = s.cfg.Duration(rateLimitUnit)
		} else {
			policy.Unit = time.Second
		}

		return policy
	}

	if p, ok := s.rateLimitPolicies.Get(methodName); ok && p != nil {
		policy := p.(*control.RateLimitPolicy)
		if time.Now().Sub(time.Unix(0, policy.Version)) > configUpdateInterval {
			return policyFromConfig()
		} else {
			return policy
		}
	} else {
		return policyFromConfig()
	}
}
