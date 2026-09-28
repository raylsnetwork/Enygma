package usdr

// USDrCircuit is a near-verbatim copy of enygma.EnygmaCircuit
// (pkg/circuits/enygma/circuit.go) — same 80-signal FingerPrint layout,
// same 13 constraint categories, same k-anonymity structure. It proves a
// private balance update over the USDr balance field (a second balance
// tracked per account in Enygma.sol, alongside the existing main-asset
// balance) instead of the main asset's balance. The circuit itself is
// asset-agnostic — nothing here references "which balance"; that's purely
// a matter of which on-chain state (balanceCommitments vs
// usdrBalanceCommitments) the contract checks this proof's public signals
// against.
//
// Domain-separation constants (see Define, below) are deliberately
// DIFFERENT from EnygmaCircuit's: the sender is expected to submit a USDr
// proof alongside a main-asset proof for the same transaction, reusing the
// same SecretKey/PreviousSenderRandomValue/BlockNumber/SharedSecrets. If
// this circuit used the identical constants, MessageTags/TxRandomValues
// would be identical or trivially correlated between the two proofs. See
// the two constants inside Define().

import (
	"math/big"

	pos "enygma_payments/gnark-server/poseidon"
	utils "enygma_payments/gnark-server/utils"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/native/twistededwards"
)

// JubJubPrimeSubGroupStr is the same subgroup order EnygmaCircuit uses —
// shared across all payment circuits, not asset-specific.
const JubJubPrimeSubGroupStr = "2736030358979909402780800718157159386076813972158567259200215660948447373041"

type USDrCircuitConfig struct {
	NCommitment int
}

type USDrCircuit struct {
	Config USDrCircuitConfig

	// Public signals
	FingerPrintofSharedSecrets [][]frontend.Variable  `gnark:",public"` // k×k matrix: FingerPrint[i][j] = Poseidon(secret[i][j]); diagonal skipped
	PublicKey                  []frontend.Variable    `gnark:",public"` // Public keys from all other PLs
	PreviousCommit             [][2]frontend.Variable `gnark:",public"` // Array of previous USDr balances (Pedersen commitments)
	TxCommit                   [][2]frontend.Variable `gnark:",public"` // Array containing the USDr commitments for this new tx
	BlockNumber                frontend.Variable      `gnark:",public"` // Block number to ensure random factors are well-generated
	AnonymitySet               []frontend.Variable    `gnark:",public"` // Array with indices of the banks in the tx ("k"-anonymity)
	MessageTags                []frontend.Variable    `gnark:",public"` // Array of tag messages for unique transactions
	Nullifier                  frontend.Variable      `gnark:",public"` // Nullifier to prevent double spend
	// FeeAmount is public (not private, unlike EnygmaCircuit's transfer
	// amount) — the USDr fee is a fixed, protocol-known value, not
	// something the sender needs to hide. Making it public lets
	// Enygma.sol read this signal directly and assert it equals the
	// contract's configured fixed fee, rejecting a proof for any other
	// amount. Appended after the shared 80-signal layout; this is slot 80.
	FeeAmount frontend.Variable `gnark:",public"`
	// Fix L-01: same rationale and same trivial self-equality pattern as
	// EnygmaCircuit.DomainId (see that field's doc comment) — this circuit
	// previously had no domain-separator signal at all, so a USDr proof
	// generated for one Enygma deployment could be paired with a valid
	// main proof for a different deployment sharing the same pre-state
	// (same registered keys/anonymity set/block number). Appended last —
	// this is slot 81. FeeRecipientKey follows at slot 82 (83 signals in all).
	DomainId frontend.Variable `gnark:",public"`
	// FeeRecipientKey is the spend public key of the account that is paid the
	// fee (slot 82, appended last). The circuit only used to require that the
	// non-sender credits ADD UP to FeeAmount; which slot got them, and in what
	// split, was left to the prover, and nothing checked that the relayer was
	// among them, so a bank could pay the fee to another participant (or split
	// it) and still have its transfer relayed. Now exactly one participant's
	// PublicKey equals this value, it must not be the sender's own slot, it
	// receives the whole FeeAmount, and every other non-sender slot receives
	// nothing. Enygma.sol requires this signal to equal the public key of the
	// account that submitted the transaction (msg.sender), i.e. the relayer.
	// Consequence: a bank cannot submit its own transfer, because it cannot be
	// its own fee recipient; a transfer needs a separate relaying account.
	FeeRecipientKey frontend.Variable `gnark:",public"`

	// Private signals
	SenderId                  frontend.Variable   // Identifier of the sender
	SharedSecrets             []frontend.Variable // Array of shared secrets (1D: sender's pre-selected row)
	SecretKey                 frontend.Variable   // Secret key of the sender
	PreviousSenderBalance     frontend.Variable   // Previous USDr balance in the last Pedersen commitment
	PreviousSenderRandomValue frontend.Variable   // Previous random factor in the last Pedersen commitment
	TxValues                  []frontend.Variable // Array of USDr balances debited/credited
	TxRandomValues            []frontend.Variable // Array of random factors for the pedersen commitments
}

func (circuit *USDrCircuit) Define(api frontend.API) error {
	k := circuit.Config.NCommitment
	// Subgroup order
	JubJubPrimeSubGroup := frontend.Variable(JubJubPrimeSubGroupStr)

	//////////////////////////////////**///////////////////////////////////
	// Check if SenderId is in K
	sumIsInK := frontend.Variable(0)
	for i := 0; i < k; i++ {
		isEqual := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		sumIsInK = api.Add(isEqual, sumIsInK)
	}
	api.AssertIsEqual(sumIsInK, 1)

	///////////////////////////////////**///////////////////////////////////
	// Check if Amount To Transfer Corresponds To Sender TxValues
	selected_v := frontend.Variable(0)
	for i := 0; i < k; i++ {
		diff := api.Sub(circuit.SenderId, circuit.AnonymitySet[i])
		eq := api.IsZero(diff)
		selected_v = api.Add(selected_v, api.Mul(eq, circuit.TxValues[i]))
	}

	selectedVBits := api.ToBinary(selected_v, 252)
	// vBits: Fix C-02 (ported from enygma/circuit.go). FeeAmount both feeds
	// a Pedersen scalar mult (via TxCommit, transitively) and was
	// independently range-checked here at 252 bits — 252 bits admits
	// values up to 2P and Com(b,r) == Com(b+kP,r). Bounded to 64 bits: ample
	// for any realistic fee, and 2^64 << P leaves no aliasing room.
	vBits := api.ToBinary(circuit.FeeAmount, 64)
	pDiffBits := api.ToBinary(JubJubPrimeSubGroup, 252)

	selectedVConstrained := api.FromBinary(selectedVBits...)
	vConstrained := api.FromBinary(vBits...)
	pDiffConstrained := api.FromBinary(pDiffBits...)

	// Compute (p - sender_tx_value) mod p
	expectedTxValue := api.Sub(pDiffConstrained, vConstrained)
	expectedTxValueMod := utils.ReduceModP(api, expectedTxValue) // Fix C-01

	api.AssertIsEqual(selectedVConstrained, expectedTxValueMod)

	///////////////////////////////////**///////////////////////////////////
	// Fix C-03 (ported from enygma/circuit.go): only the sender's own slot
	// (selected_v, just constrained above) had any bound on its magnitude.
	// Every OTHER TxValues[i] was unconstrained except for the aggregate
	// Σ TxCommit == (0,1) — since P − w (a debit of w) is a perfectly valid
	// field element, a prover could name any other registered account, put
	// a small honest credit at one slot and P − w at another, and the
	// on-chain contract would apply both deltas verbatim: a silent debit of
	// w from an account that never consented. Range-check every non-sender
	// slot to 64 bits and separately assert the non-sender credits sum to
	// the sender's declared fee AS INTEGERS, not just mod P.
	sumNonSenderValues := frontend.Variable(0)
	recipientCount := frontend.Variable(0)
	for i := 0; i < k; i++ {
		isSenderSlot := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		nonSenderValue := api.Select(isSenderSlot, frontend.Variable(0), circuit.TxValues[i])
		nonSenderBits := api.ToBinary(nonSenderValue, 64)
		nonSenderConstrained := api.FromBinary(nonSenderBits...)
		sumNonSenderValues = api.Add(sumNonSenderValues, nonSenderConstrained)

		// Fee recipient binding: the slot whose public key is FeeRecipientKey
		// gets the whole fee; every other non-sender slot gets exactly 0; the
		// sender's own slot can never be the recipient.
		isRecipient := api.IsZero(api.Sub(circuit.PublicKey[i], circuit.FeeRecipientKey))
		recipientCount = api.Add(recipientCount, isRecipient)
		api.AssertIsEqual(api.Mul(isRecipient, isSenderSlot), 0)
		api.AssertIsEqual(nonSenderConstrained, api.Mul(isRecipient, vConstrained))
	}
	api.AssertIsEqual(recipientCount, 1)
	api.AssertIsEqual(sumNonSenderValues, vConstrained)

	///////////////////////////////////**///////////////////////////////////
	// Check if previous commits and tx commits are on Curve
	for i := 0; i < k; i++ {
		utils.AssertPointsIsOnCurve(api, circuit.PreviousCommit[i][0], circuit.PreviousCommit[i][1])
		utils.AssertPointsIsOnCurve(api, circuit.TxCommit[i][0], circuit.TxCommit[i][1])
	}

	///////////////////////////////////**///////////////////////////////////
	// Check knowledge of secret of sender
	selectedSecret := frontend.Variable(0)
	for i := 0; i < k; i++ {
		eq := api.IsZero(api.Sub(circuit.SenderId, circuit.AnonymitySet[i]))
		selectedSecret = api.Add(selectedSecret, api.Mul(eq, circuit.SharedSecrets[i]))
	}

	// Fix (nullifier canonicality): the blinding factor reaches a Pedersen
	// scalar multiplication, which only sees it mod P, but it used to reach
	// the Poseidon below as a raw field element. Every value r + k*P below Fr
	// (up to 8 of them) therefore opens the SAME on-chain commitment yet
	// produced a different secretRemain and nullifier, so one state had
	// several valid nullifiers. Hashing the reduced value makes the nullifier
	// a function of the commitment's actual opening. Honest blinding factors
	// are already < P, so honest nullifiers are unchanged.
	prevRCanonical := utils.ReduceModP(api, circuit.PreviousSenderRandomValue)
	secretSenderCalculated := pos.Poseidon(api, []frontend.Variable{prevRCanonical, circuit.SecretKey})
	secretRemain := utils.ReduceModP(api, secretSenderCalculated) // Fix C-01

	api.AssertIsEqual(secretRemain, selectedSecret)

	///////////////////////////////////**//////////////////////////////////////
	// Knowledge of Nullifier
	// Preimage = secretRemain (Poseidon(prevR, sk) mod p) — unique per sender per round
	// Diagonal of FingerPrintofSharedSecrets is skipped, so we use the sender's self-derived secret
	//
	// Computed here (moved up, matching enygma/circuit.go's Fix H-01/H-02
	// structure) because computedNullifier is reused below as the
	// per-transaction value mixed into the message-tag and blinding-factor
	// derivations — see those blocks for why.
	computedNullifier := pos.Poseidon(api, []frontend.Variable{secretRemain, circuit.BlockNumber})
	api.AssertIsEqual(computedNullifier, circuit.Nullifier)

	///////////////////////////////////**///////////////////////////////////
	// Check if FingerPrintofSharedSecrets is well formed for sender's column
	// SharedSecrets[i] = secret between bank i and the sender (sender's column)
	// FingerPrintofSharedSecrets[i][senderCol] = Poseidon(SharedSecrets[i]) for i ≠ senderCol
	// Diagonal entries (i == senderCol) are skipped — no self-shared-secret

	for i := 0; i < k; i++ {
		calculatedHash := pos.Poseidon(api, []frontend.Variable{circuit.SharedSecrets[i]})
		hashMod := utils.ReduceModP(api, calculatedHash) // Fix C-01

		isRowSender := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))

		for j := 0; j < k; j++ {
			isColSender := api.IsZero(api.Sub(circuit.AnonymitySet[j], circuit.SenderId))
			// isNotDiagonal = 1 - (isRowSender AND isColSender); only 0 when both i and j are the sender's position
			isNotDiagonal := api.Sub(1, api.Mul(isRowSender, isColSender))
			// Enforce: if j is the sender's column AND not the diagonal → FingerPrint[i][j] == hashMod
			shouldCheck := api.Mul(isColSender, isNotDiagonal)
			diff := api.Sub(circuit.FingerPrintofSharedSecrets[i][j], hashMod)
			api.AssertIsEqual(api.Mul(shouldCheck, diff), 0)
		}
	}

	///////////////////////////////////**///////////////////////////////////
	// Knowledge of SecretKey - check if SecretKey generates senderId's PublicKey
	selectedPK := frontend.Variable(0)

	for i := 0; i < k; i++ {
		diff := api.Sub(circuit.SenderId, circuit.AnonymitySet[i])
		eq := api.IsZero(diff)
		selectedPK = api.Add(selectedPK, api.Mul(eq, circuit.PublicKey[i]))
	}
	pk := pos.Poseidon(api, []frontend.Variable{circuit.SecretKey, circuit.SecretKey})
	pkMod := utils.ReduceModP(api, pk) // Fix C-01

	api.AssertIsEqual(selectedPK, pkMod)

	///////////////////////////////////**///////////////////////////////////
	// Check Knowledge of Previous Commitment
	selectedPreviousCommitmentX := frontend.Variable(0)
	selectedPreviousCommitmentY := frontend.Variable(0)

	for i := 0; i < k; i++ {
		diff := api.Sub(circuit.SenderId, circuit.AnonymitySet[i])
		eq := api.IsZero(diff)

		selectedPreviousCommitmentX = api.Add(selectedPreviousCommitmentX, api.Mul(eq, circuit.PreviousCommit[i][0]))
		selectedPreviousCommitmentY = api.Add(selectedPreviousCommitmentY, api.Mul(eq, circuit.PreviousCommit[i][1]))
	}

	computedPreviousCommitment := utils.PedersenCommitment(api, circuit.PreviousSenderBalance, circuit.PreviousSenderRandomValue)

	api.AssertIsEqual(selectedPreviousCommitmentX, computedPreviousCommitment.X)
	api.AssertIsEqual(selectedPreviousCommitmentY, computedPreviousCommitment.Y)

	///////////////////////////////////**///////////////////////////////////
	// Knowledge of Message Tag - verify message tags are well formed
	//
	// DOMAIN-SEPARATED from EnygmaCircuit (which uses Poseidon(12) here):
	// a USDr proof for the same transaction reuses the same SharedSecrets
	// and BlockNumber as the main-asset proof, so an identical domain
	// constant would make MessageTags identical/correlated between the two
	// proofs. 120 is arbitrary but must stay distinct from EnygmaCircuit's 12
	// and from any other circuit sharing this witness pattern.
	//
	// Fix H-01 (ported from enygma/circuit.go): the tag used to be
	// Poseidon(HashTag, SharedSecrets[i], BlockNumber) — BlockNumber is the
	// epoch anchor, not the current transaction, so every tag for a
	// receiver slot i was constant for an entire epoch, letting a passive
	// chain observer single out the sender's slot (the only one that
	// changes) and link "A pays B"/"B pays A" to the same relationship.
	// computedNullifier (fresh per transaction — see the Nullifier block
	// above) replaces BlockNumber, and SenderId/AnonymitySet[i] are mixed
	// in as ordered inputs so direction can't correlate two proofs either.
	HashTag := pos.Poseidon(api, []frontend.Variable{120})
	for i := 0; i < k; i++ {
		directionTag := pos.Poseidon(api, []frontend.Variable{circuit.SenderId, circuit.AnonymitySet[i]})
		perSlotNonce := pos.Poseidon(api, []frontend.Variable{computedNullifier, directionTag})
		calculatedMessageTag := pos.Poseidon(api, []frontend.Variable{HashTag, circuit.SharedSecrets[i], perSlotNonce})
		calculatedMessageTagMod := utils.ReduceModP(api, calculatedMessageTag) // Fix C-01

		api.AssertIsEqual(circuit.MessageTags[i], calculatedMessageTagMod)
	}

	///////////////////////////////////**///////////////////////////////////
	// Check Pedersen (Sum FeeAmount-derived TxValues, SumR) = Pedersen (0, 0) = (0,1)
	sumX := frontend.Variable(0)
	sumY := frontend.Variable(0)

	for i := 0; i < k; i++ {
		sumX = api.Add(sumX, circuit.TxValues[i])
		sumY = api.Add(sumY, circuit.TxRandomValues[i])
	}
	PedersenZero := utils.PedersenCommitment(api, sumX, sumY)

	api.AssertIsEqual(PedersenZero.X, frontend.Variable(0))
	api.AssertIsEqual(PedersenZero.Y, frontend.Variable(1))

	// Check Sum TxCommits = (0,1)
	sum := twistededwards.Point{
		X: circuit.TxCommit[0][0],
		Y: circuit.TxCommit[0][1],
	}
	for i := 1; i < k; i++ {
		point := twistededwards.Point{
			X: circuit.TxCommit[i][0],
			Y: circuit.TxCommit[i][1],
		}
		sum = utils.PointAdd(api, sum, point)
	}

	api.AssertIsEqual(sum.X, frontend.Variable(0))
	api.AssertIsEqual(sum.Y, frontend.Variable(1))

	///////////////////////////////////**///////////////////////////////////
	// Range Proof: previousV >= sender_tx_value and sender_tx_value >= 0
	// Fix C-02 (ported from enygma/circuit.go): PreviousSenderBalance is
	// passed directly into utils.PedersenCommitment(api,
	// circuit.PreviousSenderBalance, ...) above (the same wire), so a
	// 252-bit bound here let claimed = real + P (or +2P) satisfy both that
	// commitment (periodic mod P) and this solvency check with an inflated
	// balance. 64 bits closes it the same way as FeeAmount above.
	previousVBits := api.ToBinary(circuit.PreviousSenderBalance, 64)
	previousVConstrained := api.FromBinary(previousVBits...)

	// previousV >= sender_tx_value means Cmp(previousV, sender_tx_value) != -1
	prevVGreaterEqualV := api.Cmp(previousVConstrained, vConstrained)
	api.AssertIsEqual(api.IsZero(api.Add(prevVGreaterEqualV, frontend.Variable(1))), frontend.Variable(0))

	// sender_tx_value >= 0 means Cmp(sender_tx_value, 0) != -1
	vGreaterEqualZero := api.Cmp(vConstrained, frontend.Variable(0))
	api.AssertIsEqual(api.IsZero(api.Add(vGreaterEqualZero, frontend.Variable(1))), frontend.Variable(0))

	///////////////////////////////////**//////////////////////////////////////
	// Check if Tx Commitment is well formed
	for i := 0; i < k; i++ {

		computedPedersenCommitment := utils.PedersenCommitment(api, circuit.TxValues[i], circuit.TxRandomValues[i])

		api.AssertIsEqual(circuit.TxCommit[i][0], computedPedersenCommitment.X)
		api.AssertIsEqual(circuit.TxCommit[i][1], computedPedersenCommitment.Y)
	}

	///////////////////////////////////**//////////////////////////////////////
	// Check if random factors R are well formed
	calculatedRandomFactor := make([]frontend.Variable, k)
	receiverHashesModP := make([]frontend.Variable, k)
	sumOfReceiverHashes := frontend.Variable(0)

	// DOMAIN-SEPARATED from EnygmaCircuit (Poseidon(21)) — same rationale as
	// HashTag above.
	//
	// Fix H-02 (ported from enygma/circuit.go): same root cause as H-01
	// above, applied to the Pedersen blinding factor instead of the message
	// tag — r_i used to be Poseidon(HashRandom, SharedSecrets[i],
	// BlockNumber), constant for a whole epoch, so subtracting two
	// commitment deltas at a common slot across two same-epoch transactions
	// cancelled the blinding term exactly, leaving (v1-v2)*G recoverable by
	// table lookup for realistic (small) amounts. computedNullifier/
	// SenderId/AnonymitySet[i] replace BlockNumber, same as H-01.
	HashRandom := pos.Poseidon(api, []frontend.Variable{210})

	// First pass: compute all hashes, reduce modulo JubJubPrimeSubGroup
	for i := 0; i < k; i++ {
		randomDirectionTag := pos.Poseidon(api, []frontend.Variable{circuit.SenderId, circuit.AnonymitySet[i]})
		randomPerSlotNonce := pos.Poseidon(api, []frontend.Variable{computedNullifier, randomDirectionTag})
		RandomFactor := pos.Poseidon(api, []frontend.Variable{HashRandom, circuit.SharedSecrets[i], randomPerSlotNonce})
		// Reduce RandomFactor modulo JubJubPrimeSubGroup
		hashModP := utils.ReduceModP(api, RandomFactor) // Fix C-01

		receiverHashesModP[i] = hashModP

		// Check if this participant is a receiver (not the sender)
		isSender := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		isReceiver := api.Sub(1, isSender)

		// Add to sum only if this is a receiver
		sumOfReceiverHashes = api.Add(sumOfReceiverHashes, api.Mul(isReceiver, hashModP))
	}
	// Reduce the sum modulo JubJubPrimeSubGroup
	senderRandomFactor := utils.ReduceModP(api, sumOfReceiverHashes) // Fix C-01

	// Second pass: assign the correct random factors based on role
	for i := 0; i < k; i++ {
		isSender := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		// For receivers: neg(hash mod p) = p - hash
		// For sender: sum of receiver hashes mod p
		receiverRandomFactor := api.Sub(JubJubPrimeSubGroup, receiverHashesModP[i])
		calculatedRandomFactor[i] = api.Select(isSender, senderRandomFactor, receiverRandomFactor)
	}
	// Verification: check that calculated factors match provided TxRandomValues
	for i := 0; i < k; i++ {
		api.AssertIsEqual(calculatedRandomFactor[i], circuit.TxRandomValues[i])
	}

	// Fix L-01: a trivial self-equality keeps DomainId a genuinely
	// constrained (not compiler-prunable) wire — see the field's doc
	// comment for why no stronger constraint is needed.
	api.AssertIsEqual(circuit.DomainId, circuit.DomainId)

	return nil

}

type USDrRequest struct {
	// Fix M-08: these were min=1,max=6 — the handler unconditionally
	// indexes [0..NCommitment-1] (NCommitment=6), so a request with fewer
	// elements passed binding validation and then panicked on the first
	// out-of-range index (a cheap, repeatable remote DoS). len=6 (matching
	// the sibling enygma/circuit.go's own M-08 fix) requires exactly 6.
	FingerPrintofSharedSecrets [][]string  `json:"fingerprint_shared_secrets" binding:"required,len=6,dive,len=6"`
	PublicKey                  []string    `json:"public_keys" binding:"required,len=6"`
	PreviousCommit             [][2]string `json:"previous_commits" binding:"required,len=6,dive,len=2"`
	TxCommit                   [][2]string `json:"tx_commits" binding:"required,len=6,dive,len=2"`
	BlockNumber                string      `json:"block_number" binding:"required"`
	AnonymitySet               []string    `json:"anonymity_set" binding:"required,len=6"`
	MessageTags                []string    `json:"message_tags" binding:"required,len=6"`
	Nullifier                  string      `json:"nullifier" binding:"required"`

	SenderID                  string   `json:"sender_id" binding:"required"`
	SharedSecrets             []string `json:"shared_secrets" binding:"required,len=6"`
	SecretKey                 string   `json:"secret_key" binding:"required"`
	PreviousSenderBalance     string   `json:"previous_sender_balance" binding:"required"`
	PreviousSenderRandomValue string   `json:"previous_sender_random_value" binding:"required"`
	TxValues                  []string `json:"tx_values" binding:"required,len=6"`
	TxRandomValues            []string `json:"tx_random_values" binding:"required,len=6"`
	SenderTxValue             string   `json:"sender_tx_value" binding:"required"`
	DomainId                  string   `json:"domain_id" binding:"required"`
	// FeeRecipientKey: public key of the account paid the fee (the relaying
	// account); see USDrCircuit.FeeRecipientKey.
	FeeRecipientKey string `json:"fee_recipient_key" binding:"required"`
}

type USDrOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
