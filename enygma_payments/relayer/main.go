package main

import (
	"fmt"
	"log"

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

	if err := r.Run(addr); err != nil {
		log.Fatalf("run: %v", err)
	}
}
