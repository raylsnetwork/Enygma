package main

import (
    "gnark_server/server/config"
    "gnark_server/server/api"
    serverutils "gnark_server/server/utils"
)

func main() {
    cfg := config.Load()             // loads port, key paths…
    router := api.NewServer(cfg)        // wires circuits in routes
	// MEDIUM-4 fix: bind only to loopback — prevents any remote or local-network
	// process from requesting proofs from the proving-key server.
	bindAddr := "127.0.0.1:" + cfg.Port
	// merkle_status.go's SSRF check allows loopback rpcUrl targets on the
	// assumption that this server is itself loopback-only — enforce that
	// assumption here instead of leaving it as a comment, so a future
	// change to bindAddr can't silently invalidate it.
	serverutils.RequireLoopbackBind(bindAddr)
	if err := router.Run(bindAddr); err != nil {
        panic(err)
    }
}