package main

import (
    "gnark_server/server/config"
    "gnark_server/server/api"
)

func main() {
    cfg := config.Load()             // loads port, key paths…
    router := api.NewServer(cfg)        // wires circuits in routes
	// MEDIUM-4 fix: bind only to loopback — prevents any remote or local-network
	// process from requesting proofs from the proving-key server.
	if err := router.Run("127.0.0.1:" + cfg.Port); err != nil {
        panic(err)
    }
}