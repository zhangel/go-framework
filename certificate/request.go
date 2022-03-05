package certificate

import "fmt"

type Request interface {
	Id() string
	CommonName() string
	IpSans() []string
	DomainSans() []string
	CertChain() bool
	PubkeyBits() int
}

type defaultCertRequest struct {
	commonName string
	ips        []string
	domains    []string
	certChain  bool
}

func NewCertRequest(commonName string, ips, domains []string, certChain bool) *defaultCertRequest {
	return &defaultCertRequest{
		commonName: commonName,
		ips:        ips,
		domains:    domains,
		certChain:  certChain,
	}
}

func (s *defaultCertRequest) Id() string {
	return s.commonName
}

func (s *defaultCertRequest) CommonName() string {
	return s.commonName
}

func (s *defaultCertRequest) IpSans() []string {
	return s.ips
}

func (s *defaultCertRequest) DomainSans() []string {
	return s.domains
}

func (s *defaultCertRequest) CertChain() bool {
	return s.certChain
}

func (s *defaultCertRequest) PubkeyBits() int {
	return 2048
}

func (s *defaultCertRequest) String() string {
	return fmt.Sprintf("CN = %s, DNSNames = %+v, IPAddresses = %+v, CertChain = %v", s.commonName, s.domains, s.ips, s.certChain)
}
