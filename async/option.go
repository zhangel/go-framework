package async

import (
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

type RetryMode int

const (
	NoRetry RetryMode = iota
	DefaultRetry
	ForeverRetry
	CustomizedRetry
)

var defaultRetryCodes = []codes.Code{codes.DeadlineExceeded, codes.Unavailable}

const (
	targetKey                   = "x-async-target"
	requestTimeout              = "x-async-timeout"
	shardingKey                 = "x-async-sharding-key"
	clusterKey                  = "x-async-cluster"
	retryModeKey                = "x-async-retry-mode"
	retryCodesKey               = "x-async-retry-codes"
	retryMaxRetriesKey          = "x-async-retry-max-retries"
	retryIntervalKey            = "x-async-retry-interval"
	retryMaxIntervalKye         = "x-async-retry-max-interval"
	retryIntervalProgressiveKey = "x-async-retry-interval-progressive"
)

type Options struct {
	retryMode      RetryMode
	customCodes    bool
	retryCodes     []codes.Code
	retryPolicy    RetryPolicy
	requestTimeout int32
	shardingKey    string
	cluster        string
}

type RetryPolicy struct {
	maxRetries, retryInterval, maxInterval int32
	intervalProgressive                    bool
}

type Option func(*Options)

func WithRetryMode(retryMode RetryMode) func(*Options) {
	return func(options *Options) {
		options.retryMode = retryMode
	}
}

func WithRequestTimeout(tc int32) func(*Options) {
	return func(options *Options) {
		options.requestTimeout = tc
	}
}

func WithShardingkey(key string) func(*Options) {
	return func(options *Options) {
		options.shardingKey = key
	}
}

func WithCluster(clusterName string) func(*Options) {
	return func(options *Options) {
		options.cluster = clusterName
	}
}

func WithCustomRetryCode(retryCodes ...codes.Code) func(*Options) {
	return func(options *Options) {
		options.retryCodes = retryCodes
		options.customCodes = true
	}
}

func WithCustomRetryPolicy(maxRetries, retryInterval, maxInterval int32, intervalProgressive bool) func(*Options) {
	return func(options *Options) {
		options.retryPolicy.maxRetries = maxRetries
		options.retryPolicy.retryInterval = retryInterval
		options.retryPolicy.maxInterval = maxInterval
		options.retryPolicy.intervalProgressive = intervalProgressive
	}
}

func (s *Options) applyToMetadata(md metadata.MD) {
	md.Set(retryModeKey, strconv.Itoa(int(s.retryMode)))
	var retryCodes []string
	if s.customCodes {
		if len(s.retryCodes) > 0 {
			for _, code := range s.retryCodes {
				retryCodes = append(retryCodes, strconv.Itoa(int(code)))
			}
		}
	} else {
		for _, code := range defaultRetryCodes {
			retryCodes = append(retryCodes, strconv.Itoa(int(code)))
		}
	}
	md.Set(retryCodesKey, retryCodes...)
	md.Set(shardingKey, s.shardingKey)
	md.Set(clusterKey, s.cluster)

	if s.retryMode == CustomizedRetry {
		s.retryPolicy.applyToMetadata(md)
	}
	if s.requestTimeout > 0 {
		md.Set(requestTimeout, strconv.FormatInt(int64(s.requestTimeout), 10))
	}
}

func (s *RetryPolicy) applyToMetadata(md metadata.MD) {
	md.Set(retryMaxRetriesKey, strconv.FormatInt(int64(s.maxRetries), 10))
	md.Set(retryIntervalKey, strconv.FormatInt(int64(s.retryInterval), 10))
	md.Set(retryMaxIntervalKye, strconv.FormatInt(int64(s.maxInterval), 10))
	if s.intervalProgressive {
		md.Set(retryIntervalProgressiveKey, "true")
	} else {
		md.Set(retryIntervalProgressiveKey, "false")
	}
}
