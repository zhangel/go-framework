package async

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const AsyncAddr = "async.grpc.proxy"

func AsyncUnaryClientInterceptor(target string, asyncOpts Options) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}

		md.Set(targetKey, target)
		asyncOpts.applyToMetadata(md)

		ctx = metadata.NewOutgoingContext(ctx, md)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
