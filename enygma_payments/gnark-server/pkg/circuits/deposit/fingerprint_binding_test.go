package deposit

// The deposit circuit shifts every participant's commitment by a blinding
// factor derived from SharedSecrets[i], exactly like withdraw (see
// withdraw/fingerprint_binding_test.go for the freeze this enabled). It now
// publishes the transfer circuit's k×k fingerprint matrix, which Enygma.sol
// checks against the confirmed registry; these tests pin that a made-up
// secret cannot sit behind a confirmed fingerprint.
//
//	CC=/usr/bin/clang go test ./pkg/circuits/deposit -run TestFingerprint -v

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

func hash(t *testing.T, in ...*big.Int) *big.Int {
	t.Helper()
	h, err := iden3poseidon.Hash(in)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func modP(x *big.Int) *big.Int { return new(big.Int).Mod(x, utils.P) }

func pedersen(v, r *big.Int) [2]frontend.Variable {
	vG := babyjub.NewPoint().Mul(v, utils.CircuitGBabyJub)
	rH := babyjub.NewPoint().Mul(r, utils.HBabyJub)
	p := babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
	return [2]frontend.Variable{p.X, p.Y}
}

// depositWitness builds a deposit of amount to the sender (slot 0) whose
// non-sender shared secrets are secrets[1:]; the published fingerprint column
// is fingerprints[i] (for i != 0).
func depositWitness(t *testing.T, amount int64, secrets, fingerprints []*big.Int) *DepositEnygmaCircuit {
	t.Helper()
	const k, sender = 6, 0
	sk, prevR, prevBalance, blockNumber := big.NewInt(424242), big.NewInt(67890), big.NewInt(500), big.NewInt(3000)
	w := createDepositCircuitTemplate(DefaultConfig)

	ss := append([]*big.Int{modP(hash(t, prevR, sk))}, secrets[1:]...)
	hashTag, hashRandom := hash(t, big.NewInt(12)), hash(t, big.NewInt(21))
	rVals := make([]*big.Int, k)
	rSum := big.NewInt(0)
	for i := 0; i < k; i++ {
		w.AnonymitySet[i] = big.NewInt(int64(i + 1))
		skI := big.NewInt(int64(1000 + i))
		if i == sender {
			skI = sk
		}
		w.PublicKey[i] = modP(hash(t, skI, skI))
		w.SharedSecrets[i] = ss[i]
		w.MessageTags[i] = modP(hash(t, hashTag, ss[i], blockNumber))
		rVals[i] = modP(hash(t, hashRandom, ss[i], blockNumber))
		if i != sender {
			rSum = modP(new(big.Int).Add(rSum, rVals[i]))
		}
		for j := 0; j < k; j++ {
			w.FingerPrintofSharedSecrets[i][j] = big.NewInt(0)
		}
		if i != sender {
			w.FingerPrintofSharedSecrets[i][sender] = fingerprints[i]
		}
	}
	for i := 0; i < k; i++ {
		if i == sender {
			w.TxValues[i] = big.NewInt(amount)
			w.TxRandomValues[i] = rSum
			w.PreviousCommit[i] = pedersen(prevBalance, prevR)
		} else {
			w.TxValues[i] = big.NewInt(0)
			w.TxRandomValues[i] = new(big.Int).Sub(utils.P, rVals[i])
			w.PreviousCommit[i] = pedersen(big.NewInt(0), big.NewInt(int64(3000+i)))
		}
		w.TxCommit[i] = pedersen(w.TxValues[i].(*big.Int), w.TxRandomValues[i].(*big.Int))
	}
	w.BlockNumber = blockNumber
	w.Nullifier = hash(t, modP(hash(t, ss[sender], ss[sender])), blockNumber)
	w.SenderId = w.AnonymitySet[sender]
	w.SecretKey = sk
	w.PreviousSenderBalance = prevBalance
	w.PreviousSenderRandomValue = prevR
	w.SenderTxValue = big.NewInt(amount)
	w.Address = big.NewInt(777)
	w.PkDeposit = big.NewInt(555)
	w.Hash = hash(t, hash(t, big.NewInt(777), big.NewInt(amount)), big.NewInt(555))
	w.DomainId = big.NewInt(1)
	return &w
}

func realSecrets() []*big.Int {
	out := make([]*big.Int, 6)
	for i := range out {
		out[i] = big.NewInt(int64(2000 + i))
	}
	return out
}

func fingerprintsOf(t *testing.T, secrets []*big.Int) []*big.Int {
	out := make([]*big.Int, len(secrets))
	for i, s := range secrets {
		out[i] = modP(hash(t, s))
	}
	return out
}

func solve(t *testing.T, w *DepositEnygmaCircuit) error {
	t.Helper()
	solver.RegisterHint(utils.ModHint)
	template := createDepositCircuitTemplate(DefaultConfig)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	witness, err := frontend.NewWitness(w, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("witness: %v", err)
	}
	_, err = ccs.Solve(witness)
	return err
}

func TestFingerprint_HonestDepositSolves(t *testing.T) {
	s := realSecrets()
	if err := solve(t, depositWitness(t, 100, s, fingerprintsOf(t, s))); err != nil {
		t.Fatalf("honest deposit rejected: %v", err)
	}
}

func TestFingerprint_DepositFabricatedSecretsCannotPublishConfirmedFingerprints(t *testing.T) {
	fake := make([]*big.Int, 6)
	for i := range fake {
		fake[i] = new(big.Int).SetUint64(0xdeadbeef00 + uint64(i))
	}
	// Zero value: the attack needs nothing on the DvP side either.
	if err := solve(t, depositWitness(t, 0, fake, fingerprintsOf(t, realSecrets()))); err == nil {
		t.Fatal("a deposit with made-up shared secrets solved while publishing the confirmed fingerprints")
	}
}
