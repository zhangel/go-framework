package certificate

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/zhangel/go-framework.git/log"
)

type Factory struct {
	issuer   Issuer
	store    Store
	notifier *consumerNotifier
}

var defaultRenewalHook = DefaultRenewalPolicy(3 * 24 * time.Hour)

type RenewalAction byte

const (
	RenewalNow RenewalAction = iota
	RenewalAsync
	NoRenewal
)

func NewCertificateFactory(issuer Issuer, store Store) (*Factory, error) {
	if issuer == nil {
		return nil, fmt.Errorf("NewCertificateFactory, no issuer specified")
	}

	factory := &Factory{
		issuer:   issuer,
		store:    store,
		notifier: newConsumerNotifier(),
	}

	return factory, nil
}

func (s *Factory) Fetch(request Request, renewalHook func(cert *tls.Certificate, info *Info, err error) RenewalAction) (*tls.Certificate, error) {
	if renewalHook == nil {
		renewalHook = defaultRenewalHook
	}

	renewal := func(request Request) (err error) {
		defer func() {
			if err != nil {
				log.Warnf("CertificateFactory, renewal certificate with request %+v failed, err = %v", request, err)
			}
		}()

		ctx, canceler := context.WithTimeout(context.Background(), 10*time.Second)
		defer canceler()

		resultChan := make(chan error, 1)
		defer s.RegisterConsumer(&ConsumerWrapper{func(request Request, certificate *tls.Certificate, err error) {
			resultChan <- err
		}})()

		go func() {
			s.Produce(request)
		}()

		select {
		case err := <-resultChan:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	}

	cert, info, err := s.store.Get(s.issuer, request)
	switch renewalHook(cert, info, err) {
	case NoRenewal:
		return cert, err
	case RenewalNow:
		if err := renewal(request); err != nil {
			return nil, err
		}
	case RenewalAsync:
		go func() { _ = renewal(request) }()
	}

	cert, _, err = s.store.Get(s.issuer, request)
	return cert, err
}

func (s *Factory) Produce(request Request) {
	s.issuer.Issue(request, func(certificate *tls.Certificate, err error) {
		if err == nil {
			err = s.store.Store(s.issuer, request, certificate)
		}
		s.notifier.Consume(request, certificate, err)
	})
}

func (s *Factory) RegisterConsumer(consumer Consumer) func() {
	return s.notifier.RegisterConsumer(consumer)
}

func DefaultRenewalPolicy(renewalBefore time.Duration) func(cert *tls.Certificate, info *Info, err error) RenewalAction {
	return func(cert *tls.Certificate, info *Info, err error) RenewalAction {
		if cert == nil || info == nil || err != nil {
			return RenewalNow
		}

		now := time.Now()
		delta := info.NotAfter.Sub(now)
		if delta > renewalBefore {
			return NoRenewal
		}

		if delta >= 1*time.Minute {
			return RenewalAsync
		} else {
			return RenewalNow
		}
	}
}

func NoRenewalPolicy() func(cert *tls.Certificate, info *Info, err error) RenewalAction {
	return func(cert *tls.Certificate, info *Info, err error) RenewalAction {
		if cert == nil || info == nil || err != nil {
			return RenewalNow
		}

		return NoRenewal
	}
}
