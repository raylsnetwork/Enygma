package tests

// A channel payment (#451) is one authorised payload
// p = (t, C_dest, C_chg, ctxt, nf, π): EnygmaDvp.channelPayment settles the
// proof together with the tag t, the note ciphertext ctxt and the block
// block_n the tag is for, all three bound into the proof's StCiphertextHash.
// The payee's salt is random, carried only in ctxt under the tag channel's
// block key; there is no per-payment ML-KEM capsule. The payee finds the
// payment by computing, for block_n, the tag of every channel secret they
// hold, and the payment lands in blocks block_n … block_n + W, W being
// CHANNEL_PAYMENT_WINDOW (100).
//
// Needs the retail stack (Hardhat :8545, gnark :8082), on the keys and
// contracts of this change.

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"testing"

	"enygma_retail_payments/layout"
	core "github.com/raylsnetwork/enygma_dvp/src/core"
	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

// channelPaid is one channel payment, proved and ready to settle.
type channelPaid struct {
	note    *tags.ChannelPayment
	receipt onchainProofReceipt
	res     *core.PaymentResult
	amount  *big.Int
}

func TestRetailErc20_ChannelPayment(t *testing.T) {
	e := newPartialWithdrawEnv(t)
	receipts := loadOnchainReceipts(t)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	dvp := bind.NewBoundContract(dvpAddr, loadOnchainABI(t, "EnygmaDvp"), e.client, e.client, e.client)

	ss, decoySS := make([]byte, 32), make([]byte, 32)
	if _, err := rand.Read(ss); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(decoySS); err != nil {
		t.Fatal(err)
	}
	bobSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	aliceSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	aliceView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair: %v", err)
	}

	// Alice's notes, deposited before block_n is read: every deposit mines a
	// block, and the payments below must be proved for the block the first
	// of them lands in.
	note100 := depositFor(t, e, aliceSpend, aliceView, big.NewInt(100))
	note30 := depositFor(t, e, aliceSpend, aliceView, big.NewInt(30))
	note20 := depositFor(t, e, aliceSpend, aliceView, big.NewInt(20))
	note10 := depositFor(t, e, aliceSpend, aliceView, big.NewInt(10))

	head, err := e.client.BlockNumber(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	blockN := head + 1

	// Three payments to Bob, all tagged for block_n: a Payment (100 → 60 to
	// Bob, 40 change), a Payment2in (30 + 20 → 45 to Bob, 5 change) and a
	// Payment of 10 that will come too late.
	p1 := proveChannelPayment(t, e, ss, blockN, bobSpend.PublicKey, []pwNote{note100}, big.NewInt(60), big.NewInt(40))
	p2 := proveChannelPayment(t, e, ss, blockN, bobSpend.PublicKey, []pwNote{note30, note20}, big.NewInt(45), big.NewInt(5))
	late := proveChannelPayment(t, e, ss, blockN, bobSpend.PublicKey, []pwNote{note10}, big.NewInt(10), big.NewInt(0))

	if p1.note.Tag != p2.note.Tag {
		t.Fatal("two payments to Bob for one block got different tags")
	}
	if p1.note.Salt.Cmp(p2.note.Salt) == 0 || bytes.Equal(p1.note.Ctxt[:12], p2.note.Ctxt[:12]) {
		t.Fatal("two payments to Bob for one block share a salt or a nonce")
	}

	settle := func(p channelPaid, block uint64, tag [32]byte, ctxt []byte) []interface{} {
		return []interface{}{p.receipt, big.NewInt(0), new(big.Int).SetUint64(block), tag, ctxt}
	}

	// None of these settles, and none spends a note or mines a block: each
	// is a call against the pending block, block_n.
	t.Run("other_tag", func(t *testing.T) {
		tag := p1.note.Tag
		tag[31] ^= 1
		expectPendingRevert(t, e, dvp, "CiphertextHashMismatch", "channelPayment", settle(p1, blockN, tag, p1.note.Ctxt)...)
	})
	t.Run("other_ctxt", func(t *testing.T) {
		ctxt := append([]byte{}, p1.note.Ctxt...)
		ctxt[len(ctxt)-1] ^= 1
		expectPendingRevert(t, e, dvp, "CiphertextHashMismatch", "channelPayment", settle(p1, blockN, p1.note.Tag, ctxt)...)
	})
	t.Run("other_block", func(t *testing.T) {
		// block_n - 1 is inside the window too, so only the binding refuses it.
		expectPendingRevert(t, e, dvp, "CiphertextHashMismatch", "channelPayment", settle(p1, blockN-1, p1.note.Tag, p1.note.Ctxt)...)
	})
	t.Run("block_not_reached", func(t *testing.T) {
		expectPendingRevert(t, e, dvp, "ChannelPaymentOutsideWindow", "channelPayment", settle(p1, blockN+1, p1.note.Tag, p1.note.Ctxt)...)
	})
	t.Run("as_a_plain_payment", func(t *testing.T) {
		// The same bytes through payment(): the two encodings differ, so a
		// channel receipt cannot settle without its tag and block.
		expectPendingRevert(t, e, dvp, "CiphertextHashMismatch", "payment", p1.receipt, big.NewInt(0), p1.note.Tag[:], p1.note.Ctxt)
	})

	t.Run("unknown_vault", func(t *testing.T) {
		expectPendingRevert(t, e, dvp, "InvalidVaultId", "channelPayment", settle(p1, blockN, p1.note.Tag, p1.note.Ctxt)[0], big.NewInt(7), new(big.Int).SetUint64(blockN), p1.note.Tag, p1.note.Ctxt)
	})
	t.Run("window_matches_the_scan", func(t *testing.T) {
		// The payee's scan looks as far past block_n as the contract settles.
		var out []interface{}
		if err := dvp.Call(&bind.CallOpts{Context: e.ctx}, &out, "CHANNEL_PAYMENT_WINDOW"); err != nil {
			t.Fatalf("CHANNEL_PAYMENT_WINDOW: %v", err)
		}
		if w := out[0].(*big.Int); w.Cmp(big.NewInt(tags.ChannelPaymentWindow)) != 0 {
			t.Fatalf("the contract's window is %s blocks, the scan's %d", w, tags.ChannelPaymentWindow)
		}
	})
	t.Run("fee_receipt", func(t *testing.T) {
		// Only Payment and Payment2in settle here: a fee-carrying receipt
		// has its fee checked by payment*, which channelPayment does not do.
		fee := p1.receipt
		fee.Statement = make([]*big.Int, layout.PaymentFeeLen)
		for i := range fee.Statement {
			fee.Statement[i] = big.NewInt(0)
		}
		expectPendingRevert(t, e, dvp, "InvalidStatementSize", "channelPayment", fee, big.NewInt(0), new(big.Int).SetUint64(blockN), p1.note.Tag, p1.note.Ctxt)
	})
	t.Run("message_not_zero", func(t *testing.T) {
		// As in payment(): the Payment circuits leave StMessage free.
		msg := p1.receipt
		msg.Statement = append([]*big.Int{}, p1.receipt.Statement...)
		msg.Statement[layout.PaymentMessageOffset] = big.NewInt(1)
		expectPendingRevert(t, e, dvp, "InvalidPaymentMessage", "channelPayment", msg, big.NewInt(0), new(big.Int).SetUint64(blockN), p1.note.Tag, p1.note.Ctxt)
	})

	// The first payment lands in block_n itself.
	m1 := transactChannelPayment(t, e, dvp, settle(p1, blockN, p1.note.Tag, p1.note.Ctxt))
	if m1.BlockNumber.Uint64() != blockN {
		t.Fatalf("the first payment landed in block %s, want block_n = %d", m1.BlockNumber, blockN)
	}

	// The second lands in block_n + W, the window's last block (W is the
	// scan's window, which window_matches_the_scan holds to the contract's).
	const w = tags.ChannelPaymentWindow
	mineBlocks(t, e, blockN+w-1-m1.BlockNumber.Uint64())
	m2 := transactChannelPayment(t, e, dvp, settle(p2, blockN, p2.note.Tag, p2.note.Ctxt))
	if got := m2.BlockNumber.Uint64(); got != blockN+w {
		t.Fatalf("the second payment landed in block %d, want block_n + %d = %d", got, w, blockN+w)
	}

	// The window is over: block_n + W + 1 refuses the third, its proof and
	// its bound tag and ctxt intact.
	t.Run("window_over", func(t *testing.T) {
		expectPendingRevert(t, e, dvp, "ChannelPaymentOutsideWindow", "channelPayment", settle(late, blockN, late.note.Tag, late.note.Ctxt)...)
	})

	// Bob holds two channel secrets, one of them this payer's. For block_n he
	// finds exactly the two settled payments, opens both from ctxt alone and
	// rebuilds each commitment.
	bob := []tags.Channel{{SharedSecret: decoySS, PkSpend: bobSpend.PublicKey}, {SharedSecret: ss, PkSpend: bobSpend.PublicKey}}
	found, err := tags.ScanChannelPayments(e.ctx, e.client, dvpAddr, blockN, bob)
	if err != nil {
		t.Fatalf("ScanChannelPayments: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("Bob's scan of block_n found %d payments, want 2", len(found))
	}
	for i, want := range []channelPaid{p1, p2} {
		f := found[i]
		if !bytes.Equal(f.SharedSecret, ss) {
			t.Errorf("payment %d matched another channel's secret", i)
		}
		if len(f.Ctxt) != 12+96+16 {
			t.Errorf("payment %d published %d bytes of ciphertext, want only the sealed note (124)", i, len(f.Ctxt))
		}
		note, err := f.Open()
		if err != nil {
			t.Fatalf("Bob cannot open payment %d: %v", i, err)
		}
		if note.Amount.Cmp(want.amount) != 0 || note.Salt.Cmp(want.note.Salt) != 0 {
			t.Errorf("payment %d opens to %s with salt %s, want %s with %s", i, note.Amount, note.Salt, want.amount, want.note.Salt)
		}
		if f.Commitment.Cmp(want.res.CommitmentsOut()[0]) != 0 {
			t.Errorf("payment %d's event names commitment %s, want the proof's C_dest %s", i, f.Commitment, want.res.CommitmentsOut()[0])
		}
	}
	if early, err := tags.ScanChannelPayments(e.ctx, e.client, dvpAddr, m2.BlockNumber.Uint64()+1000, bob); err != nil || len(early) != 0 {
		t.Errorf("a scan for a block not yet mined found %d payments (err=%v), want none and no error", len(early), err)
	}
	if none, err := tags.ScanChannelPayments(e.ctx, e.client, dvpAddr, blockN, bob[:1]); err != nil || len(none) != 0 {
		t.Errorf("a scan with only the other channel's secret found %d payments (err=%v), want 0", len(none), err)
	}
	t.Logf("two channel payments for block %d settled in blocks %s and %s; Bob found and opened both by tag", blockN, m1.BlockNumber, m2.BlockNumber)
}

// proveChannelPayment prepares a channel payment of amount to the payee over
// the channel secret ss, tagged for block, and proves it spending ins
// (one note: Payment; two: Payment2in) with change back to the notes' owner.
func proveChannelPayment(t *testing.T, e *pwEnv, ss []byte, block uint64, payee *big.Int, ins []pwNote, amount, change *big.Int) channelPaid {
	t.Helper()
	note, err := tags.PrepareChannelPayment(ss, block, payee, amount, e.tokenId)
	if err != nil {
		t.Fatalf("PrepareChannelPayment: %v", err)
	}
	tree := rebuildVaultTree(t, e.client, e.vaultAddr, nil)
	var values, salts, treeNums []*big.Int
	var keys []core.KeyPair
	var proofs []*core.MerkleProof
	for _, n := range ins {
		mp, err := tree.GenerateProof(n.commit)
		if err != nil {
			t.Fatalf("GenerateProof: %v", err)
		}
		values, salts = append(values, n.value), append(salts, n.salt)
		keys = append(keys, core.KeyPair{PrivateKey: n.spend.PrivateKey, PublicKey: n.spend.PublicKey})
		proofs, treeNums = append(proofs, mp), append(treeNums, big.NewInt(int64(mp.TreeNumber)))
	}
	res, err := core.NewGnarkClient(gnarkURL).BoundChannelPaymentProof(vaultInt(e.vaultAddr),
		values, keys, salts, []*big.Int{amount, change}, []*big.Int{payee, ins[0].spend.PublicKey},
		core.ChannelPaymentNote{Salt: note.Salt, BlockNumber: note.BlockNumber, Tag: note.Tag, Ctxt: note.Ctxt},
		partialWithdrawDepth, proofs, treeNums, e.tokenId)
	if err != nil {
		t.Fatalf("BoundChannelPaymentProof: %v", err)
	}
	hashAt := layout.PaymentCiphertextHashOffset
	if len(ins) == 2 {
		hashAt = layout.Payment2inCiphertextHashOffset
	}
	if got, want := res.Statement[hashAt], core.ChannelPaymentCiphertextHash(block, note.Tag, note.Ctxt); got.Cmp(want) != 0 {
		t.Fatalf("the proof binds ciphertext hash %s, want the channel payment's %s", got, want)
	}
	return channelPaid{
		note: note,
		res:  res,
		receipt: onchainProofReceipt{
			Proof:           proofStringsToOnchain(t, res.Proof),
			Statement:       res.ContractStatement(),
			NumberOfInputs:  big.NewInt(int64(res.NumberOfInputs)),
			NumberOfOutputs: big.NewInt(int64(res.NumberOfOutputs)),
		},
		amount: amount,
	}
}

func transactChannelPayment(t *testing.T, e *pwEnv, dvp *bind.BoundContract, args []interface{}) *types.Receipt {
	t.Helper()
	tx, err := dvp.Transact(e.owner, "channelPayment", args...)
	if err != nil {
		t.Fatalf("channelPayment: %v", err)
	}
	mined, err := bind.WaitMined(e.ctx, e.client, tx)
	if err != nil || mined.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("channelPayment did not settle (err=%v)", err)
	}
	return mined
}

// expectPendingRevert calls method against the pending block and wants it to
// revert with the error want. A refused Transact would mine a block on
// Hardhat (the test's gas limit skips the estimate), moving block.number.
func expectPendingRevert(t *testing.T, e *pwEnv, contract *bind.BoundContract, want, method string, args ...interface{}) {
	t.Helper()
	var out []interface{}
	err := contract.Call(&bind.CallOpts{Context: e.ctx, Pending: true, From: e.owner.From}, &out, method, args...)
	if err == nil {
		t.Fatalf("%s would settle; want a %s revert", method, want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("%s reverts with %q; want %s", method, err, want)
	}
	t.Logf("%s refused: %s", method, want)
}

// mineBlocks mines n empty blocks on the Hardhat node.
func mineBlocks(t *testing.T, e *pwEnv, n uint64) {
	t.Helper()
	if n == 0 {
		return
	}
	if err := e.client.Client().CallContext(e.ctx, nil, "hardhat_mine", hexutil.EncodeUint64(n)); err != nil {
		t.Fatalf("hardhat_mine %d: %v", n, err)
	}
}

type relayChannelPaymentRequest struct {
	VaultId      string    `json:"vaultId"`
	Proof        [8]string `json:"proof"`
	PublicSignal []string  `json:"publicSignal"`
	BlockNumber  uint64    `json:"blockNumber"`
	Tag          string    `json:"tag"`
	Ctxt         string    `json:"ctxt"`
}

func channelRelayRequest(p channelPaid, block uint64, tag [32]byte, ctxt []byte) relayChannelPaymentRequest {
	var proof [8]string
	copy(proof[:], p.res.Proof)
	sig := make([]string, len(p.res.Statement))
	for i, v := range p.res.Statement {
		sig[i] = v.String()
	}
	return relayChannelPaymentRequest{
		VaultId: "0", Proof: proof, PublicSignal: sig, BlockNumber: block,
		Tag: hexutil.Encode(tag[:]), Ctxt: hexutil.Encode(ctxt),
	}
}

// The relayer's POST /relay/channel_payment settles a channel payment as one
// transaction from its own address, refusing what the chain would refuse
// before sending anything: a request whose tag differs from the bound one,
// or whose block is not yet reached, spends no note and mines no block.
func TestRetailErc20_ChannelPaymentViaRelayer(t *testing.T) {
	e := newPartialWithdrawEnv(t)
	if !serverAvailable("localhost:8090") {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}
	dvpAddr := common.HexToAddress(loadOnchainReceipts(t)["EnygmaDvp"].ContractAddress)

	ss := make([]byte, 32)
	if _, err := rand.Read(ss); err != nil {
		t.Fatal(err)
	}
	bob, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	alice := newDepositNote(t, e, big.NewInt(50))
	head, err := e.client.BlockNumber(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	blockN := head + 1
	p := proveChannelPayment(t, e, ss, blockN, bob.PublicKey, []pwNote{alice}, big.NewInt(35), big.NewInt(15))

	post := func(t *testing.T, req relayChannelPaymentRequest) (relayPaymentResponse, int) {
		t.Helper()
		body, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		httpReq, err := http.NewRequest(http.MethodPost, relayerURL+"/relay/channel_payment", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+relayerAPIKey)
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			t.Fatalf("POST /relay/channel_payment: %v", err)
		}
		defer resp.Body.Close()
		var out relayPaymentResponse
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return out, resp.StatusCode
	}
	refused := func(t *testing.T, req relayChannelPaymentRequest, wantErr string) {
		t.Helper()
		out, status := post(t, req)
		if status != http.StatusUnprocessableEntity || !strings.Contains(out.Error, wantErr) {
			t.Fatalf("got %d %q; want 422 naming %s", status, out.Error, wantErr)
		}
		if now, _ := e.client.BlockNumber(e.ctx); now != head {
			t.Fatalf("the refused request mined a block (head %d, was %d)", now, head)
		}
	}

	t.Run("other_tag", func(t *testing.T) {
		tag := p.note.Tag
		tag[0] ^= 1
		refused(t, channelRelayRequest(p, blockN, tag, p.note.Ctxt), "CiphertextHashMismatch")
	})
	t.Run("block_not_reached", func(t *testing.T) {
		refused(t, channelRelayRequest(p, blockN+1, p.note.Tag, p.note.Ctxt), "ChannelPaymentOutsideWindow")
	})
	t.Run("ctxt_too_long", func(t *testing.T) {
		// The relayer pays for the calldata: it takes at most 4096 bytes.
		out, status := post(t, channelRelayRequest(p, blockN, p.note.Tag, make([]byte, 4097)))
		if status != http.StatusBadRequest || !strings.Contains(out.Error, "ctxt must be") {
			t.Fatalf("got %d %q; want 400 naming the ctxt cap", status, out.Error)
		}
	})

	out, status := post(t, channelRelayRequest(p, blockN, p.note.Tag, p.note.Ctxt))
	if status != http.StatusOK {
		t.Fatalf("POST /relay/channel_payment: %d %s", status, out.Error)
	}
	if out.BlockNumber != blockN {
		t.Fatalf("the payment landed in block %d, want block_n = %d", out.BlockNumber, blockN)
	}
	found, err := tags.ScanChannelPayments(e.ctx, e.client, dvpAddr, blockN, []tags.Channel{{SharedSecret: ss, PkSpend: bob.PublicKey}})
	if err != nil || len(found) != 1 || found[0].TxHash != common.HexToHash(out.TxHash) {
		t.Fatalf("Bob's scan found %d payments (err=%v), want the relayed one %s", len(found), err, out.TxHash)
	}
	note, err := found[0].Open()
	if err != nil || note.Amount.Cmp(big.NewInt(35)) != 0 {
		t.Fatalf("Bob opens the relayed payment to %v (err=%v), want 35", note, err)
	}
	t.Logf("the relayer settled the channel payment in block %d (tx %s); Bob found and opened it", out.BlockNumber, out.TxHash)
}
