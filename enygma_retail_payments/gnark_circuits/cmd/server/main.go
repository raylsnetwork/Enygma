// Server entry point. Run from gnark_circuits/ with: go run ./cmd/server
// (key paths in server/config are relative to that directory)
//
// Listens on 127.0.0.1:8082 by default. GNARK_BIND_HOST picks another
// interface, GNARK_AUTH_TOKEN requires a bearer token on every request and
// GNARK_MAX_CONCURRENT_PROOFS bounds how many proofs run at once.
package main

import (
	"log"
	"net"
	"net/http"
	"time"

	"enygma_retail_payments/gnark_circuits/server/api"
	"enygma_retail_payments/gnark_circuits/server/config"
)

func main() {
	cfg := config.Load()
	router := api.NewServer(cfg)
	addr := net.JoinHostPort(cfg.BindHost, cfg.Port)
	if !isLoopback(cfg.BindHost) {
		log.Printf("WARNING: GNARK_BIND_HOST=%s is not loopback. Proof requests carry secret witness data over plain HTTP; "+
			"put TLS and access control in front, and set GNARK_AUTH_TOKEN.", cfg.BindHost)
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Printf("retail gnark server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

// isLoopback reports whether host names the loopback interface.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
