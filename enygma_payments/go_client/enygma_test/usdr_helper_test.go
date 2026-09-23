package enygma_test

// Shared USDr-leg helper for the pre-existing *_repro_test.go files
// (c04/c05/h03/h07/h09/h09item2), none of which test anything USDr-related
// — they test fingerprints, duplicate participant ids, epoch rollover,
// account-id-zero rejection, and bank self-submission. Enygma.transfer()
// now unconditionally requires a second, independent USDr proof settled
// atomically alongside the main one, so these otherwise-unrelated tests
// need a USDr leg just to reach the code path they're actually testing.
//
// Rather than live-proving a real USDr circuit witness for every call site
// (the scenario_test.go/sequential_transfer_test.go pattern — necessary
// there because those tests exercise the real USDr flow end to end, ~30s
// per proof via a live gnark server), these tests register a
// MockUsdrVerifier (contracts/mocks/MockUsdrVerifier.sol — always returns
// true, mirroring the pre-existing MockTransferVerifier convention) and
// build a public_signal that satisfies Enygma.sol's own on-chain checks
// directly: _verifyUsdrMainBinding (PublicKey/AnonymitySet/BlockNumber
// copied verbatim from the main proof's signal, so they trivially match
// regardless of what that signal actually encodes), _verifyPublicInputsUsdr
// (FeeAmount/DomainId set correctly; PreviousCommit read live from
// GetUsdrBalance per participant so it always matches on-chain state;
// TxCommit left at the neutral element (0,1) for every position, with
// matching neutral commitmentDeltas — a homomorphic no-op that trivially
// keeps checkUsdr()'s invariant intact), and _verifyBlockNumberUsdr (copied
// from the main signal, same as above).
//
// This does not attempt to make the USDr leg cryptographically meaningful
// — it only needs to satisfy the contract's structural checks, which is
// all any of these six tests' own assertions depend on.

import (
	"math/big"
	"sync/atomic"
	"testing"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Public signal offsets shared with Enygma.sol's FP_*/USDR_* constants —
// see that file's own comment block for the authoritative layout.
const (
	usdrHelperPublicKeyOffset = 36
	usdrHelperPublicKeySize   = 6
	usdrHelperPrevCommitOff   = 42
	usdrHelperTxCommitOff     = 54
	usdrHelperBlockNumberOff  = 66
	usdrHelperKIndexOffset    = 67
	usdrHelperKIndexSize      = 6
	usdrHelperNullifierOff    = 79
	usdrHelperFeeAmountOff    = 80
	usdrHelperDomainOff       = 81
)

// usdrHelperNullifierSeq guarantees a fresh, never-reused USDr nullifier
// across every call within (and across) these tests' process lifetime —
// the mock verifier means the nullifier need not derive from anything
// cryptographically real, only be distinct.
var usdrHelperNullifierSeq int64

// setupMockUsdr deploys and registers a MockUsdrVerifier, sets a fixed
// USDr fee, and calls initializeUsdrBalance for every account id in
// participantAccountIds — the one-time prerequisites transfer()'s USDr
// leg needs before any USDr proof involving those accounts will pass
// Enygma.sol's own checks. Safe to call once per fresh contract instance.
func setupMockUsdr(
	t *testing.T,
	client *ethclient.Client,
	mkAuth func() *bind.TransactOpts,
	waitTx func(*ethtypes.Transaction, error) *ethtypes.Receipt,
	instance *enygma.Enygma,
	participantAccountIds []int64,
) {
	t.Helper()

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	mockUsdrVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/mocks/MockUsdrVerifier.sol/MockUsdrVerifier.json")

	if r := waitTx(instance.AddUsdrVerifier(mkAuth(), mockUsdrVerifierAddr)); r.Status != 1 {
		t.Fatal("addUsdrVerifier failed")
	}
	if r := waitTx(instance.SetUsdrFixedFee(mkAuth(), big.NewInt(usdrFeeAmt))); r.Status != 1 {
		t.Fatal("setUsdrFixedFee failed")
	}
	usdrCx, usdrCy := regCommit(big.NewInt(usdrPrevR))
	for _, id := range participantAccountIds {
		if r := waitTx(instance.InitializeUsdrBalance(mkAuth(), big.NewInt(id), usdrCx, usdrCy)); r.Status != 1 {
			t.Fatalf("initializeUsdrBalance(accountId=%d) failed", id)
		}
	}
	t.Logf("mock USDr leg configured: verifier=%s fee=%d, %d accounts initialized",
		mockUsdrVerifierAddr.Hex(), usdrFeeAmt, len(participantAccountIds))
}

// buildMockUsdrLeg builds a structurally-valid (not cryptographically
// real) USDr leg for one transfer() call. mainSignal is whatever 81-signal
// public_signal array the caller already built for the main proof —
// PublicKey/AnonymitySet/BlockNumber are copied from it verbatim so
// _verifyUsdrMainBinding always passes, whatever those fields actually
// encode. participantAccountIds must be the same accountIds (same order)
// passed as transfer()'s participantIds argument.
func buildMockUsdrLeg(
	t *testing.T,
	instance *enygma.Enygma,
	enygmaAddr common.Address,
	mainSignal [81]*big.Int,
	participantAccountIds []int64,
) ([]enygma.IEnygmaPoint, enygma.IEnygmaUsdrProof) {
	t.Helper()

	var usdrSignal [82]*big.Int
	for i := range usdrSignal {
		usdrSignal[i] = big.NewInt(0)
	}

	for i := 0; i < usdrHelperPublicKeySize; i++ {
		usdrSignal[usdrHelperPublicKeyOffset+i] = mainSignal[usdrHelperPublicKeyOffset+i]
	}
	for i := 0; i < usdrHelperKIndexSize; i++ {
		usdrSignal[usdrHelperKIndexOffset+i] = mainSignal[usdrHelperKIndexOffset+i]
	}
	usdrSignal[usdrHelperBlockNumberOff] = mainSignal[usdrHelperBlockNumberOff]

	usdrCommitmentDeltas := make([]enygma.IEnygmaPoint, len(participantAccountIds))
	for i, accountId := range participantAccountIds {
		bal, err := instance.GetUsdrBalance(&bind.CallOpts{}, big.NewInt(accountId))
		if err != nil {
			t.Fatalf("getUsdrBalance(accountId=%d): %v", accountId, err)
		}
		usdrSignal[usdrHelperPrevCommitOff+2*i] = bal.X
		usdrSignal[usdrHelperPrevCommitOff+2*i+1] = bal.Y

		// Neutral element (0,1): zero delta — this leg doesn't need to
		// represent a real fee flow, only satisfy the contract's own
		// self-consistency check between public_signal's TxCommit and the
		// separately-passed usdrCommitmentDeltas parameter.
		usdrSignal[usdrHelperTxCommitOff+2*i] = big.NewInt(0)
		usdrSignal[usdrHelperTxCommitOff+2*i+1] = big.NewInt(1)
		usdrCommitmentDeltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
	}

	usdrSignal[usdrHelperFeeAmountOff] = big.NewInt(usdrFeeAmt)
	usdrSignal[usdrHelperDomainOff] = expectedDomainId(enygmaAddr) // Fix L-01

	seq := atomic.AddInt64(&usdrHelperNullifierSeq, 1)
	// Offset well clear of any small hand-picked seeds the *_repro_test.go
	// files use for their own main-proof nullifiers (e.g. c04's 111/222),
	// and of any real Poseidon-derived nullifier the live gnark path might
	// produce (effectively random in the ~254-bit field).
	usdrSignal[usdrHelperNullifierOff] = big.NewInt(900_000_000 + seq)

	usdrProof := enygma.IEnygmaUsdrProof{
		Proof:        [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)},
		PublicSignal: usdrSignal,
	}
	return usdrCommitmentDeltas, usdrProof
}
