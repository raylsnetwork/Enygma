package enygma_test

// TestM13_* reproduces and verifies the M-13 fix (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md,
// Medium/LATENT): transferWithFee's public_signal[50] (the fee) was never
// read or accounted for anywhere on chain. The enygma_fee circuit
// hard-asserts Σ(TxCommit) + Fee·G == identity — i.e. the participants'
// deltas alone always sum to -Fee·G, not to the identity a fee-less
// transfer requires — so value unconditionally left the accounted pool on
// every fee transfer, permanently and silently breaking check()'s
// Σ(balances)==totalSupply invariant with no way for an off-chain observer
// to even compute the missing amount (it's a curve-point offset). Nothing
// enforced the fee's value either: a prover could set fee=0 for a free
// transfer.
//
// The fix requires public_signal[50] to exactly match a new owner-set
// protocolFee (mandatory, not advisory) and burns it out of totalSupply —
// homomorphically subtracting exactly what the circuit's own Pedersen
// equation already forces the balance sum to lose, keeping check()
// consistent instead of permanently false.
//
// Uses MockFeeVerifier (always "verifies") so these tests are entirely
// about transferWithFee's own fee-accounting logic, independent of the
// real enygma_fee circuit's soundness — whose live prover route is
// separately broken today ("proof generation failed: len(points) !=
// len(scalars)", confirmed via git stash to predate this fix and any
// other change in this session) and is out of scope for M-13, which is a
// contract-side accounting gap, not a circuit bug. TestFeeTransferFlow
// (fee_transfer_test.go) exercises the real circuit end-to-end and will
// cover this fix live too, once that separate prover bug is fixed.
//
// Prerequisites:
//
//	export MY_KEY=<hex-private-key>   (or rely on the local Hardhat default)
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestM13 -v -timeout 60s

import (
	"math/big"
	"strings"
	"testing"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// m13Fixture bundles everything TestM13_* needs to submit a
// transferWithFee call: the bound instance, its owner-key auth/wait
// helpers, and the on-chain keys/balances/block hash every participant
// slot in a public_signal must match exactly.
type m13Fixture struct {
	instance   *enygma.Enygma
	enygmaAddr common.Address
	mkAuth     func() *bind.TransactOpts
	waitTx     func(*ethtypes.Transaction, error) *ethtypes.Receipt
	keys6      []*big.Int
	balances6  []enygma.IEnygmaPoint
	blockHash  *big.Int
}

// m13Setup deploys a fresh Enygma + MockFeeVerifier, registers the 6
// DEFAULT_SIZE banks (via freshSetup), mints mintAmt to bank 0, and
// returns everything needed to build a well-formed transferWithFee call.
func m13Setup(t *testing.T) m13Fixture {
	t.Helper()
	client, mkAuth, waitTx := scenarioClient(t)
	instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx) // registers accountIds 1..6, real transfer verifier wired

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	mockFeeVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockFeeVerifier.sol/MockFeeVerifier.json")
	if r := waitTx(instance.AddFeeVerifier(mkAuth(), mockFeeVerifierAddr)); r.Status != 1 {
		t.Fatal("addFeeVerifier failed")
	}

	mcx, mcy := mintCommitPt(big.NewInt(mintAmt), big.NewInt(senderMintR))
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1), mcx, mcy)); r.Status != 1 {
		t.Fatal("mintSupply failed")
	}

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("getPublicValues: %v", err)
	}

	return m13Fixture{
		instance:   instance,
		enygmaAddr: enygmaAddr,
		mkAuth:     mkAuth,
		waitTx:     waitTx,
		keys6:      pubVals.Keys[1:],
		balances6:  pubVals.Balances[1:],
		blockHash:  blockHash,
	}
}

// buildFeeSignal builds a 55-signal transferWithFee public_signal array:
// bank 0's delta is Com(-fee, 0) (matching the circuit's own convention —
// no on-chain arithmetic on a secret value, the point is asserted
// verbatim); every other bank's delta is the neutral element (0,1). Σ =
// -fee·G, exactly what the real circuit's Pedersen equation would force,
// so a well-formed fee (matching protocolFee) leaves check() intact after
// _burnFeeFromSupply runs. Slot 54 is the Fix L-01 domain separator.
func (f m13Fixture) buildFeeSignal(fee int64, nullifierSeed string) ([55]*big.Int, []enygma.IEnygmaPoint) {
	var pubSig [55]*big.Int
	for i := range pubSig {
		pubSig[i] = big.NewInt(0)
	}
	commitmentDeltas := make([]enygma.IEnygmaPoint, nBanks)

	negFeeCommit := pedersenCommitment(new(big.Int).Sub(curveP, big.NewInt(fee)), big.NewInt(0))

	for i := 0; i < nBanks; i++ {
		pubSig[6+i] = f.keys6[i]
		pubSig[12+2*i] = f.balances6[i].C1
		pubSig[12+2*i+1] = f.balances6[i].C2
		if i == 0 {
			pubSig[24+2*i] = negFeeCommit.X
			pubSig[24+2*i+1] = negFeeCommit.Y
			commitmentDeltas[i] = enygma.IEnygmaPoint{C1: negFeeCommit.X, C2: negFeeCommit.Y}
		} else {
			pubSig[24+2*i] = big.NewInt(0)
			pubSig[24+2*i+1] = big.NewInt(1)
			commitmentDeltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
		}
	}
	pubSig[36] = f.blockHash
	pubSig[49] = new(big.Int).SetBytes(crypto.Keccak256([]byte("m13-" + nullifierSeed)))
	pubSig[50] = big.NewInt(fee)
	// 51-52 (SumTxCommit) and 53 (SumTxValuesWithFee) are unread by the
	// contract (that's the whole point of M-13 — see FEE_OFFSET's doc in
	// Enygma.sol) so they're left at 0 deliberately.
	pubSig[54] = expectedDomainId(f.enygmaAddr) // Fix L-01
	return pubSig, commitmentDeltas
}

func participantIds6() []*big.Int {
	ids := make([]*big.Int, nBanks)
	for i := range ids {
		ids[i] = big.NewInt(int64(i + 1))
	}
	return ids
}

func TestM13_MismatchedFeeRejected(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	f := m13Setup(t)

	if r := f.waitTx(f.instance.SetProtocolFee(f.mkAuth(), big.NewInt(20))); r.Status != 1 {
		t.Fatal("setProtocolFee failed")
	}

	// Proof claims fee=5, but protocolFee is 20 — must be rejected.
	pubSig, deltas := f.buildFeeSignal(5, "mismatch")
	proof := enygma.IEnygmaFeeProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	_, sendErr := f.instance.TransferWithFee(f.mkAuth(), deltas, proof, participantIds6(), "") // Fix H-09: no attribution for a direct test call
	if sendErr == nil {
		t.Fatal("FAIL (M-13 regressed): transferWithFee with fee=5 (protocolFee=20) was accepted")
	}
	if !strings.Contains(sendErr.Error(), "InvalidFee") {
		t.Fatalf("transferWithFee with mismatched fee reverted, but not with InvalidFee: %v", sendErr)
	}
	t.Logf("transferWithFee correctly rejected a fee not matching protocolFee: %v", sendErr)
}

func TestM13_MatchingFeeBurnsFromSupplyAndKeepsCheckValid(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	f := m13Setup(t)

	const fee = 20
	if r := f.waitTx(f.instance.SetProtocolFee(f.mkAuth(), big.NewInt(fee))); r.Status != 1 {
		t.Fatal("setProtocolFee failed")
	}

	supplyBefore, err := f.instance.TotalSupply(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalSupply before: %v", err)
	}
	checkBefore, err := f.instance.Check(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("check() before: %v", err)
	}
	if !checkBefore {
		t.Fatal("test setup bug: check() already false before any fee transfer")
	}

	pubSig, deltas := f.buildFeeSignal(fee, "match")
	proof := enygma.IEnygmaFeeProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	// Fix H-09: no attribution for a direct test call.
	if r := f.waitTx(f.instance.TransferWithFee(f.mkAuth(), deltas, proof, participantIds6(), "")); r.Status != 1 {
		t.Fatal("honest fee transfer (fee matches protocolFee) reverted — setup problem, not what this test is checking")
	}

	supplyAfter, err := f.instance.TotalSupply(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalSupply after: %v", err)
	}
	wantSupply := new(big.Int).Sub(supplyBefore, big.NewInt(fee))
	if supplyAfter.Cmp(wantSupply) != 0 {
		t.Fatalf("FAIL (M-13 regressed): totalSupply = %s, want %s (%s minted − %d fee)",
			supplyAfter, wantSupply, supplyBefore, fee)
	}
	t.Logf("totalSupply correctly decremented by fee: %s → %s ✓", supplyBefore, supplyAfter)

	checkAfter, err := f.instance.Check(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("check() after: %v", err)
	}
	if !checkAfter {
		t.Fatal("FAIL (M-13 regressed): check() invariant broken after a fee transfer")
	}
	t.Log("check() PASSED after fee transfer — Σ(balances)==totalSupply invariant maintained ✓")
}

func TestM13_ZeroFeeAllowedWhenProtocolFeeIsZero(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	f := m13Setup(t)
	// protocolFee defaults to 0 — deliberately not calling SetProtocolFee.

	pubSig, deltas := f.buildFeeSignal(0, "zero")
	proof := enygma.IEnygmaFeeProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}

	// Fix H-09: no attribution for a direct test call.
	if r := f.waitTx(f.instance.TransferWithFee(f.mkAuth(), deltas, proof, participantIds6(), "")); r.Status != 1 {
		t.Fatal("fee=0 transfer with default protocolFee=0 reverted — should succeed")
	}
	t.Log("fee=0 transfer succeeds against the default protocolFee=0 ✓ (no fee required until owner opts in)")
}
