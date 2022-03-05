package credentials

import (
	"strings"
	"sync"

	"github.com/zhangel/go-framework.git/certificate"
)

type IssuerPool struct {
	mu   sync.RWMutex
	pool map[string]certificate.Issuer
}

func NewIssuerPool() *IssuerPool {
	return &IssuerPool{
		pool: map[string]certificate.Issuer{},
	}
}

func (s *IssuerPool) id(cert, key, passphrase []byte) string {
	builder := strings.Builder{}
	builder.Write(cert)
	builder.WriteString("|")
	builder.Write(key)
	builder.WriteString("|")
	builder.Write(passphrase)

	return builder.String()
}

func (s *IssuerPool) Put(cert, key, passphrase []byte, issuer certificate.Issuer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pool[s.id(cert, key, passphrase)] = issuer
}

func (s *IssuerPool) Get(cert, key, passphrase []byte) certificate.Issuer {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.id(cert, key, passphrase)

	if provider, ok := s.pool[id]; ok {
		return provider
	} else if cert != nil && key != nil {
		issuer, err := certificate.NewIssuer(
			certificate.IssuerWithCAProvider(func() ([]byte, []byte, []byte, error) {
				return cert, key, passphrase, nil
			}),
		)
		if err != nil {
			return nil
		}
		s.pool[id] = issuer
		return s.pool[id]
	} else {
		return nil
	}
}
