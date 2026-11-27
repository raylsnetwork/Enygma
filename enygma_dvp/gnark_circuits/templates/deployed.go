package templates

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

// MerkleTreeDepth is the depth of the vaults' Merkle trees. The
// Payment-family circuits and the DvP legs (DvpInitiator, DvpDestination)
// spend notes from the same vaults, so all of them are compiled at it.
// cmd/genlayout renders it into the layout package.
const MerkleTreeDepth = 16

// EnygmaErc20CoinVaultTreeDepth is the depth of the DvP deployment's
// EnygmaErc20CoinVault tree, through which Enygma (enygma_payments) moves
// value in and out. No circuit spends from that tree: Enygma redeems a
// note by its commitment (withdrawThroughEnygma), so a deeper tree would
// hide nothing, and each depositThroughEnygma pays about 39k gas a level
// to insert. Enygma's MAX_REGISTERED_ACCOUNTS is sized so that a withdraw
// into ten deposits fits EIP-7825's per-transaction gas cap at this
// depth. cmd/genlayout renders it into the layout package.
const EnygmaErc20CoinVaultTreeDepth = 8

func paymentConfig(nInputs, mOutputs int) PaymentCircuitConfig {
	return PaymentCircuitConfig{
		TmNInputs:         nInputs,
		TmMOutputs:        mOutputs,
		TmMerkleTreeDepth: MerkleTreeDepth,
	}
}
