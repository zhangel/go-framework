package picker

import "google.golang.org/grpc"

type AddressInfo interface {
	Address() grpc.Address
	Connected() bool
}
