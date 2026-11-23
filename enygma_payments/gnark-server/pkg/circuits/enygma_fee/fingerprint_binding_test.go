package enygma_fee

// Every participant's commitment in a fee transfer shifts by a blinding
// factor derived from SharedSecrets[i], a private witness the sender
// chooses. The circuit used to publish only HashedSharedSecrets[i] =
// Poseidon(s_i, s_i), which Enygma.sol never read, so a registered bank
// could make up s_i for any other bank and shift that bank's commitment by a
// blinding factor only the attacker knows, freezing its balance for good.
//
// The circuit now publishes the transfer circuit's k×k fingerprint matrix,
// with FingerPrint[i][senderCol] = Poseidon(s_i), and transferWithFee
// requires the matrix to equal the fingerprints both parties of each pair
// confirmed on chain. These tests pin the circuit's half of that: a made-up
// secret cannot sit behind the confirmed fingerprint, and the nullifier is
// not computable from the public signals.
//
//	CC=/usr/bin/clang go test ./pkg/circuits/enygma_fee -run TestFingerprint -v

import (
	"math/big"
	"testing"

	utils "enygma_payments/gnark-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/iden3/go-iden3-crypto/babyjub"
	iden3poseidon "github.com/iden3/go-iden3-crypto/poseidon"
)

const fpSenderIdx = 0

func fpReduce(x *big.Int) *big.Int { return new(big.Int).Mod(x, utils.P) }

func fpNeg(x *big.Int) *big.Int { return fpReduce(new(big.Int).Sub(utils.P, fpReduce(x))) }

func fpPedersen(v, r *big.Int) [2]*big.Int {
	vG := babyjub.NewPoint().Mul(v, utils.CircuitGBabyJub)
	rH := babyjub.NewPoint().Mul(r, utils.HBabyJub)
	pt := babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
	return [2]*big.Int{pt.X, pt.Y}
}

func fpHash(t *testing.T, in ...*big.Int) *big.Int {
	t.Helper()
	h, err := iden3poseidon.Hash(in)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// feeValues is every raw field of one fee-transfer witness.
type feeValues struct {
	fingerprints [][]*big.Int
	pks          []*big.Int
	prevCommit   [][2]*big.Int
	txCommit     [][2]*big.Int
	blockNumber  *big.Int
	anonymitySet []*big.Int
	messageTags  []*big.Int
	nullifier    *big.Int
	fee          *big.Int
	secrets      []*big.Int
	sk           *big.Int
	prevBalance  *big.Int
	prevR        *big.Int
	txValues     []*big.Int
	txRandom     []*big.Int
	amount       *big.Int
}

// buildFeeValues mirrors Define() for an honest fee transfer: the sender
// (slot 0) pays amount to slot 1 plus fee, from an opening (prevBalance,
// prevR). If secrets is nil the other slots' secrets are 2000+i; the sender's
// fingerprint column is computed from whichever secrets are used, unless
// fingerprints is given, which is published as is.
func buildFeeValues(t *testing.T, secrets []*big.Int, fingerprints [][]*big.Int) feeValues {
	t.Helper()
	sk, prevR := big.NewInt(424242), big.NewInt(67890)
	prevBalance, amount, fee := big.NewInt(500), big.NewInt(100), big.NewInt(20)
	blockNumber := big.NewInt(3000)

	pks := make([]*big.Int, k)
	for i := 0; i < k; i++ {
		ski := big.NewInt(int64(1000 + i))
		if i == fpSenderIdx {
			ski = sk
		}
		pks[i] = fpReduce(fpHash(t, ski, ski))
	}

	s := make([]*big.Int, k)
	s[fpSenderIdx] = fpReduce(fpHash(t, prevR, sk))
	for i := 0; i < k; i++ {
		if i == fpSenderIdx {
			continue
		}
		if secrets != nil {
			s[i] = secrets[i]
		} else {
			s[i] = big.NewInt(int64(2000 + i))
		}
	}

	if fingerprints == nil {
		fingerprints = make([][]*big.Int, k)
		for i := 0; i < k; i++ {
			fingerprints[i] = make([]*big.Int, k)
			for j := 0; j < k; j++ {
				fingerprints[i][j] = big.NewInt(0)
			}
		}
		for i := 0; i < k; i++ {
			if i != fpSenderIdx {
				fingerprints[i][fpSenderIdx] = fpReduce(fpHash(t, s[i]))
			}
		}
	}

	anonymitySet := make([]*big.Int, k)
	for i := range anonymitySet {
		anonymitySet[i] = big.NewInt(int64(i + 1))
	}
	hashTag := fpHash(t, big.NewInt(12))
	hashRandom := fpHash(t, big.NewInt(21))
	messageTags := make([]*big.Int, k)
	rVals := make([]*big.Int, k)
	rSum := big.NewInt(0)
	for i := 0; i < k; i++ {
		messageTags[i] = fpReduce(fpHash(t, hashTag, s[i], blockNumber))
		rVals[i] = fpReduce(fpHash(t, hashRandom, s[i], blockNumber))
		if i != fpSenderIdx {
			rSum = fpReduce(new(big.Int).Add(rSum, rVals[i]))
		}
	}

	txValues := make([]*big.Int, k)
	txRandom := make([]*big.Int, k)
	txCommit := make([][2]*big.Int, k)
	prevCommit := make([][2]*big.Int, k)
	for i := 0; i < k; i++ {
		switch i {
		case fpSenderIdx:
			txValues[i] = fpNeg(new(big.Int).Add(amount, fee))
			txRandom[i] = rSum
			prevCommit[i] = fpPedersen(prevBalance, prevR)
		case 1:
			txValues[i] = new(big.Int).Set(amount)
		default:
			txValues[i] = big.NewInt(0)
		}
		if i != fpSenderIdx {
			txRandom[i] = fpNeg(rVals[i])
			prevCommit[i] = fpPedersen(big.NewInt(0), big.NewInt(int64(3000+i)))
		}
		txCommit[i] = fpPedersen(txValues[i], txRandom[i])
	}

	preimage := fpReduce(fpHash(t, s[fpSenderIdx], s[fpSenderIdx]))
	return feeValues{
		fingerprints: fingerprints, pks: pks, prevCommit: prevCommit, txCommit: txCommit,
		blockNumber: blockNumber, anonymitySet: anonymitySet, messageTags: messageTags,
		nullifier: fpHash(t, preimage, blockNumber), fee: fee, secrets: s, sk: sk,
		prevBalance: prevBalance, prevR: prevR, txValues: txValues, txRandom: txRandom,
		amount: amount,
	}
}

func (v feeValues) circuit() *EnygmaFeeCircuit {
	w := createCircuitTemplate(DefaultConfig)
	for i := 0; i < k; i++ {
		for j := 0; j < k; j++ {
			w.FingerPrintofSharedSecrets[i][j] = v.fingerprints[i][j]
		}
		w.PublicKey[i] = v.pks[i]
		w.PreviousCommit[i] = [2]frontend.Variable{v.prevCommit[i][0], v.prevCommit[i][1]}
		w.TxCommit[i] = [2]frontend.Variable{v.txCommit[i][0], v.txCommit[i][1]}
		w.AnonymitySet[i] = v.anonymitySet[i]
		w.MessageTags[i] = v.messageTags[i]
		w.SharedSecrets[i] = v.secrets[i]
		w.TxValues[i] = v.txValues[i]
		w.TxRandomValues[i] = v.txRandom[i]
	}
	w.BlockNumber = v.blockNumber
	w.Nullifier = v.nullifier
	w.Fee = v.fee
	w.SumTxCommit = [2]frontend.Variable{0, 1}
	w.SumTxValuesWithFee = 0
	w.DomainId = 1 // unconstrained by Define()
	w.SenderId = v.anonymitySet[fpSenderIdx]
	w.SecretKey = v.sk
	w.PreviousSenderBalance = v.prevBalance
	w.PreviousSenderRandomValue = v.prevR
	w.SenderTxValue = v.amount
	return &w
}

func solve(t *testing.T, w *EnygmaFeeCircuit) error {
	t.Helper()
	solver.RegisterHint(utils.ModHint)
	tmpl := createCircuitTemplate(DefaultConfig)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	full, err := frontend.NewWitness(w, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("witness: %v", err)
	}
	return ccs.IsSolved(full)
}

func fabricatedSecrets() []*big.Int {
	out := make([]*big.Int, k)
	for i := range out {
		out[i] = new(big.Int).SetUint64(0xdeadbeef00 + uint64(i))
	}
	return out
}

func TestFingerprint_HonestFeeTransferSolves(t *testing.T) {
	if err := solve(t, buildFeeValues(t, nil, nil).circuit()); err != nil {
		t.Fatalf("honest fee transfer rejected: %v", err)
	}
}

// The attack: made-up secrets for every other bank, publishing the
// fingerprints the banks actually confirmed (the only ones transferWithFee
// accepts). The circuit must reject it.
func TestFingerprint_FabricatedSecretsCannotPublishConfirmedFingerprints(t *testing.T) {
	honest := buildFeeValues(t, nil, nil)
	attack := buildFeeValues(t, fabricatedSecrets(), honest.fingerprints)
	if err := solve(t, attack.circuit()); err == nil {
		t.Fatal("a fee transfer with made-up shared secrets solved while publishing the confirmed fingerprints")
	}
}

// Made-up secrets solve only behind their own fingerprints, which are not the
// confirmed ones: that is what transferWithFee compares and rejects (on chain:
// go_client/enygma_test/fee_coverage_test.go).
func TestFingerprint_FabricatedSecretsPublishDifferentFingerprints(t *testing.T) {
	honest := buildFeeValues(t, nil, nil)
	attack := buildFeeValues(t, fabricatedSecrets(), nil)
	if err := solve(t, attack.circuit()); err != nil {
		t.Fatalf("attack witness with its own fingerprints did not solve: %v", err)
	}
	for i := 0; i < k; i++ {
		if i != fpSenderIdx && attack.fingerprints[i][fpSenderIdx].Cmp(honest.fingerprints[i][fpSenderIdx]) == 0 {
			t.Fatalf("slot %d: made-up secret published the confirmed fingerprint", i)
		}
	}
}

// The nullifier's preimage used to be public (the sender's
// HashedSharedSecrets slot), so Poseidon(signal, BlockNumber) == Nullifier
// picked out the sender. No public signal may do that now.
func TestFingerprint_NullifierNotComputableFromPublicSignals(t *testing.T) {
	v := buildFeeValues(t, nil, nil)
	if err := solve(t, v.circuit()); err != nil {
		t.Fatalf("honest witness rejected: %v", err)
	}
	var public []*big.Int
	for _, row := range v.fingerprints {
		public = append(public, row...)
	}
	public = append(public, v.pks...)
	public = append(public, v.anonymitySet...)
	public = append(public, v.messageTags...)
	for _, p := range append(append([][2]*big.Int{}, v.prevCommit...), v.txCommit...) {
		public = append(public, p[0], p[1])
	}
	for _, x := range public {
		if fpHash(t, x, v.blockNumber).Cmp(v.nullifier) == 0 {
			t.Fatalf("Poseidon(%s, BlockNumber) equals the nullifier", x)
		}
	}
}
