// Package retailkeys checks that the Payment-family keys in scripts/keys are
// enygma_retail_payments' committed ones.
//
// enygma_dvp's gnark server proves the Payment-family circuits from
// enygma_retail_payments' templates, with retail's keys, which make dvp-keygen
// copies into scripts/keys (the Makefile's DVP_RETAIL_KEYS). Retail's keys are
// built from the circuit sources their committed stamp names (make
// retail-keys-check). Any other keys there, such as those an older make
// dvp-keygen built from enygma_dvp's former copies of the templates, would
// prove for the server, or register in a deployment's VK slots, circuits other
// than retail's. cmd/export_vk_payment and cmd/export_vk_init refuse them with
// Require before they write anything.
package retailkeys

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// KeysDir holds the keys the gnark server loads, and Dir
// enygma_retail_payments' committed keys (through LFS), both relative to
// gnark_circuits/, where the server and the cmd tools run.
const (
	KeysDir = "scripts/keys"
	Dir     = "../../enygma_retail_payments/gnark_circuits/scripts/keys"
)

// PaymentFamily are the Payment-family circuits, in the Payment deployment's
// VK slot order (0-4).
var PaymentFamily = []string{"Payment", "Payment2in", "PaymentFee", "PaymentRelayerFeePublic", "UsdrFee"}

// Require fails unless each circuit's proving and verifying key in dir
// (<circuit>PK.key and <circuit>VK.key) is, byte for byte, the one in
// retailDir. Its error names the first key that is missing or differs, and
// make dvp-keygen, which copies them.
func Require(dir, retailDir string, circuits ...string) error {
	for _, c := range circuits {
		for _, kind := range []string{"PK", "VK"} {
			name := c + kind + ".key"
			ours, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return fmt.Errorf("%w: make dvp-keygen copies enygma_retail_payments' keys", err)
			}
			retail, err := os.ReadFile(filepath.Join(retailDir, name))
			if err != nil {
				return err
			}
			if !bytes.Equal(ours, retail) {
				return fmt.Errorf("%s is not enygma_retail_payments' %s: the Payment-family circuits are proved and registered with retail's keys, which make dvp-keygen copies",
					filepath.Join(dir, name), filepath.Join(retailDir, name))
			}
		}
	}
	return nil
}
