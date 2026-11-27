package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"enygma_payments/gnark-server/config"
	"enygma_payments/gnark-server/pkg/api"
)

func main() {
	cfg := config.Load()
	addr, tlsConfig, err := proverTransport(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Fix M-08: router.Run(...) constructs an all-zero http.Server — every
	// timeout unset, so a slowloris connection (or one that simply never
	// sends a body) holds a goroutine indefinitely. An explicit
	// http.Server gives every connection a real deadline. WriteTimeout is
	// generous because proving itself can legitimately take longer than a
	// typical HTTP timeout under load (ProveLimiter's queue, plus ~240ms
	// measured proving time per request).
	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewServer(cfg),
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if tlsConfig == nil {
		log.Printf("Enygma gnark proving server listening on http://%s", addr)
		err = srv.ListenAndServe()
	} else {
		log.Printf("Enygma gnark proving server listening with mutual TLS on https://%s", addr)
		err = srv.ListenAndServeTLS("", "")
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func proverTransport(cfg *config.Config) (string, *tls.Config, error) {
	host := cfg.BindHost
	if host == "localhost" {
		host = "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "", nil, fmt.Errorf("GNARK_BIND_HOST must be an IP address or localhost")
	}
	addr := net.JoinHostPort(host, cfg.Port)
	if !cfg.AllowRemote {
		if !ip.IsLoopback() || cfg.TLSCert != "" || cfg.TLSKey != "" || cfg.TLSClientCA != "" {
			return "", nil, fmt.Errorf("non-loopback binds and TLS settings require GNARK_ALLOW_REMOTE=1 and mutual TLS")
		}
		return addr, nil, nil
	}

	if cfg.TLSCert == "" || cfg.TLSKey == "" || cfg.TLSClientCA == "" {
		return "", nil, fmt.Errorf("GNARK_ALLOW_REMOTE=1 requires GNARK_TLS_CERT, GNARK_TLS_KEY and GNARK_TLS_CLIENT_CA")
	}
	cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
	if err != nil {
		return "", nil, fmt.Errorf("load prover TLS identity: %w", err)
	}
	caPEM, err := os.ReadFile(cfg.TLSClientCA)
	if err != nil {
		return "", nil, fmt.Errorf("read prover client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		return "", nil, fmt.Errorf("GNARK_TLS_CLIENT_CA contains no valid certificates")
	}
	return addr, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
	}, nil
}
