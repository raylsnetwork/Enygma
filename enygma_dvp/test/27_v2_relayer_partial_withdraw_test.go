package tests

// Partial withdrawal through the relayer: POST /relay/partial_withdraw.
//
// The note's owner proves a PaymentRelayerFeePublic spend (1-in/3-out, VK
// slot 3) whose outputs are the withdrawal to the recipient, the owner's
// change note and the relayer's fee note, and hands it to the relayer. The
// relayer checks the request, then submits
// Erc20CoinVault.partialWithdrawWithRelayerFee from its own account, so the
// owner pays no gas: the relayer is paid from the note instead.
//
// TestV2Relayer_PartialWithdraw_FullFlow:
//
//  1. deposit 100
//  2. relay a withdrawal of 10 with a fee of 1 → recipient +10, the relayer's
//     account sent the transaction, a change note of 89 and a fee note of 1
//     are in the tree, and the change note decrypts to 89 for the owner
//  3. relay 20 more from the 89 note (fee 1) → a change note of 68
//  4. the relayer spends its two fee notes (opened with its fee key), showing
//     the fee is real value in the vault
//
// TestV2Relayer_PartialWithdraw_Rejections: the relayer refuses a fee note
// addressed to another key, a fee below RELAYER_MIN_FEE, a request whose
// recipient or amount differs from the proof, a payment proof (StMessage 0),
// a plain Payment receipt and a replay, and nothing it refuses spends the note. It also checks the vault
// itself rejects a direct partialWithdrawWithRelayerFee call with the wrong
// recipient.
//
// Prerequisites: the dedicated Payment deployment and the gnark server (see
// 12_v2_payment_usdr_fee_test.go), and the relayer on :8092 with
// RELAYER_FEE_SPEND_PRIVATE_KEY=123456789 and RELAYER_MIN_FEE=1:
//
//	cd relayer && \
//	  RELAYER_PRIVATE_KEY=9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda \ // gitleaks:allow (public Hardhat demo account, see enygmadvp.config.json)
//	  RELAYER_API_KEY=test-api-key-dev-only \
//	  RELAYER_DVP_ADDR=<from build/payment_receipts.json> \
//	  RELAYER_RECEIPTS_PATH=../build/payment_receipts.json \
//	  RELAYER_FEE_SPEND_PRIVATE_KEY=123456789 RELAYER_MIN_FEE=1 \
//	  RELAYER_PORT=8092 go run .
//
// Run:
//
//	cd test && CC=/usr/bin/clang go test -run TestV2Relayer_PartialWithdraw -v -timeout 600s

import (
	"encoding/hex"
	"math/big"
	"net/http"
	"strings"
	"testing"

	core "github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// relayerFeeSpendKey is the RELAYER_FEE_SPEND_PRIVATE_KEY the relayer is
// started with for these tests.
var relayerFeeSpendKey = big.NewInt(123456789)

type relayPartialWithdrawReq struct {
	VaultId    string                `json:"vaultId"`
	Receipt    paymentReceiptPayload `json:"receipt"`
	Recipient  string                `json:"recipient"`
	Amount     string                `json:"amount"`
	TokenId    string                `json:"tokenId"`
	FeeSalt    string                `json:"feeSalt"`
	CipherText string                `json:"cipherText"`
	EncTxData  string                `json:"encTxData"`
}

// relayerEnv is pwEnv plus the running relayer's fee key.
type relayerEnv struct {
	*pwEnv
	relayerAddr common.Address
	feePk       *big.Int
	vaultId     string
}

func newRelayerPartialWithdrawEnv(t *testing.T) *relayerEnv {
	t.Helper()
	e := newPartialWithdrawEnv(t)
	if !serverAvailable("localhost:8092") {
		t.Skip("relayer not running on localhost:8092 — skipping")
	}
	info := fetchPaymentRelayerInfo(t)
	if info.FeeSpendPubKey == "" {
		t.Skip("the relayer has no RELAYER_FEE_SPEND_PRIVATE_KEY — skipping")
	}
	feePk, ok := new(big.Int).SetString(info.FeeSpendPubKey, 10)
	if !ok {
		t.Fatalf("bad feeSpendPubKey from /relay/info: %q", info.FeeSpendPubKey)
	}
	want, err := poseidon.Hash([]*big.Int{relayerFeeSpendKey})
	if err != nil {
		t.Fatalf("poseidon.Hash: %v", err)
	}
	if feePk.Cmp(want) != 0 {
		t.Skipf("the relayer's fee key is not RELAYER_FEE_SPEND_PRIVATE_KEY=%s — skipping", relayerFeeSpendKey)
	}
	return &relayerEnv{
		pwEnv:       e,
		relayerAddr: common.HexToAddress(info.RelayerAddr),
		feePk:       feePk,
		vaultId:     "0", // the main payment-token vault (see init_payment)
	}
}

// proveRelayed builds a PaymentRelayerFeePublic withdrawal whose fee note is
// addressed to feePk.
func (r *relayerEnv) proveRelayed(t *testing.T, in pwNote, amount, change, fee, feePk *big.Int, recipient common.Address) *pwSpend {
	t.Helper()
	s, err := r.proveWithdrawWith(t, pwPaymentRelayerFeePublic, []pwNote{in}, amount, change, fee, feePk, recipient)
	if err != nil {
		t.Fatalf("gnark /proof/paymentRelayerFeePublic: %v", err)
	}
	return s
}

func (r *relayerEnv) request(s *pwSpend, recipient common.Address, amount *big.Int) relayPartialWithdrawReq {
	var proof [8]string
	copy(proof[:], s.proofStrs)
	sig := make([]string, len(s.receipt.Statement))
	for i, v := range s.receipt.Statement {
		sig[i] = v.String()
	}
	return relayPartialWithdrawReq{
		VaultId: r.vaultId,
		Receipt: paymentReceiptPayload{
			Proof:           proof,
			PublicSignal:    sig,
			NumberOfInputs:  int(s.receipt.NumberOfInputs.Int64()),
			NumberOfOutputs: int(s.receipt.NumberOfOutputs.Int64()),
		},
		Recipient:  recipient.Hex(),
		Amount:     amount.String(),
		TokenId:    r.tokenId.String(),
		FeeSalt:    s.feeSalt.String(),
		CipherText: hex.EncodeToString(s.ctI),
		EncTxData:  hex.EncodeToString(s.ctII),
	}
}

// relay posts a partial withdrawal and requires it to settle.
func (r *relayerEnv) relay(t *testing.T, s *pwSpend) *types.Receipt {
	t.Helper()
	resp, status := postToPaymentRelayer(t, "/relay/partial_withdraw", r.request(s, s.recipient, s.amount))
	if status != http.StatusOK {
		t.Fatalf("/relay/partial_withdraw: HTTP %d: %s", status, resp.Error)
	}
	mined, err := r.client.TransactionReceipt(r.ctx, common.HexToHash(resp.TxHash))
	if err != nil {
		t.Fatalf("TransactionReceipt(%s): %v", resp.TxHash, err)
	}
	if mined.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("relayed transaction %s failed", resp.TxHash)
	}
	if from := txSenderFor12(t, r.client, resp.TxHash); from != r.relayerAddr {
		t.Fatalf("relayed transaction was sent by %s, want the relayer %s", from.Hex(), r.relayerAddr.Hex())
	}
	tx, _, err := r.client.TransactionByHash(r.ctx, common.HexToHash(resp.TxHash))
	if err != nil {
		t.Fatalf("TransactionByHash(%s): %v", resp.TxHash, err)
	}
	if tx.To() == nil || *tx.To() != r.vaultAddr {
		t.Fatalf("relayed transaction went to %v, want the vault %s", tx.To(), r.vaultAddr.Hex())
	}
	return mined
}

// expectRelayRefused posts a request and requires the relayer to answer with
// wantStatus and an error containing wantMsg.
func (r *relayerEnv) expectRelayRefused(t *testing.T, req relayPartialWithdrawReq, wantStatus int, wantMsg string) {
	t.Helper()
	resp, status := postToPaymentRelayer(t, "/relay/partial_withdraw", req)
	if status != wantStatus || !strings.Contains(resp.Error, wantMsg) {
		t.Fatalf("relayer answered HTTP %d %q; want HTTP %d containing %q", status, resp.Error, wantStatus, wantMsg)
	}
	t.Logf("relayer refused as expected: HTTP %d %.100s", status, resp.Error)
}

// checkRelayedSettled is checkSettled plus the fee note: it must be in the
// tree and be a Commitment of this transaction.
func (r *relayerEnv) checkRelayedSettled(t *testing.T, mined *types.Receipt, s *pwSpend) {
	t.Helper()
	sawFee := false
	for _, lg := range mined.Logs {
		if lg.Address == r.vaultAddr && len(lg.Topics) >= 3 {
			if ev, err := r.vaultABI.EventByID(lg.Topics[0]); err == nil && ev.Name == "Commitment" && lg.Topics[2].Big().Cmp(s.feeCommit) == 0 {
				sawFee = true
			}
		}
	}
	if !sawFee {
		t.Errorf("no Commitment event for the relayer's fee note %s", s.feeCommit)
	}
	// checkSettled flags any Commitment other than the change note; drop the
	// fee note's before handing over.
	filtered := *mined
	filtered.Logs = nil
	for _, lg := range mined.Logs {
		if len(lg.Topics) >= 3 && lg.Topics[2].Big().Cmp(s.feeCommit) == 0 {
			continue
		}
		filtered.Logs = append(filtered.Logs, lg)
	}
	r.checkSettled(t, &filtered, s)
	mt := loadVaultMerkleTree(t, r.client, r.vaultAddr, partialWithdrawDepth)
	if _, err := mt.GenerateProof(s.feeCommit); err != nil {
		t.Errorf("the relayer's fee note is not in the vault's Merkle tree: %v", err)
	}
}

func TestV2Relayer_PartialWithdraw_FullFlow(t *testing.T) {
	r := newRelayerPartialWithdrawEnv(t)
	recipient := common.HexToAddress("0x0000000000000000000000000000000000005e81")
	recipientStart := r.balanceOf(t, recipient)
	vaultStart := r.balanceOf(t, r.vaultAddr)

	// 1. Deposit 100.
	note := newDepositNote(t, r.pwEnv, big.NewInt(100))
	ownerEthBefore, err := r.client.BalanceAt(r.ctx, r.owner.From, nil)
	if err != nil {
		t.Fatalf("BalanceAt: %v", err)
	}

	// 2. Relay a withdrawal of 10 with a fee of 1: 100 = 10 + 89 + 1.
	s1 := r.proveRelayed(t, note, big.NewInt(10), big.NewInt(89), big.NewInt(1), r.feePk, recipient)
	m1 := r.relay(t, s1)
	r.checkRelayedSettled(t, m1, s1)
	if got := new(big.Int).Sub(r.balanceOf(t, recipient), recipientStart); got.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("recipient received %s, want 10", got)
	}
	if got := new(big.Int).Sub(r.balanceOf(t, r.vaultAddr), vaultStart); got.Cmp(big.NewInt(90)) != 0 {
		t.Fatalf("vault holds %s of the deposit, want 90 (change 89 + fee 1)", got)
	}
	ownerEthAfter, err := r.client.BalanceAt(r.ctx, r.owner.From, nil)
	if err != nil {
		t.Fatalf("BalanceAt: %v", err)
	}
	if ownerEthAfter.Cmp(ownerEthBefore) != 0 {
		t.Fatalf("the owner's ETH balance changed (%s → %s): the owner paid gas", ownerEthBefore, ownerEthAfter)
	}
	t.Logf("relayed: withdrew 10, fee 1, change 89 (relayer paid %d gas, the owner none)", m1.GasUsed)

	// 3. Relay 20 more from the 89 note: 89 = 20 + 68 + 1.
	s2 := r.proveRelayed(t, s1.change, big.NewInt(20), big.NewInt(68), big.NewInt(1), r.feePk, recipient)
	m2 := r.relay(t, s2)
	r.checkRelayedSettled(t, m2, s2)
	if got := new(big.Int).Sub(r.balanceOf(t, recipient), recipientStart); got.Cmp(big.NewInt(30)) != 0 {
		t.Fatalf("recipient received %s in total, want 30", got)
	}
	t.Logf("relayed: withdrew 20 from the note of 89, fee 1, change 68")

	// 4. The relayer's fee notes are spendable with its fee key: withdraw both.
	relayerView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair: %v", err)
	}
	relayerSpend := &core.SpendKeyPair{PrivateKey: relayerFeeSpendKey, PublicKey: r.feePk}
	payout := common.HexToAddress("0x0000000000000000000000000000000000005e82")
	payoutStart := r.balanceOf(t, payout)
	for i, s := range []*pwSpend{s1, s2} {
		feeNote := pwNote{spend: relayerSpend, view: relayerView, salt: s.feeSalt, value: big.NewInt(1), commit: s.feeCommit}
		fs := r.mustProveWithdraw(t, feeNote, big.NewInt(1), big.NewInt(0), payout)
		tx, err := r.vault.Transact(r.owner, "withdrawV2", []*big.Int{big.NewInt(1), r.tokenId}, payout, fs.receipt)
		if err != nil {
			t.Fatalf("withdrawV2 of fee note %d: %v", i, err)
		}
		if mined, err := bind.WaitMined(r.ctx, r.client, tx); err != nil || mined.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("withdrawV2 of fee note %d reverted (receipt=%v err=%v)", i, mined, err)
		}
	}
	if got := new(big.Int).Sub(r.balanceOf(t, payout), payoutStart); got.Cmp(big.NewInt(2)) != 0 {
		t.Fatalf("the relayer withdrew %s from its fee notes, want 2", got)
	}
	if got := new(big.Int).Sub(r.balanceOf(t, r.vaultAddr), vaultStart); got.Cmp(big.NewInt(68)) != 0 {
		t.Fatalf("vault holds %s of the deposit, want the owner's 68", got)
	}
	t.Logf("the relayer withdrew its two fee notes (1 + 1); the owner's note of 68 remains")
}

func TestV2Relayer_PartialWithdraw_Rejections(t *testing.T) {
	r := newRelayerPartialWithdrawEnv(t)
	recipient := common.HexToAddress("0x0000000000000000000000000000000000005e83")
	note := newDepositNote(t, r.pwEnv, big.NewInt(100))

	t.Run("fee_note_for_another_key", func(t *testing.T) {
		other, err := core.NewSpendKeyPair()
		if err != nil {
			t.Fatalf("NewSpendKeyPair: %v", err)
		}
		s := r.proveRelayed(t, note, big.NewInt(10), big.NewInt(89), big.NewInt(1), other.PublicKey, recipient)
		r.expectRelayRefused(t, r.request(s, recipient, big.NewInt(10)), http.StatusPaymentRequired, "not addressed to this relayer")
	})
	t.Run("fee_below_minimum", func(t *testing.T) {
		s := r.proveRelayed(t, note, big.NewInt(10), big.NewInt(90), big.NewInt(0), r.feePk, recipient)
		r.expectRelayRefused(t, r.request(s, recipient, big.NewInt(10)), http.StatusPaymentRequired, "below the relayer's minimum")
	})

	s := r.proveRelayed(t, note, big.NewInt(10), big.NewInt(89), big.NewInt(1), r.feePk, recipient)
	t.Run("different_recipient", func(t *testing.T) {
		attacker := common.HexToAddress("0x000000000000000000000000000000000000bad1")
		r.expectRelayRefused(t, r.request(s, attacker, big.NewInt(10)), http.StatusBadRequest, "does not withdraw this amount to this recipient")
	})
	t.Run("different_amount", func(t *testing.T) {
		r.expectRelayRefused(t, r.request(s, recipient, big.NewInt(11)), http.StatusBadRequest, "does not withdraw this amount to this recipient")
	})
	t.Run("plain_payment_receipt", func(t *testing.T) {
		plain := r.mustProveWithdraw(t, note, big.NewInt(10), big.NewInt(90), recipient)
		req := r.request(plain, recipient, big.NewInt(10))
		req.FeeSalt = "1"
		r.expectRelayRefused(t, req, http.StatusBadRequest, "expected a 1-input/3-output")
	})
	t.Run("payment_message", func(t *testing.T) {
		// A payment proof (StMessage 0) is not a withdrawal.
		pay, err := r.proveWithdrawMsg(t, pwPaymentRelayerFeePublic, pwPaymentMessage, []pwNote{note}, big.NewInt(10), big.NewInt(89), big.NewInt(1), r.feePk, recipient)
		if err != nil {
			t.Fatalf("gnark /proof/paymentRelayerFeePublic (message 0): %v", err)
		}
		r.expectRelayRefused(t, r.request(pay, recipient, big.NewInt(10)), http.StatusBadRequest, "StMessage")
	})
	t.Run("vault_rejects_wrong_recipient_directly", func(t *testing.T) {
		attacker := common.HexToAddress("0x000000000000000000000000000000000000bad1")
		_, err := r.vault.Transact(r.owner, "partialWithdrawWithRelayerFee",
			[]*big.Int{big.NewInt(10), r.tokenId}, attacker, s.receipt, s.ctI, s.ctII)
		if err == nil || !strings.Contains(err.Error(), "InvalidOpening") {
			t.Fatalf("direct call with the wrong recipient: err=%v, want InvalidOpening", err)
		}
	})

	// None of the refusals spent the note: the honest request settles once.
	r.relay(t, s)
	t.Run("replay", func(t *testing.T) {
		r.expectRelayRefused(t, r.request(s, recipient, big.NewInt(10)), http.StatusBadRequest, "nullifier already spent")
	})
}
