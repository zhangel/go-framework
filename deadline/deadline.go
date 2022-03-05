package deadline

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func DeadlineCheckerUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if err := CheckContext(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func CheckContext(ctx context.Context) error {
	switch ctx.Err() {
	case context.Canceled:
		return status.Newf(codes.Canceled, "Interrupted by deadline checker[context is canceled], ctx = %v", ctx).Err()
	case context.DeadlineExceeded:
		return status.Newf(codes.DeadlineExceeded, "Interrupted by deadline checker[context deadline is exceeded], ctx = %v", ctx).Err()
	case nil:
		return nil
	default:
		return status.Newf(codes.Internal, "Interrupted by deadline checker[caught context error], ctx = %v, ctx.Err = %v", ctx, ctx.Err()).Err()
	}
}
