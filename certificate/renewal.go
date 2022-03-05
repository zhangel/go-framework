package certificate

type RenewalNotifier interface {
	WatchRenewal(observer RenewalObserver)
	Renewal(issuer Issuer, request Request) error
}

type RenewalObserver interface {
	OnRenewal(issuer Issuer, request Request) error
}
