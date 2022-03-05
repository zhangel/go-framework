package server_list

import "sync"

var (
	mu             sync.RWMutex
	grpcServerList []string
	httpServerList []string
)

func GrpcServerList() []string {
	mu.RLock()
	defer mu.RUnlock()
	return grpcServerList
}

func HttpServerList() []string {
	mu.RLock()
	defer mu.RUnlock()
	return httpServerList
}

func AddGrpcServer(addr string) {
	mu.Lock()
	defer mu.Unlock()

	grpcServerList = append(grpcServerList, addr)
}

func RemoveGrpcServer(addr string) {
	mu.Lock()
	defer mu.Unlock()

	var l []string
	for _, a := range grpcServerList {
		if a == addr {
			continue
		}
		l = append(l, a)
	}
	grpcServerList = l
}

func AddHttpServer(addr string) {
	mu.Lock()
	defer mu.Unlock()

	httpServerList = append(httpServerList, addr)
}

func RemoveHttpServer(addr string) {
	mu.Lock()
	defer mu.Unlock()

	var l []string
	for _, a := range httpServerList {
		if a == addr {
			continue
		}
		l = append(l, a)
	}
	httpServerList = l
}
