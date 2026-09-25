// Server entry point. Run from gnark_circuits/ with: go run ./cmd/server
// (key paths in server/config are relative to that directory)
package main

import (
	"enygma_retail_payments/gnark_circuits/server/api"
	"enygma_retail_payments/gnark_circuits/server/config"
)

func main() {
	cfg := config.Load()
	router := api.NewServer(cfg)
	if err := router.Run(":" + cfg.Port); err != nil {
		panic(err)
	}
}
