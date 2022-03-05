package internal

import "google.golang.org/grpc"

type AddressInfo struct {
	addr      grpc.Address
	connected bool
}

func (s *AddressInfo) Address() grpc.Address {
	return s.addr
}

func (s *AddressInfo) Connected() bool {
	return s.connected
}
