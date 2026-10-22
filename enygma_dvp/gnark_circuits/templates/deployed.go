package templates

import "github.com/consensys/gnark/frontend"

// The configs enygma_dvp's Payment-family circuits are compiled, keyed and
// served at. Keygen, the prove handlers and the statement layouts all read
// these, so a layout is derived at the shape the deployed verifying key was
// made for.
var (
	// PaymentConfig: 1 input / 2 outputs.
	PaymentConfig = paymentConfig(1, 2)
	// Payment2inConfig: 2 inputs / 2 outputs. The 1-in and 2-in circuits
	// have different R1CS and must use distinct VKs.
	Payment2inConfig = paymentConfig(2, 2)
	// PaymentFeeConfig: 1 input / 2 outputs, the fee absorbed into the
	// sender's input rather than paid out as a note.
	PaymentFeeConfig = paymentConfig(1, 2)
	// PaymentRelayerFeePublicConfig: 1 input / 3 outputs, the relayer's fee
	// paid out as its own spendable note.
	PaymentRelayerFeePublicConfig = paymentConfig(1, 3)
	// UsdrFeeConfig: 1 input / 2 outputs of a second, independent
	// relayer-fee asset with a public token id.
	UsdrFeeConfig = paymentConfig(1, 2)
)

const (
	// paymentMerkleTreeDepth is the depth of the vaults' Merkle trees.
	paymentMerkleTreeDepth = 8
	// paymentValueRange is the exclusive upper bound on every note value
	// and fee (TmRange): 10^36.
	paymentValueRange = "1000000000000000000000000000000000000"
)

func paymentConfig(nInputs, mOutputs int) PaymentCircuitConfig {
	return PaymentCircuitConfig{
		TmNInputs:         nInputs,
		TmMOutputs:        mOutputs,
		TmMerkleTreeDepth: paymentMerkleTreeDepth,
		TmRange:           frontend.Variable(paymentValueRange),
	}
}
