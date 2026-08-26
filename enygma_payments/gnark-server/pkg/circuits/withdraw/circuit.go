package withdraw

import (
	"math/big"

	pos "enygma-server/poseidon"
	utils "enygma-server/utils"

	"github.com/consensys/gnark/frontend"
)

const JubJubPrimeSubGroupStr = "2736030358979909402780800718157159386076813972158567259200215660948447373041"

type WithdrawEnygmaCircuitConfig struct {
	NCommitment int
}

// const nSplit =6
type WithdrawEnygmaCircuit struct {
	Config WithdrawEnygmaCircuitConfig

	// Public signals
	HashedSharedSecrets []frontend.Variable    `gnark:",public"` // Array of hash of shared secrets (1D)
	PublicKey           []frontend.Variable    `gnark:",public"` // Public keys from all other PLs
	PreviousCommit      [][2]frontend.Variable `gnark:",public"` // Previous balances (Pedersen commitments)
	TxCommit            [][2]frontend.Variable `gnark:",public"` // Commitments for this tx
	BlockNumber         frontend.Variable      `gnark:",public"` // Block number
	AnonymitySet        []frontend.Variable    `gnark:",public"` // K-anonymity set
	MessageTags         []frontend.Variable    `gnark:",public"` // Tag messages
	Nullifier           frontend.Variable      `gnark:",public"` // Nullifier
	// Fix C-09: Σ VPerDeposit, exposed so Enygma.sol can bind it against
	// Σ depositParams[i].amount before forwarding to the DvP vault — see
	// the assertion below for why this alone closes the finding for the
	// withdraw leg (deposit's symmetric gap is a separate, cross-repo
	// binding this fix does not attempt — see Enygma.sol's withdraw()
	// comment).
	TotalDepositValue frontend.Variable `gnark:",public"`
	// Fix L-01: chainId<<160 | contractAddress — see enygma/circuit.go's
	// DomainId field doc comment for the full reasoning; identical here.
	DomainId frontend.Variable `gnark:",public"`

	// Private signals
	SenderId                  frontend.Variable     // Identifier of the sender
	SharedSecrets             []frontend.Variable   // Shared secrets (1D: sender's row)
	SecretKey                 frontend.Variable     // Secret key
	PreviousSenderBalance     frontend.Variable     // Previous balance
	PreviousSenderRandomValue frontend.Variable     // Previous random factor
	TxValues                  []frontend.Variable   // Balances debited/credited
	TxRandomValues            []frontend.Variable   // Random factors for pedersen commitments
	SenderTxValue             frontend.Variable     // Amount to withdraw
	Hashes                    [10]frontend.Variable // Deposit hashes (always 10)
	SkDeposits                [10]frontend.Variable // Secret keys for deposits
	VPerDeposit               [10]frontend.Variable // Value per deposit
	Address                   frontend.Variable     // Withdraw address
}

func (circuit *WithdrawEnygmaCircuit) Define(api frontend.API) error {
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
	// Check if Amount To Withdraw Corresponds To Sender TxValues
	//
	// Fix M-15 (direction inversion): this used to assert
	// selectedVConstrained == vConstrained — i.e. the sender's own
	// TxValues slot equals +SenderTxValue, a CREDIT. That is backwards:
	// withdraw() moves value OUT of the shielded pool (to the DvP side),
	// so the sender must be DEBITED, exactly like deposit's own (correct)
	// direction before this fix swapped the two. Encoded the same way
	// the base transfer circuit encodes every debit: P - SenderTxValue.
	selected_v := frontend.Variable(0)
	for i := 0; i < k; i++ {
		diff := api.Sub(circuit.SenderId, circuit.AnonymitySet[i])
		eq := api.IsZero(diff)
		contribution := api.Mul(eq, circuit.TxValues[i])
		selected_v = api.Add(selected_v, contribution)
	}
	// selectedVBits stays at 252 bits: it's the sender's TxValues slot, the
	// mod-P NEGATIVE (debit) encoding, which is legitimately close to P
	// (~251 bits) for any nonzero amount — narrowing it would reject
	// honest withdrawals. See deposit/circuit.go's identical comment.
	selectedVBits := api.ToBinary(selected_v, 252)
	// vBits: Fix C-02. Bounded to 64 bits so it can't alias +P/+2P.
	vBits := api.ToBinary(circuit.SenderTxValue, 64)
	pDiffBits := api.ToBinary(JubJubPrimeSubGroup, 252)

	selectedVConstrained := api.FromBinary(selectedVBits...)
	vConstrained := api.FromBinary(vBits...)
	pDiffConstrained := api.FromBinary(pDiffBits...)

	expectedTxValue := api.Sub(pDiffConstrained, vConstrained)
	expectedTxValueMod := utils.ReduceModP(api, expectedTxValue) // Fix C-01

	api.AssertIsEqual(selectedVConstrained, expectedTxValueMod)

	///////////////////////////////////**///////////////////////////////////
	// Fix C-03: only the sender's own slot had any bound on its magnitude.
	// Every other TxValues[i] was unconstrained except for the aggregate
	// Pedersen equality further below (Σ_all TxValues[i] ≡ selected_v mod
	// P, i.e. Σ_{i≠sender} ≡ 0 mod P) — and P − w (a debit of w) is a
	// perfectly valid field element satisfying that congruence. Unlike
	// transfer/deposit, withdraw has no legitimate non-sender credits at
	// all (this is a single-party operation), so the target here is 0,
	// not SenderTxValue: range-checking every non-sender slot to 64 bits
	// and requiring their sum to be exactly 0 forces each one individually
	// to be 0 (all terms non-negative), closing the gap completely.
	sumNonSenderValues := frontend.Variable(0)
	for i := 0; i < k; i++ {
		isSenderSlot := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		nonSenderValue := api.Select(isSenderSlot, frontend.Variable(0), circuit.TxValues[i])
		nonSenderBits := api.ToBinary(nonSenderValue, 64)
		nonSenderConstrained := api.FromBinary(nonSenderBits...)
		sumNonSenderValues = api.Add(sumNonSenderValues, nonSenderConstrained)
	}
	api.AssertIsEqual(sumNonSenderValues, frontend.Variable(0))

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

	///////////////////////////////////**///////////////////////////////////
	// Check if Hash Array of Secret is well formed
	for i := 0; i < k; i++ {
		calculatedHash := pos.Poseidon(api, []frontend.Variable{circuit.SharedSecrets[i], circuit.SharedSecrets[i]})
		hashMod := utils.ReduceModP(api, calculatedHash) // Fix C-01

		api.AssertIsEqual(hashMod, circuit.HashedSharedSecrets[i])
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
	// Check if previous commits and tx commits are on Curve
	for i := 0; i < k; i++ {
		utils.AssertPointsIsOnCurve(api, circuit.PreviousCommit[i][0], circuit.PreviousCommit[i][1])
		utils.AssertPointsIsOnCurve(api, circuit.TxCommit[i][0], circuit.TxCommit[i][1])
	}

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
	// Check Pedersen (Sum SenderTxValue, SumR) = Pedersen (Sender TxValues, 0)
	sumX := frontend.Variable(0)
	sumY := frontend.Variable(0)
	senderV := frontend.Variable(0)

	for i := 0; i < k; i++ {
		sumX = api.Add(sumX, circuit.TxValues[i])
		sumY = api.Add(sumY, circuit.TxRandomValues[i])
		senderV = selected_v
	}

	PedersenObtained := utils.PedersenCommitment(api, sumX, sumY)
	PedersenExpected := utils.PedersenCommitment(api, senderV, frontend.Variable(0))

	api.AssertIsEqual(PedersenObtained.X, PedersenExpected.X)
	api.AssertIsEqual(PedersenObtained.Y, PedersenExpected.Y)

	///////////////////////////////////**///////////////////////////////////
	// Range Proof: previousV >= sender_tx_value and sender_tx_value >= 0
	// Fix C-08: this circuit had no solvency comparator at all — only the
	// sender_tx_value >= 0 check below, which api.Cmp(v, 0) makes vacuous
	// anyway (compiles to ~3800 constraints that prove nothing: Cmp is an
	// unsigned comparison, and 0 as the right operand means every bit of
	// the right side is 0, so the "less than" branch is unreachable for
	// any field element — that gadget is a pre-existing no-op in all four
	// circuits, harmless elsewhere only because the OTHER comparator
	// (previousV >= v) actually does the work; withdraw was the one
	// circuit missing that other comparator entirely). Without it, an
	// account with balance 0 could credit itself any amount and instruct
	// the DvP payout leg to pay it out, with no debit and no rejection.
	//
	// Also range-checks PreviousSenderBalance to 64 bits (the C-02 fix,
	// not previously needed here since this witness had no second,
	// narrower use to create the alias — adding the comparator below
	// creates exactly that second use, so the bound has to come with it,
	// not after).
	previousVBits := api.ToBinary(circuit.PreviousSenderBalance, 64)
	previousVConstrained := api.FromBinary(previousVBits...)

	prevVGreaterEqualV := api.Cmp(previousVConstrained, vConstrained)
	api.AssertIsEqual(api.IsZero(api.Add(prevVGreaterEqualV, frontend.Variable(1))), frontend.Variable(0))

	vGreaterEqualZero := api.Cmp(vConstrained, frontend.Variable(0))
	api.AssertIsEqual(api.IsZero(api.Add(vGreaterEqualZero, frontend.Variable(1))), frontend.Variable(0))

	///////////////////////////////////**//////////////////////////////////////
	// Knowledge of Nullifier
	selectedPreImage := frontend.Variable(0)
	for i := 0; i < k; i++ {
		diff := api.Sub(circuit.SenderId, circuit.AnonymitySet[i])
		eq := api.IsZero(diff)
		selectedPreImage = api.Add(selectedPreImage, api.Mul(eq, circuit.HashedSharedSecrets[i]))
	}

	computedNullifier := pos.Poseidon(api, []frontend.Variable{selectedPreImage, circuit.BlockNumber})
	api.AssertIsEqual(computedNullifier, circuit.Nullifier)

	///////////////////////////////////**//////////////////////////////////////
	// Check if Tx Commitment is well formed
	for i := 0; i < k; i++ {
		computedPedersenCommitment := utils.PedersenCommitment(api, circuit.TxValues[i], circuit.TxRandomValues[i])
		api.AssertIsEqual(circuit.TxCommit[i][0], computedPedersenCommitment.X)
		api.AssertIsEqual(circuit.TxCommit[i][1], computedPedersenCommitment.Y)
	}

	///////////////////////////////////**//////////////////////////////////////
	// Knowledge of Message Tag
	HashTag := pos.Poseidon(api, []frontend.Variable{12})
	for i := 0; i < k; i++ {
		calculatedMessageTag := pos.Poseidon(api, []frontend.Variable{HashTag, circuit.SharedSecrets[i], circuit.BlockNumber})
		calculatedMessageTagMod := utils.ReduceModP(api, calculatedMessageTag) // Fix C-01

		api.AssertIsEqual(circuit.MessageTags[i], calculatedMessageTagMod)
	}

	///////////////////////////////////**//////////////////////////////////////
	// Check if random factors R are well formed
	calculatedRandomFactor := make([]frontend.Variable, k)
	receiverHashesModP := make([]frontend.Variable, k)
	sumOfReceiverHashes := frontend.Variable(0)

	HashRandom := pos.Poseidon(api, []frontend.Variable{21})

	for i := 0; i < k; i++ {
		RandomFactor := pos.Poseidon(api, []frontend.Variable{HashRandom, circuit.SharedSecrets[i], circuit.BlockNumber})

		hashModP := utils.ReduceModP(api, RandomFactor) // Fix C-01

		receiverHashesModP[i] = hashModP

		isSender := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		isReceiver := api.Sub(1, isSender)

		sumOfReceiverHashes = api.Add(sumOfReceiverHashes, api.Mul(isReceiver, hashModP))
	}

	senderRandomFactor := utils.ReduceModP(api, sumOfReceiverHashes) // Fix C-01

	for i := 0; i < k; i++ {
		isSender := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		receiverRandomFactor := api.Sub(JubJubPrimeSubGroup, receiverHashesModP[i])
		calculatedRandomFactor[i] = api.Select(isSender, senderRandomFactor, receiverRandomFactor)
	}

	for i := 0; i < k; i++ {
		api.AssertIsEqual(calculatedRandomFactor[i], circuit.TxRandomValues[i])
	}

	///////////////////////////////////**//////////////////////////////////////
	// Process multiple commitment withdraw - always process exactly 10 deposits
	//
	// Fix C-09: this loop's per-slot Hash check was, and remains,
	// self-consistent only — the prover picks VPerDeposit/SkDeposits/
	// Address freely and computes Hashes to match, so on its own it
	// constrains nothing about how much value actually leaves the
	// shielded pool. Before this fix, NOTHING anywhere related
	// Σ VPerDeposit to SenderTxValue (the amount this circuit's own
	// solvency/debit logic above actually debits the sender for), so a
	// prover could debit an arbitrarily small (or zero) shielded amount
	// while claiming an arbitrarily large VPerDeposit sum — unlimited
	// unbacked value creation on the DvP side, the moment the bridge is
	// wired up (C-09's "Critical-on-arming" framing). sumVPerDeposit
	// below, asserted equal to SenderTxValue and exposed as the new
	// TotalDepositValue public signal, is what Enygma.sol's withdraw()
	// then binds against Σ depositParams[i].amount before ever calling
	// out to the DvP vault — closing the gap for this leg completely.
	sumVPerDeposit := frontend.Variable(0)
	for i := 0; i < 10; i++ {
		// Check if deposit value is zero
		isDepositZero := api.IsZero(circuit.VPerDeposit[i])

		// Get public key from each sk_deposit using Poseidon hash
		publicKeyFromSk := pos.Poseidon(api, []frontend.Variable{circuit.SkDeposits[i]})

		// Check if Hash(commitment in Dvp - MerkleTree) is well formed
		// First Poseidon hash: Hash(address, v_per_deposit[i])
		firstHash := pos.Poseidon(api, []frontend.Variable{circuit.Address, circuit.VPerDeposit[i]})

		// Second Poseidon hash: Hash(firstHash, publicKey)
		secondHash := pos.Poseidon(api, []frontend.Variable{firstHash, publicKeyFromSk})

		// Conditional check: if v_per_deposit[i] is zero, skip the equality check
		// enabled = 1 - isZero = 1 if value is NOT zero, 0 if value is zero
		enabled := api.Sub(frontend.Variable(1), isDepositZero)

		// If enabled == 1, assert equality; if enabled == 0, skip assertion
		difference := api.Sub(circuit.Hashes[i], secondHash)
		conditionalDifference := api.Mul(enabled, difference)

		api.AssertIsEqual(conditionalDifference, frontend.Variable(0))

		sumVPerDeposit = api.Add(sumVPerDeposit, circuit.VPerDeposit[i])
	}
	// VPerDeposit entries are individually unbounded field elements, but
	// bounding sumVPerDeposit here (not each entry) is sufficient: it's
	// asserted equal to SenderTxValue immediately below, and vBits above
	// already range-checks SenderTxValue itself to 64 bits (Fix C-02) —
	// so this equality transitively bounds the sum without needing a
	// second, redundant range check on the sum itself.
	api.AssertIsEqual(sumVPerDeposit, circuit.SenderTxValue)
	// Bind the public TotalDepositValue signal itself to the same sum —
	// a prover cannot publish anything other than the true Σ VPerDeposit
	// (equivalently, SenderTxValue) here.
	api.AssertIsEqual(circuit.TotalDepositValue, sumVPerDeposit)

	// Fix L-01: keep DomainId a genuinely constrained wire.
	api.AssertIsEqual(circuit.DomainId, circuit.DomainId)

	return nil
}

// Fix M-08 (remote panics): see the identical note on enygma's
// EnygmaRequest — these tags used to allow 1-6 elements while the handler
// always indexes [0..5]. len=6 (and dive,len=2 on the [2]string pairs)
// matches what the handler actually requires. Hashes/SkDeposits/
// VPerDeposit are already fixed-size Go arrays ([10]string), so their
// length can't vary; a short JSON array would silently zero-pad rather
// than error, but ParseBigInt's Fix M-08 error-return (see utils.go)
// already rejects an empty-string element before any witness is built.
type WithdrawRequest struct {
	HashedSharedSecrets       []string    `json:"hashed_shared_secrets" binding:"required,len=6"`
	PublicKey                 []string    `json:"public_keys" binding:"required,len=6"`
	PreviousCommit            [][2]string `json:"previous_commits" binding:"required,len=6,dive,len=2"`
	TxCommit                  [][2]string `json:"tx_commits" binding:"required,len=6,dive,len=2"`
	BlockNumber               string      `json:"block_number" binding:"required"`
	AnonymitySet              []string    `json:"anonymity_set" binding:"required,len=6"`
	MessageTags               []string    `json:"message_tags" binding:"required,len=6"`
	Nullifier                 string      `json:"nullifier" binding:"required"`
	SenderID                  string      `json:"sender_id" binding:"required"`
	SharedSecrets             []string    `json:"shared_secrets" binding:"required,len=6"`
	SecretKey                 string      `json:"secret_key" binding:"required"`
	PreviousSenderBalance     string      `json:"previous_sender_balance" binding:"required"`
	PreviousSenderRandomValue string      `json:"previous_sender_random_value" binding:"required"`
	TxValues                  []string    `json:"tx_values" binding:"required,len=6"`
	TxRandomValues            []string    `json:"tx_random_values" binding:"required,len=6"`
	SenderTxValue             string      `json:"sender_tx_value" binding:"required"`
	Hashes                    [10]string  `json:"hashes" binding:"required"`
	SkDeposits                [10]string  `json:"sk_deposits" binding:"required"`
	VPerDeposit               [10]string  `json:"v_per_deposit" binding:"required"`
	Address                   string      `json:"address" binding:"required"`
	// Fix L-01: caller-supplied chainId<<160|contractAddress.
	DomainId string `json:"domain_id" binding:"required"`
}

type WithdrawOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
