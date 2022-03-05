package registry

import "google.golang.org/grpc/naming"

type _NoopRegistry struct{}

func (*_NoopRegistry) RegisterService(serviceName string, addr string, meta interface{}) error {
	return nil
}

func (*_NoopRegistry) UnregisterService(serviceName string, addr string) error {
	return nil
}

func (*_NoopRegistry) Resolver() naming.Resolver {
	return nil
}

func (*_NoopRegistry) HasService(serviceName string) (bool, error) {
	return false, nil
}

func (*_NoopRegistry) ListServiceAddresses(serviceName string) ([]string, error) {
	return []string{}, nil
}

func (*_NoopRegistry) ListServiceByPrefix(prefix string) (map[string][]string, error) {
	return map[string][]string{}, nil
}

func (*_NoopRegistry) ListAllService() ([]Entry, error) {
	return nil, nil
}

func (*_NoopRegistry) WatchAllServiceChanges(watcher Watcher) (cancel func(), err error) {
	return func() {}, nil
}

func (*_NoopRegistry) Noop() {}
