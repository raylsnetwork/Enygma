package main

import (
    "enygma-server/config"
    "enygma-server/pkg/api"
)

func main() {
    cfg := config.Load()             // loads port, key paths…
    router := api.NewServer(cfg)        // wires circuits in routes
	if err := router.Run(":" + cfg.Port); err != nil {
        panic(err)
    }
}