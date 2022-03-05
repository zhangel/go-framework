package hooks

import (
	"context"

	"github.com/modern-go/reflect2"
	"google.golang.org/grpc"
)

var (
	reflectMField           reflect2.StructField
	reflectMServerField     reflect2.StructField
	reflectMMethodDescField reflect2.StructField
	reflectMStreamDescField reflect2.StructField
)

func init() {
	srv := &grpc.Server{}
	reflectSrvType := reflect2.TypeOf(srv).(reflect2.PtrType).Elem().(reflect2.StructType)
	reflectMField = reflectSrvType.FieldByName("m")
	reflectMType := reflect2.TypeOf(reflectMField.Get(srv)).(reflect2.PtrType).Elem().(reflect2.MapType)
	reflectMValType := reflectMType.Elem().(reflect2.PtrType).Elem().(reflect2.StructType)
	reflectMServerField = reflectMValType.FieldByName("server")
	reflectMMethodDescField = reflectMValType.FieldByName("md")
	reflectMStreamDescField = reflectMValType.FieldByName("sd")

	if reflectMField == nil || reflectMServerField == nil || reflectMMethodDescField == nil || reflectMStreamDescField == nil {
		panic("Incompatible grpc, replace google.golang.org/grpc => google.golang.org/grpc v1.26.0 in go.mod !")
	}
}

type UnaryHookHandler func(srv interface{}, ctx context.Context) (interface{}, error)
type UnaryHookInterceptor func(srv interface{}, ctx context.Context, service, method string, handler UnaryHookHandler) (interface{}, error)
type StreamHookHandler func(srv interface{}, stream grpc.ServerStream) error
type StreamHookInterceptor func(srv interface{}, service, method string, stream grpc.ServerStream, handler StreamHookHandler) error

func ChainUnaryHook(interceptors ...UnaryHookInterceptor) UnaryHookInterceptor {
	return func(srv interface{}, ctx context.Context, service, method string, handler UnaryHookHandler) (resp interface{}, err error) {
		for i := len(interceptors) - 1; i >= 0; i-- {
			func(unaryInt UnaryHookInterceptor) {
				prevHandler := handler
				handler = func(srv interface{}, ctx context.Context) (resp interface{}, err error) {
					return unaryInt(srv, ctx, service, method, prevHandler)
				}
			}(interceptors[i])
		}

		return handler(srv, ctx)
	}
}

func ChainStreamHook(interceptors ...StreamHookInterceptor) StreamHookInterceptor {
	return func(srv interface{}, service, method string, stream grpc.ServerStream, handler StreamHookHandler) (err error) {
		for i := len(interceptors) - 1; i >= 0; i-- {
			func(streamInt StreamHookInterceptor) {
				prevHandler := handler
				handler = func(srv interface{}, stream grpc.ServerStream) (err error) {
					return streamInt(srv, service, method, stream, prevHandler)
				}
			}(interceptors[i])
		}

		return handler(srv, stream)
	}
}

type hookOptions struct {
	unaryInt  []UnaryHookInterceptor
	streamInt []StreamHookInterceptor
}

type HookOption func(options *hookOptions) error

func HookWithUnaryInterceptors(unaryInt ...UnaryHookInterceptor) HookOption {
	return func(opts *hookOptions) error {
		opts.unaryInt = append(opts.unaryInt, unaryInt...)
		return nil
	}
}

func HookWithStreamInterceptors(streamInt ...StreamHookInterceptor) HookOption {
	return func(opts *hookOptions) error {
		opts.streamInt = append(opts.streamInt, streamInt...)
		return nil
	}
}

func HookServer(srv *grpc.Server, opt ...HookOption) error {
	mField := reflectMField.Get(srv)
	mType := reflect2.TypeOf(mField).(reflect2.PtrType).Elem().(reflect2.MapType)

	var opts hookOptions
	for _, o := range opt {
		if err := o(&opts); err != nil {
			return err
		}
	}

	var unaryInt UnaryHookInterceptor
	if len(opts.unaryInt) > 0 {
		unaryInt = ChainUnaryHook(opts.unaryInt...)
	}

	var streamInt StreamHookInterceptor
	if len(opts.streamInt) > 0 {
		streamInt = ChainStreamHook(opts.streamInt...)
	}

	for iter := mType.Iterate(mField); iter.HasNext(); {
		k, v := iter.Next()

		v = reflect2.TypeOf(v).(reflect2.PtrType).Elem().Indirect(v)
		server := reflectMServerField.Get(v)
		server = reflect2.TypeOf(server).(reflect2.PtrType).Elem().Indirect(server)
		md := reflectMMethodDescField.Get(v).(*map[string]*grpc.MethodDesc)
		sd := reflectMStreamDescField.Get(v).(*map[string]*grpc.StreamDesc)

		for mdK, mdV := range *md {
			handler := (*mdV).Handler
			(*mdV).Handler = func(method string) func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
				return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
					if unaryInt != nil {
						return unaryInt(srv, ctx, *(k.(*string)), method, func(srv interface{}, ctx context.Context) (interface{}, error) {
							return handler(srv, ctx, dec, interceptor)
						})
					} else {
						return handler(srv, ctx, dec, interceptor)
					}
				}
			}(mdK)
		}

		for sdK, sdV := range *sd {
			handler := (*sdV).Handler
			(*sdV).Handler = func(method string) func(srv interface{}, stream grpc.ServerStream) error {
				return func(srv interface{}, stream grpc.ServerStream) error {
					if streamInt != nil {
						return streamInt(srv, *(k.(*string)), method, stream, func(srv interface{}, stream grpc.ServerStream) error {
							return handler(srv, stream)
						})
					} else {
						return handler(srv, stream)
					}
				}
			}(sdK)
		}
	}

	return nil
}
