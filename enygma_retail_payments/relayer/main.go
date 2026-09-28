package main

import (
	"fmt"
	"log"

	"enygma_retail_payments/relayer/config"
	"enygma_retail_payments/relayer/server"
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
	log.Printf("Enygma relayer listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("run: %v", err)
	}
}
