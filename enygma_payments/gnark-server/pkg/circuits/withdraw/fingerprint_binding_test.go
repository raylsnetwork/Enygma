package withdraw

// Every participant's commitment in a withdrawal shifts by a blinding factor
// derived from SharedSecrets[i], a private witness the sender chooses. The
// circuit used to publish only HashedSharedSecrets[i] = Poseidon(s_i, s_i),
// which Enygma.sol never read, so a registered bank could make up s_i for
// any other bank and, with a zero-value withdrawal (no funds needed), shift
// that bank's commitment by Com(0, P - h(s_i)): a blinding factor only the
// attacker knows, so the victim can never open its balance again.
//
// The circuit now publishes the transfer circuit's k×k fingerprint matrix,
// with FingerPrint[i][senderCol] = Poseidon(s_i), and Enygma.sol requires
// the matrix to equal the fingerprints both parties of each pair confirmed
// on chain. These tests pin the circuit's half of that: a made-up secret
// cannot sit behind the confirmed fingerprint, and the nullifier is no
// longer computable from the public signals.
//
//	CC=/usr/bin/clang go test ./pkg/circuits/withdraw -run TestFingerprint -v

import (
	"math/big"
	"testing"

	iden3poseidon "github.com/iden3/go-iden3-crypto/poseidon"
)

// withSecrets returns v rebuilt for a zero-value withdrawal (no deposit
// slots) whose non-sender shared secrets are secrets[i] (index 0, the
// sender's, is kept): message tags, blinding factors, tx commitments, and,
// if republish, the sender's fingerprint column, all recomputed from them.
func withSecrets(t *testing.T, v m03Values, secrets []*big.Int, republish bool) m03Values {
	t.Helper()
	v.withdrawAmt = big.NewInt(0)
	for i := range v.hashes {
		v.hashes[i], v.skDeposits[i], v.vPerDeposit[i] = big.NewInt(0), big.NewInt(0), big.NewInt(0)
	}
	v.secrets = append([]*big.Int{v.secrets[0]}, secrets[1:]...)
	hashTag, _ := iden3poseidon.Hash([]*big.Int{big.NewInt(12)})
	hashRandom, _ := iden3poseidon.Hash([]*big.Int{big.NewInt(21)})
	rVals := make([]*big.Int, v.k)
	rSum := big.NewInt(0)
	v.messageTags = make([]*big.Int, v.k)
	for i := 0; i < v.k; i++ {
		mt, err := iden3poseidon.Hash([]*big.Int{hashTag, v.secrets[i], v.blockNumber})
		if err != nil {
			t.Fatal(err)
		}
		v.messageTags[i] = m03ReduceModP(mt)
		rf, err := iden3poseidon.Hash([]*big.Int{hashRandom, v.secrets[i], v.blockNumber})
		if err != nil {
			t.Fatal(err)
		}
		rVals[i] = m03ReduceModP(rf)
		if i != v.senderIdx {
			rSum = m03ReduceModP(new(big.Int).Add(rSum, rVals[i]))
		}
	}
	v.txValues = make([]*big.Int, v.k)
	v.txRandom = make([]*big.Int, v.k)
	v.txCommit = make([][2]*big.Int, v.k)
	fps := make([][]*big.Int, v.k)
	for i := 0; i < v.k; i++ {
		fps[i] = append([]*big.Int(nil), v.fingerprints[i]...)
		if i == v.senderIdx {
			v.txValues[i] = big.NewInt(0) // a debit of 0: P - 0 reduces to 0
			v.txRandom[i] = rSum
		} else {
			v.txValues[i] = big.NewInt(0)
			v.txRandom[i] = m03NegMod(rVals[i])
			if republish {
				h, err := iden3poseidon.Hash([]*big.Int{v.secrets[i]})
				if err != nil {
					t.Fatal(err)
				}
				fps[i][v.senderIdx] = m03ReduceModP(h)
			}
		}
		pt := m03PedersenCommit(v.txValues[i], v.txRandom[i])
		v.txCommit[i] = [2]*big.Int{pt.X, pt.Y}
	}
	v.fingerprints = fps
	return v
}

func fabricatedSecrets(k int) []*big.Int {
	out := make([]*big.Int, k)
	for i := range out {
		out[i] = new(big.Int).SetUint64(0xdeadbeef00 + uint64(i))
	}
	return out
}

// The attack: a zero-value withdrawal with made-up secrets for every other
// bank, publishing the fingerprints the banks actually confirmed (the only
// ones Enygma.sol accepts). The circuit must reject it.
func TestFingerprint_FabricatedSecretsCannotPublishConfirmedFingerprints(t *testing.T) {
	honest := m03BuildValues(t, big.NewInt(424242), big.NewInt(67890), big.NewInt(500), big.NewInt(100), big.NewInt(3000))
	attack := withSecrets(t, honest, fabricatedSecrets(honest.k), false)
	if err := m03Solve(t, attack.circuit()); err == nil {
		t.Fatal("a withdrawal with made-up shared secrets solved while publishing the confirmed fingerprints")
	}
}

// The same withdrawal with the real secrets solves: the zero-value shape
// itself is legitimate, only the secrets were the problem.
func TestFingerprint_ZeroValueWithdrawWithRealSecretsSolves(t *testing.T) {
	honest := m03BuildValues(t, big.NewInt(424242), big.NewInt(67890), big.NewInt(500), big.NewInt(100), big.NewInt(3000))
	v := withSecrets(t, honest, honest.secrets, false)
	if err := m03Solve(t, v.circuit()); err != nil {
		t.Fatalf("zero-value withdrawal with the real secrets rejected: %v", err)
	}
}

// Made-up secrets solve only behind their own fingerprints, which are not
// the confirmed ones: that is the value Enygma.sol's
// _verifyFingerprintsDeposit compares and rejects (covered on chain by
// go_client/enygma_test/withdraw_deposit_fingerprint_test.go).
func TestFingerprint_FabricatedSecretsPublishDifferentFingerprints(t *testing.T) {
	honest := m03BuildValues(t, big.NewInt(424242), big.NewInt(67890), big.NewInt(500), big.NewInt(100), big.NewInt(3000))
	attack := withSecrets(t, honest, fabricatedSecrets(honest.k), true)
	if err := m03Solve(t, attack.circuit()); err != nil {
		t.Fatalf("attack witness with its own fingerprints did not solve: %v", err)
	}
	for i := 0; i < honest.k; i++ {
		if i == honest.senderIdx {
			continue
		}
		if attack.fingerprints[i][honest.senderIdx].Cmp(honest.fingerprints[i][honest.senderIdx]) == 0 {
			t.Fatalf("slot %d: made-up secret published the confirmed fingerprint", i)
		}
	}
}

// The nullifier's preimage used to be public (the sender's
// HashedSharedSecrets slot), so Poseidon(signal, BlockNumber) == Nullifier
// picked out the sender. No public signal may do that now.
func TestFingerprint_NullifierNotComputableFromPublicSignals(t *testing.T) {
	v := m03BuildValues(t, big.NewInt(424242), big.NewInt(67890), big.NewInt(500), big.NewInt(100), big.NewInt(3000))
	if err := m03Solve(t, v.circuit()); err != nil {
		t.Fatalf("honest witness rejected: %v", err)
	}
	var public []*big.Int
	for _, row := range v.fingerprints {
		public = append(public, row...)
	}
	public = append(public, v.pks...)
	public = append(public, v.anonymitySet...)
	public = append(public, v.messageTags...)
	for _, p := range v.prevCommit {
		public = append(public, p[0], p[1])
	}
	for _, p := range v.txCommit {
		public = append(public, p[0], p[1])
	}
	for _, x := range public {
		h, err := iden3poseidon.Hash([]*big.Int{x, v.blockNumber})
		if err != nil {
			t.Fatal(err)
		}
		if h.Cmp(v.nullifier) == 0 {
			t.Fatalf("Poseidon(%s, BlockNumber) equals the nullifier", x)
		}
	}
}
