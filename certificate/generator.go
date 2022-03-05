package certificate

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/zhangel/go-framework/pkix"
)

type CreateCertOptions struct {
	caProvider    func() (caCert, caKey, passphrase []byte, err error)
	domainList    []string
	ipList        []string
	validDuration time.Duration
	bits          int
	certChain     bool
	commonName    string
}

type CreateCertOpt func(options *CreateCertOptions) error

func CreateCertWithDomainList(domainList ...string) func(options *CreateCertOptions) error {
	return func(options *CreateCertOptions) error {
		options.domainList = domainList
		return nil
	}
}

func CreateCertWithIpList(ipList ...string) func(options *CreateCertOptions) error {
	return func(options *CreateCertOptions) error {
		options.ipList = ipList
		return nil
	}
}

func CreateCertWithCAProvider(provider func() (caCert, caKey, passphrase []byte, err error)) func(options *CreateCertOptions) error {
	return func(options *CreateCertOptions) error {
		options.caProvider = provider
		return nil
	}
}

func CreateCertWithValidDuration(validDuration time.Duration) func(options *CreateCertOptions) error {
	return func(options *CreateCertOptions) error {
		options.validDuration = validDuration
		return nil
	}
}

func CreateCertWithPubKeyBits(bits int) func(options *CreateCertOptions) error {
	return func(options *CreateCertOptions) error {
		options.bits = bits
		return nil
	}
}

func CreateCertWithCertChain(certChain bool) func(options *CreateCertOptions) error {
	return func(options *CreateCertOptions) error {
		options.certChain = certChain
		return nil
	}
}

func CreateCertWithCommonName(commonName string) func(options *CreateCertOptions) error {
	return func(options *CreateCertOptions) error {
		options.commonName = commonName
		return nil
	}
}

func CreateCertPEM(opt ...CreateCertOpt) ([]byte, []byte, error) {
	opts := &CreateCertOptions{
		validDuration: time.Hour * 24 * 365 * 2,
		bits:          2048,
		certChain:     false,
	}

	for _, o := range opt {
		if err := o(opts); err != nil {
			return nil, nil, fmt.Errorf("Credentials::CreateCert, apply options failed, err = %v", err)
		}
	}

	if opts.caProvider == nil {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, no CA provider found")
	}

	cert, key, passphrase, err := opts.caProvider()
	if err != nil {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, intermediate CA provider returns error, err = %v", err)
	}

	if len(cert) == 0 {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, intermediate CA certificate is nil")
	}

	if len(key) == 0 {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, intermediate CA key is nil")
	}

	var ipList []net.IP
	for _, n := range opts.ipList {
		if ip := net.ParseIP(n); ip != nil {
			ipList = append(ipList, ip)
		}
	}

	rsaKey, err := pkix.CreateRSAKey(opts.bits)
	if err != nil {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, create RSA key failed, err = %v", err)
	}

	csr, err := pkix.CreateCertificateSigningRequest(rsaKey, "", ipList, opts.domainList, nil, "QAX", "CN", "", "", opts.commonName)
	if err != nil {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, create RSA certificate signing request failed, err = %v", err)
	}

	caCert, err := pkix.NewCertificateFromPEM(cert)
	if err != nil {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, create CertificateFromPEM failed, err = %v", err)
	}

	rawCaCert, err := caCert.GetRawCertificate()
	if err != nil {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, get Raw certificate failed, err = %v", err)
	}

	if !rawCaCert.IsCA {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, selected CA certificate is not allowed to sign certificates")
	}

	var caKey *pkix.Key
	if len(passphrase) == 0 {
		caKey, err = pkix.NewKeyFromPrivateKeyPEM(key)
	} else {
		caKey, err = pkix.NewKeyFromEncryptedPrivateKeyPEM(key, passphrase)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, get CA key failed, err = %v", err)
	}

	crt, err := pkix.CreateCertificateHost(caCert, caKey, csr, time.Now().Add(opts.validDuration))
	if err != nil {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, create Certificate host failed, err = %v", err)
	}

	crtExport, err := crt.Export()
	if err != nil {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, export certificate failed, err = %v", err)
	}

	keyExport, err := rsaKey.ExportPrivate()
	if err != nil {
		return nil, nil, fmt.Errorf("Credentials::CreateCert, export key failed, err = %v", err)
	}

	if !opts.certChain {
		return crtExport, keyExport, nil
	} else {
		return append(crtExport, cert...), keyExport, nil
	}
}

func CreateCert(opt ...CreateCertOpt) (*tls.Certificate, error) {
	cert, key, err := CreateCertPEM(opt...)
	if err != nil {
		return nil, err
	}

	certificate, err := tls.X509KeyPair(cert, key)
	return &certificate, err
}
