package enygma_test

// TestH13BurnRejects* exercise Enygma.burn()'s own logic under the H-13
// fix — the contract-side binding checks around a burn proof — using
// MockBurnVerifier (always "verifies") so these tests are entirely about
// burn()'s public-signal binding and nullifier bookkeeping, independent of
// the real burn circuit's solvency/correctness soundness (which the mock
// cannot exercise by construction — a mock that always returns true cannot
// demonstrate a circuit rejecting an unsatisfiable witness). That half of
// H-13 — a witness claiming previousBalance < amount, or a wrapped
// near-P balance, has no satisfying witness at all — is proven directly
// against the real circuit in
// gnark-server/pkg/circuits/burn/h13_repro_test.go.
//
// Prerequisites:
//
//	export MY_KEY=<hex-private-key>   (or rely on the local Hardhat default)
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestH13 -v -timeout 60s

import (
	"math/big"
	"strings"
	"testing"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

func TestH13BurnRejectsMismatchedPreviousCommit(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}

	client, mkAuth, waitTx := scenarioClient(t)
	instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx)

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	mockBurnVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockBurnVerifier.sol/MockBurnVerifier.json")
	if r := waitTx(instance.AddBurnVerifier(mkAuth(), mockBurnVerifierAddr)); r.Status != 1 {
		t.Fatal("addBurnVerifier failed")
	}
	// Fix H-02 residual: senderMintR — senderRegR (freshSetup) +
	// senderMintR == senderPrevR, used directly below.
	mcx, mcy := mintCommitPt(big.NewInt(mintAmt), big.NewInt(senderMintR))
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1), mcx, mcy)); r.Status != 1 {
		t.Fatal("mintSupply failed")
	}

	sk := big.NewInt(senderSk)
	prevR := big.NewInt(senderPrevR)
	pkHash, _ := poseidon.Hash([]*big.Int{sk, sk})
	publicKey := pkHash.Mod(pkHash, curveP)

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	secretRemainHash, _ := poseidon.Hash([]*big.Int{prevR, sk})
	secretRemain := secretRemainHash.Mod(secretRemainHash, curveP)
	nullifier, _ := poseidon.Hash([]*big.Int{secretRemain, blockHash})

	// PreviousCommit deliberately wrong (an arbitrary point, not bank 0's
	// actual on-chain commitment) — the contract must catch this itself,
	// since the mock verifier does not. NewCommit reuses prevR as its
	// blinding factor (required — see scenario_test.go's comment); it's
	// moot for this specific test since the wrong PreviousCommit is caught
	// first, but keeping every proof in this file internally consistent
	// avoids leaving a construction that would itself be invalid.
	newCommit := pedersenCommitment(big.NewInt(300), prevR)
	burnProof := enygma.IEnygmaBurnProof{
		Proof: [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: [9]*big.Int{
			publicKey,
			big.NewInt(1), big.NewInt(1), // wrong PreviousCommit
			newCommit.X, newCommit.Y,
			big.NewInt(200),
			blockHash,
			nullifier,
			expectedDomainId(enygmaAddr), // Fix L-01
		},
	}

	_, sendErr := instance.Burn(mkAuth(), big.NewInt(1), burnProof)
	// Hardhat preflights the call before broadcasting, so a revert surfaces
	// as a send error naming the custom error directly (decoded text, e.g.
	// "reverted with custom error 'InvalidPublicInputs()'" — not the raw
	// selector, which is what earlier tests in this suite check for).
	const wantError = "InvalidPublicInputs"
	if sendErr == nil {
		t.Fatal("FAIL: burn() with a mismatched PreviousCommit was accepted")
	}
	if !strings.Contains(sendErr.Error(), wantError) {
		t.Fatalf("burn() reverted, but not with InvalidPublicInputs: %v", sendErr)
	}
	t.Logf("burn() with mismatched PreviousCommit correctly reverted with InvalidPublicInputs(): %v", sendErr)
}

func TestH13BurnNullifierReuseRejected(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}

	client, mkAuth, waitTx := scenarioClient(t)
	instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx)

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	mockBurnVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockBurnVerifier.sol/MockBurnVerifier.json")
	if r := waitTx(instance.AddBurnVerifier(mkAuth(), mockBurnVerifierAddr)); r.Status != 1 {
		t.Fatal("addBurnVerifier failed")
	}
	// Fix H-02 residual: senderMintR — senderRegR (freshSetup) +
	// senderMintR == senderPrevR, used directly below.
	mcx, mcy := mintCommitPt(big.NewInt(mintAmt), big.NewInt(senderMintR))
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1), mcx, mcy)); r.Status != 1 {
		t.Fatal("mintSupply failed")
	}

	prevBal, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("GetBalance(1): %v", err)
	}

	sk := big.NewInt(senderSk)
	prevR := big.NewInt(senderPrevR)
	pkHash, _ := poseidon.Hash([]*big.Int{sk, sk})
	publicKey := pkHash.Mod(pkHash, curveP)

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	secretRemainHash, _ := poseidon.Hash([]*big.Int{prevR, sk})
	secretRemain := secretRemainHash.Mod(secretRemainHash, curveP)
	nullifier, _ := poseidon.Hash([]*big.Int{secretRemain, blockHash})

	// NewCommit reuses prevR as its blinding factor — required for the
	// first burn below to actually succeed; see scenario_test.go's comment.
	newCommit := pedersenCommitment(big.NewInt(400), prevR)
	burnProof := enygma.IEnygmaBurnProof{
		Proof: [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: [9]*big.Int{
			publicKey,
			prevBal.X, prevBal.Y,
			newCommit.X, newCommit.Y,
			big.NewInt(100),
			blockHash,
			nullifier,
			expectedDomainId(enygmaAddr), // Fix L-01
		},
	}

	if r := waitTx(instance.Burn(mkAuth(), big.NewInt(1), burnProof)); r.Status != 1 {
		t.Fatal("first burn() reverted unexpectedly")
	}
	t.Log("first burn() with this proof succeeded")

	// Mint the same 100 back to bank 0 with r=0 (deliberately, unlike every
	// other mint in this suite — see epoch_test.go's identity-mint comment
	// for the same reasoning). mintSupply adds Com(100,0), and the burn
	// above left bank 0 at Com(400,prevR), so this restores the EXACT SAME
	// commitment point (Com(500,prevR)) bank 0 had when the proof above
	// was built — a nonzero r here would restore the right VALUE but a
	// DIFFERENT commitment point, and this test needs the exact original
	// point back: otherwise replaying the proof would be rejected on
	// InvalidPublicInputs (a stale PreviousCommit) before ever reaching
	// the nullifier check, which would not actually test nullifier reuse.
	ix, iy := mintCommitPt(big.NewInt(100), big.NewInt(0))
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(100), big.NewInt(1), ix, iy)); r.Status != 1 {
		t.Fatal("mint-back failed")
	}
	restoredBal, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("GetBalance(1) after mint-back: %v", err)
	}
	if restoredBal.X.Cmp(prevBal.X) != 0 || restoredBal.Y.Cmp(prevBal.Y) != 0 {
		t.Fatalf("mint-back did not restore the original commitment: got (%s,%s), want (%s,%s)",
			restoredBal.X, restoredBal.Y, prevBal.X, prevBal.Y)
	}

	// Replay the identical proof. PreviousCommit matches again (just
	// confirmed above), so the nullifier check is what rejects this.
	_, sendErr := instance.Burn(mkAuth(), big.NewInt(1), burnProof)
	const wantError = "NullifierAlreadyUsed"
	if sendErr == nil {
		t.Fatal("FAIL: replaying the same burn proof was accepted")
	}
	if !strings.Contains(sendErr.Error(), wantError) {
		t.Fatalf("replayed burn() reverted, but not with NullifierAlreadyUsed: %v", sendErr)
	}
	t.Logf("replayed burn() correctly reverted with NullifierAlreadyUsed(): %v", sendErr)
}
