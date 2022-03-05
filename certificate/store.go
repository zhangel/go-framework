package certificate

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sync"
	"time"

	"github.com/zhangel/go-framework.git/log"
)

type Store interface {
	Store(issuer Issuer, request Request, cert *tls.Certificate) error
	Get(issuer Issuer, request Request) (*tls.Certificate, *Info, error)
}

type Info struct {
	NotBefore time.Time
	NotAfter  time.Time
}

type certificateVal struct {
	request Request
	cert    *tls.Certificate
	info    *Info
}

type DefaultStore struct {
	mu      sync.RWMutex
	certMap map[Issuer]map[string]*certificateVal
}

func NewDefaultStore() *DefaultStore {
	return &DefaultStore{
		certMap: make(map[Issuer]map[string]*certificateVal),
	}
}

func (s *DefaultStore) Store(issuer Issuer, request Request, cert *tls.Certificate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idMap, ok := s.certMap[issuer]
	if !ok {
		idMap = make(map[string]*certificateVal)
		s.certMap[issuer] = idMap
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return err
	}

	idMap[request.Id()] = &certificateVal{
		request: request,
		cert:    cert,
		info: &Info{
			NotBefore: x509Cert.NotBefore,
			NotAfter:  x509Cert.NotAfter,
		},
	}

	caInfo := ""
	if infoProvider, ok := issuer.(CAInfoProvider); ok {
		caInfo = infoProvider.CAInfo()
	}

	log.Infof("CertificateStore, update certificate from CA [%s] with request %+v, CN = %s, expiry from %v to %v",
		caInfo, request, x509Cert.Subject.CommonName, x509Cert.NotBefore.Local().Format("2006-01-02 15:04:05"), x509Cert.NotAfter.Local().Format("2006-01-02 15:04:05"))
	return nil
}

func (s *DefaultStore) Get(issuer Issuer, request Request) (*tls.Certificate, *Info, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if idMap, ok := s.certMap[issuer]; !ok {
		return nil, nil, fmt.Errorf("specified issuer found in certificate store")
	} else if cert, ok := idMap[request.Id()]; !ok {
		return nil, nil, fmt.Errorf("no certificate found in certificate store")
	} else if cert.cert == nil || cert.info == nil {
		return nil, nil, fmt.Errorf("certificate is nil in certificate store")
	} else {
		return cert.cert, cert.info, nil
	}
}
