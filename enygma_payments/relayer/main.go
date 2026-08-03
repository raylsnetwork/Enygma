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

	if err := r.Run(addr); err != nil {
		log.Fatalf("run: %v", err)
	}
}
