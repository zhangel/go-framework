package option

import (
	"crypto/tls"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/zhangel/go-framework.git/server/internal/grpc_reflection"

	"github.com/zhangel/go-framework.git/authentication"
	"github.com/zhangel/go-framework.git/config"
	"github.com/zhangel/go-framework.git/credentials"
	"github.com/zhangel/go-framework.git/healthy"
	"github.com/zhangel/go-framework.git/log"
	"github.com/zhangel/go-framework.git/registry"
	"github.com/zhangel/go-framework.git/server/internal"
	"github.com/zhangel/go-framework.git/tracing"
)

const pemPrefix = "-----BEGIN"

func ParseOptions(opts ...Option) *Options {
	options := append([]Option{},
		WithAddr(config.String(internal.FlagAddr)),
		WithHttpAddr(config.String(internal.FlagHttpAddr)),
		WithRegistry(registry.DefaultRegistry()),
		WithTracer(tracing.DefaultTracer()),
		WithRecovery(config.Bool(internal.FlagRecovery)),
		WithEnableHttpServer(config.Bool(internal.FlagHttpServer)),
		WithEnableHttpGzip(config.Bool(internal.FlagHttpGzip)),
		WithJsonOrigNameOption(config.Bool(internal.FlagOrigName)),
		WithJsonEnumAsIntsOption(config.Bool(internal.FlagEnumAsInts)),
		WithJsonEmitDefaultsOption(config.Bool(internal.FlagEmitDefaults)),
		WithPackageName(config.Bool(internal.FlagWithPackageName)),
		WithEnableGrpcWeb(config.Bool(internal.FlagGrpcWeb)),
		WithEnableGrpcWebWithWs(config.Bool(internal.FlagGrpcWebWithWs)),
		WithGrpcGatewayCompatible(config.Bool(internal.FlagWithGrpcGatewayCompatible)),
		WithAllowedOrigins(config.StringList(internal.FlagAllowedOrigins)),
		WithAllowedRequestHeaders(config.StringList(internal.FlagAllowedRequestHeaders)),
		WithAllowedMethods(config.StringList(internal.FlagAllowedMethods)),
		WithAuthentication(authentication.DefaultProvider()),
		WithIgnoreInternalInterceptors(config.Bool(internal.FlagIgnoreInternalInterceptors)),
		WithHttpMaxConcurrentConnection(config.Int(internal.FlagMaxHttpConcurrentConnection)),
		WithHttpReadTimeout(config.Duration(internal.FlagHttpReadTimeout)),
		WithHttpWriteTimeout(config.Duration(internal.FlagHttpWriteTimeout)),
		WithHttpIdleTimeout(config.Duration(internal.FlagHttpIdleTimeout)),
		WithHttpKeepAlive(config.Bool(internal.FlagHttpKeepAlive)),
		WithPortRange(config.String(internal.FlagPortRange)),
		WithLegacyHttpRegistry(config.Bool(internal.FlagLegacyHttpRegistry)),
		WithCorsLogger(config.Bool(internal.FlagCorsLogger)),
		WithGrpcServerReflection(config.Bool(internal.FlagServerReflection)),
	)

	if config.String(internal.FlagServerReflectionToken) != "" {
		options = append(options, WithGrpcServerReflectionAuth(grpc_reflection.GrpcReflectionTokenAuth(config.String(internal.FlagServerReflectionToken))))
	}

	if config.Bool(internal.FlagHealthServiceEnable) {
		options = append(options, WithHealthService(healthy.NewDefaultHealthService()))
	}

	insecure := config.Bool(internal.FlagInsecure)
	options = append(options, WithCredentialsOption(credentials.WithInsecureListen(insecure)))
	options = append(options, WithCredentialsOption(credentials.WithIgnoreClientCertExpiresCheck(!config.Bool(internal.FlagCheckClientCertExpiration))))

	selfCertSAN := config.StringList(internal.FlagSelfSignCertSAN)
	hasCert := false
	if len(selfCertSAN) != 0 {
		hasCert = true
		options = append(options, WithCredentialsOption(credentials.WithSelfSignedCertificate(selfCertSAN)))
	} else {
		certFile := config.String(internal.FlagCert)
		keyFile := config.String(internal.FlagKey)
		if certFile != "" && keyFile != "" {
			hasCert = true
			if strings.HasPrefix(certFile, pemPrefix) && strings.HasPrefix(keyFile, pemPrefix) {
				options = append(options, WithCredentialsOption(credentials.WithServerCertificate(func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
					certificate, err := tls.X509KeyPair([]byte(certFile), []byte(keyFile))
					if err != nil {
						return nil, err
					}
					return &certificate, nil
				})))
			} else {
				options = append(options, WithCredentialsOption(credentials.WithServerCertificateFile(certFile, keyFile)))
			}
		}
	}

	caFiles := config.StringList(internal.FlagCaCert)

	var GetClientCA func() ([][]byte, error)
	if len(caFiles) > 0 {
		var caCerts [][]byte
		for _, caCertFile := range caFiles {
			if strings.HasPrefix(caCertFile, pemPrefix) {
				caCerts = append(caCerts, []byte(caCertFile))
			} else {
				caCert, err := ioutil.ReadFile(caCertFile)
				if err != nil {
					log.Fatalf("[ERROR] Read CA certificate failed, err = %v", err)
				}
				caCerts = append(caCerts, caCert)
			}
		}

		GetClientCA = func() ([][]byte, error) {
			return caCerts, nil
		}
		options = append(options, WithCredentialsOption(credentials.WithClientCACert(GetClientCA)))

		clientAuthType := config.Int(internal.FlagClientAuthType)
		if clientAuthType < 0 || clientAuthType > int(tls.RequireAndVerifyClientCert) {
			log.Fatal("[ERROR] Invalid client auth type.")
		}
		options = append(options, WithCredentialsOption(credentials.WithClientAuthType(tls.ClientAuthType(clientAuthType))))
		options = append(options, WithCredentialsOption(credentials.WithVerifyClientAuthority(GetClientCA, credentials.LeafAuthorityChecker(func() []string { return config.StringList(internal.FlagClientAuthorities) }))))
	}

	if !insecure && !hasCert {
		options = append(options, WithCredentialsOption(credentials.WithSelfSignedCertificate(nil)))
	}
	options = append(options, opts...)

	result := &Options{
		HttpKeepAlive:    true,
		HttpReadTimeout:  30 * time.Second,
		HttpWriteTimeout: 30 * time.Second,
		HttpIdleTimeout:  90 * time.Second,
	}

	for _, opt := range options {
		if err := opt(result); err != nil {
			log.Fatal("[ERROR]", err)
		}
	}

	if (result.Addr == result.HttpAddr && !IsAutoPort(result.HttpAddr)) || result.HttpAddr == internal.MultiplexServerAddr {
		result.MultiplexAddr = true
	} else {
		result.MultiplexAddr = false
	}

	return result
}

func IsAutoPort(addr string) bool {
	if strings.TrimSpace(strings.ToLower(addr)) == internal.AutoSelectAddr {
		return true
	}

	if !strings.Contains(addr, ":") {
		return true
	}

	_, port, err := net.SplitHostPort(addr)
	if err == nil && port == "0" {
		return true
	} else {
		return false
	}
}
