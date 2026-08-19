package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"enygma_payments_relayer/config"
	"enygma_payments_relayer/server"
)

// shutdownTimeout bounds how long graceful shutdown waits for in-flight
// requests to finish before forcing the listener closed.
const shutdownTimeout = 10 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	r, h, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server init: %v", err)
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Enygma payments relayer listening on %s", addr)
	log.Printf("  contract : %s", cfg.ContractAddr)
	log.Printf("  chain ID : %s", cfg.ChainID.String())
	log.Printf("  RPC URL  : %s", cfg.RPCURL)
	if cfg.MinFee.Sign() > 0 {
		log.Printf("  min fee  : %s (enforced on /relay/transfer_fee)", cfg.MinFee)
	} else {
		log.Printf("  min fee  : disabled — /relay/transfer_fee accepts any fee, set RELAYER_MIN_FEE to enforce")
	}
	if cfg.RateLimitRPS > 0 {
		log.Printf("  rate limit: %.2f req/s, burst %d, per caller (Bearer token)", cfg.RateLimitRPS, cfg.RateLimitBurst)
	} else {
		log.Printf("  rate limit: disabled — set RELAYER_RATE_LIMIT_RPS to enforce")
	}
	log.Printf("  idempotency store: %s (dedup staleness %s)", cfg.StorePath, cfg.DedupStaleness)
	if cfg.MinBalanceWei.Sign() > 0 {
		log.Printf("  readiness: /health/ready requires balance >= %s wei", cfg.MinBalanceWei)
	} else {
		log.Printf("  readiness: /health/ready checks RPC connectivity only — set RELAYER_MIN_BALANCE_WEI for a balance floor")
	}

	httpServer := &http.Server{Addr: addr, Handler: r}

	// Run the listener in the background so the main goroutine is free to
	// wait on a shutdown signal instead of blocking forever in Run/ListenAndServe.
	serveErr := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
		close(serveErr)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		if err != nil {
			log.Fatalf("run: %v", err)
		}
	case sig := <-sigCh:
		log.Printf("received %s, shutting down (up to %s for in-flight requests)...", sig, shutdownTimeout)
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("http server shutdown: %v", err)
		}
	}

	// Release the idempotency store's bbolt file lock cleanly. bbolt fsyncs
	// each write regardless, so no data is at risk either way — this just
	// avoids leaving the lock held past a clean process exit.
	if err := h.Close(); err != nil {
		log.Printf("close idempotency store: %v", err)
	}
}
