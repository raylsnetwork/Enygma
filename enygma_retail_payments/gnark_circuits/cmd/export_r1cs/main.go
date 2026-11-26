// Command export_r1cs compiles every circuit the gnark server proves and
// writes each one's constraint system to <dir>/<Name>.r1cs: the input of the
// setup ceremony (enygma_ceremony), whose phase2-seal writes back
// <Name>PK.key and <Name>VK.key, the files server/config loads. Run from
// gnark_circuits/:
//
//	go run ./cmd/export_r1cs -out <dir>
//
// It lives apart from cmd/keygen and the packages keygen builds, so adding
// it leaves scripts/keys/keys.src, the fingerprint of the keys' circuit
// sources, unchanged.
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

	"enygma_retail_payments/gnark_circuits/templates"
)

// circuits are the circuits the server proves, by the name their keys are
// saved under, at the configs cmd/keygen and the prove handlers use.
var circuits = []struct {
	name    string
	circuit func() frontend.Circuit
}{
	{"Payment", func() frontend.Circuit { c := templates.NewPaymentCircuit(templates.PaymentConfig); return &c }},
	{"Payment2in", func() frontend.Circuit { c := templates.NewPaymentCircuit(templates.Payment2inConfig); return &c }},
	{"PaymentFee", func() frontend.Circuit { c := templates.NewPaymentFeeCircuit(templates.PaymentFeeConfig); return &c }},
	{"PaymentRelayerFeePublic", func() frontend.Circuit {
		c := templates.NewPaymentRelayerFeePublicCircuit(templates.PaymentRelayerFeePublicConfig)
		return &c
	}},
	{"UsdrFee", func() frontend.Circuit { c := templates.NewUsdrFeeCircuit(templates.UsdrFeeConfig); return &c }},
	{"PrivateMint", func() frontend.Circuit { return &templates.PrivateMintCircuit{} }},
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
