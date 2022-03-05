package credentials

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"time"

	"google.golang.org/grpc/credentials"

	"github.com/zhangel/go-framework.git/certificate"
)

var certStore = certificate.NewDefaultStore()
var issuerPool = NewIssuerPool()

func init() {
	if issuer, err := certificate.NewIssuer(
		certificate.IssuerWithCAProvider(certificate.SelfSignedCACertProvider()),
	); err == nil {
		issuerPool.Put(nil, nil, nil, issuer)
	}
}

type ClientOptions struct {
	rootCAs             func() ([][]byte, error)
	certificate         func(info *tls.CertificateRequestInfo) (*tls.Certificate, error)
	serverName          string
	insecure            bool
	insecureSkipVerify  bool
	certificateVerifier []func(rawCerts [][]byte, ignoreExpiresCheck bool) ([][]*x509.Certificate, error)
	verifyCertificate   func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error
	ignoreExpiresCheck  bool
	perRPCCredentials   credentials.PerRPCCredentials
}

type ClientOptionFunc func(*ClientOptions) error

func (s *ClientOptions) Insecure() bool {
	return s.insecure
}

func WithPerRPCCredentials(cred credentials.PerRPCCredentials) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		opts.perRPCCredentials = cred
		return nil
	}
}

func WithRootCACertFile(caCertFiles []string) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		var caCerts [][]byte
		for _, caCertFile := range caCertFiles {
			caCert, err := ioutil.ReadFile(caCertFile)
			if err != nil {
				return err
			}
			caCerts = append(caCerts, caCert)
		}
		return WithRootCACert(func() ([][]byte, error) { return caCerts, nil })(opts)
	}
}

func WithRootCACert(caCertFunc func() ([][]byte, error)) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		opts.rootCAs = caCertFunc
		return nil
	}
}

func WithClientCertificateFile(certFile, keyFile string) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		cert, err := ioutil.ReadFile(certFile)
		if err != nil {
			return err
		}

		key, err := ioutil.ReadFile(keyFile)
		if err != nil {
			return err
		}

		x509Cert, err := tls.X509KeyPair(cert, key)
		if err != nil {
			return err
		}

		return WithClientCertificate(func(info *tls.CertificateRequestInfo) (*tls.Certificate, error) { return &x509Cert, nil })(opts)
	}
}

func WithClientCertificate(certificate func(info *tls.CertificateRequestInfo) (*tls.Certificate, error)) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		if err := WithInsecureDial(false)(opts); err != nil {
			return err
		}

		opts.certificate = certificate
		return nil
	}
}

func WithClientCertificateFactory(factory *certificate.Factory, request certificate.Request, autoRenewal bool) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		return WithClientCertificate(func(info *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			var renewalHook func(*tls.Certificate, *certificate.Info, error) certificate.RenewalAction
			if autoRenewal {
				renewalHook = certificate.DefaultRenewalPolicy(72 * time.Hour)
			} else {
				renewalHook = certificate.NoRenewalPolicy()
			}

			return factory.Fetch(request, renewalHook)
		})(opts)
	}
}
func WithServerNameOverride(serverNameOverride string) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		opts.serverName = serverNameOverride
		return nil
	}
}

func WithInsecureDial(insecure bool) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		opts.insecure = insecure
		return nil
	}
}

func WithInsecureSkipVerify(insecureSkipVerify bool) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		opts.insecureSkipVerify = insecureSkipVerify
		return nil
	}
}

func WithVerifyServerAuthority(rootCAs func() ([][]byte, error), authorityChecker func(chainLength, current int, certificate *x509.Certificate) error) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		opts.certificateVerifier = append(opts.certificateVerifier, certificateChainVerifier(rootCAs, authorityChecker, nil))
		return nil
	}
}

func WithVerifyServerCertificate(verifyCertificate func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		opts.verifyCertificate = verifyCertificate
		return nil
	}
}

func WithIgnoreServerCertExpiresCheck(ignoreExpiresCheck bool) ClientOptionFunc {
	return func(opts *ClientOptions) error {
		opts.ignoreExpiresCheck = ignoreExpiresCheck
		return nil
	}
}

type ServerOptions struct {
	clientCAs                func() ([][]byte, error)
	certificate              func(info *tls.ClientHelloInfo) (*tls.Certificate, error)
	insecure                 bool
	clientAuthType           tls.ClientAuthType
	certificateChainVerifier []func(rawCerts [][]byte, ignoreExpiresCheck bool) ([][]*x509.Certificate, error)
	verifyCertificate        func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error
	ignoreExpiresCheck       bool
}

type ServerOptionFunc func(*ServerOptions) error

func WithClientCACertFile(caCertFiles []string) ServerOptionFunc {
	return func(opts *ServerOptions) error {
		var caCerts [][]byte
		for _, caCertFile := range caCertFiles {
			caCert, err := ioutil.ReadFile(caCertFile)
			if err != nil {
				return err
			}
			caCerts = append(caCerts, caCert)
		}
		return WithClientCACert(func() ([][]byte, error) { return caCerts, nil })(opts)
	}
}

func WithClientCACert(caCertFunc func() ([][]byte, error)) ServerOptionFunc {
	return func(opts *ServerOptions) error {
		opts.clientCAs = caCertFunc
		return nil
	}
}

func WithServerCertificateFile(certFile, keyFile string) ServerOptionFunc {
	return func(opts *ServerOptions) error {
		cert, err := ioutil.ReadFile(certFile)
		if err != nil {
			return err
		}

		key, err := ioutil.ReadFile(keyFile)
		if err != nil {
			return err
		}

		tlsCert, err := tls.X509KeyPair(cert, key)
		if err != nil {
			return err
		}

		return WithServerCertificate(func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &tlsCert, nil
		})(opts)
	}
}

func WithServerCertificate(certificate func(info *tls.ClientHelloInfo) (*tls.Certificate, error)) ServerOptionFunc {
	return func(opts *ServerOptions) error {
		if err := WithInsecureListen(false)(opts); err != nil {
			return err
		}

		opts.certificate = certificate
		return nil
	}
}

func WithInsecureListen(insecure bool) ServerOptionFunc {
	return func(opts *ServerOptions) error {
		opts.insecure = insecure
		return nil
	}
}

func WithClientAuthType(clientAuthType tls.ClientAuthType) ServerOptionFunc {
	return func(opts *ServerOptions) error {
		opts.clientAuthType = clientAuthType
		return nil
	}
}

func WithVerifyClientAuthority(clientCAs func() ([][]byte, error), authorityChecker func(chainLength, current int, certificate *x509.Certificate) error) ServerOptionFunc {
	return func(opts *ServerOptions) error {
		opts.certificateChainVerifier = append(opts.certificateChainVerifier, certificateChainVerifier(clientCAs, authorityChecker, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}))
		return nil
	}
}

func WithVerifyClientCertificate(verifyCertificate func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error) ServerOptionFunc {
	return func(opts *ServerOptions) error {
		opts.verifyCertificate = verifyCertificate
		return nil
	}
}

func WithIgnoreClientCertExpiresCheck(ignoreExpiresCheck bool) ServerOptionFunc {
	return func(opts *ServerOptions) error {
		opts.ignoreExpiresCheck = ignoreExpiresCheck
		return nil
	}
}

func WithSelfSignedCertificate(SAN []string) ServerOptionFunc {
	return WithSelfSignedCertificateWithCACert(SAN, true, nil, nil, nil)
}

func WithSelfSignedCertificateWithCACert(SAN []string, autoRenewal bool, caCert, caKey, passphrase []byte) ServerOptionFunc {
	SAN = GenerateSAN(SAN)
	request := certificate.NewCertRequest(SAN[0], SAN, SAN, true)
	return func(opts *ServerOptions) error {
		issuer := issuerPool.Get(caCert, caKey, passphrase)
		if issuer == nil {
			return fmt.Errorf("issuer for self signed certificate not found")
		}

		factory, err := certificate.NewCertificateFactory(issuer, certStore)
		if err != nil {
			return err
		}

		if _, err := factory.Fetch(request, nil); err != nil {
			return err
		}

		return WithServerCertificateFactory(factory, request, autoRenewal)(opts)
	}
}

func WithServerCertificateFactory(factory *certificate.Factory, request certificate.Request, autoRenewal bool) ServerOptionFunc {
	return func(opts *ServerOptions) error {
		return WithServerCertificate(func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			var renewalHook func(*tls.Certificate, *certificate.Info, error) certificate.RenewalAction
			if autoRenewal {
				renewalHook = certificate.DefaultRenewalPolicy(72 * time.Hour)
			} else {
				renewalHook = certificate.NoRenewalPolicy()
			}

			return factory.Fetch(request, renewalHook)
		})(opts)
	}
}
