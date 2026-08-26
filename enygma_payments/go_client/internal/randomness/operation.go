package randomness

import (
	"math/big"

	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"

	"enygma/internal/curve"

	enygma "enygma/contracts"
)

// perSlotNonce folds nullifier (a per-transaction value — see the Fix
// H-01/H-02 comment on TagMessageGen below) and a direction tag
// Poseidon(senderId, receiverId) into a single value, matching
// gnark-server/pkg/circuits/enygma/circuit.go's perSlotNonce exactly
// (chained as two 2-input Poseidon calls, not one 3-input call, because
// the in-circuit Poseidon gadget's S-box constant table tops out at 3
// inputs — see that file's comment for the full explanation).
func perSlotNonce(nullifier *big.Int, senderId, receiverId int) *big.Int {
	directionTag, _ := poseidon.Hash([]*big.Int{big.NewInt(int64(senderId)), big.NewInt(int64(receiverId))})
	nonce, _ := poseidon.Hash([]*big.Int{nullifier, directionTag})
	return nonce
}

// GetRValues computes the Pedersen blinding factor r_i for every bank i in
// k_index.
//
// Fix H-02: nullifier replaces block_hash (the epoch anchor) as the
// per-transaction value, and senderId/i are folded in via perSlotNonce for
// direction-asymmetry — see circuit.go's comment on the random-factor
// block for why (epoch-constant + direction-symmetric r let a passive
// chain observer cancel the H term across two same-epoch transactions and
// recover exact amounts).
//
// AnonymitySet[i] is assumed to equal bank id i (every shipped client
// hardcodes the full 6-bank set as its own anonymity set — see H-01's
// "structural aggravators" — so slot position and bank id coincide here).
func GetRValues(senderId int, s []*big.Int, nullifier *big.Int, k_index []*big.Int) []*big.Int {
	var rValues []*big.Int
	randomInt := big.NewInt(21)
	HashRandom, _ := poseidon.Hash([]*big.Int{randomInt})

	for i := 0; i < len(s); i++ {
		if ContainsBigInt(k_index, i) {
			nonce := perSlotNonce(nullifier, senderId, i)
			inputs := []*big.Int{HashRandom, s[i], nonce}
			PoseidonHash, _ := poseidon.Hash(inputs)
			PoseidonHash.Mod(PoseidonHash, curve.P)
			rValues = append(rValues, PoseidonHash)
		}
	}
	return rValues
}

// FingerPrintGen builds the k×k FingerPrintofSharedSecrets matrix the
// current enygma circuit (Fix C-04) actually requires:
// fp[i][senderId] = Poseidon(s[i]) mod P for every i != senderId; every
// other entry (including the diagonal) is unconstrained by the circuit
// and left at zero.
//
// Fix L-08 (blockers 1 and 2): this replaces the old HashArrayGen, whose
// output shape (a flat []*big.Int) and derivation (a two-input
// Poseidon([s[j], s[j]])) matched neither the circuit's current
// [][]frontend.Variable field name/shape nor its one-input
// Poseidon([SharedSecrets[i]]) formula — HashArrayGen was computing a
// value the current circuit does not accept under any field name.
func FingerPrintGen(s []*big.Int, senderId int) [][]*big.Int {
	k := len(s)
	fp := make([][]*big.Int, k)
	for i := range fp {
		fp[i] = make([]*big.Int, k)
		for j := range fp[i] {
			fp[i][j] = big.NewInt(0)
		}
	}
	for i := 0; i < k; i++ {
		if i == senderId {
			continue
		}
		h, err := poseidon.Hash([]*big.Int{s[i]})
		if err != nil {
			panic(err)
		}
		fp[i][senderId] = h.Mod(h, curve.P)
	}
	return fp
}

// TagMessageGen computes the message tag for every bank i in k_index.
//
// Fix H-01 (mechanism 1): nullifier replaces block_hash (the epoch
// anchor). The tag used to be constant for a whole epoch and identical
// regardless of who sent — see circuit.go's comment on the message-tag
// block for the full attack this closes (a passive chain observer
// comparing two same-epoch transactions saw exactly one tag slot differ,
// which is the sender's slot; nullifier is guaranteed fresh per
// transaction, so that signature is gone). See perSlotNonce above for the
// direction-asymmetry half.
func TagMessageGen(senderId int, s []*big.Int, nullifier *big.Int, k_index []*big.Int) []*big.Int {
	var tagMessage []*big.Int
	tag := big.NewInt(12)
	HashTag, _ := poseidon.Hash([]*big.Int{tag})

	for i := 0; i < len(s); i++ {
		if ContainsBigInt(k_index, i) {
			nonce := perSlotNonce(nullifier, senderId, i)
			inputs := []*big.Int{HashTag, s[i], nonce}
			PoseidonHash, _ := poseidon.Hash(inputs)
			PoseidonHash.Mod(PoseidonHash, curve.P)
			tagMessage = append(tagMessage, PoseidonHash)
		}
	}

	return tagMessage
}

func GetRSum(senderId int, s []*big.Int, nullifier *big.Int, k_index []*big.Int) *big.Int {
	sum := big.NewInt(0)
	randomInt := big.NewInt(21)
	HashRandom, _ := poseidon.Hash([]*big.Int{randomInt})

	for i := 0; i < len(s); i++ {
		if senderId != i {
			if ContainsBigInt(k_index, i) {
				nonce := perSlotNonce(nullifier, senderId, i)
				inputs := []*big.Int{HashRandom, s[i], nonce}
				PoseidonHash, _ := poseidon.Hash(inputs)
				PoseidonHash.Mod(PoseidonHash, curve.P)
				sum.Add(sum, PoseidonHash)
				sum.Mod(sum, curve.P)
			}
		}
	}
	return sum
}

func ContainsBigInt(s []*big.Int, val int) bool {
	valBig := big.NewInt(int64(val))
	for _, v := range s {
		if v.Cmp(valBig) == 0 {
			return true
		}
	}
	return false
}

// nullifier (Fix H-02) is this transaction's public Nullifier signal —
// callers must compute it before calling this function; it is what makes
// txRandom fresh per transaction instead of constant for an epoch. See
// GetRValues/TagMessageGen above.
func GenCommitmentAndRandom(qtyBanks int, v *big.Int, senderId int, txValues []*big.Int, nullifier *big.Int, kIndex []*big.Int, secrets []*big.Int) ([]enygma.IEnygmaPoint, []*big.Int) {
	txRandom := GetRValues(senderId, secrets, nullifier, kIndex)
	rSum := GetRSum(senderId, secrets, nullifier, kIndex)
	txRandom[senderId] = rSum
	var txCommit []*babyjub.Point

	for i := 0; i < len(kIndex); i++ {
		if i == senderId {
			txCommit = append(txCommit, curve.PedersenCommitment(curve.GetNegative(v), txRandom[i]))
		} else {
			txCommit = append(txCommit, curve.PedersenCommitment(txValues[i], curve.GetNegative(txRandom[i])))
		}
	}

	// Negate receiver random values to match circuit expectation:
	// circuit asserts TxRandomValues[receiver] = p - hashModP[receiver]
	for i := 0; i < len(kIndex); i++ {
		if i != senderId {
			txRandom[i] = curve.GetNegative(txRandom[i])
		}
	}

	commitments := make([]enygma.IEnygmaPoint, len(kIndex))
	for i := 0; i < len(kIndex); i++ {
		commit := enygma.IEnygmaPoint{C1: txCommit[i].X, C2: txCommit[i].Y}
		commitments[i] = commit
	}

	return commitments, txRandom
}
