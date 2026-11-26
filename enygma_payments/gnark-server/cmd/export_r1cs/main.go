// Command export_r1cs compiles every circuit the gnark server proves and
// writes each one's constraint system to <dir>/<Name>.r1cs: the input of the
// setup ceremony (enygma_ceremony), whose phase2-seal writes back
// <Name>PK.key and <Name>VK.key. The server loads them from the paths
// config.Load names, <Base>Pk<N>.key and <Base>Vk<N>.key where Name is
// <Base><N> (Withdraw6 is keys/zkdvp/WithdrawPk6.key), Deposit's and
// Withdraw6's under keys/zkdvp/, so the sealed keys are renamed into place.
// Run from gnark-server/:
//
//	go run ./cmd/export_r1cs -out <dir>
//
// It lives apart from ./keygen and the packages keygen builds, so adding it
// leaves keys/keys.src, the fingerprint of the keys' circuit sources,
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

	"enygma_payments/gnark-server/pkg/circuits/burn"
	"enygma_payments/gnark-server/pkg/circuits/deposit"
	"enygma_payments/gnark-server/pkg/circuits/enygma"
	"enygma_payments/gnark-server/pkg/circuits/enygma_fee"
	"enygma_payments/gnark-server/pkg/circuits/usdr"
	"enygma_payments/gnark-server/pkg/circuits/withdraw"
)

// circuits are the circuits the server proves, by the name their keys are
// sealed under, each built as keygen/generate_keys.go builds it (and as its
// prove handler's template builds it), at its package's DefaultConfig.
// Burn's keys are gitignored (make payments-burn-keygen) but served all the
// same.
var circuits = []struct {
	name    string
	circuit func() frontend.Circuit
}{
	{"Enygma", enygmaCircuit},
	{"EnygmaFee", enygmaFeeCircuit},
	{"Usdr", usdrCircuit},
	{"Deposit", depositCircuit},
	{"Withdraw6", withdrawCircuit},
	{"Burn", func() frontend.Circuit { return &burn.BurnCircuit{} }},
}

func enygmaCircuit() frontend.Circuit {
	config := enygma.DefaultConfig
	n := config.NCommitment
	return &enygma.EnygmaCircuit{
		Config:                     config,
		FingerPrintofSharedSecrets: square(n),
		PublicKey:                  make([]frontend.Variable, n),
		PreviousCommit:             make([][2]frontend.Variable, n),
		TxCommit:                   make([][2]frontend.Variable, n),
		AnonymitySet:               make([]frontend.Variable, n),
		SharedSecrets:              make([]frontend.Variable, n),
		MessageTags:                make([]frontend.Variable, n),
		TxValues:                   make([]frontend.Variable, n),
		TxRandomValues:             make([]frontend.Variable, n),
	}
}

func enygmaFeeCircuit() frontend.Circuit {
	config := enygma_fee.DefaultConfig
	n := config.NCommitment
	return &enygma_fee.EnygmaFeeCircuit{
		Config:                     config,
		FingerPrintofSharedSecrets: square(n),
		PublicKey:                  make([]frontend.Variable, n),
		PreviousCommit:             make([][2]frontend.Variable, n),
		TxCommit:                   make([][2]frontend.Variable, n),
		AnonymitySet:               make([]frontend.Variable, n),
		SharedSecrets:              make([]frontend.Variable, n),
		MessageTags:                make([]frontend.Variable, n),
		TxValues:                   make([]frontend.Variable, n),
		TxRandomValues:             make([]frontend.Variable, n),
	}
}

func usdrCircuit() frontend.Circuit {
	config := usdr.DefaultConfig
	n := config.NCommitment
	return &usdr.USDrCircuit{
		Config:                     config,
		FingerPrintofSharedSecrets: square(n),
		PublicKey:                  make([]frontend.Variable, n),
		PreviousCommit:             make([][2]frontend.Variable, n),
		TxCommit:                   make([][2]frontend.Variable, n),
		AnonymitySet:               make([]frontend.Variable, n),
		SharedSecrets:              make([]frontend.Variable, n),
		MessageTags:                make([]frontend.Variable, n),
		TxValues:                   make([]frontend.Variable, n),
		TxRandomValues:             make([]frontend.Variable, n),
	}
}

func depositCircuit() frontend.Circuit {
	config := deposit.DefaultConfig
	n := config.NCommitment
	return &deposit.DepositEnygmaCircuit{
		Config:                     config,
		FingerPrintofSharedSecrets: square(n),
		PublicKey:                  make([]frontend.Variable, n),
		PreviousCommit:             make([][2]frontend.Variable, n),
		TxCommit:                   make([][2]frontend.Variable, n),
		AnonymitySet:               make([]frontend.Variable, n),
		SharedSecrets:              make([]frontend.Variable, n),
		MessageTags:                make([]frontend.Variable, n),
		TxValues:                   make([]frontend.Variable, n),
		TxRandomValues:             make([]frontend.Variable, n),
	}
}

func withdrawCircuit() frontend.Circuit {
	config := withdraw.DefaultConfig
	n := config.NCommitment
	return &withdraw.WithdrawEnygmaCircuit{
		Config:                     config,
		FingerPrintofSharedSecrets: square(n),
		PublicKey:                  make([]frontend.Variable, n),
		PreviousCommit:             make([][2]frontend.Variable, n),
		TxCommit:                   make([][2]frontend.Variable, n),
		AnonymitySet:               make([]frontend.Variable, n),
		SharedSecrets:              make([]frontend.Variable, n),
		MessageTags:                make([]frontend.Variable, n),
		TxValues:                   make([]frontend.Variable, n),
		TxRandomValues:             make([]frontend.Variable, n),
	}
}

// square is an n×n matrix of variables, a FingerPrintofSharedSecrets.
func square(n int) [][]frontend.Variable {
	m := make([][]frontend.Variable, n)
	for i := range m {
		m[i] = make([]frontend.Variable, n)
	}
	return m
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
