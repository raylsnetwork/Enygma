package enygma

import (
	"math/big"

	pos "enygma-server/poseidon"
	utils "enygma-server/utils"

	"github.com/consensys/gnark/frontend"
	cmp "github.com/consensys/gnark/std/math/cmp"
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
	HashedSharedSecrets []frontend.Variable    `gnark:",public"` // Array of hash of shared secrets (1D: sender's row)
	PublicKey           []frontend.Variable    `gnark:",public"` // Public keys from all other PLs
	PreviousCommit      [][2]frontend.Variable `gnark:",public"` // Array of previous balances (Pedersen commitments)
	TxCommit            [][2]frontend.Variable `gnark:",public"` // Array containing the commitments for this new tx
	BlockNumber         frontend.Variable      `gnark:",public"` // Block number to ensure random factors are well-generated
	AnonymitySet        []frontend.Variable    `gnark:",public"` // Array with indices of the banks in the tx ("k"-anonymity)
	MessageTags         []frontend.Variable    `gnark:",public"` // Array of tag messages for unique transactions
	Nullifier           frontend.Variable      `gnark:",public"` // Nullifier to prevent double spend

	// Private signals
	SenderId                  frontend.Variable   // Identifier of the sender
	SharedSecrets             []frontend.Variable  // Array of shared secrets (1D: sender's pre-selected row)
	SecretKey                 frontend.Variable    // Secret key of the sender
	PreviousSenderBalance     frontend.Variable    // Previous balance in the last Pedersen commitment
	PreviousSenderRandomValue frontend.Variable    // Previous random factor in the last Pedersen commitment
	TxValues                  []frontend.Variable  // Array of balances debited/credited
	TxRandomValues            []frontend.Variable  // Array of random factors for the pedersen commitments
	SenderTxValue             frontend.Variable    // Balance to be spent in this tx
}

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
	selectedVBits := api.ToBinary(selected_v, 252)
	vBits := api.ToBinary(circuit.SenderTxValue, 252)
	pDiffBits := api.ToBinary(JubJubPrimeSubGroup, 252)

	selectedVConstrained := api.FromBinary(selectedVBits...)
	vConstrained := api.FromBinary(vBits...)
	pDiffConstrained := api.FromBinary(pDiffBits...)

	// Compute (p - sender_tx_value) mod p
	expectedTxValue := api.Sub(pDiffConstrained, vConstrained)
	expectedTxValueInter, _ := api.NewHint(utils.ModHint, 2, expectedTxValue)
	expectedTxValueMod := expectedTxValueInter[0] // remainder (mod p)

	api.AssertIsEqual(selectedVConstrained, expectedTxValueMod)

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
	secretInter, _ := api.NewHint(utils.ModHint, 2, secretSenderCalculated)
	secretRemain := secretInter[0] // remainder

	api.AssertIsEqual(secretRemain, selectedSecret)

	///////////////////////////////////**///////////////////////////////////
	// Check if Hash Array of Secret is well formed
	for i := 0; i < k; i++ {
		calculatedHash := pos.Poseidon(api, []frontend.Variable{circuit.SharedSecrets[i], circuit.SharedSecrets[i]})
		hashInter, _ := api.NewHint(utils.ModHint, 2, calculatedHash)
		hashMod := hashInter[0] // remainder

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
	pkInter, _ := api.NewHint(utils.ModHint, 2, pk)
	pkMod := pkInter[0] // remainder

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
	HashTag := pos.Poseidon(api, []frontend.Variable{12})
	for i := 0; i < k; i++ {
		calculatedMessageTag := pos.Poseidon(api, []frontend.Variable{HashTag, circuit.SharedSecrets[i], circuit.BlockNumber})
		calculatedMessageTagInter, _ := api.NewHint(utils.ModHint, 2, calculatedMessageTag)
		calculatedMessageTagMod := calculatedMessageTagInter[0]

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
	previousVBits := api.ToBinary(circuit.PreviousSenderBalance, 252)
	previousVConstrained := api.FromBinary(previousVBits...)

	// previousV >= sender_tx_value means Cmp(previousV, sender_tx_value) != -1
	prevVGreaterEqualV := api.Cmp(previousVConstrained, vConstrained)
	api.AssertIsEqual(api.IsZero(api.Add(prevVGreaterEqualV, frontend.Variable(1))), frontend.Variable(0))

	// sender_tx_value >= 0 means Cmp(sender_tx_value, 0) != -1
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
	// Check if random factors R are well formed
	calculatedRandomFactor := make([]frontend.Variable, k)
	receiverHashesModP := make([]frontend.Variable, k)
	sumOfReceiverHashes := frontend.Variable(0)

	HashRandom := pos.Poseidon(api, []frontend.Variable{21})

	// First pass: compute all hashes, reduce modulo JubJubPrimeSubGroup
	for i := 0; i < k; i++ {
		RandomFactor := pos.Poseidon(api, []frontend.Variable{HashRandom, circuit.SharedSecrets[i], circuit.BlockNumber})

		// Reduce RandomFactor modulo JubJubPrimeSubGroup
		randomInter, _ := api.NewHint(utils.ModHint, 2, RandomFactor)
		hashModP := randomInter[0]
		q := randomInter[1]

		api.AssertIsEqual(api.Add(api.Mul(q, JubJubPrimeSubGroup), hashModP), RandomFactor)
		isValid := cmp.IsLess(api, hashModP, JubJubPrimeSubGroup)
		api.AssertIsEqual(isValid, 1)

		receiverHashesModP[i] = hashModP

		// Check if this participant is a receiver (not the sender)
		isSender := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		isReceiver := api.Sub(1, isSender)

		// Add to sum only if this is a receiver
		sumOfReceiverHashes = api.Add(sumOfReceiverHashes, api.Mul(isReceiver, hashModP))
	}

	// Reduce the sum modulo JubJubPrimeSubGroup
	sumInter, _ := api.NewHint(utils.ModHint, 2, sumOfReceiverHashes)
	senderRandomFactor := sumInter[0]
	sumQ := sumInter[1]

	api.AssertIsEqual(api.Add(api.Mul(sumQ, JubJubPrimeSubGroup), senderRandomFactor), sumOfReceiverHashes)
	isSumValid := cmp.IsLess(api, senderRandomFactor, JubJubPrimeSubGroup)
	api.AssertIsEqual(isSumValid, 1)

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

	return nil
}

type EnygmaRequest struct {
	HashedSharedSecrets       []string    `json:"hashed_shared_secrets" binding:"required,min=1,max=6"`
	PublicKey                 []string    `json:"public_keys" binding:"required,min=1,max=6"`
	PreviousCommit            [][2]string `json:"previous_commits" binding:"required,min=1,max=6,dive,len=2"`
	TxCommit                  [][2]string `json:"tx_commits" binding:"required,min=1,max=6,dive,len=2"`
	BlockNumber               string      `json:"block_number" binding:"required"`
	AnonymitySet              []string    `json:"anonymity_set" binding:"required,min=1,max=6"`
	MessageTags               []string    `json:"message_tags" binding:"required,min=1,max=6"`
	Nullifier                 string      `json:"nullifier" binding:"required"`

	SenderID                  string      `json:"sender_id" binding:"required"`
	SharedSecrets             []string    `json:"shared_secrets" binding:"required,min=1,max=6"`
	SecretKey                 string      `json:"secret_key" binding:"required"`
	PreviousSenderBalance     string      `json:"previous_sender_balance" binding:"required"`
	PreviousSenderRandomValue string      `json:"previous_sender_random_value" binding:"required"`
	TxValues                  []string    `json:"tx_values" binding:"required,min=1,max=6"`
	TxRandomValues            []string    `json:"tx_random_values" binding:"required,min=1,max=6"`
	SenderTxValue             string      `json:"sender_tx_value" binding:"required"`
}

type EnygmaOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
