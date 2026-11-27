package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"enygma_payments/gnark-server/config"
)

func TestProverTransportLoopback(t *testing.T) {
	t.Setenv("GNARK_BIND_HOST", "")
	t.Setenv("GNARK_PORT", "")
	t.Setenv("GNARK_ALLOW_REMOTE", "")
	t.Setenv("GNARK_TLS_CERT", "")
	t.Setenv("GNARK_TLS_KEY", "")
	t.Setenv("GNARK_TLS_CLIENT_CA", "")
	for host, want := range map[string]string{
		"127.0.0.1": "127.0.0.1:8080",
		"localhost": "127.0.0.1:8080",
		"::1":       "[::1]:8080",
	} {
		cfg := config.Load()
		if host != "127.0.0.1" {
			cfg.BindHost = host
		}
		addr, tlsConfig, err := proverTransport(cfg)
		if err != nil || tlsConfig != nil || addr != want {
			t.Errorf("loopback %s gave address %q, TLS %v and error %v", host, addr, tlsConfig != nil, err)
		}
	}
}

func TestProverTransportRejectsUnsecuredRemote(t *testing.T) {
	cfg := &config.Config{BindHost: "0.0.0.0", Port: "8080"}
	if _, _, err := proverTransport(cfg); err == nil || !strings.Contains(err.Error(), "GNARK_ALLOW_REMOTE=1") {
		t.Fatalf("remote bind without consent gave error %v", err)
	}
	cfg.AllowRemote = true
	if _, _, err := proverTransport(cfg); err == nil || !strings.Contains(err.Error(), "GNARK_TLS_CLIENT_CA") {
		t.Fatalf("remote bind without credentials gave error %v", err)
	}
}

func TestProverTransportAuthenticatesClients(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caCert, caKey := issueTestCertificate(t, ca, ca, nil)
	server := &x509.Certificate{
		SerialNumber: big.NewInt(2), NotBefore: ca.NotBefore, NotAfter: ca.NotAfter,
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverCert, serverKey := issueTestCertificate(t, server, ca, caKey)
	client := &x509.Certificate{
		SerialNumber: big.NewInt(3), NotBefore: ca.NotBefore, NotAfter: ca.NotAfter,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientCert, clientKey := issueTestCertificate(t, client, ca, caKey)

	certPath := filepath.Join(dir, "server.pem")
	keyPath := filepath.Join(dir, "server-key.pem")
	caPath := filepath.Join(dir, "ca.pem")
	for path, contents := range map[string][]byte{
		certPath: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCert}),
		keyPath:  encodeTestKey(t, serverKey),
		caPath:   pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert}),
	} {
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		BindHost: "0.0.0.0", Port: "8080", AllowRemote: true,
		TLSCert: certPath, TLSKey: keyPath, TLSClientCA: caPath,
	}
	_, tlsConfig, err := proverTransport(cfg)
	if err != nil || tlsConfig == nil {
		t.Fatalf("configured remote bind gave TLS %v and error %v", tlsConfig != nil, err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	srv.TLS = tlsConfig
	srv.StartTLS()
	defer srv.Close()

	untrustedClient := *client
	untrustedClient.SerialNumber = big.NewInt(4)
	untrusted, untrustedKey := issueTestCertificate(t, &untrustedClient, &untrustedClient, nil)

	for _, tc := range []struct {
		name  string
		cert  tls.Certificate
		valid bool
	}{
		{name: "no certificate"},
		{name: "untrusted certificate", cert: tls.Certificate{Certificate: [][]byte{untrusted}, PrivateKey: untrustedKey}},
		{name: "bank certificate", cert: tls.Certificate{Certificate: [][]byte{clientCert}, PrivateKey: clientKey}, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := srv.Client().Transport.(*http.Transport).Clone()
			defer transport.CloseIdleConnections()
			transport.TLSClientConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
				return &tc.cert, nil
			}
			c := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			resp, err := c.Get(srv.URL)
			if resp != nil {
				resp.Body.Close()
			}
			if tc.valid {
				if err != nil || resp.StatusCode != http.StatusNoContent {
					t.Fatalf("mutual TLS rejected the bank certificate: %v", err)
				}
			} else if err == nil {
				t.Fatalf("mutual TLS accepted %s", tc.name)
			}
		})
	}
}

func issueTestCertificate(t *testing.T, cert, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if issuerKey == nil {
		issuerKey = key
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, issuer, &key.PublicKey, issuerKey)
	if err != nil {
		t.Fatal(err)
	}
	return der, key
}

func encodeTestKey(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
}
