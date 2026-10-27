// set_usdr_fee sets EnygmaDvp's governance USDr fixed fee, the way the USDr
// on-chain relayer-fee test does, and checks the chain reports it.
//
// Usage (from enygma_retail_payments/shield_demo):
//
//	go run ./cmd/set_usdr_fee -fee 1
package main

import (
	"flag"
	"log"
	"math/big"

	"enygma_retail_payments/shield_demo/engine"
)

func main() {
	root := flag.String("root", "..", "enygma_retail_payments directory")
	fee := flag.Int64("fee", 1, "USDr fixed fee")
	flag.Parse()

	got, err := engine.SetUsdrFixedFee(engine.DefaultConfig(*root), big.NewInt(*fee))
	if err != nil {
		log.Fatalf("set USDr fixed fee: %v", err)
	}
	log.Printf("EnygmaDvp.usdrFixedFeeAmount = %s", got)
}
