package connection

import (
	"context"
	"testing"

	"google.golang.org/grpc"
)

func TestMakeConnection(t *testing.T) {
	_, err := MakeConnection("Hello", func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(ctx, method, req, reply, cc, opts...)
	}, func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(ctx, desc, cc, method, opts...)
	})

	if err != nil {
		t.Fatal(err)
	}
}
