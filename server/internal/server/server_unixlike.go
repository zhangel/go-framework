// +build !windows

package server

import (
	"net"
)

func Listen(addr string, minPort, maxPort int) (net.Listener, error) {
	return tcpListen(addr, minPort, maxPort)
}

func defaultAddress() string {
	return ""
}
