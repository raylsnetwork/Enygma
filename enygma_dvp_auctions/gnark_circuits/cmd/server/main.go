package main

import (
	"log"
	"os"

	"gnark_server/server/api"
	"gnark_server/server/config"
)

func main() {
	cfgPath := ""
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	r := api.NewServer(cfg)
	log.Println("auction gnark server listening on 127.0.0.1:8083")
	if err := r.Run("127.0.0.1:8083"); err != nil {
		log.Fatalf("server: %v", err)
	}
}
