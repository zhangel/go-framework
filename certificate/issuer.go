package certificate

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/zhangel/go-framework.git/pkix"
)

//go:generate mockery -name Issuer -inpkg -case underscore
type Issuer interface {
	Issue(request Request, receiver func(*tls.Certificate, error))
}

type CAInfoProvider interface {
	CAInfo() string
}

type issuer struct {
	opts issuerOpts
}

type issuerOpts struct {
	caProvider    func() (caCert, caKey, passphrase []byte, err error)
	validDuration time.Duration
}

type IssuerOption func(opts *issuerOpts) error

func IssuerWithCAProvider(caProvider func() (caCert, caKey, passphrase []byte, err error)) func(opts *issuerOpts) error {
	return func(opts *issuerOpts) error {
		if caProvider == nil {
			return fmt.Errorf("ca provider is nil")
		}

		opts.caProvider = caProvider
		return nil
	}
}

func IssuerWithValidDuration(validDuration time.Duration) func(opts *issuerOpts) error {
	return func(opts *issuerOpts) error {
		if validDuration <= 0 {
			return fmt.Errorf("valid duration of certificate must larger than 0")
		}

		if validDuration > 2*365*24*time.Hour {
			return fmt.Errorf("valid duration of certificate too long, 2 years allowed at max")
		}

		opts.validDuration = validDuration
		return nil
	}
}

func NewIssuer(opt ...IssuerOption) (Issuer, error) {
	result := &issuer{
		opts: issuerOpts{
			validDuration: 2 * 365 * 24 * time.Hour,
		},
	}
	for _, o := range opt {
		if err := o(&result.opts); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (s *issuer) Issue(request Request, receiver func(*tls.Certificate, error)) {
	cert, err := CreateCert(
		CreateCertWithCAProvider(s.opts.caProvider),
		CreateCertWithValidDuration(s.opts.validDuration),
		CreateCertWithCommonName(request.CommonName()),
		CreateCertWithDomainList(request.DomainSans()...),
		CreateCertWithIpList(request.IpSans()...),
		CreateCertWithCertChain(request.CertChain()),
		CreateCertWithPubKeyBits(request.PubkeyBits()),
	)

	receiver(cert, err)
}

func (s *issuer) CAInfo() string {
	if s.opts.caProvider != nil {
		cert, _, _, err := s.opts.caProvider()
		if err != nil {
			return fmt.Sprintf("CA-Provider failed, err = %v", err)
		}
		caCert, err := pkix.NewCertificateFromPEM(cert)
		if err != nil {
			return fmt.Sprintf("Parse cert failed, err = %v", err)
		}
		rawCert, err := caCert.GetRawCertificate()
		if err != nil {
			return fmt.Sprintf("Get RAW cert failed, err = %v", err)
		}

		return fmt.Sprintf("RootCA.CN = %q, CN = %q, %v - %v", rawCert.Issuer.CommonName, rawCert.Subject.CommonName, rawCert.NotBefore.Local().Format("2006-01-02 15:04:05"), rawCert.NotAfter.Local().Format("2006-01-02 15:04:05"))
	}

	return "no ca-provider found"
}

func SelfSignedCACertProvider() func() (caCert, caKey, passphrase []byte, err error) {
	return func() (caCert, caKey, passphrase []byte, err error) {
		return []byte(selfSignedCACert), []byte(selfSignedCAKey), nil, nil
	}
}
