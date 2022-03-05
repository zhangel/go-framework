package internal

const (
	DialerPrefix                   string = "dialer"
	FlagConnTimeout                string = "dialer.timeout"
	FlagDialInMemory               string = "dialer.in_memory"
	FlagInMemoryMaxConcurrent      string = "dialer.in_memory.max_concurrent_stream"
	FlagIgnoreProxyEnv             string = "dialer.no_proxy"
	FlagInsecure                   string = "dialer.insecure"
	FlagCert                       string = "dialer.tls.cert"
	FlagKey                        string = "dialer.tls.key"
	FlagCaCert                     string = "dialer.tls.cacert"
	FlagInsecureSkipVerify         string = "dialer.tls.insecure_skip_verify"
	FlagUseServerCert              string = "dialer.tls.use_server_cert"
	FlagIgnoreInternalInterceptors string = "dialer.no_internal_interceptors"
)
