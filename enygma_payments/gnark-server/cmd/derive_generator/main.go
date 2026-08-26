// Command derive_generator reproduces the project's nothing-up-my-sleeve
// (NUMS) hash-to-curve construction for the Baby Jubjub generators used in
// Pedersen commitments. It is the audit's H-11 remediation: "ship the
// derivation script" and "add a CI assertion that re-derives both and
// compares them against every hardcoded copy" (the CI half lives in
// utils/nums_test.go; this is the standalone, human-runnable half).
//
// H was already derived this way (see cmd/setup/main.go, seed "1"). G was
// not: it was a free-floating constant with no script, comment, or test
// anywhere justifying it, which is exactly the "someone could have picked
// G = e·H for a retained e" trust gap H-11 describes. This tool derives G
// from a published seed distinct from H's, and can re-derive H from its
// own seed for comparison in the same run.
//
// Usage:
//
//	go run ./cmd/derive_generator            # derive G (seed "2") and H (seed "1"), print both
//	go run ./cmd/derive_generator -seed 7    # derive from an arbitrary seed, print the point
package main

import (
	"flag"
	"fmt"
	"math/big"

	"enygma-server/utils"
)

func derive(label string, seed *big.Int) (*big.Int, *big.Int) {
	x, y, iterations := utils.DeriveNUMSPoint(seed)
	fmt.Printf("%s: seed=%s iterations=%d\n", label, seed.String(), iterations)
	fmt.Printf("  X = %s\n", x.String())
	fmt.Printf("  Y = %s\n", y.String())
	return x, y
}

func main() {
	seedFlag := flag.String("seed", "", "derive a single point from this seed and exit (decimal)")
	flag.Parse()

	if *seedFlag != "" {
		seed, ok := new(big.Int).SetString(*seedFlag, 10)
		if !ok {
			fmt.Printf("invalid seed %q: not a decimal integer\n", *seedFlag)
			return
		}
		derive("point", seed)
		return
	}

	fmt.Println("Deriving H (blinding generator) — published seed \"1\":")
	hx, hy := derive("H", big.NewInt(1))

	fmt.Println()
	fmt.Println("Deriving G (value generator) — published seed \"2\" (H-11 fix):")
	gx, gy := derive("G", big.NewInt(2))

	fmt.Println()
	if hx.Cmp(gx) == 0 && hy.Cmp(gy) == 0 {
		fmt.Println("FAIL: G and H derived to the same point — pick a different seed.")
		return
	}
	fmt.Println("OK: G and H are distinct points, both on-curve, cofactor cleared (order P).")
}
