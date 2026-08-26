package enygma

import (
	"math/big"

	pos "enygma-server/poseidon"
	utils "enygma-server/utils"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/native/twistededwards"
)

// JubJubPrimeSubGroup constant used across payment circuits
const JubJubPrimeSubGroupStr = "2736030358979909402780800718157159386076813972158567259200215660948447373041"

type EnygmaCircuitConfig struct {
	NCommitment int
}

type EnygmaCircuit struct {
	Config EnygmaCircuitConfig

	// Public signals
	FingerPrintofSharedSecrets [][]frontend.Variable  `gnark:",public"` // k×k matrix: FingerPrint[i][j] = Poseidon(secret[i][j]); diagonal skipped
	PublicKey                  []frontend.Variable    `gnark:",public"` // Public keys from all other PLs
	PreviousCommit             [][2]frontend.Variable `gnark:",public"` // Array of previous balances (Pedersen commitments)
	TxCommit                   [][2]frontend.Variable `gnark:",public"` // Array containing the commitments for this new tx
	BlockNumber                frontend.Variable      `gnark:",public"` // Block number to ensure random factors are well-generated
	AnonymitySet               []frontend.Variable    `gnark:",public"` // Array with indices of the banks in the tx ("k"-anonymity)
	MessageTags                []frontend.Variable    `gnark:",public"` // Array of tag messages for unique transactions
	Nullifier                  frontend.Variable      `gnark:",public"` // Nullifier to prevent double spend
	// Fix L-01: chainId<<160 | contractAddress, packed into one field
	// element. Nothing previously bound a proof to a specific chain id or
	// deployed contract address, so an identical proof verified against
	// any fresh deployment sharing the same pre-state. No in-circuit
	// constraint needed beyond exposing it as a public signal — Groth16's
	// verification equation already cryptographically binds every public
	// input to the specific proof, so Enygma.sol comparing this against
	// block.chainid/address(this) is sufficient on its own; see
	// Enygma.sol's _expectedDomainId() doc comment for the full reasoning.
	DomainId frontend.Variable `gnark:",public"`

	// Private signals
	SenderId                  frontend.Variable   // Identifier of the sender
	SharedSecrets             []frontend.Variable // Array of shared secrets (1D: sender's pre-selected row)
	SecretKey                 frontend.Variable   // Secret key of the sender
	PreviousSenderBalance     frontend.Variable   // Previous balance in the last Pedersen commitment
	PreviousSenderRandomValue frontend.Variable   // Previous random factor in the last Pedersen commitment
	TxValues                  []frontend.Variable // Array of balances debited/credited
	TxRandomValues            []frontend.Variable // Array of random factors for the pedersen commitments
	SenderTxValue             frontend.Variable   // Balance to be spent in this tx

}

// Changes of Random Factor Hash("random_factors", s, block_number)
// Changes of Tag Message Hash("tags", s, block_number)

func (circuit *EnygmaCircuit) Define(api frontend.API) error {
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

	// selectedVBits stays at 252 bits: it's the sender's TxValues slot, the
	// mod-P NEGATIVE encoding (P - amount), which is legitimately close to
	// P (~251 bits) for any nonzero amount — narrowing it would reject
	// honest transfers. It needs no separate C-02 bound anyway: the
	// AssertIsEqual below pins it to expectedTxValueMod, which
	// ReduceModP (Fix C-01) already guarantees is < P.
	selectedVBits := api.ToBinary(selected_v, 252)
	// vBits: Fix C-02. SenderTxValue both feeds a Pedersen scalar mult
	// (via TxCommit, transitively) and was independently range-checked
	// here at 252 bits — the mismatch C-02 exploits, since 252 bits admits
	// values up to 2P and Com(b,r) == Com(b+kP,r). Bounded to 64 bits:
	// ample for any realistic amount, and 2^64 << P leaves no aliasing room.
	vBits := api.ToBinary(circuit.SenderTxValue, 64)
	pDiffBits := api.ToBinary(JubJubPrimeSubGroup, 252)

	selectedVConstrained := api.FromBinary(selectedVBits...)
	vConstrained := api.FromBinary(vBits...)
	pDiffConstrained := api.FromBinary(pDiffBits...)

	// Compute (p - sender_tx_value) mod p
	expectedTxValue := api.Sub(pDiffConstrained, vConstrained)
	expectedTxValueMod := utils.ReduceModP(api, expectedTxValue) // Fix C-01

	api.AssertIsEqual(selectedVConstrained, expectedTxValueMod)

	///////////////////////////////////**///////////////////////////////////
	// Fix C-03: only the sender's own slot (selected_v, just constrained
	// above) had any bound on its magnitude. Every OTHER TxValues[i] was
	// unconstrained except for the aggregate Σ TxCommit == (0,1) — and
	// since P − w (a debit of w) is a perfectly valid field element, a
	// prover could name any other registered account, put a small honest
	// credit at one slot and P − w at another, and the on-chain contract
	// would apply both deltas verbatim: a silent debit of w from an
	// account that never consented, with no accomplice needed. Range-check
	// every non-sender slot to 64 bits — the same bound C-02 puts on every
	// other monetary quantity — so a hidden ~251-bit debit can no longer
	// be placed in any slot but the sender's own, and separately assert
	// the non-sender credits sum to the sender's declared spend AS
	// INTEGERS, not just mod P, per the audit's own remediation ("assert
	// Σ_{i≠sender} TxValues[i] == SenderTxValue over the integers instead
	// of relying on a mod-P point identity").
	sumNonSenderValues := frontend.Variable(0)
	for i := 0; i < k; i++ {
		isSenderSlot := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		nonSenderValue := api.Select(isSenderSlot, frontend.Variable(0), circuit.TxValues[i])
		nonSenderBits := api.ToBinary(nonSenderValue, 64)
		nonSenderConstrained := api.FromBinary(nonSenderBits...)
		sumNonSenderValues = api.Add(sumNonSenderValues, nonSenderConstrained)
	}
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

	secretSenderCalculated := pos.Poseidon(api, []frontend.Variable{circuit.PreviousSenderRandomValue, circuit.SecretKey})
	secretRemain := utils.ReduceModP(api, secretSenderCalculated) // Fix C-01

	api.AssertIsEqual(secretRemain, selectedSecret)

	///////////////////////////////////**//////////////////////////////////////
	// Knowledge of Nullifier
	// Preimage = secretRemain (Poseidon(prevR, sk) mod p) — unique per sender per round
	// Diagonal of FingerPrintofSharedSecrets is skipped, so we use the sender's self-derived secret
	//
	// Computed here (moved up from its original position further below) because
	// Fix H-01/H-02 reuse computedNullifier as the per-transaction value mixed
	// into the message-tag and blinding-factor derivations below — see those
	// blocks for why. Nullifier's own formula is unchanged.
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
	// Fix H-01 (mechanism 1): the tag used to be Poseidon(HashTag,
	// SharedSecrets[i], BlockNumber) — BlockNumber is the epoch anchor, not
	// the current transaction, so every tag for a receiver slot i is
	// constant for an entire epoch. A passive chain observer comparing any
	// two transactions in the same epoch saw 5 of 6 tags identical and
	// exactly one differ — the sender's own slot, since that is the only
	// one not pinned to a real, epoch-constant pairwise secret — which
	// singles out the sender from one transaction with zero keys. And
	// because the shared secret is direction-agnostic (pairKey canonicalises
	// min_max), the SAME tag reappears verbatim whichever of the pair sends,
	// letting an observer link "A pays B" and "B pays A" transactions to the
	// same relationship. Both are closed together:
	//   - computedNullifier (moved up above) replaces BlockNumber. It is
	//     public, already guaranteed distinct on every transaction — that
	//     is its entire purpose as a double-spend guard — and it changes on
	//     every send by this sender because secretRemain (its preimage)
	//     depends on PreviousSenderRandomValue, which is the output blinding
	//     factor of the sender's own previous transaction. So every one of
	//     the k tags now changes on every transaction, not just the
	//     sender's own slot — the "5 identical, 1 different" signature is
	//     gone regardless of which slot is the real sender.
	//   - circuit.SenderId and circuit.AnonymitySet[i] are mixed in as
	//     ordered (not summed/combined) inputs, so tag_{A→B} and tag_{B→A}
	//     hash a different argument order even under the same symmetric
	//     secret and, hypothetically, the same nonce — defense in depth so
	//     a future regression in nonce freshness alone would not reopen the
	//     direction-correlation half of the leak. AnonymitySet is entirely
	//     public and SenderId is provably one of its entries (the
	//     sumIsInK check above), so a legitimate receiver reconstructs this
	//     exactly as before: try each other public AnonymitySet entry as
	//     the candidate sender, no new communication needed.
	//
	//     computedNullifier and (SenderId, AnonymitySet[i]) are folded into
	//     one perSlotNonce via two chained 2-input Poseidon calls, rather
	//     than passed as extra arguments to the final call directly:
	//     enygma-server/poseidon's S-box constant table (GetPoseidonS) only
	//     has entries for state size t = len(inputs)+1 ∈ {2,3,4} — i.e. at
	//     most 3 inputs per call — even though its round-constant table
	//     (GetPoseidonC) goes up to t=5, so a naive 4- or 5-input call
	//     compiles as Go but panics inside frontend.Compile. Every existing
	//     Poseidon call in this circuit already respects the 3-input limit;
	//     this fix keeps doing so.
	HashTag := pos.Poseidon(api, []frontend.Variable{12})
	for i := 0; i < k; i++ {
		directionTag := pos.Poseidon(api, []frontend.Variable{circuit.SenderId, circuit.AnonymitySet[i]})
		perSlotNonce := pos.Poseidon(api, []frontend.Variable{computedNullifier, directionTag})
		calculatedMessageTag := pos.Poseidon(api, []frontend.Variable{HashTag, circuit.SharedSecrets[i], perSlotNonce})
		calculatedMessageTagMod := utils.ReduceModP(api, calculatedMessageTag) // Fix C-01

		api.AssertIsEqual(circuit.MessageTags[i], calculatedMessageTagMod)
	}

	///////////////////////////////////**///////////////////////////////////
	// Check Pedersen (Sum SenderTxValue, SumR) = Pedersen (0, 0) = (0,1)
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
	// Fix C-02: PreviousSenderBalance is the exact case the audit
	// demonstrated end to end — it's passed directly into
	// utils.PedersenCommitment(api, circuit.PreviousSenderBalance, ...)
	// above (the same wire), so a 252-bit bound here left claimed =
	// real + P or real + 2P satisfying both that commitment (periodic mod
	// P) and this solvency check with an inflated balance. 64 bits closes
	// it the same way as SenderTxValue above.
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
	//
	// Fix H-02: same root cause as H-01 above, applied to the Pedersen
	// blinding factor instead of the message tag. r_i used to be
	// Poseidon(HashRandom, SharedSecrets[i], BlockNumber) — constant for a
	// whole epoch and symmetric in direction — so subtracting two
	// commitment deltas at a common slot across two transactions in the
	// same epoch cancelled H exactly, leaving (v1-v2)*G, and the protocol's
	// own small-amount requirement makes that discrete log trivial to
	// solve by table lookup: a passive chain observer recovers exact
	// amounts with no keys at all. computedNullifier/SenderId/AnonymitySet[i]
	// replace BlockNumber for exactly the reasons in the message-tag
	// comment above — fresh per transaction, ordered by direction. The
	// legitimate receiver still self-derives r the same way as before,
	// just reading computedNullifier (public, same signal already used for
	// double-spend prevention) instead of BlockNumber off calldata.
	// (SenderId/AnonymitySet[i]/computedNullifier folded into perSlotNonce
	// via two chained 2-input Poseidon calls — see the message-tag comment
	// above for why it isn't one 4- or 5-input call.)
	calculatedRandomFactor := make([]frontend.Variable, k)
	receiverHashesModP := make([]frontend.Variable, k)
	sumOfReceiverHashes := frontend.Variable(0)

	HashRandom := pos.Poseidon(api, []frontend.Variable{21})

	// First pass: compute all hashes, reduce modulo JubJubPrimeSubGroup
	for i := 0; i < k; i++ {
		randomDirectionTag := pos.Poseidon(api, []frontend.Variable{circuit.SenderId, circuit.AnonymitySet[i]})
		randomPerSlotNonce := pos.Poseidon(api, []frontend.Variable{computedNullifier, randomDirectionTag})
		RandomFactor := pos.Poseidon(api, []frontend.Variable{HashRandom, circuit.SharedSecrets[i], randomPerSlotNonce})
		// Reduce RandomFactor modulo JubJubPrimeSubGroup — Fix C-01
		hashModP := utils.ReduceModP(api, RandomFactor)

		receiverHashesModP[i] = hashModP

		// Check if this participant is a receiver (not the sender)
		isSender := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		isReceiver := api.Sub(1, isSender)

		// Add to sum only if this is a receiver
		sumOfReceiverHashes = api.Add(sumOfReceiverHashes, api.Mul(isReceiver, hashModP))
	}
	// Reduce the sum modulo JubJubPrimeSubGroup — Fix C-01
	senderRandomFactor := utils.ReduceModP(api, sumOfReceiverHashes)

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

// Fix M-08 (remote panics): binding tags used to permit 1-6 element
// arrays ("min=1,max=6") while the handler unconditionally indexes
// [0..NCommitment-1] (NCommitment==6) — a request with, say, 2 public
// keys passed binding validation and then panicked on the first
// out-of-range index (recovered by gin.Recovery into a 500, but still an
// uncontrolled remote panic on the cheap, pre-proving path). Every slice
// here is exactly as long as the circuit's fixed anonymity set, so "len=6"
// is what the handler actually requires, not "up to 6". FingerPrintofSharedSecrets
// additionally gets "dive,len=6" on the outer slice (it didn't have any
// per-element validation at all before) since the handler also indexes its
// inner slices [0..5].
type EnygmaRequest struct {
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
	// Fix L-01: caller-supplied chainId<<160|contractAddress — the
	// handler doesn't compute this itself since it has no chain
	// connection of its own; the client (which does) supplies it.
	DomainId string `json:"domain_id" binding:"required"`
}

type EnygmaOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
