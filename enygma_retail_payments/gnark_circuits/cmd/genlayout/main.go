// Command genlayout renders the retail Payment-family circuits' statement
// layouts into the committed Go package (enygma_retail_payments/layout,
// imported by the relayer, the test suites and the demo) and Solidity library
// (enygma_dvp's PaymentStatementLayout.sol, used by the contracts that verify
// retail receipts: EnygmaDvp.sol and Erc20CoinVault.sol).
//
// Regenerate after changing any Payment-family circuit's public signals or
// deployed config:
//
//	cd gnark_circuits && go generate ./cmd/genlayout
//
// main_test.go fails while the committed files are stale; arity_test.go
// fails while one of Erc20CoinVault.sol's Payment-family length checks or
// length-dispatched verifier calls disagrees with its circuit's statement
// length, which the compiler cannot check.
package main

//go:generate go run .

import (
	"log"
	"os"

	"enygma_gnark_shared/statement"

	"enygma_retail_payments/gnark_circuits/templates"
)

// Output paths, relative to this directory (go generate and go test both
// run here).
const (
	goOut  = "../../../layout/layout.go"
	solOut = "../../../../enygma_dvp/contracts/core/contracts/PaymentStatementLayout.sol"
)

var target = statement.Target{
	Generator:       "enygma_retail_payments/gnark_circuits/cmd/genlayout",
	Subject:         "the retail Payment-family circuits'",
	GoPackage:       "layout",
	SolidityLibrary: "PaymentStatementLayout",
}

func generate() map[string][]byte {
	layouts := templates.StatementLayouts()
	return map[string][]byte{
		goOut:  target.RenderGo(layouts),
		solOut: target.RenderSolidity(layouts),
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
