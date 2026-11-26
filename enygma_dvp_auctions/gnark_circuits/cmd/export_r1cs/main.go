// Command export_r1cs compiles every circuit the auctions gnark server proves
// and writes each one's constraint system to <dir>/<Name>.r1cs: the input of
// the setup ceremony (enygma_ceremony). Its phase2-seal writes back
// <Name>PK.key and <Name>VK.key, but this server loads <Name>.pk and
// <Name>.vk (server/config's defaults): install the sealed keys under those
// names, or name them in a config file passed to cmd/server. Run from
// gnark_circuits/:
//
//	go run ./cmd/export_r1cs -out <dir>
//
// It lives apart from cmd/keygen and the packages keygen builds, so adding
// it leaves the fingerprint of the keys' circuit sources
// (scripts/keys_src.sh enygma_dvp_auctions/gnark_circuits ./cmd/keygen)
// unchanged.
package main

import (
	"bufio"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	"enygma_dvp_auctions/gnark_circuits/templates"
)

// merkleDepth is the Merkle depth AuctionLock and AuctionBid are compiled
// at: a const in cmd/keygen and in each one's prove handler, as templates
// exports none. It sets both the config and the path's length, as they do.
const merkleDepth = 8

// circuits are the circuits the server proves, by the name their keys are
// saved under, at the shapes cmd/keygen and the prove handlers compile.
var circuits = []struct {
	name    string
	circuit func() frontend.Circuit
}{
	{"AuctionLock", func() frontend.Circuit {
		return &templates.AuctionLockCircuit{
			Config:         templates.AuctionLockCircuitConfig{TmMerkleTreeDepth: merkleDepth},
			WtPathElements: make([]frontend.Variable, merkleDepth),
		}
	}},
	{"AuctionBid", func() frontend.Circuit {
		return &templates.AuctionBidCircuit{
			Config:         templates.AuctionBidCircuitConfig{TmMerkleTreeDepth: merkleDepth},
			WtPathElements: make([]frontend.Variable, merkleDepth),
		}
	}},
	{"AuctionWithdraw", func() frontend.Circuit { return &templates.AuctionWithdrawCircuit{} }},
	{"AuctionBatch", func() frontend.Circuit {
		n := templates.AuctionBatchSize
		return &templates.AuctionBatchCircuit{
			StBidCommitA: make([]frontend.Variable, n),
			WtActive:     make([]frontend.Variable, n),
			WtPkBidder:   make([]frontend.Variable, n),
			WtSaltA:      make([]frontend.Variable, n),
			WtAmount:     make([]frontend.Variable, n),
			WtTokenId:    make([]frontend.Variable, n),
		}
	}},
	{"AuctionFinal", func() frontend.Circuit {
		k := templates.AuctionFinalBatches
		return &templates.AuctionFinalCircuit{
			StBatchWinnerCommit: make([]frontend.Variable, k),
			StBatchWinnerPk:     make([]frontend.Variable, k),
			StBatchWinnerAmount: make([]frontend.Variable, k),
			WtBatchActive:       make([]frontend.Variable, k),
		}
	}},
	{"AuctionRevert", func() frontend.Circuit { return &templates.AuctionRevertCircuit{} }},
}

func main() {
	out := flag.String("out", "", "the directory to write <Name>.r1cs into (created if missing; no file may exist)")
	flag.Parse()
	if *out == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := export(*out, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

// export compiles each circuit and writes its constraint system into dir,
// never over an existing file, and prints each one's constraint count and
// sha256 to report.
func export(dir string, report io.Writer) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeManifest(dir); err != nil {
		return err
	}
	for _, c := range circuits {
		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, c.circuit())
		if err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
		path := filepath.Join(dir, c.name+".r1cs")
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		h := sha256.New()
		w := bufio.NewWriter(io.MultiWriter(f, h))
		if _, err := ccs.WriteTo(w); err != nil {
			f.Close()
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := w.Flush(); err != nil {
			f.Close()
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := f.Close(); err != nil {
			return err
		}
		fmt.Fprintf(report, "%-24s %9d constraints  sha256 %x\n", c.name, ccs.GetNbConstraints(), h.Sum(nil))
	}
	return nil
}
