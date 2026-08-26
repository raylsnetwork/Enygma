package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"enygma_payments_relayer/config"
	"enygma_payments_relayer/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	r, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server init: %v", err)
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Enygma payments relayer listening on %s", addr)
	log.Printf("  contract : %s", cfg.ContractAddr)
	log.Printf("  chain ID : %s", cfg.ChainID.String())
	log.Printf("  RPC URL  : %s", cfg.RPCURL)
	log.Printf("  banks    : %d credential(s) issued", len(cfg.APIKeys))

	// Fix M-09: r.Run(addr) is http.ListenAndServe with no timeouts at all
	// — ReadHeaderTimeout, ReadTimeout, WriteTimeout and IdleTimeout were
	// all zero, so a connection that never finishes sending its request
	// headers holds a goroutine and a file descriptor forever, before
	// bearerAuth (which only runs once the handler chain is reached) ever
	// sees a token. An explicit http.Server with real timeouts closes that
	// unauthenticated resource-exhaustion path.
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// WriteTimeout must exceed the slowest legitimate response: a
		// relay call can block for up to txTimeout (45s) waiting on
		// bind.WaitMined before writing a response.
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("run: %v", err)
	}
}
