package enygma_test

// transferWithFee applies a proof of the enygma_fee circuit, which proves a
// fixed DEFAULT_SIZE-slot update: Σ(TxCommit) + Fee·G == identity over all
// six slots. It used to check only the slots participantIds named, so a
// caller could submit a proof that conserves over six slots, apply only the
// slots that credit, and mint the debits away. It also never checked the
// shared-secret fingerprints transfer() checks (C-04), so the sender could
// shift any participant's commitment by a blinding factor only the sender
// knows.
//
// transferWithFee now requires DEFAULT_SIZE participants and commitment
// deltas, and the proof's k×k fingerprint matrix to equal the mutually
// confirmed fingerprints. That the circuit ties the sender's column to the
// secrets that move the blinding factors is covered by
// gnark-server/pkg/circuits/enygma_fee/fingerprint_binding_test.go.
//
// Uses MockFeeVerifier (always "verifies"): these are the contract's checks.
//
//	CC=/usr/bin/clang go test -run TestFeeTransferCoverage -v -timeout 600s

import (
	"math/big"
	"strings"
	"testing"

	enygma "enygma_payments/go_client/contracts"
	"enygma_payments/layout"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

func TestFeeTransferCoverage(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	const fee = 20

	zeroProof := func(sig [layout.TransferFeeLen]*big.Int) enygma.IEnygmaFeeProof {
		var p [8]*big.Int
		for i := range p {
			p[i] = big.NewInt(0)
		}
		return enygma.IEnygmaFeeProof{Proof: p, PublicSignal: sig}
	}
	wantRevert := func(t *testing.T, err error, want string) {
		t.Helper()
		if err == nil {
			t.Fatalf("transferWithFee was accepted, want a %s revert", want)
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("transferWithFee reverted, but not with %s: %v", want, err)
		}
	}
	setFee := func(t *testing.T, f m13Fixture) {
		t.Helper()
		if r := f.waitTx(f.instance.SetProtocolFee(f.mkAuth(), big.NewInt(fee))); r.Status != 1 {
			t.Fatal("setProtocolFee failed")
		}
	}

	t.Run("a proof applied to fewer participants than its slots is rejected", func(t *testing.T) {
		f := m13Setup(t)
		setFee(t, f)
		// Conserves over six slots: bank 0 is credited 100, bank 5 pays
		// 100 + fee. Applying bank 0's slot alone would mint 100.
		sig, deltas := f.buildFeeSignal(fee, "arity")
		credit := pedersenCommitment(big.NewInt(100), big.NewInt(0))
		debit := pedersenCommitment(new(big.Int).Sub(curveP, big.NewInt(100+fee)), big.NewInt(0))
		for i, pt := range map[int]struct{ x, y *big.Int }{0: {credit.X, credit.Y}, 5: {debit.X, debit.Y}} {
			sig[layout.TransferFeeTxCommitmentsOffset+2*i] = pt.x
			sig[layout.TransferFeeTxCommitmentsOffset+2*i+1] = pt.y
			deltas[i] = enygma.IEnygmaPoint{C1: pt.x, C2: pt.y}
		}
		_, err := f.instance.TransferWithFee(f.mkAuth(), deltas[:1], zeroProof(sig), participantIds6()[:1], "")
		wantRevert(t, err, "InvalidParticipantCount")
	})

	t.Run("fewer commitment deltas than participants are rejected", func(t *testing.T) {
		f := m13Setup(t)
		setFee(t, f)
		sig, deltas := f.buildFeeSignal(fee, "deltas")
		_, err := f.instance.TransferWithFee(f.mkAuth(), deltas[:nBanks-1], zeroProof(sig), participantIds6(), "")
		wantRevert(t, err, "ParticipantIdsLengthMismatch")
	})

	t.Run("no confirmed fingerprints: the transfer is rejected", func(t *testing.T) {
		f := m13Deploy(t)
		setFee(t, f)
		sig, deltas := f.buildFeeSignal(fee, "unconfirmed")
		_, err := f.instance.TransferWithFee(f.mkAuth(), deltas, zeroProof(sig), participantIds6(), "")
		wantRevert(t, err, "FingerprintNotConfirmed")
	})

	t.Run("a sender column other than the confirmed fingerprints is rejected", func(t *testing.T) {
		f := m13Setup(t)
		setFee(t, f)
		f.fingerprints = fabricatedColumn(t, f.fingerprints)
		sig, deltas := f.buildFeeSignal(fee, "fabricated")
		_, err := f.instance.TransferWithFee(f.mkAuth(), deltas, zeroProof(sig), participantIds6(), "")
		wantRevert(t, err, "InvalidPublicInputs")
	})

	t.Run("the confirmed fingerprints settle and keep the supply invariant", func(t *testing.T) {
		f := m13Setup(t)
		setFee(t, f)
		supplyBefore, err := f.instance.TotalSupply(&bind.CallOpts{})
		if err != nil {
			t.Fatalf("TotalSupply: %v", err)
		}
		sig, deltas := f.buildFeeSignal(fee, "honest")
		if r := f.waitTx(f.instance.TransferWithFee(f.mkAuth(), deltas, zeroProof(sig), participantIds6(), "")); r.Status != 1 {
			t.Fatal("fee transfer with the confirmed fingerprints reverted")
		}
		supplyAfter, err := f.instance.TotalSupply(&bind.CallOpts{})
		if err != nil {
			t.Fatalf("TotalSupply: %v", err)
		}
		if want := new(big.Int).Sub(supplyBefore, big.NewInt(fee)); supplyAfter.Cmp(want) != 0 {
			t.Fatalf("totalSupply = %s, want %s", supplyAfter, want)
		}
		ok, err := f.instance.Check(&bind.CallOpts{})
		if err != nil {
			t.Fatalf("check(): %v", err)
		}
		if !ok {
			t.Fatal("check() is false after a fee transfer")
		}
	})
}
