package credentials

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/cpu"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/zhangel/go-framework/log"
)

const Http2Proto = "h2"
const TlsInfoSign = "x-frame-tls-info"
const MinTLSVersion = tls.VersionTLS11

var (
	defaultPubkey      *rsa.PublicKey
	tlsInfoToMetaMap   sync.Map
	tlsInfoFromMetaMap sync.Map
	mutex              sync.RWMutex

	cipherSuitesOnce    sync.Once
	AllowedCipherSuites []uint16
)

func init() {
	cipherSuitesOnce.Do(initDefaultCipherSuites)
}

func initDefaultCipherSuites() {
	// Check the cpu flags for each platform that has optimized GCM implementations.
	// Worst case, these variables will just all be false.
	var (
		hasGCMAsmAMD64 = cpu.X86.HasAES && cpu.X86.HasPCLMULQDQ
		hasGCMAsmARM64 = cpu.ARM64.HasAES && cpu.ARM64.HasPMULL
		// Keep in sync with crypto/aes/cipher_s390x.go.
		hasGCMAsmS390X = cpu.S390X.HasAES && cpu.S390X.HasAESCBC && cpu.S390X.HasAESCTR && (cpu.S390X.HasGHASH || cpu.S390X.HasAESGCM)

		hasGCMAsm = hasGCMAsmAMD64 || hasGCMAsmARM64 || hasGCMAsmS390X
	)

	var topCipherSuites []uint16
	if hasGCMAsm {
		// If AES-GCM hardware is provided then prioritise AES-GCM
		// cipher suites.
		topCipherSuites = []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		}
	} else {
		// Without AES-GCM hardware, we put the ChaCha20-Poly1305
		// cipher suites first.
		topCipherSuites = []uint16{
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		}
	}

	otherCipherSuites := []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	}

	AllowedCipherSuites = make([]uint16, 0, len(topCipherSuites)+len(otherCipherSuites))
	AllowedCipherSuites = append(AllowedCipherSuites, topCipherSuites...)
	AllowedCipherSuites = append(AllowedCipherSuites, otherCipherSuites...)
}

type credentialsBundle struct {
	transportCred credentials.TransportCredentials
	perRPCCred    credentials.PerRPCCredentials
}

func newCredBundle(transportCred credentials.TransportCredentials, perRPCCred credentials.PerRPCCredentials) credentials.Bundle {
	return credentialsBundle{
		transportCred: transportCred,
		perRPCCred:    perRPCCred,
	}
}

func (s credentialsBundle) TransportCredentials() credentials.TransportCredentials {
	return s.transportCred
}

func (s credentialsBundle) PerRPCCredentials() credentials.PerRPCCredentials {
	return s.perRPCCred
}

func (s credentialsBundle) NewWithMode(_ string) (credentials.Bundle, error) {
	return nil, nil
}

type bundledDialOption struct {
	grpc.EmptyDialOption
	dialOpts []grpc.DialOption
}

func withBundledDialOption(opts ...grpc.DialOption) grpc.DialOption {
	return bundledDialOption{dialOpts: opts}
}

func (s bundledDialOption) ExtractDialOptions() []grpc.DialOption {
	return s.dialOpts
}

func DialOptionWithOpts(opts *ClientOptions) (grpc.DialOption, error) {
	if opts.insecure {
		if opts.perRPCCredentials == nil {
			return grpc.WithInsecure(), nil
		} else {
			return withBundledDialOption(
				grpc.WithInsecure(),
				grpc.WithPerRPCCredentials(opts.perRPCCredentials),
			), nil
		}
	}

	tlsConfig, err := configClientTLS(opts)
	if err != nil {
		return nil, err
	}

	if opts.perRPCCredentials != nil {
		return grpc.WithCredentialsBundle(newCredBundle(credentials.NewTLS(tlsConfig), opts.perRPCCredentials)), nil
	} else {
		return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
	}
}

func DialOption(opt ...ClientOptionFunc) (grpc.DialOption, error) {
	opts, err := ParseClientOptions(opt...)
	if err != nil {
		return nil, err
	}

	return DialOptionWithOpts(opts)
}

func ServerOptionWithOpts(opts *ServerOptions) (grpc.ServerOption, error) {
	if opts.insecure {
		return nil, nil
	}

	tlsConfig, err := configServerTLS(opts)
	if err != nil {
		return nil, err
	}

	if len(tlsConfig.Certificates) == 0 && tlsConfig.GetCertificate == nil {
		return nil, fmt.Errorf("no certificate specified for server")
	}

	return grpc.Creds(credentials.NewTLS(tlsConfig)), nil
}

func ServerOption(opt ...ServerOptionFunc) (grpc.ServerOption, error) {
	opts, err := ParseServerOptions(opt...)
	if err != nil {
		return nil, err
	}

	return ServerOptionWithOpts(opts)
}

func ServerTlsConfig(opt ...ServerOptionFunc) (*tls.Config, error) {
	opts, err := ParseServerOptions(opt...)
	if err != nil {
		return nil, err
	}

	return configServerTLS(opts)
}

func ClientTlsConfig(opt ...ClientOptionFunc) (*tls.Config, error) {
	opts, err := ParseClientOptions(opt...)
	if err != nil {
		return nil, err
	}

	return configClientTLS(opts)
}

func HttpServerTlsConfig(opt ...ServerOptionFunc) (*tls.Config, error) {
	opts, err := ParseServerOptions(opt...)
	if err != nil {
		return nil, err
	}

	if opts.insecure {
		return nil, nil
	}

	tlsConfig, err := configServerTLS(opts)
	if err != nil {
		return nil, err
	}

	if tlsConfig != nil {
		tlsConfig.NextProtos = []string{Http2Proto}
	}

	return tlsConfig, nil
}

func ParseClientOptions(opt ...ClientOptionFunc) (*ClientOptions, error) {
	opts := &ClientOptions{}
	for _, o := range opt {
		if err := o(opts); err != nil {
			return nil, fmt.Errorf("config credentials failed, err = %v", err)
		}
	}
	return opts, nil
}

func ParseServerOptions(opt ...ServerOptionFunc) (*ServerOptions, error) {
	opts := &ServerOptions{}
	for _, o := range opt {
		if err := o(opts); err != nil {
			return nil, fmt.Errorf("config credentials failed, err = %v", err)
		}
	}
	return opts, nil
}

func configClientTLS(opts *ClientOptions) (_ *tls.Config, err error) {
	defer func() {
		if err != nil {
			log.Errorf("Config client tls config failed, err = %v", err)
		}
	}()

	tlsConfig := &tls.Config{
		ServerName:           opts.serverName,
		InsecureSkipVerify:   opts.insecureSkipVerify,
		GetClientCertificate: opts.certificate,
		MinVersion:           MinTLSVersion,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) (err error) {
			return verifyCertificate(rawCerts, verifiedChains, opts.certificateVerifier, opts.verifyCertificate, opts.ignoreExpiresCheck, true)
		},
	}

	if opts.certificateVerifier != nil || opts.verifyCertificate != nil {
		tlsConfig.InsecureSkipVerify = true
	}

	if opts.rootCAs != nil {
		caCerts, err := opts.rootCAs()
		if err != nil {
			return nil, err
		}

		certPool := x509.NewCertPool()
		for _, caCert := range caCerts {
			if !certPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("certPool::ConfigClientTLS AppendCACerts failed, cacert = %s", caCert)
			}
		}

		tlsConfig.RootCAs = certPool
	}

	return tlsConfig, nil
}

func configServerTLS(opts *ServerOptions) (_ *tls.Config, err error) {
	defer func() {
		if err != nil {
			log.Errorf("Config server tls config failed, err = %v", err)
		}
	}()

	tlsConfig := &tls.Config{
		GetCertificate: opts.certificate,
		ClientAuth:     opts.clientAuthType,
		MinVersion:     MinTLSVersion,
		CipherSuites:   AllowedCipherSuites,
	}

	if opts.clientCAs != nil {
		caCerts, err := opts.clientCAs()
		if err != nil {
			return nil, err
		}

		certPool := x509.NewCertPool()
		for _, caCert := range caCerts {
			if !certPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("certPool::ConfigServerTLS AppendCACerts failed, caCert = %s", caCert)
			}
		}

		tlsConfig.ClientCAs = certPool
	}

	if tlsConfig.ClientAuth < tls.VerifyClientCertIfGiven {
		return tlsConfig, nil
	}

	if len(opts.certificateChainVerifier) != 0 || opts.verifyCertificate != nil {
		if tlsConfig.ClientAuth == tls.VerifyClientCertIfGiven {
			tlsConfig.ClientAuth = tls.RequestClientCert
		} else if tlsConfig.ClientAuth == tls.RequireAndVerifyClientCert {
			tlsConfig.ClientAuth = tls.RequireAnyClientCert
		}

		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) (err error) {
			if tlsConfig.ClientAuth == tls.RequestClientCert && len(rawCerts) == 0 {
				return nil
			}

			return verifyCertificate(rawCerts, verifiedChains, opts.certificateChainVerifier, opts.verifyCertificate, opts.ignoreExpiresCheck, false)
		}
	} else {
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return nil
		}
	}

	return tlsConfig, nil
}

func verifyCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate, certificateVerifier []func(rawCerts [][]byte, ignoreExpiresCheck bool) ([][]*x509.Certificate, error), verifyCertificate func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error, ignoreExpiresCheck, server bool) (err error) {
	defer func() {
		if err != nil {
			if server {
				log.Errorf("Verify server certificate failed, err = %v", err)
			} else {
				log.Errorf("Verify client certificate failed, err = %v", err)
			}
		}
	}()

	if certificateVerifier != nil {
		for _, verifier := range certificateVerifier {
			chains, err := verifier(rawCerts, ignoreExpiresCheck)
			if err != nil {
				return err
			}

			if verifiedChains == nil {
				verifiedChains = chains
			}
		}
	}

	if verifyCertificate != nil {
		return verifyCertificate(rawCerts, verifiedChains)
	} else {
		return nil
	}
}

func certificateChainVerifier(rootCAs func() ([][]byte, error), authorityChecker func(length, current int, certificate *x509.Certificate) error, extKeyUsage []x509.ExtKeyUsage) func(rawCerts [][]byte, ignoreExpiresCheck bool) ([][]*x509.Certificate, error) {
	return func(rawCerts [][]byte, ignoreExpiresCheck bool) ([][]*x509.Certificate, error) {
		if rootCAs == nil {
			return nil, fmt.Errorf("tls: verify peer certificate without root ca set")
		}

		if len(rawCerts) == 0 {
			return nil, fmt.Errorf("tls: no certificates specified for certificate verification")
		}

		notBefore := time.Now()

		certs := make([]*x509.Certificate, len(rawCerts))
		for i, asn1Data := range rawCerts {
			cert, err := x509.ParseCertificate(asn1Data)
			if err != nil {
				return nil, errors.New("tls: failed to parse certificate from server: " + err.Error())
			}
			certs[i] = cert
			if cert.NotBefore.After(notBefore) {
				notBefore = cert.NotBefore
			}
		}

		caCerts, err := rootCAs()
		if err != nil {
			return nil, err
		}

		certPool := x509.NewCertPool()
		for _, caCert := range caCerts {
			if !certPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("tls: AppendCACerts failed, cacert = %s", caCert)
			}
		}

		verifyOpts := x509.VerifyOptions{
			CurrentTime:   time.Now(),
			Intermediates: x509.NewCertPool(),
			Roots:         certPool,
			KeyUsages:     extKeyUsage,
		}

		if ignoreExpiresCheck {
			verifyOpts.CurrentTime = notBefore.Add(time.Second)
		}

		for _, cert := range certs[1:] {
			verifyOpts.Intermediates.AddCert(cert)
		}

		verifiedChains, err := certs[0].Verify(verifyOpts)
		if err != nil {
			return nil, err
		}

		for idx, cert := range certs {
			if err := authorityChecker(len(certs), idx, cert); err != nil {
				return nil, err
			}
		}

		return verifiedChains, nil
	}
}

func LeafAuthorityChecker(authorities func() []string) func(length, current int, certificate *x509.Certificate) error {
	return IntermediateAuthorityChecker(0, authorities)
}

func IntermediateAuthorityChecker(level int, authorities func() []string) func(length, current int, certificate *x509.Certificate) error {
	return func(length, current int, certificate *x509.Certificate) error {
		if authorities == nil {
			return fmt.Errorf("authority getter function is nil")
		}

		if length <= level {
			return fmt.Errorf("no certificate of level %d found", level)
		}

		if current == level {
			var errs []string
			for _, authority := range authorities() {
				if authority == "*" {
					return nil
				}

				if err := VerifyHostname(certificate, authority); err == nil {
					return nil
				} else {
					errs = append(errs, err.Error())
				}
			}
			return fmt.Errorf("Verify certificate authorities failed, authorities accepted = %v, err = %s", authorities(), strings.Join(errs, ","))
		} else {
			return nil
		}
	}
}

// deprecated: Use PeerTlsInfo instead
func TlsAuthServerName(ctx context.Context) (string, error) {
	tlsInfo, err := PeerTlsInfo(ctx)
	if err != nil {
		return "", err
	}

	return tlsInfo.State.ServerName, nil
}

// deprecated: Use PeerTlsCommonName instead
func TlsAuthSubject(ctx context.Context) (string, error) {
	return PeerTlsCommonName(ctx, 0)
}

func PeerTlsCommonName(ctx context.Context, position int) (string, error) {
	tlsInfo, err := PeerTlsInfo(ctx)
	if err != nil {
		return "", err
	}

	if !tlsInfo.State.HandshakeComplete {
		return "", fmt.Errorf("PeerTlsCommonName, tls handshake incomplete")
	}

	if len(tlsInfo.State.PeerCertificates) <= position {
		return "", fmt.Errorf("TlsAuthSubject, tlsInfo.State.PeerCertificates is empty")
	}

	return tlsInfo.State.PeerCertificates[position].Subject.CommonName, nil
}

func PeerTlsInfo(ctx context.Context) (*credentials.TLSInfo, error) {
	if tlsInfo, err := peerTlsInfoFromMeta(ctx, nil); err == nil {
		return tlsInfo, nil
	} else if tlsInfo, err := peerTlsInfoFromPeer(ctx); err == nil {
		return tlsInfo, nil
	} else {
		return nil, err
	}
}

func peerTlsInfoFromPeer(ctx context.Context) (*credentials.TLSInfo, error) {
	peerInfo, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("TlsInfoFromPeer, retrive peerInfo from context failed")
	}

	if peerInfo.AuthInfo == nil {
		return nil, fmt.Errorf("TlsInfoFromPeer, no authInfo found")
	}

	if peerInfo.AuthInfo.AuthType() != "tls" {
		return nil, fmt.Errorf("TlsInfoFromPeer, authType of peerInfo isn't tls")
	}

	tlsInfo, ok := peerInfo.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, fmt.Errorf("TlsInfoFromPeer, type switch of peerInfo failed")
	}

	return &tlsInfo, nil
}

func peerTlsInfoFromMeta(ctx context.Context, publicKey *rsa.PublicKey) (*credentials.TLSInfo, error) {
	if publicKey == nil {
		publicKey = DefaultPublicKeyForTlsInfoVerify()
	}

	if publicKey == nil {
		return nil, fmt.Errorf("TlsInfoFromMeta, no pubkey set for tls info verifiy")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("TlsInfoFromMeta, no incoming md found")
	}

	tlsInfoBase64 := md.Get(TlsInfoSign)
	if len(tlsInfoBase64) == 0 {
		return nil, fmt.Errorf("TlsInfoFromMeta, no %s found in incoming md", TlsInfoSign)
	}

	if tlsInfo, ok := tlsInfoFromMetaMap.Load(tlsInfoBase64[0]); ok {
		return tlsInfo.(*credentials.TLSInfo), nil
	}

	gobTlsInfoWithSign, err := base64.StdEncoding.DecodeString(tlsInfoBase64[0])
	if err != nil {
		return nil, fmt.Errorf("TlsInfoFromMeta, base64 decode tls info failed, err = %v", err)
	}

	if len(gobTlsInfoWithSign) < 256 {
		return nil, fmt.Errorf("TlsInfoFromMeta, invalid tls info header, length too small")
	}

	gobTlsInfoCompressed := gobTlsInfoWithSign[:len(gobTlsInfoWithSign)-256]
	signature := gobTlsInfoWithSign[len(gobTlsInfoWithSign)-256:]

	hashed := sha256.Sum256(gobTlsInfoCompressed)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], signature); err != nil {
		return nil, fmt.Errorf("TlsInfoFromMeta, verify tls info rsa-sha256 signature failed, err = %v", err)
	}

	gzipReader, err := gzip.NewReader(bytes.NewBuffer(gobTlsInfoCompressed))
	if err != nil {
		return nil, fmt.Errorf("TlsInfoFromMeta, gzip decode failed, err = %v", err)
	}
	defer gzipReader.Close()

	gobTlsInfo, err := ioutil.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("TlsInfoFromMeta, gzip decode failed, err = %v", err)
	}

	decoder := gob.NewDecoder(bytes.NewBuffer(gobTlsInfo))
	var tlsInfo credentials.TLSInfo
	if err := decoder.Decode(&tlsInfo); err != nil {
		return nil, err
	}

	tlsInfoFromMetaMap.Store(tlsInfoBase64[0], &tlsInfo)
	return &tlsInfo, nil
}

func PeerTlsInfoToMeta(ctx context.Context, tlsInfo *credentials.TLSInfo, key *rsa.PrivateKey) (context.Context, error) {
	if tlsInfo == nil {
		return ctx, nil
	}

	if key == nil {
		return ctx, fmt.Errorf("TlsInfoToMeta, rsaPrivateKey is nil")
	}

	SetTlsInfoHeader := func(ctx context.Context, tlsInfoHeader string) (context.Context, error) {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}

		md.Set(TlsInfoSign, tlsInfoHeader)
		return metadata.NewOutgoingContext(ctx, md), nil
	}

	var tlsInfoKeyBuilder strings.Builder
	for i := 0; i < len(tlsInfo.State.PeerCertificates); i++ {
		if tlsInfo.State.PeerCertificates[i] == nil {
			continue
		}
		tlsInfoKeyBuilder.WriteString(tlsInfo.State.PeerCertificates[i].SerialNumber.String())
	}
	tlsInfoKey := tlsInfoKeyBuilder.String()

	if tlsInfoKey == "" {
		return ctx, nil
	}

	if tlsInfoHeader, ok := tlsInfoToMetaMap.Load(tlsInfoKey); ok {
		return SetTlsInfoHeader(ctx, tlsInfoHeader.(string))
	}

	copyCertificateInfo := func(src, dest *x509.Certificate) {
		dest.Version = src.Version
		dest.Issuer = src.Issuer
		dest.Subject = src.Subject
		dest.NotBefore = src.NotBefore
		dest.NotAfter = src.NotAfter
		dest.KeyUsage = src.KeyUsage
		dest.DNSNames = src.DNSNames
		dest.EmailAddresses = src.EmailAddresses
		dest.IPAddresses = src.IPAddresses
		dest.URIs = src.URIs
		dest.Extensions = src.Extensions
		dest.ExtraExtensions = src.ExtraExtensions
	}

	var tlsInfoForward credentials.TLSInfo
	tlsInfoForward.State.ServerName = tlsInfo.State.ServerName
	tlsInfoForward.State.CipherSuite = tlsInfo.State.CipherSuite
	tlsInfoForward.State.Version = tlsInfo.State.Version
	tlsInfoForward.State.HandshakeComplete = tlsInfo.State.HandshakeComplete
	tlsInfoForward.State.PeerCertificates = make([]*x509.Certificate, len(tlsInfo.State.PeerCertificates))
	for i := 0; i < len(tlsInfo.State.PeerCertificates); i++ {
		tlsInfoForward.State.PeerCertificates[i] = new(x509.Certificate)
		copyCertificateInfo(tlsInfo.State.PeerCertificates[i], tlsInfoForward.State.PeerCertificates[i])
	}

	tlsInfoForward.State.VerifiedChains = make([][]*x509.Certificate, len(tlsInfo.State.VerifiedChains))
	for i := 0; i < len(tlsInfo.State.VerifiedChains); i++ {
		tlsInfoForward.State.VerifiedChains[i] = make([]*x509.Certificate, len(tlsInfo.State.VerifiedChains[i]))
		for j := 0; j < len(tlsInfo.State.VerifiedChains[i]); j++ {
			tlsInfoForward.State.VerifiedChains[i][j] = new(x509.Certificate)
			copyCertificateInfo(tlsInfo.State.VerifiedChains[i][j], tlsInfoForward.State.VerifiedChains[i][j])
		}
	}

	var gobTlsInfoBuf bytes.Buffer
	encoder := gob.NewEncoder(&gobTlsInfoBuf)
	if err := encoder.Encode(&tlsInfoForward); err != nil {
		return ctx, fmt.Errorf("TlsInfoToMeta, gob encode tls info failed, err = %v", err)
	}

	var gobTlsInfoCompressedBuf bytes.Buffer
	gzipWriter := gzip.NewWriter(&gobTlsInfoCompressedBuf)
	if _, err := gzipWriter.Write(gobTlsInfoBuf.Bytes()); err != nil {
		return ctx, fmt.Errorf("TlsInfoToMeta, gzip encode tls info failed, err = %v", err)
	}
	if err := gzipWriter.Flush(); err != nil {
		return ctx, fmt.Errorf("TlsInfoToMeta, gzip encode tls info failed, err = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return ctx, fmt.Errorf("TlsInfoToMeta, gzip encode tls info failed, err = %v", err)
	}

	hashed := sha256.Sum256(gobTlsInfoCompressedBuf.Bytes())
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hashed[:])
	if err != nil {
		return ctx, fmt.Errorf("TlsInfoToMeta, create rsa-sha256 signature failed, err = %v", err)
	}

	tlsInfoHeader := base64.StdEncoding.EncodeToString(append(gobTlsInfoCompressedBuf.Bytes(), signature...))
	tlsInfoToMetaMap.Store(tlsInfoKey, tlsInfoHeader)
	return SetTlsInfoHeader(ctx, tlsInfoHeader)
}

func SetDefaultPublicKeyForTlsInfoVerify(publicKey *rsa.PublicKey) {
	mutex.Lock()
	defer mutex.Unlock()
	defaultPubkey = publicKey
}

func DefaultPublicKeyForTlsInfoVerify() *rsa.PublicKey {
	mutex.RLock()
	defer mutex.RUnlock()
	return defaultPubkey
}
