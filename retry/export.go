package retry

import (
	"context"

	"github.com/zhangel/go-framework.git/internal/retry"

	"github.com/zhangel/go-framework.git/control"
	"github.com/zhangel/go-framework.git/log"
	"github.com/zhangel/go-framework.git/registry"
	"github.com/zhangel/go-framework.git/utils"

	cmap "github.com/orcaman/concurrent-map"
	"google.golang.org/grpc"
)

var lastStatus = cmap.New()

func retryHandler(ctx context.Context, service, method string, handler func(ctx context.Context) (err error)) (err error) {
	cp, err := control.DefaultControlPlane().ClientControlPlane(service)
	if err != nil {
		return handler(ctx)
	}

	policy := cp.RetryPolicy(service, method)
	key := service + "/" + method
	log.Infof("key = %s, service = %s, method = %s", key, service, method)
	if st, ok := lastStatus.Get(key); !ok || st.(bool) != policy.Enabled {
		if policy.Enabled {
			log.Infof("retry_handler: retry handler of %q enabled.", key)
		} else if ok {
			log.Infof("retry_handler: retry handler of %q disabled.", key)
		}
		lastStatus.Set(key, policy.Enabled)
	}

	if !policy.Enabled {
		return handler(ctx)
	}

	return retryManager().RetryControlByPolicy(key, policy).Invoke(ctx, policy.RetriableStatusCodes, func(ctx context.Context) error {
		return handler(ctx)
	})
}

func UnaryClientInterceptor(targetTransformer registry.TargetTransformer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, fullMethod string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		service, method := utils.ClientServiceMethodName(targetTransformer(cc.Target()), fullMethod)
		return retryHandler(ctx, service, method, func(ctx context.Context) (err error) {
			return invoker(ctx, fullMethod, req, reply, cc, opts...)
		})
	}
}

func StreamClientInterceptor(targetTransformer registry.TargetTransformer) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, fullMethod string, streamer grpc.Streamer, opts ...grpc.CallOption) (resp grpc.ClientStream, err error) {
		service, method := utils.ClientServiceMethodName(targetTransformer(cc.Target()), fullMethod)
		err = retryHandler(ctx, service, method, func(ctx context.Context) (err error) {
			resp, err = streamer(ctx, desc, cc, fullMethod, opts...)
			return err
		})
		return resp, err
	}
}

func InvokeWithRetry(ctx context.Context, invoker func(context.Context) (interface{}, error), opt ...Option) (interface{}, error) {
	opts := Options{
		overlap:            false,
		backoffDuration:    0,
		backoffJitter:      0.0,
		backoffExponential: false,
	}

	for _, o := range opt {
		if err := o(&opts); err != nil {
			return nil, err
		}
	}

	retryOpts := []func(*retry.Options){
		retry.WithTimeoutPolicy(FixedTimeoutPolicy(opts.perTryTimeout)),
		retry.WithRetryPolicy(FixedRetryCountPolicy(opts.numberOfRetries)),
		retry.WithRetryIntervalPolicy(NewBackoffPolicy(opts.backoffDuration, opts.backoffJitter, opts.backoffExponential)),
	}

	if opts.overlap {
		retryOpts = append(retryOpts, retry.WithConcurrencyPolicy(retry.AlwaysConcurrencyPolicy()))
	} else {
		retryOpts = append(retryOpts, retry.WithConcurrencyPolicy(retry.NeverConcurrencyPolicy()))
	}

	retryController := retry.NewRetryController(retryOpts...)

	logger := log.WithContext(ctx).WithField("tag", "retry::InvokeWithRetry")
	retryTime := 1

	if resp, err := invoker(ctx); err == nil {
		return resp, err
	} else {
		return retryController.Invoke(ctx, func(ctx context.Context, cb retry.TaskCallback) {
			logger.Tracef("retrying, invoker = %q, attempts = %d, reason = %+v", opts.invokerName, retryTime, err)
			retryTime++

			if resp, err = invoker(ctx); err == nil {
				cb.OnSuccess(resp)
			} else if opts.retriablePredict == nil || opts.retriablePredict.IsRetriable(err) {
				cb.Retry(err)
			} else {
				cb.OnError(err)
			}
		})
	}
}
