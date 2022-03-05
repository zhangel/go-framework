package registry

func ResolveWithTls(tls bool) func(serviceName string, instance Instance) bool {
	return func(serviceName string, instance Instance) bool {
		if jsonMeta, ok := instance.Meta.(string); !ok {
			return true
		} else if meta, err := UnmarshalRegistryMeta(jsonMeta); err != nil {
			return true
		} else {
			return meta.TlsServer == tls
		}
	}
}
