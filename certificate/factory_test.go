package certificate

import (
	"crypto/tls"
	"testing"
	"time"
)

func TestFactory_Fetch(t *testing.T) {
	issuer, err := NewIssuer(
		IssuerWithCAProvider(SelfSignedCACertProvider()),
		IssuerWithValidDuration(2*365*24*time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	factory, err := NewCertificateFactory(issuer, NewDefaultStore())
	if err != nil {
		t.Fatal(err)
	}

	defer factory.RegisterConsumer(&ConsumerWrapper{func(request Request, certificate *tls.Certificate, err error) {
		t.Log("request =", request, ", certificate =", certificate)
	}})()

	request := NewCertRequest("127.0.0.1", []string{"127.0.0.1"}, nil, true)
	_, err = factory.Fetch(request, nil)
	if err != nil {
		t.Fatal(err)
	}

	factory.Produce(request)

	_, err = factory.Fetch(request, nil)
	if err != nil {
		t.Fatal(err)
	}
}
