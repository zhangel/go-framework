package certificate

import (
	"crypto/tls"
	"sync"
)

type Consumer interface {
	Consume(certInfo Request, certificate *tls.Certificate, err error)
}

type ConsumerWrapper struct {
	f func(certInfo Request, certificate *tls.Certificate, err error)
}

func (s *ConsumerWrapper) Consume(certInfo Request, certificate *tls.Certificate, err error) {
	s.f(certInfo, certificate, err)
}

type consumerNotifier struct {
	consumers []Consumer
	mu        sync.RWMutex
}

func newConsumerNotifier(consumers ...Consumer) *consumerNotifier {
	return &consumerNotifier{consumers: consumers, mu: sync.RWMutex{}}
}

func (s *consumerNotifier) RegisterConsumer(consumer Consumer) (cancel func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.consumers = append(s.consumers, consumer)
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		var canceled []Consumer
		for _, w := range s.consumers {
			if w == consumer {
				continue
			}
			canceled = append(canceled, w)
		}
		s.consumers = canceled
	}
}

func (s *consumerNotifier) _Consumers() []Consumer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Consumer, len(s.consumers))
	copy(result, s.consumers)

	return result
}

func (s *consumerNotifier) Consume(certInfo Request, certificate *tls.Certificate, err error) {
	if s.Size() == 0 {
		return
	}

	for _, c := range s._Consumers() {
		c.Consume(certInfo, certificate, err)
	}
}

func (s *consumerNotifier) Size() int {
	return len(s._Consumers())
}
