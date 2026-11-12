package tests

// A receipt's numberOfInputs and numberOfOutputs are the caller's claim, and
// the vaults compute every statement offset from them (which elements are
// nullifiers, which are inserted as commitments), while the verifier only
// checks the statement's length. So each verifying key must be reached only
// with the exact (inputs, outputs, statement length) of its circuit:
//
//	(a) a DvP Initiator receipt claiming 4 outputs made settlement insert
//	    commitA and revertCommitA (the initiator's refund) next to commitB;
//	(b) a Payment2in receipt resubmitted claiming 1 output spent both inputs
//	    but inserted only output[0], destroying the sender's change note, and
//	    skipped the vault-address check (which only ran for 2 or 3 outputs);
//	(c) a DvP Initiator receipt claiming 0 outputs could be parked by
//	    submitPartialSettlement but never settle (insertLeaves([]) reverts),
//	    locking the note until the timeout.
//
// Every path that nullifies or inserts from a Payment-family receipt
// (transfer, payment, withdraw, partial withdraw) first calls the vault's
// checkReceiptConditions, and every DvP leg (DvP settlement, SwapRelayer)
// goes through checkDvpReceiptConditions instead, which reads the leg by
// name and never by its claimed counts (ADR 0001). So a DvP Initiator receipt
// must be refused by the payment gate whatever counts it claims (a, c), and
// taken by the leg gate. This checks both gates directly, against
// AcceptAllVerifier so only the vault's own checks apply. Runs on
// go-ethereum's simulated backend; needs `npx hardhat compile` and the
// circomlibjs Poseidon artifacts (node scripts/regen_poseidon.js). Uses the
// harness in 28_v2_vault_legacy_withdraw_lock_test.go.

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type shapeVault struct {
	e     *lockEnv
	abi   abi.ABI
	addr  common.Address
	root  *big.Int
	vault *big.Int
}

func newShapeVault(t *testing.T, artifact string, asset common.Address) *shapeVault {
	e := newLockEnv(t)
	vABI, vault := e.newVault(artifact, asset)
	return &shapeVault{e: e, abi: vABI, addr: vault,
		root: e.call(vABI, vault, "getRoot").(*big.Int), vault: new(big.Int).SetBytes(vault.Bytes())}
}

// check returns the custom error checkReceiptConditions reverts with, or "".
func (s *shapeVault) check(statement []*big.Int, nIn, nOut int64) string {
	s.e.t.Helper()
	return s.e.send(s.abi, s.addr, "checkReceiptConditions", legacyReceipt(statement, nIn, nOut))
}

// checkDvp returns the custom error checkDvpReceiptConditions reverts with,
// or "".
func (s *shapeVault) checkDvp(statement []*big.Int, nIn, nOut int64) string {
	s.e.t.Helper()
	return s.e.send(s.abi, s.addr, "checkDvpReceiptConditions", legacyReceipt(statement, nIn, nOut))
}

func n(v int64) *big.Int { return big.NewInt(v) }

func TestV2ReceiptShape_Erc20(t *testing.T) {
	s := newShapeVault(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json",
		common.HexToAddress("0x00000000000000000000000000000000000000a0"))
	root, vault := s.root, s.vault

	// Statements as each circuit publishes them, bound to this vault.
	payment := []*big.Int{n(0), n(0), root, n(11), n(21), n(22), vault}
	payment2in := []*big.Int{n(0), n(0), n(0), root, root, n(11), n(12), n(21), n(22), vault}
	paymentFee := []*big.Int{n(0), n(0), root, n(11), n(21), n(22), vault, n(5)}
	relayerFee := []*big.Int{n(0), n(0), root, n(11), n(21), n(22), n(23), vault, n(5)}
	usdrFee := []*big.Int{n(0), n(0), root, n(11), n(21), n(22), vault, n(5), n(1)}
	counterVault := n(0xc0)
	// [message = commitA, tree, root, nullifier, commitB, commitA, revertCommitA, counterVault]
	initiator := []*big.Int{n(31), n(0), root, n(11), n(32), n(31), n(33), counterVault}

	honest := []struct {
		name      string
		statement []*big.Int
		nIn, nOut int64
	}{
		{"Payment", payment, 1, 2},
		{"Payment2in", payment2in, 2, 2},
		{"PaymentFee", paymentFee, 1, 2},
		{"PaymentRelayerFeePublic", relayerFee, 1, 3},
		{"UsdrFee", usdrFee, 1, 2},
	}
	for _, c := range honest {
		if r := s.check(c.statement, c.nIn, c.nOut); r != "" {
			t.Errorf("honest %s receipt (%d in, %d out): %s", c.name, c.nIn, c.nOut, r)
		}
	}

	misreported := []struct {
		name      string
		statement []*big.Int
		nIn, nOut int64
	}{
		{"DvP Initiator claiming its 1 output", initiator, 1, 1},
		{"(a) DvP Initiator claiming 4 outputs", initiator, 1, 4},
		{"(a) DvP Initiator claiming 3 outputs", initiator, 1, 3},
		{"(b) Payment2in claiming 1 output", payment2in, 2, 1},
		{"(b) Payment2in claiming 3 outputs", payment2in, 2, 3},
		{"(c) DvP Initiator claiming 0 outputs", initiator, 1, 0},
		{"Payment claiming 1 output", payment, 1, 1},
		{"Payment claiming 2 inputs", payment, 2, 2},
		{"Payment2in claiming 1 input", payment2in, 1, 2},
		{"PaymentFee claiming 3 outputs", paymentFee, 1, 3},
	}
	// (UsdrFee and PaymentRelayerFeePublic both have 9 signals, so a receipt
	// of one claiming the other's output count is the other's exact shape:
	// it goes to the other circuit's verifying key, which its proof does not
	// satisfy.)
	for _, c := range misreported {
		if r := s.check(c.statement, c.nIn, c.nOut); r != "ReceiptShapeMismatch" {
			t.Errorf("%s: got %q, want ReceiptShapeMismatch", c.name, r)
		}
	}

	// The leg gate takes the initiator, whatever counts it claims: it
	// nullifies and inserts exactly its one nullifier and CommitB.
	for _, nOut := range []int64{1, 0, 3, 4} {
		if r := s.checkDvp(initiator, 1, nOut); r != "" {
			t.Errorf("checkDvpReceiptConditions on a DvP Initiator claiming %d outputs: %s", nOut, r)
		}
	}

	// The vault-address binding runs for every Payment-family shape.
	for _, c := range honest {
		other := append([]*big.Int(nil), c.statement...)
		for i, v := range other {
			if v == vault {
				other[i] = n(0xbad)
			}
		}
		if r := s.check(other, c.nIn, c.nOut); r == "" {
			t.Errorf("%s receipt bound to another vault was accepted", c.name)
		}
	}
}

func TestV2ReceiptShape_Erc1155(t *testing.T) {
	s := newShapeVault(t, "core/contracts/vaults/Erc1155CoinVault.sol/Erc1155CoinVault.json",
		common.HexToAddress("0x00000000000000000000000000000000000000a0"))

	filled := func(size int) []*big.Int {
		st := make([]*big.Int, size)
		for i := range st {
			st[i] = n(int64(100 + i))
		}
		st[0] = n(0)
		return st
	}
	// 23 signals is JoinSplitErc1155WithAuditor's length for 2 in, 2 out,
	// and also what the length formula gives for 1 in, 5 out. 61 is the
	// 10-note batch, whose length does not depend on the outputs at all.
	if r := s.check(filled(23), 1, 5); r != "ReceiptShapeMismatch" {
		t.Errorf("fungible auditor receipt claiming 1 in, 5 out: got %q, want ReceiptShapeMismatch", r)
	}
	if r := s.check(filled(61), 10, 3); r != "ReceiptShapeMismatch" {
		t.Errorf("batch receipt claiming 3 outputs: got %q, want ReceiptShapeMismatch", r)
	}
	if r := s.check(filled(61), 10, 30); r != "ReceiptShapeMismatch" {
		t.Errorf("batch receipt claiming 30 outputs: got %q, want ReceiptShapeMismatch", r)
	}
}
