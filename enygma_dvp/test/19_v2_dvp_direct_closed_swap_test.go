package tests

// 19_v2_dvp_direct_closed_swap_test.go
//
// The direct DvP path (submitPartialSettlement / claimSwapTimeout) must treat a
// settled or timed-out swap as closed. Before the fix, both deleted the pending
// record outright, so a late counterparty leg fell through to the first-leg
// branch:
//
//   - ERC-20-for-ERC-20 exchange: the late leg (initiator-shaped) was accepted
//     as a NEW swap, its note was locked, and it could never settle; the note
//     came back only after the leg's own deadline via claimSwapTimeout.
//   - ERC-20-for-ERC-721 DvP: the late (destination-shaped) leg reverted with
//     an unrelated error (SwapDeadlineMustBeInFuture, or a 0x32 out-of-bounds
//     panic). A destination leg sent before the initiator's leg hit the same panic.
//
// Now a leg for a closed swap reverts with SwapClosed, and a destination leg
// with no pending swap reverts with SwapNotFound; neither locks anything.
//
// Prerequisites: fresh Hardhat node + deploy + init, gnark server on :8081.
//
//	cd test && CC=/usr/bin/clang go test -run TestV2DvP_DirectClosedSwap -v -timeout 600s

import (
	"context"
	"math/big"
	"regexp"
	"testing"
	"time"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type directSwap struct {
	t   *testing.T
	tc  *dvpTestContext
	abi abi.ABI
	dvp common.Address
}

var hardhatRevertName = regexp.MustCompile(`custom error '(\w+)\(|(panic code 0x[0-9a-f]+)`)

// submit simulates a DvP call first (to read a revert), then mines it.
// It returns the custom error name / panic code, or "" on success.
func (d *directSwap) submit(method string, args ...interface{}) (string, *types.Receipt) {
	d.t.Helper()
	data, err := d.abi.Pack(method, args...)
	if err != nil {
		d.t.Fatalf("pack %s: %v", method, err)
	}
	call := ethereum.CallMsg{From: d.tc.auth.From, To: &d.dvp, Gas: d.tc.auth.GasLimit, Data: data}
	if _, err := d.tc.client.CallContract(context.Background(), call, nil); err != nil {
		if m := hardhatRevertName.FindStringSubmatch(err.Error()); m != nil {
			return m[1] + m[2], nil
		}
		return err.Error(), nil
	}
	tx, err := d.tc.dvp.RawTransact(d.tc.auth, data)
	if err != nil {
		d.t.Fatalf("send %s: %v", method, err)
	}
	rcpt, err := bind.WaitMined(context.Background(), d.tc.client, tx)
	if err != nil || rcpt.Status != types.ReceiptStatusSuccessful {
		d.t.Fatalf("%s mined but failed: %v", method, err)
	}
	return "", rcpt
}

func (d *directSwap) mustSubmit(what, method string, args ...interface{}) *types.Receipt {
	d.t.Helper()
	e, rcpt := d.submit(method, args...)
	if e != "" {
		d.t.Fatalf("%s: %s", what, e)
	}
	return rcpt
}

// noteState reports whether a single-input receipt's note is locked or spent.
func (d *directSwap) noteState(vault *bind.BoundContract, rc onchainProofReceipt) (locked, spent bool) {
	d.t.Helper()
	tree, nf := rc.Statement[1], rc.Statement[3]
	var out []interface{}
	if err := vault.Call(nil, &out, "isLocked", tree, nf); err != nil {
		d.t.Fatalf("isLocked: %v", err)
	}
	locked = out[0].(bool)
	out = nil
	if err := vault.Call(nil, &out, "nullifiers", tree, nf); err != nil {
		d.t.Fatalf("nullifiers: %v", err)
	}
	return locked, out[0].(bool)
}

// minSwapDeadline is a deadline offset just over EnygmaDvp.MIN_SWAP_DURATION
// (15 minutes), the shortest a first leg can set.
const minSwapDeadline = 16 * 60

func (d *directSwap) deadlineIn(seconds int64) *big.Int {
	return new(big.Int).Add(currentBlockTimestamp(d.t, d.tc.client), big.NewInt(seconds))
}

func waitOK(t *testing.T, client *ethclient.Client, what string, tx *types.Transaction, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	rcpt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil || rcpt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("%s: not mined successfully (err=%v)", what, err)
	}
}

// exchangeLegs deposits two ERC-20 notes and builds cross-referencing
// DvPInitiator receipts for an ERC-20-for-ERC-20 exchange (the same
// construction as TestDvP_ExchangeViaRelayer). Alice's swap key is her
// commitB, which is also Bob's statement message. Each proof names the vault
// its prover expects to be paid from: the ERC-20 vault unless overridden.
func exchangeLegs(t *testing.T, tc *dvpTestContext, expects ...common.Address) (alice, bob onchainProofReceipt) {
	t.Helper()
	aliceExpects, bobExpects := tc.erc20VaultAddr, tc.erc20VaultAddr
	if len(expects) == 2 {
		aliceExpects, bobExpects = expects[0], expects[1]
	}
	tokenId := big.NewInt(0)
	aliceAmt, bobAmt := big.NewInt(20), big.NewInt(15)
	type note struct {
		spend *core.SpendKeyPair
		salt  *big.Int
		cmt   *big.Int
	}
	deposit := func(amt *big.Int) note {
		spend, err := core.NewSpendKeyPair()
		if err != nil {
			t.Fatal(err)
		}
		view, err := core.NewViewKeyPair()
		if err != nil {
			t.Fatal(err)
		}
		ss, capsule, err := core.Encapsulate(view.EncapsKey)
		if err != nil {
			t.Fatal(err)
		}
		saltBytes, err := core.DerivePaymentSalt(ss)
		if err != nil {
			t.Fatal(err)
		}
		encKey, err := core.DerivePaymentKey(ss)
		if err != nil {
			t.Fatal(err)
		}
		salt := core.SaltBToField(saltBytes)
		cmt, err := core.Erc20CommitmentV2(spend.PublicKey, salt, amt, tokenId)
		if err != nil {
			t.Fatal(err)
		}
		enc, err := core.EncryptPayload(encKey, tokenId, amt)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := tc.erc20Token.Transact(tc.auth, "mint", tc.auth.From, amt)
		waitOK(t, tc.client, "ERC20.mint", tx, err)
		tx, err = tc.erc20Token.Transact(tc.auth, "approve", tc.erc20VaultAddr, amt)
		waitOK(t, tc.client, "ERC20.approve", tx, err)
		tx, err = tc.erc20Vault.Transact(tc.auth, "depositV2", []*big.Int{amt, spend.PublicKey, salt, tokenId}, capsule, enc)
		waitOK(t, tc.client, "depositV2", tx, err)
		return note{spend, salt, cmt}
	}
	a, b := deposit(aliceAmt), deposit(bobAmt)

	saltForAlice, err := core.RandomInField()
	if err != nil {
		t.Fatal(err)
	}
	saltForBob, err := core.RandomInField()
	if err != nil {
		t.Fatal(err)
	}
	mt := loadVaultMerkleTree(t, tc.client, tc.erc20VaultAddr, tc.merkleDepth)
	aProof, err := mt.GenerateProof(a.cmt)
	if err != nil {
		t.Fatal(err)
	}
	bProof, err := mt.GenerateProof(b.cmt)
	if err != nil {
		t.Fatal(err)
	}
	ar, err := tc.gnarkClient.DvPInitiatorProofFromSalts(
		core.KeyPair{PrivateKey: a.spend.PrivateKey, PublicKey: a.spend.PublicKey},
		a.salt, aliceAmt, tokenId, b.spend.PublicKey, saltForBob, bobAmt, tokenId, saltForAlice,
		big.NewInt(0), aProof, tc.merkleDepth, vaultInt(aliceExpects))
	if err != nil {
		t.Fatalf("DvPInitiatorProofFromSalts (Alice): %v", err)
	}
	br, err := tc.gnarkClient.DvPInitiatorProofFromSalts(
		core.KeyPair{PrivateKey: b.spend.PrivateKey, PublicKey: b.spend.PublicKey},
		b.salt, bobAmt, tokenId, a.spend.PublicKey, saltForAlice, aliceAmt, tokenId, saltForBob,
		big.NewInt(0), bProof, tc.merkleDepth, vaultInt(bobExpects))
	if err != nil {
		t.Fatalf("DvPInitiatorProofFromSalts (Bob): %v", err)
	}
	toReceipt := func(st []*big.Int, proof []string, nIn, nOut int) onchainProofReceipt {
		return onchainProofReceipt{
			Proof:           proofStringsToOnchain(t, proof),
			Statement:       st,
			NumberOfInputs:  big.NewInt(int64(nIn)),
			NumberOfOutputs: big.NewInt(int64(nOut)),
		}
	}
	return toReceipt(ar.Statement, ar.Proof, ar.NumberOfInputs, ar.NumberOfOutputs),
		toReceipt(br.Statement, br.Proof, br.NumberOfInputs, br.NumberOfOutputs)
}

func TestV2DvP_DirectClosedSwap(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping")
	}
	tc := newDvpTestContext(t)
	defer tc.client.Close()
	d := &directSwap{
		t: t, tc: tc,
		abi: loadOnchainABI(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json"),
		dvp: common.HexToAddress(loadOnchainReceipts(t)["EnygmaDvp"].ContractAddress),
	}
	zero, one := big.NewInt(0), big.NewInt(1)
	// Unique NFT ids per run, so the test does not need a fresh node.
	nftId := func() int64 { return 1000 + time.Now().UnixNano()%1_000_000_000 }

	t.Run("exchange: late leg after timeout is rejected", func(t *testing.T) {
		d.t = t
		alice, bob := exchangeLegs(t, tc)

		d.mustSubmit("Alice's leg", "submitPartialSettlement", alice, zero, zero, d.deadlineIn(minSwapDeadline))
		hardhatIncreaseTime(t, tc.client, minSwapDeadline+60)
		d.mustSubmit("claimSwapTimeout", "claimSwapTimeout", alice.Statement[4])

		if e, _ := d.submit("submitPartialSettlement", bob, zero, zero, d.deadlineIn(7*24*3600)); e != "SwapClosed" {
			t.Fatalf("Bob's late leg: got %q, want SwapClosed", e)
		}
		if locked, spent := d.noteState(tc.erc20Vault, bob); locked || spent {
			t.Fatalf("Bob's note after the rejected late leg: locked=%v spent=%v, want both false", locked, spent)
		}
		t.Log("late exchange leg rejected with SwapClosed; Bob's note untouched")
	})

	t.Run("exchange: normal settlement still works", func(t *testing.T) {
		d.t = t
		alice, bob := exchangeLegs(t, tc)
		d.mustSubmit("Alice's leg", "submitPartialSettlement", alice, zero, zero, d.deadlineIn(minSwapDeadline))
		d.mustSubmit("Bob's leg", "submitPartialSettlement", bob, zero, zero, zero)
		for who, rc := range map[string]onchainProofReceipt{"Alice": alice, "Bob": bob} {
			if locked, spent := d.noteState(tc.erc20Vault, rc); locked || !spent {
				t.Fatalf("%s's note after settlement: locked=%v spent=%v", who, locked, spent)
			}
		}
		// The settled swap is closed too: Bob's leg again is rejected (by
		// the spent-nullifier check, which runs first).
		if e, _ := d.submit("submitPartialSettlement", bob, zero, zero, d.deadlineIn(minSwapDeadline)); e != "InvalidNullifier" {
			t.Fatalf("Bob's leg after settlement: got %q, want InvalidNullifier", e)
		}
		t.Log("exchange settled; resubmission rejected")
	})

	t.Run("dvp: late destination leg after timeout is rejected", func(t *testing.T) {
		d.t = t
		p := dvpGenerateProofs(t, tc, dvpSetupDeposits(t, tc, nftId()))

		d.mustSubmit("Alice's leg", "submitPartialSettlement", p.aliceReceipt, zero, zero, d.deadlineIn(minSwapDeadline))
		hardhatIncreaseTime(t, tc.client, minSwapDeadline+60)
		d.mustSubmit("claimSwapTimeout", "claimSwapTimeout", p.commitB)

		for _, dl := range []*big.Int{zero, d.deadlineIn(3600)} {
			if e, _ := d.submit("submitPartialSettlement", p.bobReceipt, one, one, dl); e != "SwapClosed" {
				t.Fatalf("Bob's late destination leg (deadline %s): got %q, want SwapClosed", dl, e)
			}
		}
		if locked, spent := d.noteState(tc.erc721Vault, p.bobReceipt); locked || spent {
			t.Fatalf("Bob's NFT note: locked=%v spent=%v, want both false", locked, spent)
		}
		t.Log("late destination leg rejected with SwapClosed; Bob's NFT untouched")
	})

	t.Run("dvp: destination leg before the initiator leg", func(t *testing.T) {
		d.t = t
		p := dvpGenerateProofs(t, tc, dvpSetupDeposits(t, tc, nftId()))

		if e, _ := d.submit("submitPartialSettlement", p.bobReceipt, one, one, d.deadlineIn(minSwapDeadline)); e != "SwapNotFound" {
			t.Fatalf("destination leg with no pending swap: got %q, want SwapNotFound", e)
		}
		// The normal order still settles afterwards.
		d.mustSubmit("Alice's leg", "submitPartialSettlement", p.aliceReceipt, zero, zero, d.deadlineIn(minSwapDeadline))
		d.mustSubmit("Bob's leg", "submitPartialSettlement", p.bobReceipt, one, one, zero)
		if _, spent := d.noteState(tc.erc721Vault, p.bobReceipt); !spent {
			t.Fatal("Bob's NFT note not spent after settlement")
		}
		t.Log("early destination leg rejected with SwapNotFound; swap then settled in order")
	})

	t.Run("claimSwapTimeout on a closed swap", func(t *testing.T) {
		d.t = t
		p := dvpGenerateProofs(t, tc, dvpSetupDeposits(t, tc, nftId()))
		d.mustSubmit("Alice's leg", "submitPartialSettlement", p.aliceReceipt, zero, zero, d.deadlineIn(minSwapDeadline))
		hardhatIncreaseTime(t, tc.client, minSwapDeadline+60)
		d.mustSubmit("claimSwapTimeout", "claimSwapTimeout", p.commitB)
		if e, _ := d.submit("claimSwapTimeout", p.commitB); e != "SwapNotFound" {
			t.Fatalf("second claimSwapTimeout: got %q, want SwapNotFound", e)
		}
		// Alice's leg cannot reopen the swap either: her note is spent.
		if e, _ := d.submit("submitPartialSettlement", p.aliceReceipt, zero, zero, d.deadlineIn(minSwapDeadline)); e != "InvalidNullifier" {
			t.Fatalf("Alice's leg after timeout: got %q, want InvalidNullifier", e)
		}
	})
}
