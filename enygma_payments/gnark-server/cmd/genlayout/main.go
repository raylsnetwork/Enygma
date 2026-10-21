// Command genlayout renders the payments circuits' statement layouts into
// the committed Go package (enygma_payments/layout, imported by the relayer)
// and Solidity library (StatementLayout.sol, used by Enygma.sol).
//
// Regenerate after changing any circuit's public signals:
//
//	cd gnark-server && go generate ./cmd/genlayout
//
// main_test.go fails while the committed files are stale.
package main

//go:generate go run .

import (
	"log"
	"os"

	enygma "enygma_payments/gnark-server/pkg/circuits/enygma"
	"enygma_payments/gnark-server/pkg/circuits/enygma_fee"
	"enygma_payments/gnark-server/pkg/circuits/usdr"
	"enygma_payments/gnark-server/pkg/statement"
)

// Output paths, relative to this directory (go generate and go test both
// run here).
const (
	goOut  = "../../../layout/layout.go"
	solOut = "../../../contracts/enygma/contracts/StatementLayout.sol"
)

func generate() map[string][]byte {
	layouts := []statement.Layout{
		enygma.StatementLayout(),
		enygma_fee.StatementLayout(),
		usdr.StatementLayout(),
	}
	return map[string][]byte{
		goOut:  statement.RenderGo(layouts),
		solOut: statement.RenderSolidity(layouts),
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
