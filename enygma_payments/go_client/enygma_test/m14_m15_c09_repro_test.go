package enygma_test

// TestM14_*, TestM15_*, TestC09_* reproduce and verify the M-14, M-15 and
// C-09 fixes to the zkDvp bridge (deposit()/withdraw()) in
// Enygma.sol (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md):
//
//   M-14 — deposit's true public-signal arity is 51 (HashedSharedSecrets..
//     Nullifier at 0-49, Hash — the deposit note commitment — at 50), but
//     IEnygma.DepositProof declared [50] and the verifier call site
//     encoded verifyProof(uint256[8],uint256[50]) — a selector mismatch
//     against every deployed DepositVerifier.sol, so deposit() reverted
//     InvalidProof unconditionally regardless of proof validity.
//   M-15 — deposit and withdraw moved value in the wrong (and mutually
//     inverted) directions: a deposit debited the depositor instead of
//     crediting them (paying twice — once via the redeemed DvP note,
//     once via the shielded debit), and a withdrawal credited the
//     withdrawer instead of debiting them. Neither touched totalSupply at
//     all, so check()'s Σ(balances)==totalSupply invariant went
//     permanently false on the first bridge operation either way.
//   C-09 — withdraw()'s depositParams (forwarded verbatim to the DvP
//     vault) had no relationship to the proof's own shielded debit at
//     all: Σ VPerDeposit was never bound to SenderTxValue anywhere,
//     so a proof could debit an arbitrarily small (or zero) shielded
//     amount while depositParams minted an arbitrarily large DvP note.
//
// Uses MockDepositVerifier/MockWithdrawVerifier (always "verifies") and
// MockZkDvp, so these tests are entirely about Enygma.sol's own
// arity/binding/supply-accounting logic — independent of the real
// deposit/withdraw circuits' soundness, whose live prover route is
// separately broken today (M-03, pre-existing). The circuit-level half of
// M-15/C-09 (that the circuit itself enforces the right direction and
// binding) is verified directly against the real circuits in
// gnark-server/pkg/circuits/{deposit,withdraw}/m15*_repro_test.go.
//
// Prerequisites:
//
//	export MY_KEY=<hex-private-key>   (or rely on the local Hardhat default)
//
// Run:
//
//	CC=/usr/bin/clang go test -run "TestM14|TestM15|TestC09" -v -timeout 60s

import (
	"math/big"
	"strings"
	"testing"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iden3/go-iden3-crypto/babyjub"
)

func TestM14_DepositWithArity51Succeeds(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mkAuth, waitTx := scenarioClient(t)
	instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx)

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	mockDepositVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockDepositVerifier.sol/MockDepositVerifier.json")
	if r := waitTx(instance.AddDepositVerifier(mkAuth(), mockDepositVerifierAddr)); r.Status != 1 {
		t.Fatal("addDepositVerifier failed")
	}
	mockZkDvpAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockZkDvp.sol/MockZkDvp.json")
	if r := waitTx(instance.AddZkDvp(mkAuth(), mockZkDvpAddr)); r.Status != 1 {
		t.Fatal("addZkDvp failed")
	}

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("getPublicValues: %v", err)
	}
	keys6, balances6 := pubVals.Keys[1:], pubVals.Balances[1:]

	// A zero-delta deposit (every participant's TxCommit is the neutral
	// element) — this test is purely about whether the [51]-arity call
	// reaches and passes verification at all (Fix M-14), not about the
	// credit direction (Fix M-15, covered separately below).
	var pubSig [52]*big.Int
	for i := range pubSig {
		pubSig[i] = big.NewInt(0)
	}
	commitmentDeltas := make([]enygma.IEnygmaPoint, nBanks)
	participantIds := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		pubSig[6+i] = keys6[i]
		pubSig[12+2*i] = balances6[i].C1
		pubSig[12+2*i+1] = balances6[i].C2
		pubSig[24+2*i] = big.NewInt(0)
		pubSig[24+2*i+1] = big.NewInt(1)
		commitmentDeltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
		participantIds[i] = big.NewInt(int64(i + 1))
	}
	pubSig[36] = blockHash
	pubSig[49] = deterministicNullifier(t, "m14-arity")
	pubSig[50] = big.NewInt(424242) // Hash — unbound by the contract (documented C-09 deposit-side gap), any value accepted
	pubSig[51] = expectedDomainId(enygmaAddr) // Fix L-01

	proof := enygma.IEnygmaDepositProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}
	withdrawParam := emptyWithdrawParams()

	if r := waitTx(instance.Deposit(mkAuth(), commitmentDeltas, proof, withdrawParam, participantIds)); r.Status != 1 {
		t.Fatal("FAIL (M-14 regressed): deposit() with a correctly-shaped [51]-signal proof reverted")
	}
	t.Log("deposit() with [51]-arity public_signal succeeded — M-14's arity mismatch is fixed")
}

func TestM15_DepositCreditsSenderAndTotalSupply(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mkAuth, waitTx := scenarioClient(t)
	instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx)

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	mockDepositVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockDepositVerifier.sol/MockDepositVerifier.json")
	if r := waitTx(instance.AddDepositVerifier(mkAuth(), mockDepositVerifierAddr)); r.Status != 1 {
		t.Fatal("addDepositVerifier failed")
	}
	mockZkDvpAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockZkDvp.sol/MockZkDvp.json")
	if r := waitTx(instance.AddZkDvp(mkAuth(), mockZkDvpAddr)); r.Status != 1 {
		t.Fatal("addZkDvp failed")
	}

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("getPublicValues: %v", err)
	}
	keys6, balances6 := pubVals.Keys[1:], pubVals.Balances[1:]

	supplyBefore, err := instance.TotalSupply(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalSupply before: %v", err)
	}
	checkBefore, err := instance.Check(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("check() before: %v", err)
	}
	if !checkBefore {
		t.Fatal("test setup bug: check() already false before any deposit")
	}

	const depositAmount = 250
	creditCommit := pedersenCommitment(big.NewInt(depositAmount), big.NewInt(0))

	var pubSig [52]*big.Int
	for i := range pubSig {
		pubSig[i] = big.NewInt(0)
	}
	commitmentDeltas := make([]enygma.IEnygmaPoint, nBanks)
	participantIds := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		pubSig[6+i] = keys6[i]
		pubSig[12+2*i] = balances6[i].C1
		pubSig[12+2*i+1] = balances6[i].C2
		if i == 0 {
			pubSig[24+2*i] = creditCommit.X
			pubSig[24+2*i+1] = creditCommit.Y
			commitmentDeltas[i] = enygma.IEnygmaPoint{C1: creditCommit.X, C2: creditCommit.Y}
		} else {
			pubSig[24+2*i] = big.NewInt(0)
			pubSig[24+2*i+1] = big.NewInt(1)
			commitmentDeltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
		}
		participantIds[i] = big.NewInt(int64(i + 1))
	}
	pubSig[36] = blockHash
	pubSig[49] = deterministicNullifier(t, "m15-deposit-credit")
	pubSig[50] = big.NewInt(1) // Hash — unbound (documented C-09 deposit-side gap)
	pubSig[51] = expectedDomainId(enygmaAddr) // Fix L-01

	proof := enygma.IEnygmaDepositProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}
	if r := waitTx(instance.Deposit(mkAuth(), commitmentDeltas, proof, emptyWithdrawParams(), participantIds)); r.Status != 1 {
		t.Fatal("honest deposit reverted — setup problem, not what this test is checking")
	}

	supplyAfter, err := instance.TotalSupply(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalSupply after: %v", err)
	}
	// deposit() has no plaintext-amount signal (SenderTxValue stays
	// private — see Enygma.sol's comment on _applySupplyDelta), so
	// totalSupplyAmount (the plaintext mirror) is deliberately not
	// touched by deposit/withdraw; only the POINT (which check() actually
	// verifies) is. Confirm the mirror is unchanged...
	if supplyAfter.Cmp(supplyBefore) != 0 {
		t.Errorf("totalSupplyAmount (plaintext mirror) changed from %s to %s — deposit()/withdraw() are documented as not touching it", supplyBefore, supplyAfter)
	}
	// ...and confirm the POINT-based invariant — the one that actually
	// matters — holds, which is only possible if totalSupplyX/Y credited
	// by exactly the same commitmentDeltas sum applied to balances.
	checkAfter, err := instance.Check(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("check() after: %v", err)
	}
	if !checkAfter {
		t.Fatal("FAIL (M-15 regressed): check() invariant broken after an honest deposit — totalSupply did not credit alongside the balance")
	}
	t.Log("check() PASSED after deposit — totalSupply credited alongside the balance ✓ (Fix M-15)")

	balAfter, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("getBalance(1): %v", err)
	}
	// balAfter must be balBefore homomorphically ADDED to creditCommit
	// (Enygma writes the proof's TxCommit delta verbatim, then the
	// contract's own point-addition applies it — see
	// _updateBalancesForTransfer's doc) — not creditCommit alone.
	expected := addBJPoints(
		&babyjub.Point{X: balances6[0].C1, Y: balances6[0].C2},
		&babyjub.Point{X: creditCommit.X, Y: creditCommit.Y},
	)
	if balAfter.X.Cmp(expected.X) != 0 || balAfter.Y.Cmp(expected.Y) != 0 {
		t.Fatalf("bank 0's balance = (%s,%s), want balBefore+creditCommit = (%s,%s)", balAfter.X, balAfter.Y, expected.X, expected.Y)
	}
	t.Log("bank 0's balance was credited (not debited) by the deposit ✓")
}

func TestC09_WithdrawRejectsMismatchedDepositValue(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mkAuth, waitTx := scenarioClient(t)
	instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx)

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	mockWithdrawVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
	if r := waitTx(instance.AddWithdrawVerifier(mkAuth(), mockWithdrawVerifierAddr, big.NewInt(nBanks))); r.Status != 1 {
		t.Fatal("addWithdrawVerifier failed")
	}
	mockZkDvpAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockZkDvp.sol/MockZkDvp.json")
	if r := waitTx(instance.AddZkDvp(mkAuth(), mockZkDvpAddr)); r.Status != 1 {
		t.Fatal("addZkDvp failed")
	}

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("getPublicValues: %v", err)
	}
	keys6, balances6 := pubVals.Keys[1:], pubVals.Balances[1:]

	// The proof debits SenderTxValue=1 (TotalDepositValue signal = 1),
	// but depositParams claims a DvP note worth 1,000,000 — the exact
	// "unlimited unbacked value creation" construction C-09 describes.
	const claimedDebit = 1
	const claimedNoteValue = 1_000_000
	debitCommit := pedersenCommitment(new(big.Int).Sub(curveP, big.NewInt(claimedDebit)), big.NewInt(0))

	var pubSig [52]*big.Int
	for i := range pubSig {
		pubSig[i] = big.NewInt(0)
	}
	commitmentDeltas := make([]enygma.IEnygmaPoint, nBanks)
	participantIds := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		pubSig[6+i] = keys6[i]
		pubSig[12+2*i] = balances6[i].C1
		pubSig[12+2*i+1] = balances6[i].C2
		if i == 0 {
			pubSig[24+2*i] = debitCommit.X
			pubSig[24+2*i+1] = debitCommit.Y
			commitmentDeltas[i] = enygma.IEnygmaPoint{C1: debitCommit.X, C2: debitCommit.Y}
		} else {
			pubSig[24+2*i] = big.NewInt(0)
			pubSig[24+2*i+1] = big.NewInt(1)
			commitmentDeltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
		}
		participantIds[i] = big.NewInt(int64(i + 1))
	}
	pubSig[36] = blockHash
	pubSig[49] = deterministicNullifier(t, "c09-mismatch")
	pubSig[50] = big.NewInt(claimedDebit) // TotalDepositValue: proof claims only 1
	pubSig[51] = expectedDomainId(enygmaAddr) // Fix L-01

	proof := enygma.IEnygmaWithdrawProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}
	depositParams := []enygma.IEnygmaDepositParams{
		{Amount: big.NewInt(claimedNoteValue), Erc20Adress: common.Address{}, PublicKey: big.NewInt(1)},
	}

	_, sendErr := instance.Withdraw(mkAuth(), commitmentDeltas, proof, depositParams, participantIds)
	if sendErr == nil {
		t.Fatal("FAIL (C-09 regressed): withdraw() debiting 1 while depositParams claims a 1,000,000-value DvP note was accepted")
	}
	if !strings.Contains(sendErr.Error(), "DepositValueMismatch") {
		t.Fatalf("withdraw() with mismatched deposit value reverted, but not with DepositValueMismatch: %v", sendErr)
	}
	t.Logf("withdraw() correctly rejected a shielded-debit / DvP-note-value mismatch: %v", sendErr)
}

func TestM15_WithdrawDebitsSenderAndTotalSupply(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mkAuth, waitTx := scenarioClient(t)
	instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx)

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	mockWithdrawVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
	if r := waitTx(instance.AddWithdrawVerifier(mkAuth(), mockWithdrawVerifierAddr, big.NewInt(nBanks))); r.Status != 1 {
		t.Fatal("addWithdrawVerifier failed")
	}
	mockZkDvpAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockZkDvp.sol/MockZkDvp.json")
	if r := waitTx(instance.AddZkDvp(mkAuth(), mockZkDvpAddr)); r.Status != 1 {
		t.Fatal("addZkDvp failed")
	}

	// Mint first so bank 0 has a real, non-identity balance to withdraw from.
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
	keys6, balances6 := pubVals.Keys[1:], pubVals.Balances[1:]

	supplyBefore, err := instance.TotalSupply(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalSupply before: %v", err)
	}
	checkBefore, err := instance.Check(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("check() before: %v", err)
	}
	if !checkBefore {
		t.Fatal("test setup bug: check() already false before any withdraw")
	}

	const withdrawAmount = 300
	debitCommit := pedersenCommitment(new(big.Int).Sub(curveP, big.NewInt(withdrawAmount)), big.NewInt(0))

	var pubSig [52]*big.Int
	for i := range pubSig {
		pubSig[i] = big.NewInt(0)
	}
	commitmentDeltas := make([]enygma.IEnygmaPoint, nBanks)
	participantIds := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		pubSig[6+i] = keys6[i]
		pubSig[12+2*i] = balances6[i].C1
		pubSig[12+2*i+1] = balances6[i].C2
		if i == 0 {
			pubSig[24+2*i] = debitCommit.X
			pubSig[24+2*i+1] = debitCommit.Y
			commitmentDeltas[i] = enygma.IEnygmaPoint{C1: debitCommit.X, C2: debitCommit.Y}
		} else {
			pubSig[24+2*i] = big.NewInt(0)
			pubSig[24+2*i+1] = big.NewInt(1)
			commitmentDeltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
		}
		participantIds[i] = big.NewInt(int64(i + 1))
	}
	pubSig[36] = blockHash
	pubSig[49] = deterministicNullifier(t, "m15-withdraw-debit")
	pubSig[50] = big.NewInt(withdrawAmount) // TotalDepositValue: matches Σ depositParams below (Fix C-09)
	pubSig[51] = expectedDomainId(enygmaAddr) // Fix L-01

	proof := enygma.IEnygmaWithdrawProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: pubSig,
	}
	depositParams := []enygma.IEnygmaDepositParams{
		{Amount: big.NewInt(withdrawAmount), Erc20Adress: common.Address{}, PublicKey: big.NewInt(1)},
	}

	if r := waitTx(instance.Withdraw(mkAuth(), commitmentDeltas, proof, depositParams, participantIds)); r.Status != 1 {
		t.Fatal("honest withdraw (Σ depositParams matches TotalDepositValue) reverted — setup problem, not what this test is checking")
	}

	supplyAfter, err := instance.TotalSupply(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalSupply after: %v", err)
	}
	if supplyAfter.Cmp(supplyBefore) != 0 {
		t.Errorf("totalSupplyAmount (plaintext mirror) changed from %s to %s — deposit()/withdraw() are documented as not touching it", supplyBefore, supplyAfter)
	}
	checkAfter, err := instance.Check(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("check() after: %v", err)
	}
	if !checkAfter {
		t.Fatal("FAIL (M-15 regressed): check() invariant broken after an honest withdraw — totalSupply did not debit alongside the balance")
	}
	t.Log("check() PASSED after withdraw — totalSupply debited alongside the balance ✓ (Fix M-15)")
}

// ── shared helpers ────────────────────────────────────────────────────────────

// deterministicNullifier returns a distinct, deterministic nullifier per
// test/seed so repeated runs against the same fresh deployment (each test
// deploys its own instance via freshSetup, so collisions across tests
// aren't actually possible, but this keeps every call site self-describing).
func deterministicNullifier(t *testing.T, seed string) *big.Int {
	t.Helper()
	return new(big.Int).SetBytes(crypto.Keccak256([]byte(seed)))
}

// emptyWithdrawParams is a zero-value IEnygmaWithdrawParams — deposit()
// forwards it verbatim to MockZkDvp.withdrawThroughEnygma, which accepts
// any JoinSplitTransaction shape and returns true.
func emptyWithdrawParams() enygma.IEnygmaWithdrawParams {
	return enygma.IEnygmaWithdrawParams{
		Transaction: enygma.IZkDvpJoinSplitTransaction{
			Proof: enygma.IZkDvpSnarkProof{
				A: enygma.IZkDvpG1Point{X: big.NewInt(0), Y: big.NewInt(0)},
				B: enygma.IZkDvpG2Point{X: [2]*big.Int{big.NewInt(0), big.NewInt(0)}, Y: [2]*big.Int{big.NewInt(0), big.NewInt(0)}},
				C: enygma.IZkDvpG1Point{X: big.NewInt(0), Y: big.NewInt(0)},
			},
			Statement:       []*big.Int{},
			NumberOfInputs:  big.NewInt(0),
			NumberOfOutputs: big.NewInt(0),
		},
	}
}
