package credentials

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"net"
	"strings"
	"unicode/utf8"

	"github.com/zhangel/go-framework/utils"
)

var (
	oidExtensionSubjectAltName = []int{2, 5, 29, 17}
)

var hostIp []string

func GenerateSAN(SAN []string) []string {
	if len(SAN) != 0 {
		return SAN
	}

	if len(hostIp) == 0 {
		if ip, err := utils.HostIp(); err == nil {
			hostIp = []string{ip, "127.0.0.1"}
		} else {
			hostIp = []string{"127.0.0.1"}
		}
	}

	return hostIp
}

func VerifyHostname(c *x509.Certificate, h string) error {
	candidateIP := h
	if len(h) >= 3 && h[0] == '[' && h[len(h)-1] == ']' {
		candidateIP = h[1 : len(h)-1]
	}
	if ip := net.ParseIP(candidateIP); ip != nil {
		for _, candidate := range c.IPAddresses {
			if ip.Equal(candidate) {
				return nil
			}
		}
		return x509.HostnameError{Certificate: c, Host: candidateIP}
	}

	names := c.DNSNames
	if commonNameAsHostname(c) {
		names = []string{c.Subject.CommonName}
	}

	candidateName := toLowerCaseASCII(h) // Save allocations inside the loop.
	validCandidateName := validHostnameInput(candidateName)

	for _, match := range names {
		if validCandidateName && validHostnamePattern(match) {
			if matchHostnames(match, candidateName) {
				return nil
			}
		} else {
			if matchExactly(match, candidateName) {
				return nil
			}
		}
	}

	return x509.HostnameError{Certificate: c, Host: h}
}

func commonNameAsHostname(c *x509.Certificate) bool {
	return !hasSANExtension(c) && validHostnamePattern(c.Subject.CommonName)
}

func matchExactly(hostA, hostB string) bool {
	if hostA == "" || hostA == "." || hostB == "" || hostB == "." {
		return false
	}
	return toLowerCaseASCII(hostA) == toLowerCaseASCII(hostB)
}

func matchHostnames(pattern, host string) bool {
	pattern = toLowerCaseASCII(pattern)
	host = toLowerCaseASCII(strings.TrimSuffix(host, "."))

	if len(pattern) == 0 || len(host) == 0 {
		return false
	}

	patternParts := strings.Split(pattern, ".")
	hostParts := strings.Split(host, ".")

	if len(patternParts) != len(hostParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if i == 0 && patternPart == "*" {
			continue
		}
		if patternPart != hostParts[i] {
			return false
		}
	}

	return true
}

func toLowerCaseASCII(in string) string {
	isAlreadyLowerCase := true
	for _, c := range in {
		if c == utf8.RuneError {
			isAlreadyLowerCase = false
			break
		}
		if 'A' <= c && c <= 'Z' {
			isAlreadyLowerCase = false
			break
		}
	}

	if isAlreadyLowerCase {
		return in
	}

	out := []byte(in)
	for i, c := range out {
		if 'A' <= c && c <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return string(out)
}

func hasSANExtension(c *x509.Certificate) bool {
	return oidInExtensions(oidExtensionSubjectAltName, c.Extensions)
}

func validHostnamePattern(host string) bool { return validHostname(host, true) }

func validHostnameInput(host string) bool { return validHostname(host, false) }

func validHostname(host string, isPattern bool) bool {
	if !isPattern {
		host = strings.TrimSuffix(host, ".")
	}
	if len(host) == 0 {
		return false
	}

	for i, part := range strings.Split(host, ".") {
		if part == "" {
			return false
		}
		if isPattern && i == 0 && part == "*" {
			continue
		}
		for j, c := range part {
			if 'a' <= c && c <= 'z' {
				continue
			}
			if '0' <= c && c <= '9' {
				continue
			}
			if 'A' <= c && c <= 'Z' {
				continue
			}
			if c == '-' && j != 0 {
				continue
			}
			if c == '_' {
				continue
			}
			return false
		}
	}

	return true
}

func oidInExtensions(oid asn1.ObjectIdentifier, extensions []pkix.Extension) bool {
	for _, e := range extensions {
		if e.Id.Equal(oid) {
			return true
		}
	}
	return false
}
