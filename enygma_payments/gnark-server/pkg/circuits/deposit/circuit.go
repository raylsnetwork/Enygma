package deposit

import (
	"math/big"

	pos "enygma-server/poseidon"
	utils "enygma-server/utils"

	"github.com/consensys/gnark/frontend"
	cmp "github.com/consensys/gnark/std/math/cmp"
)

// JubJubPrimeSubGroup constant
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

	// Private signals
	SenderId                  frontend.Variable   // Identifier of the sender
	SharedSecrets             []frontend.Variable  // Shared secrets (1D: sender's row)
	SecretKey                 frontend.Variable    // Secret key
	PreviousSenderBalance     frontend.Variable    // Previous balance
	PreviousSenderRandomValue frontend.Variable    // Previous random factor
	TxValues                  []frontend.Variable  // Balances debited/credited
	TxRandomValues            []frontend.Variable  // Random factors for pedersen commitments
	SenderTxValue             frontend.Variable    // Balance to deposit
	PkDeposit                 frontend.Variable    // Public key for deposit
	Address                   frontend.Variable    // Deposit address
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
	// Check if Negative of Amount To Deposit Corresponds To Sender TxValues
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

	// Compute (p - sender_tx_value) mod p to handle sender_tx_value=0 case correctly
	expectedTxValue := api.Sub(pDiffConstrained, vConstrained)
	expectedTxValueInter, _ := api.NewHint(utils.ModHint, 2, expectedTxValue)
	expectedTxValueMod := expectedTxValueInter[0]

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
	secretRemain := secretInter[0]

	api.AssertIsEqual(secretRemain, selectedSecret)

	///////////////////////////////////**///////////////////////////////////
	// Check if Hash Array of Secret is well formed
	for i := 0; i < k; i++ {
		calculatedHash := pos.Poseidon(api, []frontend.Variable{circuit.SharedSecrets[i], circuit.SharedSecrets[i]})
		hashInter, _ := api.NewHint(utils.ModHint, 2, calculatedHash)
		hashMod := hashInter[0]

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
	pkMod := pkInter[0]

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
	// Range Proof: previousV >= sender_tx_value and sender_tx_value >= 0
	previousVBits := api.ToBinary(circuit.PreviousSenderBalance, 252)
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
		calculatedMessageTagInter, _ := api.NewHint(utils.ModHint, 2, calculatedMessageTag)
		calculatedMessageTagMod := calculatedMessageTagInter[0]

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

		randomInter, _ := api.NewHint(utils.ModHint, 2, RandomFactor)
		hashModP := randomInter[0]
		q := randomInter[1]

		api.AssertIsEqual(api.Add(api.Mul(q, JubJubPrimeSubGroup), hashModP), RandomFactor)
		isValid := cmp.IsLess(api, hashModP, JubJubPrimeSubGroup)
		api.AssertIsEqual(isValid, 1)

		receiverHashesModP[i] = hashModP

		isSender := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		isReceiver := api.Sub(1, isSender)

		sumOfReceiverHashes = api.Add(sumOfReceiverHashes, api.Mul(isReceiver, hashModP))
	}

	sumInter, _ := api.NewHint(utils.ModHint, 2, sumOfReceiverHashes)
	senderRandomFactor := sumInter[0]
	sumQ := sumInter[1]

	api.AssertIsEqual(api.Add(api.Mul(sumQ, JubJubPrimeSubGroup), senderRandomFactor), sumOfReceiverHashes)
	isSumValid := cmp.IsLess(api, senderRandomFactor, JubJubPrimeSubGroup)
	api.AssertIsEqual(isSumValid, 1)

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

	return nil
}

type DepositRequest struct {
	HashedSharedSecrets       []string    `json:"hashed_shared_secrets" binding:"required,min=1,max=6"`
	PublicKey                 []string    `json:"public_keys" binding:"required,min=1,max=6"`
	PreviousCommit            [][2]string `json:"previous_commits" binding:"required,min=1,max=6,dive,len=2"`
	TxCommit                  [][2]string `json:"tx_commits" binding:"required,min=1,max=6,dive,len=2"`
	BlockNumber               string      `json:"block_number" binding:"required"`
	AnonymitySet              []string    `json:"anonymity_set" binding:"required,min=1,max=6"`
	MessageTags               []string    `json:"message_tags" binding:"required,min=1,max=6"`
	Nullifier                 string      `json:"nullifier" binding:"required"`
	Hash                      string      `json:"hash" binding:"required"`

	SenderID                  string      `json:"sender_id" binding:"required"`
	SharedSecrets             []string    `json:"shared_secrets" binding:"required,min=1,max=6"`
	SecretKey                 string      `json:"secret_key" binding:"required"`
	PreviousSenderBalance     string      `json:"previous_sender_balance" binding:"required"`
	PreviousSenderRandomValue string      `json:"previous_sender_random_value" binding:"required"`
	TxValues                  []string    `json:"tx_values" binding:"required,min=1,max=6"`
	TxRandomValues            []string    `json:"tx_random_values" binding:"required,min=1,max=6"`
	SenderTxValue             string      `json:"sender_tx_value" binding:"required"`
	PkDeposit                 string      `json:"pk_deposit" binding:"required"`
	Address                   string      `json:"address" binding:"required"`
}

type DepositOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
