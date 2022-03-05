package dialer

import (
	"context"

	"google.golang.org/grpc/naming"

	"google.golang.org/grpc"

	"github.com/zhangel/go-framework/credentials"
	"github.com/zhangel/go-framework/dialer/internal/option"
	"github.com/zhangel/go-framework/registry"
)

func Dial(ctx context.Context, target string, dialOpts *option.DialOptions) (*grpc.ClientConn, error) {
	dialOpt := []grpc.DialOption{grpc.WithDisableRetry()}

	clientCredentialOpts, err := credentials.ParseClientOptions(append([]credentials.ClientOptionFunc{
		credentials.WithServerNameOverride(target),
		credentials.WithIgnoreServerCertExpiresCheck(true),
	}, dialOpts.CredentialOpts...)...)
	if err != nil {
		return nil, err
	}

	if credentialOpt, err := credentials.DialOptionWithOpts(clientCredentialOpts); err != nil {
		return nil, err
	} else {
		if extractor, ok := credentialOpt.(interface {
			ExtractDialOptions() []grpc.DialOption
		}); ok && extractor != nil {
			dialOpt = append(dialOpt, extractor.ExtractDialOptions()...)
		} else {
			dialOpt = append(dialOpt, credentialOpt)
		}
	}

	var resolver naming.Resolver
	if dialOpts != nil && dialOpts.Registry != nil {
		resolver = dialOpts.Registry.Resolver()
	}

	if dialOpts != nil && dialOpts.BalancerBuilder != nil && dialOpts.Registry != nil && resolver != nil {
		if resolverOpt, ok := resolver.(registry.ResolverOption); ok {
			if err := resolverOpt.WithOption(registry.ResolverWithFilter(registry.ResolveWithTls(!clientCredentialOpts.Insecure()))); err != nil {
				return nil, err
			}
		}

		dialOpt = append(dialOpt, grpc.WithBalancer(dialOpts.BalancerBuilder.Build(resolver)))

		target = registry.ServicePath(dialOpts.Registry, target)
	}

	if dialOpts != nil && dialOpts.Codec != nil {
		dialOpt = append(dialOpt, grpc.WithCodec(dialOpts.Codec))
	}

	if dialOpts != nil && dialOpts.DialFunc != nil {
		dialOpt = append(dialOpt, grpc.WithContextDialer(dialOpts.DialFunc))
	}

	if dialOpts != nil && dialOpts.ReadBufferSize != 0 {
		dialOpt = append(dialOpt, grpc.WithReadBufferSize(dialOpts.ReadBufferSize))
	}

	if dialOpts != nil && dialOpts.WriteBufferSize != 0 {
		dialOpt = append(dialOpt, grpc.WithWriteBufferSize(dialOpts.WriteBufferSize))
	}

	if dialOpts != nil && dialOpts.KeepAliveParameters != nil {
		dialOpt = append(dialOpt, grpc.WithKeepaliveParams(*dialOpts.KeepAliveParameters))
	}

	cc, err := grpc.DialContext(ctx, target, dialOpt...)
	return cc, err
}
