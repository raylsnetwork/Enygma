package tests

// 23_v2_cross_circuit_nullifier_test.go
//
// One ERC20 note must have one nullifier however it is spent. The vault records
// spends by nullifier value only, and both the Payment circuits and the DvP
// Initiator circuit can spend the same note from the same vault. When their
// nullifier formulas differed (Payment hashed the vault address in, DvP did
// not), a note spent by a payment still had an unspent DvP nullifier: its owner
// could open a swap with it against an old root, let it time out, and get the
// refund note as well — the note's value twice.
//
// Prerequisites are the same as 04_v2_dvp_deadline_test.go.
//
// Run:
//   cd test && CC=/usr/bin/clang go test -run TestV2CrossCircuit -v -timeout 900s

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"

	core "github.com/raylsnetwork/enygma_dvp/src/core"
)

const crossCircuitToken = int64(117)

func TestV2CrossCircuit_NoteSpentByPaymentCannotOpenASwap(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping on-chain test")
	}

	tc := newDvpTestContext(t)
	defer tc.client.Close()

	d := dvpSetupDeposits(t, tc, crossCircuitToken)
	// Alice's DvP Initiator leg over her ERC20 note, proven against the
	// current root (which stays valid after the note is spent).
	p := dvpGenerateProofs(t, tc, d)

	// Alice spends the same note with an ordinary payment to herself.
	alice := core.KeyPair{PrivateKey: d.aliceSpend.PrivateKey, PublicKey: d.aliceSpend.PublicKey}
	keep := new(big.Int).Sub(d.erc20Amount, big.NewInt(1))
	pay, err := tc.gnarkClient.BoundPaymentProof(
		new(big.Int).SetBytes(tc.erc20VaultAddr.Bytes()),
		big.NewInt(0),
		[]*big.Int{d.erc20Amount},
		[]core.KeyPair{alice},
		[]*big.Int{d.aliceSaltField},
		[]*big.Int{keep, big.NewInt(1)},
		[]*big.Int{alice.PublicKey, alice.PublicKey},
		[][]byte{d.aliceView.EncapsKey, d.aliceView.EncapsKey},
		tc.merkleDepth,
		[]*core.MerkleProof{d.aliceMerkleProof},
		[]*big.Int{big.NewInt(int64(d.aliceMerkleProof.TreeNumber))},
		d.erc20TokenId,
	)
	if err != nil {
		t.Fatalf("BoundPaymentProof: %v", err)
	}
	payReceipt := onchainProofReceipt{
		Proof:           proofStringsToOnchain(t, pay.Proof),
		Statement:       pay.ContractStatement(),
		NumberOfInputs:  big.NewInt(1),
		NumberOfOutputs: big.NewInt(2),
	}
	tx, err := tc.erc20Vault.Transact(tc.auth, "transfer", payReceipt)
	if err != nil {
		t.Fatalf("payment transfer: %v", err)
	}
	if r, err := bind.WaitMined(tc.ctx, tc.client, tx); err != nil || r.Status != 1 {
		t.Fatalf("payment transfer did not succeed: err=%v", err)
	}
	t.Log("Alice's note spent by a payment")

	payNullifier, dvpNullifier := pay.ContractStatement()[3], p.aliceReceipt.Statement[3]
	if payNullifier.Cmp(dvpNullifier) != 0 {
		t.Errorf("VULNERABLE: the same note has two nullifiers: payment %s, DvP %s", payNullifier, dvpNullifier)
	}

	// The DvP leg over the now-spent note must be refused.
	deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(minSwapDeadline))
	status, err := submitLeg(t, tc, tc.auth, p.aliceReceipt, 0, 0, deadline)
	if err == nil && status == 1 {
		t.Fatal("VULNERABLE: a DvP leg over a note already spent by a payment was accepted; " +
			"timing it out would refund the note a second time")
	}
	if !isRevert(err, "InvalidNullifier") {
		t.Fatalf("the DvP leg was refused, but not with InvalidNullifier: status=%d err=%v", status, err)
	}
	t.Log("the DvP leg over the spent note is refused (InvalidNullifier)")
}
