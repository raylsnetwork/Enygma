package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"enygma-server/config"
	"enygma-server/pkg/api"
)

func main() {
	cfg := config.Load()         // loads port, key paths…
	router := api.NewServer(cfg) // wires circuits in routes

	addr := fmt.Sprintf("%s:%s", cfg.BindHost, cfg.Port)

	// Fix H-04: warn loudly on every non-loopback bind — this endpoint has
	// no authentication or TLS of any kind (M-08) and every request body
	// carries a bank's secret_key and every blinding factor in cleartext.
	// Mirrors the same pattern used for demo/main.go's H-05 fix.
	if cfg.BindHost != "127.0.0.1" && cfg.BindHost != "localhost" && cfg.BindHost != "::1" {
		log.Printf("WARNING: GNARK_BIND_HOST=%s is not loopback-only. This server has NO "+
			"authentication and NO TLS (H-04/M-08) — every /proof/* request body carries a "+
			"bank's secret_key and every blinding factor in cleartext. Only do this behind a "+
			"network boundary you control (e.g. a private VPC with its own TLS termination and "+
			"access control in front) — never expose this port directly.", cfg.BindHost)
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
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("Enygma gnark proving server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
