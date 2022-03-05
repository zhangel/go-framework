package option

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/zhangel/go-framework.git/control"
	"github.com/zhangel/go-framework.git/log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zhangel/go-framework.git/async"
	"github.com/zhangel/go-framework.git/balancer"
	"github.com/zhangel/go-framework.git/config"
	"github.com/zhangel/go-framework.git/credentials"
	"github.com/zhangel/go-framework.git/dialer/internal"
	"github.com/zhangel/go-framework.git/prometheus"
	"github.com/zhangel/go-framework.git/registry"
	"github.com/zhangel/go-framework.git/retry"
	"github.com/zhangel/go-framework.git/server"
	"github.com/zhangel/go-framework.git/timeout"
	"github.com/zhangel/go-framework.git/tracing"
)

var (
	mutex       sync.RWMutex
	dialOptions = map[string][]DialOption{}
)

const pemPrefix = "-----BEGIN"

type dialOptRegisterErr struct {
	dialOptTag string
}

func (s dialOptRegisterErr) Error() string {
	return fmt.Sprintf("dialer::RegisterDialOption: found duplicate options for %s", s.dialOptTag)
}

func RegisterDialOption(dialOptTag string, opt ...DialOption) error {
	mutex.RLock()
	if _, ok := dialOptions[dialOptTag]; ok {
		mutex.RUnlock()
		return dialOptRegisterErr{dialOptTag: dialOptTag}
	}
	mutex.RUnlock()

	mutex.Lock()
	defer mutex.Unlock()

	if _, ok := dialOptions[dialOptTag]; ok {
		return dialOptRegisterErr{dialOptTag: dialOptTag}
	}

	dialOptions[dialOptTag] = opt
	return nil
}

func IsDialOptionRegistered(dialOptTag string) bool {
	mutex.RLock()
	defer mutex.RUnlock()

	_, ok := dialOptions[dialOptTag]
	return ok
}

func QueryDialOption(dialOptTag string) ([]DialOption, bool) {
	mutex.RLock()
	dialOpt, ok := dialOptions[dialOptTag]
	mutex.RUnlock()

	if ok {
		return append([]DialOption{}, dialOpt...), true
	} else {
		return nil, false
	}
}

var (
	dialInMemory               bool
	ignoreInternalInterceptors bool
	configOnce                 sync.Once
	methodMap                  sync.Map
)

func parseCallOpt(method string, callOpt []grpc.CallOption, opts *CallOptions) error {
	for _, opt := range callOpt {
		switch o := opt.(type) {
		case *DialOptTagOpt:
			hasTag := false
			for _, tag := range opts.DialOptTag {
				if tag == o.DialOptTag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				opts.DialOptTag = append(opts.DialOptTag, o.DialOptTag)
			}
		case *UseInProcDialOpt:
			opts.UseInProcDial = o.UseInProcDial
		case *TargetCallOpt:
			opts.Target = o.Target
		case *TracerCallOpt:
			opts.Tracer = o.Tracer
		case *AsyncCallOpt:
			opts.AsyncOpt = o
		case *UnaryInterceptorCallOpt:
			opts.UnaryInt = append(opts.UnaryInt, o.UnaryInt)
		case *StreamInterceptorCallOpt:
			opts.StreamInt = append(opts.StreamInt, o.StreamInt)
		case *IgnoreInternalInterceptorsCallOpt:
			opts.IgnoreInternalInterceptors = o.Ignore
		case *DialOptTagAsPoolIdOpt:
			opts.DialOptTagAsPoolId = o.Enable
		}
	}

	opts.poolIdentity = func() string {
		if opts.DialOptTagAsPoolId {
			return strings.Join(opts.DialOptTag, ":")
		} else {
			return opts.Target + ":" + strings.Join(opts.DialOptTag, ":")
		}
	}

	if path, ok := methodMap.Load(method); ok {
		opts.ServiceName = path.(string)
	} else {
		path := strings.Split(method, "/")
		if len(path) != 3 {
			return status.Error(codes.InvalidArgument, "Invoke grpc method without target specified")
		}
		opts.ServiceName = path[1]
		methodMap.Store(method, path[1])
	}

	if opts.Target == "" {
		opts.Target = opts.ServiceName
	}
	return nil
}

func PrepareCallOptions(ctx context.Context, method string, callOpt []grpc.CallOption, callOptHook CallOptionHook) ([]grpc.CallOption, *CallOptions, error) {
	var targetTransformer registry.TargetTransformer = func(target string) string { return target }

	configOnce.Do(func() {
		dialInMemory = config.Bool(internal.FlagDialInMemory)
		ignoreInternalInterceptors = config.Bool(internal.FlagIgnoreInternalInterceptors)
	})

	result := &CallOptions{
		TargetTransformer:          &targetTransformer,
		UseInProcDial:              dialInMemory,
		Tracer:                     tracing.DefaultTracer(),
		IgnoreInternalInterceptors: ignoreInternalInterceptors,
		CallOpt:                    callOpt,
	}

	if err := parseCallOpt(method, callOpt, result); err != nil {
		return nil, nil, err
	}

	if callOptHook != nil {
		callOpt = callOptHook(ctx, callOpt, result)
		if err := parseCallOpt(method, callOpt, result); err != nil {
			return nil, nil, err
		}
	}

	if result.AsyncOpt != nil {
		result.UnaryInt = append(result.UnaryInt, async.AsyncUnaryClientInterceptor(result.Target, result.AsyncOpt.Options))
		result.Target = async.AsyncAddr
	}

	return callOpt, result, nil
}

func PrepareDialOption(callOpts *CallOptions) (*DialOptions, error) {
	dialOpts := DefaultDialOption(callOpts.Target)

	if len(callOpts.DialOptTag) > 0 {
		for _, tag := range callOpts.DialOptTag {
			if opt, ok := QueryDialOption(tag); ok {
				for _, o := range opt {
					if err := o(dialOpts); err != nil {
						return nil, err
					}
				}
			} else {
				return nil, fmt.Errorf("dialer::PrepareDialOption: dialOpt tag [%s] not found in registry", callOpts.DialOptTag)
			}
		}
	} else if opt, ok := QueryDialOption(callOpts.Target); ok {
		for _, o := range opt {
			if err := o(dialOpts); err != nil {
				return nil, err
			}
		}
	}

	if dialOpts.CredentialFromRegistry {
		if cred, err := credentialOptsFromRegistry(callOpts.Target, dialOpts); err == nil {
			dialOpts.CredentialOpts = cred
		} else {
			return nil, err
		}
	}

	return dialOpts, nil
}

func DefaultDialOption(target string) *DialOptions {
	dialOpts := &DialOptions{
		PoolSize: 1,
	}

	if registry.DefaultRegistry() != nil {
		dialOpts.Registry = registry.DefaultRegistry()
	}

	if balancer.DefaultBalancerBuilder() != nil {
		dialOpts.BalancerBuilder = balancer.DefaultBalancerBuilder()
	}

	if cp, err := control.DefaultControlPlane().ClientControlPlane(target); err != nil || cp == nil {
		dialOpts.DialTimeout = config.Duration(internal.FlagConnTimeout)
	} else if policy := cp.TimeoutPolicy(target, ""); policy == nil || !policy.Enabled || policy.ConnTimeout == 0 {
		dialOpts.DialTimeout = config.Duration(internal.FlagConnTimeout)
	} else {
		dialOpts.DialTimeout = policy.ConnTimeout
	}

	certFile := config.String(internal.FlagCert)
	keyFile := config.String(internal.FlagKey)
	if certFile == "" && keyFile == "" && config.Bool(internal.FlagUseServerCert) {
		certFile = config.String(server.FlagCert)
		keyFile = config.String(server.FlagKey)
	}

	if certFile != "" && keyFile != "" {
		if strings.HasPrefix(certFile, pemPrefix) && strings.HasPrefix(keyFile, pemPrefix) {
			dialOpts.CredentialOpts = append(dialOpts.CredentialOpts, credentials.WithClientCertificate(func(info *tls.CertificateRequestInfo) (*tls.Certificate, error) {
				certificate, err := tls.X509KeyPair([]byte(certFile), []byte(keyFile))
				if err != nil {
					return nil, err
				}
				return &certificate, nil
			}))
		} else {
			dialOpts.CredentialOpts = append(dialOpts.CredentialOpts, credentials.WithClientCertificateFile(certFile, keyFile))
		}
	}

	caFiles := config.StringList(internal.FlagCaCert)
	if len(caFiles) > 0 {
		dialOpts.CredentialOpts = append(dialOpts.CredentialOpts, credentials.WithRootCACert(func() ([][]byte, error) {
			var caCerts [][]byte
			for _, caCertFile := range caFiles {
				if strings.HasPrefix(caCertFile, pemPrefix) {
					caCerts = append(caCerts, []byte(caCertFile))
				} else {
					caCert, err := ioutil.ReadFile(caCertFile)
					if err != nil {
						return nil, err
					}
					caCerts = append(caCerts, caCert)
				}
			}
			return caCerts, nil
		}))
	}

	dialOpts.CredentialOpts = append(dialOpts.CredentialOpts, credentials.WithInsecureSkipVerify(config.Bool(internal.FlagInsecureSkipVerify)))
	dialOpts.CredentialOpts = append(dialOpts.CredentialOpts, credentials.WithInsecureDial(config.Bool(internal.FlagInsecure)))

	return dialOpts
}

func credentialOptsFromRegistry(target string, dialOpts *DialOptions) ([]credentials.ClientOptionFunc, error) {
	if dialOpts.Registry == nil || registry.IsNoopRegistry(dialOpts.Registry) {
		return nil, fmt.Errorf("CredentialOptsFromRegistry: no registry instance found")
	}

	entries, err := dialOpts.Registry.ListAllService()
	if err != nil {
		return nil, fmt.Errorf("CredentialOptsFromRegistry: try to get service info failed, err = %v", err)
	}

	for _, entry := range entries {
		if entry.ServiceName != target {
			continue
		}

		if len(entry.Instances) == 0 {
			return nil, fmt.Errorf("CredentialOptsFromRegistry: no service instance for %q found", entry.ServiceName)
		}

		jsonMeta, ok := entry.Instances[0].Meta.(string)
		if !ok {
			log.Debugf("CredentialOptsFromRegistry: meta of %q not found, treat service as no tls", entry.ServiceName)
			return []credentials.ClientOptionFunc{
				credentials.WithInsecureDial(true),
			}, nil
		}

		meta, err := registry.UnmarshalRegistryMeta(jsonMeta)
		if err != nil {
			log.Debugf("CredentialOptsFromRegistry: unmarshal meta of %q failed, err = %v, treat service as no tls", entry.ServiceName, err)
			return []credentials.ClientOptionFunc{
				credentials.WithInsecureDial(true),
			}, nil
		}

		if meta.TlsServer {
			log.Infof("CredentialOptsFromRegistry: use tls for %q, meta from registry = %+v", target, meta)
			return []credentials.ClientOptionFunc{
				credentials.WithInsecureSkipVerify(true),
				credentials.WithInsecureDial(false),
			}, nil
		} else {
			log.Infof("CredentialOptsFromRegistry: use none-tls for %q, meta from registry = %+v", target, meta)
			return []credentials.ClientOptionFunc{
				credentials.WithInsecureDial(true),
			}, nil
		}
	}

	return nil, fmt.Errorf("CredentialOptsFromRegistry: no entry of %q found", target)
}

func PrepareUnaryInterceptor(callOpts *CallOptions, ignoreInternalInterceptors bool) []grpc.UnaryClientInterceptor {
	unaryInt := callOpts.UnaryInt

	if callOpts.Tracer != nil {
		unaryInt = append(unaryInt, tracing.OpenTracingClientInterceptor(callOpts.Tracer, false))
	}

	if prometheusUnaryInt := prometheus.UnaryClientInterceptor(); prometheusUnaryInt != nil {
		unaryInt = append(unaryInt, prometheusUnaryInt)
	}

	if !ignoreInternalInterceptors {
		unaryInt = append(unaryInt, tracing.RequestIdTracingUnaryClientInterceptor)
		unaryInt = append(unaryInt, retry.UnaryClientInterceptor(*callOpts.TargetTransformer))
		unaryInt = append(unaryInt, timeout.UnaryClientInterceptor(*callOpts.TargetTransformer))
	}

	return unaryInt
}

func PrepareStreamInterceptor(callOpts *CallOptions, ignoreInternalInterceptors bool) []grpc.StreamClientInterceptor {
	streamInt := callOpts.StreamInt

	if callOpts.Tracer != nil {
		streamInt = append(streamInt, tracing.OpenTracingStreamClientInterceptor(callOpts.Tracer, false))
	}

	if prometheusStreamInt := prometheus.StreamClientInterceptor(); prometheusStreamInt != nil {
		streamInt = append(streamInt, prometheusStreamInt)
	}

	if !ignoreInternalInterceptors {
		streamInt = append(streamInt, tracing.RequestIdTracingStreamingClientInterceptor)
		streamInt = append(streamInt, retry.StreamClientInterceptor(*callOpts.TargetTransformer))
		streamInt = append(streamInt, timeout.StreamClientInterceptor(*callOpts.TargetTransformer))
	}

	return streamInt
}
