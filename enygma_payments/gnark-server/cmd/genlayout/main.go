// Command genlayout renders the payments circuits' statement layouts into
// the committed Go package (enygma_payments/layout, imported by the relayer
// and the test suites) and Solidity library (StatementLayout.sol, used by
// Enygma.sol and IEnygma.sol).
//
// Regenerate after changing any circuit's public signals:
//
//	cd gnark-server && go generate ./cmd/genlayout
//
// main_test.go fails while the committed files are stale; arity_test.go
// fails while one of Enygma.sol's verifyProof selector strings disagrees
// with its circuit's statement length.
package main

//go:generate go run .

import (
	"log"
	"os"

	"enygma_gnark_shared/statement"
	"enygma_payments/gnark-server/pkg/circuits/burn"
	"enygma_payments/gnark-server/pkg/circuits/deposit"
	enygma "enygma_payments/gnark-server/pkg/circuits/enygma"
	"enygma_payments/gnark-server/pkg/circuits/enygma_fee"
	"enygma_payments/gnark-server/pkg/circuits/usdr"
	"enygma_payments/gnark-server/pkg/circuits/withdraw"
)

// Output paths, relative to this directory (go generate and go test both
// run here).
const (
	goOut  = "../../../layout/layout.go"
	solOut = "../../../contracts/enygma/contracts/StatementLayout.sol"
)

var target = statement.Target{
	Generator:       "gnark-server/cmd/genlayout",
	Subject:         "the payments circuits'",
	GoPackage:       "layout",
	SolidityLibrary: "StatementLayout",
}

func generate() map[string][]byte {
	layouts := []statement.Layout{
		enygma.StatementLayout(),
		enygma_fee.StatementLayout(),
		usdr.StatementLayout(),
		withdraw.StatementLayout(),
		deposit.StatementLayout(),
		burn.StatementLayout(),
	}
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
