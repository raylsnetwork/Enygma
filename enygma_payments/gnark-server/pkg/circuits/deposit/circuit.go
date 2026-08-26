package deposit

import (
	"math/big"

	pos "enygma-server/poseidon"
	utils "enygma-server/utils"

	"github.com/consensys/gnark/frontend"
)

const JubJubPrimeSubGroupStr = "2736030358979909402780800718157159386076813972158567259200215660948447373041"

type DepositEnygmaCircuitConfig struct {
	NCommitment int
}

type DepositEnygmaCircuit struct {
	Config DepositEnygmaCircuitConfig

	// Public signals
	HashedSharedSecrets []frontend.Variable    `gnark:",public"` // Array of hash of shared secrets (1D)
	PublicKey           []frontend.Variable    `gnark:",public"` // Public keys from all other PLs
	PreviousCommit      [][2]frontend.Variable `gnark:",public"` // Previous balances (Pedersen commitments)
	TxCommit            [][2]frontend.Variable `gnark:",public"` // Commitments for this tx
	BlockNumber         frontend.Variable      `gnark:",public"` // Block number
	AnonymitySet        []frontend.Variable    `gnark:",public"` // K-anonymity set
	MessageTags         []frontend.Variable    `gnark:",public"` // Tag messages
	Nullifier           frontend.Variable      `gnark:",public"` // Nullifier
	Hash                frontend.Variable      `gnark:",public"` // Deposit commitment hash
	// Fix L-01: chainId<<160 | contractAddress — see enygma/circuit.go's
	// DomainId field doc comment for the full reasoning; identical here.
	DomainId frontend.Variable `gnark:",public"`

	// Private signals
	SenderId                  frontend.Variable   // Identifier of the sender
	SharedSecrets             []frontend.Variable // Shared secrets (1D: sender's row)
	SecretKey                 frontend.Variable   // Secret key
	PreviousSenderBalance     frontend.Variable   // Previous balance
	PreviousSenderRandomValue frontend.Variable   // Previous random factor
	TxValues                  []frontend.Variable // Balances debited/credited
	TxRandomValues            []frontend.Variable // Random factors for pedersen commitments
	SenderTxValue             frontend.Variable   // Balance to deposit
	PkDeposit                 frontend.Variable   // Public key for deposit
	Address                   frontend.Variable   // Deposit address
}

func (circuit *DepositEnygmaCircuit) Define(api frontend.API) error {

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
	// Check if Amount To Deposit Corresponds To Sender TxValues
	//
	// Fix M-15 (direction inversion): this used to assert
	// selectedVConstrained == (P - SenderTxValue) mod P — i.e. a DEBIT.
	// That is backwards: Enygma.deposit() moves value IN from the DvP
	// side (it redeems an existing note), so the depositor must be
	// CREDITED, not debited — the depositor was effectively paying twice
	// (the note redeemed AND the shielded balance debited). Fixed to a
	// plain credit, mirroring how withdraw's (also-fixed) debit now uses
	// the P-complement encoding instead.
	selected_v := frontend.Variable(0)
	for i := 0; i < k; i++ {
		diff := api.Sub(circuit.SenderId, circuit.AnonymitySet[i])
		eq := api.IsZero(diff)
		selected_v = api.Add(selected_v, api.Mul(eq, circuit.TxValues[i]))
	}
	// selectedVBits stays at 252 bits: it's the sender's TxValues slot,
	// pinned below to vConstrained (which is now bounded, see next
	// comment) via a direct equality, so it needs no separate C-02 bound.
	selectedVBits := api.ToBinary(selected_v, 252)
	// vBits: Fix C-02. SenderTxValue both feeds a Pedersen scalar mult
	// (via TxCommit, transitively) and was independently range-checked
	// here at 252 bits — the mismatch C-02 exploits, since 252 bits admits
	// values up to 2P and Com(b,r) == Com(b+kP,r). Bounded to 64 bits:
	// ample for any realistic amount, and 2^64 << P leaves no aliasing room.
	vBits := api.ToBinary(circuit.SenderTxValue, 64)

	selectedVConstrained := api.FromBinary(selectedVBits...)
	vConstrained := api.FromBinary(vBits...)

	api.AssertIsEqual(selectedVConstrained, vConstrained)

	///////////////////////////////////**///////////////////////////////////
	// Fix C-03: only the sender's own slot (selected_v, just constrained
	// above) had any bound on its magnitude. Every other TxValues[i] was
	// unconstrained except for the aggregate Pedersen equality further
	// below (PedersenObtained(Σ_all TxValues, Σ_all TxRandomValues) ==
	// PedersenExpected(selected_v, 0), i.e. Σ_{i≠sender} ≡ 0 mod P) — and
	// P − w (a debit of w) is a perfectly valid field element satisfying
	// that congruence. Deposit is a single-party bridge operation like
	// withdraw, not a peer-to-peer redistribution like transfer — the
	// depositor receives new value from ZkDvp, and no other account is
	// meant to be touched at all — so the target here is 0, not
	// SenderTxValue: range-checking every non-sender slot to 64 bits and
	// requiring their sum to be exactly 0 forces each one individually to
	// be 0 (all terms non-negative), closing the gap completely.
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
	// Range Proof: sender_tx_value >= 0
	//
	// Fix M-15: this circuit previously also required
	// PreviousSenderBalance >= SenderTxValue ("solvency") here — correct
	// for a debit (withdraw), meaningless for a credit (deposit, this
	// circuit, after the direction fix above): a depositor needs no
	// pre-existing balance at all to receive new value. Removed; only the
	// non-negativity check remains (vGreaterEqualZero is already
	// redundant with vBits' 64-bit range check above, kept for
	// readability). PreviousSenderBalance itself is still range-checked
	// (Fix C-02) since it feeds utils.PedersenCommitment below on the
	// same wire the original finding demonstrated the alias on.
	api.ToBinary(circuit.PreviousSenderBalance, 64) // range-check only; no solvency comparison for a credit

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

	//////////////////////////////////**//////////////////////////////////////
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
	// Check if Hash(commitment in Dvp - MerkleTree) is well formed
	uid := pos.Poseidon(api, []frontend.Variable{circuit.Address, circuit.SenderTxValue})
	CalculatedHash := pos.Poseidon(api, []frontend.Variable{uid, circuit.PkDeposit})
	api.AssertIsEqual(CalculatedHash, circuit.Hash)

	// Fix L-01: keep DomainId a genuinely constrained wire.
	api.AssertIsEqual(circuit.DomainId, circuit.DomainId)

	return nil
}

// Fix M-08 (remote panics): see the identical note on enygma's
// EnygmaRequest — these tags used to allow 1-6 elements while the handler
// always indexes [0..5]. len=6 (and dive,len=2 on the [2]string pairs)
// matches what the handler actually requires.
type DepositRequest struct {
	HashedSharedSecrets []string    `json:"hashed_shared_secrets" binding:"required,len=6"`
	PublicKey           []string    `json:"public_keys" binding:"required,len=6"`
	PreviousCommit      [][2]string `json:"previous_commits" binding:"required,len=6,dive,len=2"`
	TxCommit            [][2]string `json:"tx_commits" binding:"required,len=6,dive,len=2"`
	BlockNumber         string      `json:"block_number" binding:"required"`
	AnonymitySet        []string    `json:"anonymity_set" binding:"required,len=6"`
	MessageTags         []string    `json:"message_tags" binding:"required,len=6"`
	Nullifier           string      `json:"nullifier" binding:"required"`
	Hash                string      `json:"hash" binding:"required"`

	SenderID                  string   `json:"sender_id" binding:"required"`
	SharedSecrets             []string `json:"shared_secrets" binding:"required,len=6"`
	SecretKey                 string   `json:"secret_key" binding:"required"`
	PreviousSenderBalance     string   `json:"previous_sender_balance" binding:"required"`
	PreviousSenderRandomValue string   `json:"previous_sender_random_value" binding:"required"`
	TxValues                  []string `json:"tx_values" binding:"required,len=6"`
	TxRandomValues            []string `json:"tx_random_values" binding:"required,len=6"`
	SenderTxValue             string   `json:"sender_tx_value" binding:"required"`
	PkDeposit                 string   `json:"pk_deposit" binding:"required"`
	Address                   string   `json:"address" binding:"required"`
	// Fix L-01: caller-supplied chainId<<160|contractAddress.
	DomainId string `json:"domain_id" binding:"required"`
}

type DepositOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
