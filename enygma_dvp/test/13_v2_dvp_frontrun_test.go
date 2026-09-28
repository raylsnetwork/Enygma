package tests

// Front-running tests for EnygmaDvp.submitPartialSettlement / claimSwapTimeout.
//
// submitPartialSettlement is permissionless, and the swap's first leg
// (Alice's DvP Initiator receipt) is public the moment it is broadcast. Two
// pieces of state the contract stores for that leg are NOT authenticated by
// Alice's proof:
//
//   - `deadline` is a plain function argument (not a public signal).
//   - `initiator` is msg.sender of whoever submits the first leg, and
//     claimSwapTimeout (HIGH-10) is restricted to that address.
//
// So an attacker who submits Alice's receipt first chooses the deadline and
// would become the only address able to release Alice's locked note. The
// contract therefore lets ANYONE call claimSwapTimeout after the deadline
// (the refund can only go to the proven revertCommitA) and caps the deadline
// at MAX_SWAP_DURATION so a hijacked swap cannot hold the note indefinitely.
//
// Each sub-test asserts a property the hardened contract must keep. A failing
// sub-test is a regression, not a flaky test — its message starts with
// "VULNERABLE:". Sub-tests labelled SAFE assert properties that must hold
// either way (assets cannot be redirected).
//
//   HijackInitiator        Attacker submits Alice's receipt with a short
//                          deadline. Alice must still be able to recover her
//                          note without the attacker's cooperation.
//   AttackerCannotRedirect (SAFE) Attacker submits leg 1, Bob settles in
//                          time: outputs are the proven commitments.
//   OverwritePending       After Alice's honest leg 1, the attacker resubmits
//                          the same receipt with a different deadline. It
//                          must not replace the stored initiator/deadline.
//   AnyoneCanFinishLeg2    (SAFE) A third party submits Bob's receipt: the
//                          swap settles exactly as if Bob had sent it.
//   FarFutureDeadline      The attacker picks a deadline far in the future to
//                          lock Alice's note for as long as possible. The
//                          contract must reject it (SwapDeadlineTooFar).
//
// The attacker is a freshly generated key funded from account[0] (no key
// material lives in this file); Alice and Bob's transactions use account[0]
// (as in the other tests in this package).
//
// Prerequisites are the same as 04_v2_dvp_deadline_test.go.
//
// Run:
//   cd test && CC=/usr/bin/clang go test -run TestV2DvP_FrontRun -v -timeout 900s

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	// ERC721 tokenIds distinct from the other tests (25, 42, 100-102).
	frontRunTokenHijack    = int64(110)
	frontRunTokenRedirect  = int64(111)
	frontRunTokenOverwrite = int64(112)
	frontRunTokenLeg2      = int64(113)
	frontRunTokenFar       = int64(114)

	// Short window the attacker picks for the victim's swap.
	frontRunAttackerDeadlineSeconds = int64(30)
)

// attackerAuth returns a transactor for a fresh, funded key distinct from
// Alice/Bob's account[0].
func attackerAuth(t *testing.T, tc *dvpTestContext) *bind.TransactOpts {
	t.Helper()
	ctx := context.Background()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("attacker GenerateKey: %v", err)
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)

	funder, err := crypto.HexToECDSA(hardhatPrivKeyHex)
	if err != nil {
		t.Fatalf("funder HexToECDSA: %v", err)
	}
	nonce, err := tc.client.PendingNonceAt(ctx, tc.auth.From)
	if err != nil {
		t.Fatalf("funder nonce: %v", err)
	}
	gasPrice, err := tc.client.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatalf("gas price: %v", err)
	}
	fund := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		To:       &addr,
		Value:    new(big.Int).SetUint64(500_000_000_000_000_000), // 0.5 ETH
		Gas:      21000,
		GasPrice: gasPrice,
	})
	signed, err := ethtypes.SignTx(fund, ethtypes.NewEIP155Signer(big.NewInt(hardhatChainID)), funder)
	if err != nil {
		t.Fatalf("sign funding tx: %v", err)
	}
	if err := tc.client.SendTransaction(ctx, signed); err != nil {
		t.Fatalf("fund attacker: %v", err)
	}
	if _, err := bind.WaitMined(ctx, tc.client, signed); err != nil {
		t.Fatalf("wait attacker funding: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(hardhatChainID))
	if err != nil {
		t.Fatalf("attacker NewKeyedTransactorWithChainID: %v", err)
	}
	auth.GasLimit = 6_000_000
	return auth
}

// submitLeg sends submitPartialSettlement from `from` and waits for it.
// It returns the mined receipt status (0 on revert) and any send error.
func submitLeg(
	t *testing.T,
	tc *dvpTestContext,
	from *bind.TransactOpts,
	receipt onchainProofReceipt,
	vaultId, groupId int64,
	deadline *big.Int,
) (uint64, error) {
	t.Helper()
	tx, err := tc.dvp.Transact(from, "submitPartialSettlement",
		receipt, big.NewInt(vaultId), big.NewInt(groupId), deadline)
	if err != nil {
		return 0, err
	}
	mined, err := bind.WaitMined(tc.ctx, tc.client, tx)
	if err != nil {
		return 0, err
	}
	return mined.Status, nil
}

func isRevert(err error, needle string) bool {
	return err != nil && strings.Contains(err.Error(), needle)
}

func TestV2DvP_FrontRun(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping on-chain test")
	}

	// ── HijackInitiator ──────────────────────────────────────────────────────
	t.Run("HijackInitiator", func(t *testing.T) {
		tc := newDvpTestContext(t)
		defer tc.client.Close()
		attacker := attackerAuth(t, tc)
		t.Logf("alice/bob=%s attacker=%s", tc.auth.From.Hex(), attacker.From.Hex())

		d := dvpSetupDeposits(t, tc, frontRunTokenHijack)
		p := dvpGenerateProofs(t, tc, d)

		// The attacker sees Alice's receipt and submits it first, with a very
		// short deadline of their own choosing.
		deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client),
			big.NewInt(frontRunAttackerDeadlineSeconds))
		status, err := submitLeg(t, tc, attacker, p.aliceReceipt, 0, 0, deadline)
		if err != nil || status != 1 {
			t.Skipf("attacker's first-leg submission was rejected (status=%d err=%v) — front-run not possible", status, err)
		}
		t.Logf("attacker's submission of Alice's receipt was ACCEPTED (deadline=%s)", deadline)

		// Bob tries to complete the swap after the attacker's deadline.
		hardhatIncreaseTime(t, tc.client, frontRunAttackerDeadlineSeconds+5)
		_, errBob := submitLeg(t, tc, tc.auth, p.bobReceipt, 1, 1, big.NewInt(0))
		if errBob == nil {
			t.Fatal("Bob's post-deadline second leg unexpectedly succeeded")
		}
		t.Logf("Bob's second leg is dead after the attacker's short deadline: %v", errBob)

		// The property under test: Alice, the note owner, must be able to
		// release her locked note without the attacker.
		tx, errAlice := tc.dvp.Transact(tc.auth, "claimSwapTimeout", p.commitB)
		if errAlice == nil {
			if _, err := bind.WaitMined(tc.ctx, tc.client, tx); err != nil {
				t.Fatalf("wait Alice claimSwapTimeout: %v", err)
			}
			t.Log("Alice recovered her note herself — initiator is not hijackable")
			return
		}
		t.Errorf("VULNERABLE: Alice cannot claimSwapTimeout after a third party submitted her receipt first: %v", errAlice)
		if isRevert(errAlice, "Unauthorized") {
			t.Log("  reason: claimSwapTimeout is restricted to msg.sender of the FIRST submission")
		}

		// Show the dependency: only the attacker can release the note.
		txA, errAtk := tc.dvp.Transact(attacker, "claimSwapTimeout", p.commitB)
		if errAtk != nil {
			t.Errorf("attacker's own claimSwapTimeout also failed: %v", errAtk)
			return
		}
		if _, err := bind.WaitMined(tc.ctx, tc.client, txA); err != nil {
			t.Fatalf("wait attacker claimSwapTimeout: %v", err)
		}
		t.Log("  only the attacker's claimSwapTimeout succeeds — Alice's note is locked at their discretion")
	})

	// ── AttackerCannotRedirect (SAFE) ────────────────────────────────────────
	t.Run("AttackerCannotRedirect", func(t *testing.T) {
		tc := newDvpTestContext(t)
		defer tc.client.Close()
		attacker := attackerAuth(t, tc)

		d := dvpSetupDeposits(t, tc, frontRunTokenRedirect)
		p := dvpGenerateProofs(t, tc, d)

		// Attacker front-runs leg 1 but with a generous deadline this time.
		deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(3600))
		status, err := submitLeg(t, tc, attacker, p.aliceReceipt, 0, 0, deadline)
		if err != nil || status != 1 {
			t.Skipf("attacker's first-leg submission was rejected (status=%d err=%v)", status, err)
		}

		// Bob completes normally.
		bobTx, err := tc.dvp.Transact(tc.auth, "submitPartialSettlement",
			p.bobReceipt, big.NewInt(1), big.NewInt(1), big.NewInt(0))
		if err != nil {
			t.Fatalf("Bob second leg after attacker's leg 1: %v", err)
		}
		mined, err := bind.WaitMined(tc.ctx, tc.client, bobTx)
		if err != nil || mined.Status != 1 {
			t.Fatalf("Bob second leg reverted (status=%v err=%v)", mined, err)
		}

		foundA, foundB := false, false
		for _, log := range mined.Logs {
			if log.Topics[0] != tc.commitmentSig || len(log.Topics) < 3 {
				continue
			}
			c := log.Topics[2].Big()
			if c.Cmp(p.commitA) == 0 {
				foundA = true
			}
			if c.Cmp(p.commitB) == 0 {
				foundB = true
			}
		}
		if !foundA || !foundB {
			t.Errorf("settlement did not insert the proven commitments (commitA=%v commitB=%v)", foundA, foundB)
		} else {
			t.Log("SAFE: attacker's front-run changed nothing about who receives what — outputs are the proven commitments")
		}
	})

	// ── OverwritePending ─────────────────────────────────────────────────────
	t.Run("OverwritePending", func(t *testing.T) {
		tc := newDvpTestContext(t)
		defer tc.client.Close()
		attacker := attackerAuth(t, tc)

		d := dvpSetupDeposits(t, tc, frontRunTokenOverwrite)
		p := dvpGenerateProofs(t, tc, d)

		// Alice submits honestly with a long deadline.
		longDeadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(3600))
		status, err := submitLeg(t, tc, tc.auth, p.aliceReceipt, 0, 0, longDeadline)
		if err != nil || status != 1 {
			t.Fatalf("Alice's honest first leg failed (status=%d err=%v)", status, err)
		}

		// Attacker resubmits the SAME receipt with a short deadline, hoping to
		// replace the stored initiator/deadline.
		shortDeadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client),
			big.NewInt(frontRunAttackerDeadlineSeconds))
		status, err = submitLeg(t, tc, attacker, p.aliceReceipt, 0, 0, shortDeadline)
		if err != nil || status != 1 {
			t.Logf("SAFE: resubmission rejected (status=%d err=%v)", status, err)
			return
		}
		t.Errorf("VULNERABLE: a second submission of the same first-leg receipt was accepted and can overwrite the pending swap")

		// Confirm the overwrite took effect: Alice should be locked out.
		hardhatIncreaseTime(t, tc.client, frontRunAttackerDeadlineSeconds+5)
		_, errAlice := tc.dvp.Transact(tc.auth, "claimSwapTimeout", p.commitB)
		if errAlice != nil {
			t.Errorf("  and Alice's claimSwapTimeout now fails: %v", errAlice)
		} else {
			t.Log("  (Alice could still claim — only the deadline was replaced)")
		}
	})

	// ── AnyoneCanFinishLeg2 (SAFE) ───────────────────────────────────────────
	t.Run("AnyoneCanFinishLeg2", func(t *testing.T) {
		tc := newDvpTestContext(t)
		defer tc.client.Close()
		attacker := attackerAuth(t, tc)

		d := dvpSetupDeposits(t, tc, frontRunTokenLeg2)
		p := dvpGenerateProofs(t, tc, d)

		deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(3600))
		if status, err := submitLeg(t, tc, tc.auth, p.aliceReceipt, 0, 0, deadline); err != nil || status != 1 {
			t.Fatalf("Alice's first leg failed (status=%d err=%v)", status, err)
		}

		// A third party submits Bob's (public) receipt.
		tx, err := tc.dvp.Transact(attacker, "submitPartialSettlement",
			p.bobReceipt, big.NewInt(1), big.NewInt(1), big.NewInt(0))
		if err != nil {
			t.Fatalf("third-party second leg rejected: %v", err)
		}
		mined, err := bind.WaitMined(tc.ctx, tc.client, tx)
		if err != nil || mined.Status != 1 {
			t.Fatalf("third-party second leg reverted (status=%v err=%v)", mined, err)
		}

		foundA, foundB := false, false
		for _, log := range mined.Logs {
			if log.Topics[0] != tc.commitmentSig || len(log.Topics) < 3 {
				continue
			}
			c := log.Topics[2].Big()
			foundA = foundA || c.Cmp(p.commitA) == 0
			foundB = foundB || c.Cmp(p.commitB) == 0
		}
		if !foundA || !foundB {
			t.Errorf("third-party settlement inserted unexpected commitments (commitA=%v commitB=%v)", foundA, foundB)
		} else {
			t.Log("SAFE: a third party can finish the swap, but only into the proven outputs")
		}
	})
	// ── FarFutureDeadline ────────────────────────────────────────────────────
	t.Run("FarFutureDeadline", func(t *testing.T) {
		tc := newDvpTestContext(t)
		defer tc.client.Close()
		attacker := attackerAuth(t, tc)

		d := dvpSetupDeposits(t, tc, frontRunTokenFar)
		p := dvpGenerateProofs(t, tc, d)

		// One year out — far beyond MAX_SWAP_DURATION (30 days).
		farDeadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client),
			big.NewInt(365*24*3600))
		status, err := submitLeg(t, tc, attacker, p.aliceReceipt, 0, 0, farDeadline)
		if err != nil || status != 1 {
			t.Logf("SAFE: far-future deadline rejected (status=%d err=%v)", status, err)
			if !isRevert(err, "SwapDeadlineTooFar") {
				t.Errorf("expected SwapDeadlineTooFar, got: %v", err)
			}
			return
		}
		t.Errorf("VULNERABLE: a deadline one year out was accepted — a front-runner can lock Alice's note for that long")
	})
}
