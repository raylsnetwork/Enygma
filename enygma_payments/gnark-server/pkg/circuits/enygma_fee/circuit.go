package enygma_fee

import (
	"math/big"

	pos "enygma-server/poseidon"
	utils "enygma-server/utils"

	"github.com/consensys/gnark/frontend"
	cmp "github.com/consensys/gnark/std/math/cmp"
	"github.com/consensys/gnark/std/algebra/native/twistededwards"
)

// JubJubPrimeSubGroupStr is the Baby JubJub prime subgroup order.
const JubJubPrimeSubGroupStr = "2736030358979909402780800718157159386076813972158567259200215660948447373041"

type EnygmaFeeCircuitConfig struct {
	NCommitment int
}

// EnygmaFeeCircuit extends EnygmaCircuit with a public Fee signal and two
// aggregate public outputs that expose the conservation sums to the verifier.
//
// Signal layout (54 public slots):
//   0–5   HashedSharedSecrets (×6)
//   6–11  PublicKey           (×6)
//   12–23 PreviousCommit      (X,Y ×6)
//   24–35 TxCommit            (X,Y ×6)
//   36    BlockNumber
//   37–42 AnonymitySet        (×6)
//   43–48 MessageTags         (×6)
//   49    Nullifier
//   50    Fee                 (PROTOCOL_FEE constant)
//   51–52 SumTxCommit         (X,Y of Σ TxCommit — neutral iff conservation holds)
//   53    SumTxValuesWithFee  (Σ(TxValues)+fee mod P — equals fee iff Σ(TxValues)=0)
//
// Conservation is NOT hard-asserted inside the circuit. Instead these public
// outputs let the smart contract enforce whichever rule it needs:
//   off-chain fee: SumTxCommit==(0,1) and SumTxValuesWithFee==PROTOCOL_FEE
//   on-chain fee:  SumTxValuesWithFee==0 and SumTxCommit+fee·G==(0,1)
type EnygmaFeeCircuit struct {
	Config EnygmaFeeCircuitConfig

	// Public signals
	HashedSharedSecrets []frontend.Variable    `gnark:",public"` // hash of shared secrets per participant
	PublicKey           []frontend.Variable    `gnark:",public"` // public keys for all participants
	PreviousCommit      [][2]frontend.Variable `gnark:",public"` // previous Pedersen commitments
	TxCommit            [][2]frontend.Variable `gnark:",public"` // new Pedersen commitments for this tx
	BlockNumber         frontend.Variable      `gnark:",public"` // block number for randomness
	AnonymitySet        []frontend.Variable    `gnark:",public"` // participant indices (k-anonymity)
	MessageTags         []frontend.Variable    `gnark:",public"` // unique transaction tags
	Nullifier           frontend.Variable      `gnark:",public"` // double-spend prevention
	Fee                 frontend.Variable      `gnark:",public"` // PROTOCOL_FEE constant (slot 50)
	SumTxCommit         [2]frontend.Variable   `gnark:",public"` // Σ(TxCommit) point X,Y (slots 51–52)
	SumTxValuesWithFee  frontend.Variable      `gnark:",public"` // (Σ(TxValues)+fee) mod P (slot 53)

	// Private signals
	SenderId                  frontend.Variable  // sender index
	SharedSecrets             []frontend.Variable // shared secrets per participant
	SecretKey                 frontend.Variable  // sender's secret key
	PreviousSenderBalance     frontend.Variable  // sender's previous balance
	PreviousSenderRandomValue frontend.Variable  // sender's previous Pedersen blinding factor
	TxValues                  []frontend.Variable // per-participant value deltas (sender negative)
	TxRandomValues            []frontend.Variable // per-participant Pedersen blinding factors
	SenderTxValue             frontend.Variable  // total amount sender spends (dest + change + fee)
}

func (circuit *EnygmaFeeCircuit) Define(api frontend.API) error {
	k := circuit.Config.NCommitment
	JubJubPrimeSubGroup := frontend.Variable(JubJubPrimeSubGroupStr)

	///////////////////////////////////**///////////////////////////////////
	// Check if SenderId is in the anonymity set
	sumIsInK := frontend.Variable(0)
	for i := 0; i < k; i++ {
		isEqual := api.IsZero(api.Sub(circuit.AnonymitySet[i], circuit.SenderId))
		sumIsInK = api.Add(isEqual, sumIsInK)
	}
	api.AssertIsEqual(sumIsInK, 1)

	///////////////////////////////////**///////////////////////////////////
	// Check sender's TxValue matches SenderTxValue (negated mod P)
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

	// Sender absorbs fee: TxValues[sender] = (P - SenderTxValue - Fee) mod P
	expectedTxValue := api.Sub(api.Sub(pDiffConstrained, vConstrained), circuit.Fee)
	expectedTxValueInter, _ := api.NewHint(utils.ModHint, 2, expectedTxValue)
	expectedTxValueMod := expectedTxValueInter[0]
	expectedTxValueQ := expectedTxValueInter[1]
	api.AssertIsEqual(api.Add(api.Mul(expectedTxValueQ, JubJubPrimeSubGroup), expectedTxValueMod), expectedTxValue)
	api.AssertIsEqual(cmp.IsLess(api, expectedTxValueMod, JubJubPrimeSubGroup), 1)
	api.AssertIsEqual(selectedVConstrained, expectedTxValueMod)

	///////////////////////////////////**///////////////////////////////////
	// Check all commits lie on the Baby JubJub curve
	for i := 0; i < k; i++ {
		utils.AssertPointsIsOnCurve(api, circuit.PreviousCommit[i][0], circuit.PreviousCommit[i][1])
		utils.AssertPointsIsOnCurve(api, circuit.TxCommit[i][0], circuit.TxCommit[i][1])
	}

	///////////////////////////////////**///////////////////////////////////
	// Check knowledge of sender's shared secret
	selectedSecret := frontend.Variable(0)
	for i := 0; i < k; i++ {
		eq := api.IsZero(api.Sub(circuit.SenderId, circuit.AnonymitySet[i]))
		selectedSecret = api.Add(selectedSecret, api.Mul(eq, circuit.SharedSecrets[i]))
	}

	secretSenderCalculated := pos.Poseidon(api, []frontend.Variable{circuit.PreviousSenderRandomValue, circuit.SecretKey})
	secretInter, _ := api.NewHint(utils.ModHint, 2, secretSenderCalculated)
	secretRemain := secretInter[0]
	secretQ := secretInter[1]
	api.AssertIsEqual(api.Add(api.Mul(secretQ, JubJubPrimeSubGroup), secretRemain), secretSenderCalculated)
	api.AssertIsEqual(cmp.IsLess(api, secretRemain, JubJubPrimeSubGroup), 1)
	api.AssertIsEqual(secretRemain, selectedSecret)

	///////////////////////////////////**///////////////////////////////////
	// Check hashed shared secrets are well-formed
	for i := 0; i < k; i++ {
		calculatedHash := pos.Poseidon(api, []frontend.Variable{circuit.SharedSecrets[i], circuit.SharedSecrets[i]})
		hashInter, _ := api.NewHint(utils.ModHint, 2, calculatedHash)
		hashMod := hashInter[0]
		hashQ := hashInter[1]
		api.AssertIsEqual(api.Add(api.Mul(hashQ, JubJubPrimeSubGroup), hashMod), calculatedHash)
		api.AssertIsEqual(cmp.IsLess(api, hashMod, JubJubPrimeSubGroup), 1)
		api.AssertIsEqual(hashMod, circuit.HashedSharedSecrets[i])
	}

	///////////////////////////////////**///////////////////////////////////
	// Check knowledge of secret key (generates sender's public key)
	selectedPK := frontend.Variable(0)
	for i := 0; i < k; i++ {
		diff := api.Sub(circuit.SenderId, circuit.AnonymitySet[i])
		eq := api.IsZero(diff)
		selectedPK = api.Add(selectedPK, api.Mul(eq, circuit.PublicKey[i]))
	}
	pk := pos.Poseidon(api, []frontend.Variable{circuit.SecretKey, circuit.SecretKey})
	pkInter, _ := api.NewHint(utils.ModHint, 2, pk)
	pkMod := pkInter[0]
	pkQ := pkInter[1]
	api.AssertIsEqual(api.Add(api.Mul(pkQ, JubJubPrimeSubGroup), pkMod), pk)
	api.AssertIsEqual(cmp.IsLess(api, pkMod, JubJubPrimeSubGroup), 1)
	api.AssertIsEqual(selectedPK, pkMod)

	///////////////////////////////////**///////////////////////////////////
	// Check knowledge of sender's previous Pedersen commitment
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
	// Check message tags are well-formed
	HashTag := pos.Poseidon(api, []frontend.Variable{12})
	for i := 0; i < k; i++ {
		calculatedMessageTag := pos.Poseidon(api, []frontend.Variable{HashTag, circuit.SharedSecrets[i], circuit.BlockNumber})
		calculatedMessageTagInter, _ := api.NewHint(utils.ModHint, 2, calculatedMessageTag)
		calculatedMessageTagMod := calculatedMessageTagInter[0]
		calculatedMessageTagQ := calculatedMessageTagInter[1]
		api.AssertIsEqual(api.Add(api.Mul(calculatedMessageTagQ, JubJubPrimeSubGroup), calculatedMessageTagMod), calculatedMessageTag)
		api.AssertIsEqual(cmp.IsLess(api, calculatedMessageTagMod, JubJubPrimeSubGroup), 1)
		api.AssertIsEqual(circuit.MessageTags[i], calculatedMessageTagMod)
	}

	///////////////////////////////////**///////////////////////////////////
	// Aggregate 1: expose (Σ(TxValues) + fee) mod P as public output SumTxValuesWithFee.
	// The circuit does NOT assert a specific value — the verifier/contract does.
	//   off-chain fee model: SumTxValuesWithFee == PROTOCOL_FEE  (Σ(TxValues)==0)
	//   on-chain  fee model: SumTxValuesWithFee == 0             (Σ(TxValues)==−fee)
	sumX := frontend.Variable(0)
	for i := 0; i < k; i++ {
		sumX = api.Add(sumX, circuit.TxValues[i])
	}
	sumXWithFee := api.Add(sumX, circuit.Fee)
	sumXWithFeeInter, _ := api.NewHint(utils.ModHint, 2, sumXWithFee)
	sumXWithFeeMod := sumXWithFeeInter[0]
	sumXWithFeeQ   := sumXWithFeeInter[1]
	api.AssertIsEqual(api.Add(api.Mul(sumXWithFeeQ, JubJubPrimeSubGroup), sumXWithFeeMod), sumXWithFee)
	api.AssertIsEqual(cmp.IsLess(api, sumXWithFeeMod, JubJubPrimeSubGroup), 1)
	api.AssertIsEqual(sumXWithFeeMod, circuit.SumTxValuesWithFee)

	// Aggregate 2: compute Σ(TxCommit) + fee·G and hard-assert == (0,1).
	// Sender's TxValue = -(amount+fee) ⟹ Σ(TxCommit) = (-fee)·G.
	// Adding fee·G gives 0·G = BJJ identity = (0,1).
	// The proof is unproducible if conservation is violated.
	txCommitSum := twistededwards.Point{
		X: circuit.TxCommit[0][0],
		Y: circuit.TxCommit[0][1],
	}
	for i := 1; i < k; i++ {
		pt := twistededwards.Point{
			X: circuit.TxCommit[i][0],
			Y: circuit.TxCommit[i][1],
		}
		txCommitSum = utils.PointAdd(api, txCommitSum, pt)
	}
	feeCommit := utils.ScalarMul(api, utils.G, circuit.Fee)
	txCommitSum = utils.PointAdd(api, txCommitSum, feeCommit)
	api.AssertIsEqual(txCommitSum.X, frontend.Variable(0))
	api.AssertIsEqual(txCommitSum.Y, frontend.Variable(1))
	api.AssertIsEqual(txCommitSum.X, circuit.SumTxCommit[0])
	api.AssertIsEqual(txCommitSum.Y, circuit.SumTxCommit[1])

	///////////////////////////////////**///////////////////////////////////
	// Range proof: previousBalance >= (senderTxValue + fee) >= 0.
	// Sender deducts (amount + fee) from commitment — must have sufficient balance.
	previousVBits := api.ToBinary(circuit.PreviousSenderBalance, 252)
	previousVConstrained := api.FromBinary(previousVBits...)

	feeBits := api.ToBinary(circuit.Fee, 252)
	feeConstrained := api.FromBinary(feeBits...)

	// Total debit = transfer amount + protocol fee (both 252-bit constrained)
	totalDebit := api.Add(vConstrained, feeConstrained)
	totalDebitBits := api.ToBinary(totalDebit, 253)
	totalDebitConstrained := api.FromBinary(totalDebitBits...)

	// previousBalance >= totalDebit
	prevVGreaterEqualTotal := api.Cmp(previousVConstrained, totalDebitConstrained)
	api.AssertIsEqual(api.IsZero(api.Add(prevVGreaterEqualTotal, frontend.Variable(1))), frontend.Variable(0))

	// senderTxValue >= 0
	vGreaterEqualZero := api.Cmp(vConstrained, frontend.Variable(0))
	api.AssertIsEqual(api.IsZero(api.Add(vGreaterEqualZero, frontend.Variable(1))), frontend.Variable(0))

	///////////////////////////////////**///////////////////////////////////
	// Nullifier: Poseidon(hashedSharedSecret[sender], blockNumber)
	selectedPreImage := frontend.Variable(0)
	for i := 0; i < k; i++ {
		diff := api.Sub(circuit.SenderId, circuit.AnonymitySet[i])
		eq := api.IsZero(diff)
		selectedPreImage = api.Add(selectedPreImage, api.Mul(eq, circuit.HashedSharedSecrets[i]))
	}
	computedNullifier := pos.Poseidon(api, []frontend.Variable{selectedPreImage, circuit.BlockNumber})
	api.AssertIsEqual(computedNullifier, circuit.Nullifier)

	///////////////////////////////////**///////////////////////////////////
	// Check each TxCommit is the correct Pedersen commitment
	for i := 0; i < k; i++ {
		computedPedersenCommitment := utils.PedersenCommitment(api, circuit.TxValues[i], circuit.TxRandomValues[i])
		api.AssertIsEqual(circuit.TxCommit[i][0], computedPedersenCommitment.X)
		api.AssertIsEqual(circuit.TxCommit[i][1], computedPedersenCommitment.Y)
	}

	///////////////////////////////////**///////////////////////////////////
	// Check random factors are well-formed (same derivation as base circuit)
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

	return nil
}

// EnygmaFeeRequest is the JSON body for POST /proof/enygma_fee.
// Fee is a non-negative decimal string (e.g. "1000000000000000" for 0.001 native token).
type EnygmaFeeRequest struct {
	HashedSharedSecrets       []string    `json:"hashed_shared_secrets" binding:"required,min=1,max=6"`
	PublicKey                 []string    `json:"public_keys" binding:"required,min=1,max=6"`
	PreviousCommit            [][2]string `json:"previous_commits" binding:"required,min=1,max=6,dive,len=2"`
	TxCommit                  [][2]string `json:"tx_commits" binding:"required,min=1,max=6,dive,len=2"`
	BlockNumber               string      `json:"block_number" binding:"required"`
	AnonymitySet              []string    `json:"anonymity_set" binding:"required,min=1,max=6"`
	MessageTags               []string    `json:"message_tags" binding:"required,min=1,max=6"`
	Nullifier                 string      `json:"nullifier" binding:"required"`
	Fee                       string      `json:"fee" binding:"required"`

	SenderID                  string      `json:"sender_id" binding:"required"`
	SharedSecrets             []string    `json:"shared_secrets" binding:"required,min=1,max=6"`
	SecretKey                 string      `json:"secret_key" binding:"required"`
	PreviousSenderBalance     string      `json:"previous_sender_balance" binding:"required"`
	PreviousSenderRandomValue string      `json:"previous_sender_random_value" binding:"required"`
	TxValues                  []string    `json:"tx_values" binding:"required,min=1,max=6"`
	TxRandomValues            []string    `json:"tx_random_values" binding:"required,min=1,max=6"`
	SenderTxValue             string      `json:"sender_tx_value" binding:"required"`
}

type EnygmaFeeOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
