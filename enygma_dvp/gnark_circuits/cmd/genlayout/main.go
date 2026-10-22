// Command genlayout renders enygma_dvp's statement layouts into the committed
// Go package enygma_dvp/layout (for the gnark server, src/core, the relayer
// and the integration tests) and the Solidity library DvpStatementLayout (for
// the contracts that verify DvP and mint receipts).
//
// The Go package covers every circuit enygma_dvp proves: DvpInitiator,
// DvpDestination, PrivateMint and its own copies of the Payment-family
// circuits, so its Go readers never depend on enygma_retail_payments.
// DvpStatementLayout covers the DvP and mint circuits only: the Solidity
// Payment-family library, PaymentStatementLayout, stays retail's.
//
// Regenerate after changing any covered circuit's public signals or deployed
// config:
//
//	cd gnark_circuits && go generate ./cmd/genlayout
//
// main_test.go fails while the committed files are stale; parity_test.go
// fails while enygma_dvp's Payment-family templates derive a layout other
// than retail's committed PaymentStatementLayout.
package main

//go:generate go run .

import (
	"log"
	"os"

	"enygma_gnark_shared/statement"

	"enygma_dvp/gnark_circuits/templates"
)

// Output paths, relative to this directory (go generate and go test both
// run here).
const (
	goOut  = "../../../layout/layout.go"
	solOut = "../../../contracts/core/contracts/DvpStatementLayout.sol"
)

const generator = "enygma_dvp/gnark_circuits/cmd/genlayout"

var (
	goTarget = statement.Target{
		Generator: generator,
		Subject: "the enygma_dvp DvpInitiator, DvpDestination, PrivateMint and " +
			"Payment-family (Payment, Payment2in, PaymentFee, " +
			"PaymentRelayerFeePublic, UsdrFee) circuits'",
		GoPackage: "layout",
	}
	solTarget = statement.Target{
		Generator:       generator,
		Subject:         "the enygma_dvp DvpInitiator, DvpDestination and PrivateMint circuits'",
		SolidityLibrary: "DvpStatementLayout",
	}
)

func generate() map[string][]byte {
	dvp := templates.DvpStatementLayouts()
	return map[string][]byte{
		goOut:  goTarget.RenderGo(append(dvp, templates.PaymentStatementLayouts()...)),
		solOut: solTarget.RenderSolidity(dvp),
	}
}

func main() {
	for path, src := range generate() {
		if err := os.WriteFile(path, src, 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote %s", path)
	}
}
