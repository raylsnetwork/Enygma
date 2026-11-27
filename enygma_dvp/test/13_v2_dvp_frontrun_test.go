package tests

// Permissionless DvP submission tests. The first leg fixes the deadline,
// asset group and refund in its proof. A third party may submit either leg
// with the proven terms or trigger the refund after the deadline.

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

	// A window under EnygmaDvp.MIN_SWAP_DURATION (15 minutes): the contract
	// must refuse it.
	frontRunTooShortDeadlineSeconds = int64(30)
	// A valid short swap window, with time for the first transaction to mine.
	frontRunDeadlineSeconds = int64(16 * 60)
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

	t.Run("ProofBoundTerms", func(t *testing.T) {
		tc := newDvpTestContext(t)
		defer tc.client.Close()
		submitter := attackerAuth(t, tc)
		d := dvpSetupDeposits(t, tc, frontRunTokenHijack)
		deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(frontRunDeadlineSeconds))
		p := dvpGenerateProofs(t, tc, d, deadline)

		t.Run("changed deadline", func(t *testing.T) {
			changed := new(big.Int).Add(deadline, big.NewInt(60))
			if status, err := submitLeg(t, tc, submitter, p.aliceReceipt, 0, 0, changed); !isRevert(err, "SwapDeadlineMismatch") {
				t.Fatalf("changed deadline gave status %d and error %v, want SwapDeadlineMismatch", status, err)
			}
		})
		t.Run("changed group", func(t *testing.T) {
			if status, err := submitLeg(t, tc, submitter, p.aliceReceipt, 0, 1, deadline); !isRevert(err, "SwapGroupMismatch") {
				t.Fatalf("changed group gave status %d and error %v, want SwapGroupMismatch", status, err)
			}
		})

		if status, err := submitLeg(t, tc, submitter, p.aliceReceipt, 0, 0, deadline); err != nil || status != 1 {
			t.Fatalf("proven terms gave status %d and error %v", status, err)
		}
		hardhatIncreaseTime(t, tc.client, frontRunDeadlineSeconds+5)
		tx, err := tc.dvp.Transact(tc.auth, "claimSwapTimeout", p.commitB)
		waitOK(t, tc.client, "claimSwapTimeout by the note owner", tx, err)
	})

	// ── AttackerCannotRedirect (SAFE) ────────────────────────────────────────
	t.Run("AttackerCannotRedirect", func(t *testing.T) {
		tc := newDvpTestContext(t)
		defer tc.client.Close()
		attacker := attackerAuth(t, tc)

		d := dvpSetupDeposits(t, tc, frontRunTokenRedirect)
		deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(3600))
		p := dvpGenerateProofs(t, tc, d, deadline)
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
		longDeadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(3600))
		p := dvpGenerateProofs(t, tc, d, longDeadline)
		status, err := submitLeg(t, tc, tc.auth, p.aliceReceipt, 0, 0, longDeadline)
		if err != nil || status != 1 {
			t.Fatalf("Alice's honest first leg failed (status=%d err=%v)", status, err)
		}

		// Attacker resubmits the SAME receipt with a short deadline, hoping to
		// replace the stored initiator/deadline.
		shortDeadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client),
			big.NewInt(frontRunDeadlineSeconds))
		status, err = submitLeg(t, tc, attacker, p.aliceReceipt, 0, 0, shortDeadline)
		if err != nil || status != 1 {
			t.Logf("SAFE: resubmission rejected (status=%d err=%v)", status, err)
			return
		}
		t.Errorf("VULNERABLE: a second submission of the same first-leg receipt was accepted and can overwrite the pending swap")

		// Confirm the overwrite took effect: Alice should be locked out.
		hardhatIncreaseTime(t, tc.client, frontRunDeadlineSeconds+5)
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
		deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(3600))
		p := dvpGenerateProofs(t, tc, d, deadline)
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
	for _, tc := range []struct {
		name        string
		id, seconds int64
		revert      string
	}{
		{"DeadlineTooSoon", 1_114, frontRunTooShortDeadlineSeconds, "SwapDeadlineTooSoon"},
		{"FarFutureDeadline", frontRunTokenFar, 365 * 24 * 3600, "SwapDeadlineTooFar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newDvpTestContext(t)
			defer ctx.client.Close()
			d := dvpSetupDeposits(t, ctx, tc.id)
			deadline := new(big.Int).Add(currentBlockTimestamp(t, ctx.client), big.NewInt(tc.seconds))
			p := dvpGenerateProofs(t, ctx, d, deadline)
			if status, err := submitLeg(t, ctx, ctx.auth, p.aliceReceipt, 0, 0, deadline); !isRevert(err, tc.revert) {
				t.Fatalf("deadline window of %d seconds gave status %d and error %v, want %s", tc.seconds, status, err, tc.revert)
			}
		})
	}
}
