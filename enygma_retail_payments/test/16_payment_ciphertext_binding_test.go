package tests

// A payment's note ciphertexts are bound into its proof (StCiphertextHash), so
// its receipt settles only with them. Before, they were plain arguments to
// EnygmaDvp.payment*, and the same receipt also settled through
// Erc20CoinVault.transfer (no ciphertexts at all) or transferV2 (any): whoever
// saw the receipt first, the relayer or a mempool watcher, could settle it with
// other ciphertexts. The sender's note was spent and the recipient's inserted,
// but the recipient's scan never found it, and the sender's own transaction
// reverted.
//
// Needs the retail stack (Hardhat :8545, gnark :8082), on the keys and
// contracts of this change.

import (
	"math/big"
	"testing"

	"enygma_retail_payments/layout"
	core "github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestRetailErc20_PaymentCiphertextsBoundToTheProof(t *testing.T) {
	e := newPartialWithdrawEnv(t)
	receipts := loadOnchainReceipts(t)
	dvp := bind.NewBoundContract(common.HexToAddress(receipts["EnygmaDvp"].ContractAddress),
		loadOnchainABI(t, "EnygmaDvp"), e.client, e.client, e.client)

	alice := newDepositNote(t, e, big.NewInt(100))
	bobSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	bobView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair: %v", err)
	}

	mp, err := rebuildVaultTree(t, e.client, e.vaultAddr, nil).GenerateProof(alice.commit)
	if err != nil {
		t.Fatalf("GenerateProof: %v", err)
	}
	res, err := core.NewGnarkClient(gnarkURL).BoundPaymentProof(vaultInt(e.vaultAddr), big.NewInt(0),
		[]*big.Int{alice.value}, []core.KeyPair{{PrivateKey: alice.spend.PrivateKey, PublicKey: alice.spend.PublicKey}},
		[]*big.Int{alice.salt}, []*big.Int{big.NewInt(60), big.NewInt(40)},
		[]*big.Int{bobSpend.PublicKey, alice.spend.PublicKey}, [][]byte{bobView.EncapsKey, alice.view.EncapsKey},
		partialWithdrawDepth, []*core.MerkleProof{mp}, []*big.Int{big.NewInt(int64(mp.TreeNumber))}, e.tokenId)
	if err != nil {
		t.Fatalf("BoundPaymentProof: %v", err)
	}
	receipt := onchainProofReceipt{
		Proof:           proofStringsToOnchain(t, res.Proof),
		Statement:       res.ContractStatement(),
		NumberOfInputs:  big.NewInt(int64(res.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(res.NumberOfOutputs)),
	}
	if got, want := res.Statement[layout.PaymentCiphertextHashOffset], core.PaymentCiphertextHash([][]byte{res.CipherText}, [][]byte{res.EncTxData}); got.Cmp(want) != 0 {
		t.Fatalf("the proof binds ciphertext hash %s, want %s", got, want)
	}

	junk := append([]byte{}, res.CipherText...)
	junk[0] ^= 1
	t.Run("payment_with_other_ciphertexts", func(t *testing.T) {
		expectCallRevert(t, e, dvp, "CiphertextHashMismatch", "payment", receipt, big.NewInt(0), junk, res.EncTxData)
		expectCallRevert(t, e, dvp, "CiphertextHashMismatch", "payment", receipt, big.NewInt(0), []byte{}, []byte{})
	})
	t.Run("vault_transfer_without_ciphertexts", func(t *testing.T) {
		expectCallRevert(t, e, e.vault, "CiphertextHashMismatch", "transfer", receipt)
	})
	t.Run("vault_transferV2_with_chosen_ciphertexts", func(t *testing.T) {
		expectCallRevert(t, e, e.vault, "CiphertextHashMismatch", "transferV2", receipt,
			[][]byte{junk, {1}}, [][]byte{res.EncTxData, {1}})
		// Even the payment's own ciphertexts, as output 0's in transferV2's
		// per-output form: the encoding differs, so the hash does.
		expectCallRevert(t, e, e.vault, "CiphertextHashMismatch", "transferV2", receipt,
			[][]byte{res.CipherText, {}}, [][]byte{res.EncTxData, {}})
	})

	// None of the refusals spent the note: the sender's own payment settles,
	// and its Payment event carries the bound ciphertexts, which Bob's scan
	// opens.
	tx, err := dvp.Transact(e.owner, "payment", receipt, big.NewInt(0), res.CipherText, res.EncTxData)
	if err != nil {
		t.Fatalf("payment with the bound ciphertexts: %v", err)
	}
	mined, err := bind.WaitMined(e.ctx, e.client, tx)
	if err != nil || mined.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("payment with the bound ciphertexts did not settle (err=%v)", err)
	}
	paymentSig := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	dvpABI := loadOnchainABI(t, "EnygmaDvp")
	var events []core.OnChainErc20Event
	for _, l := range mined.Logs {
		if len(l.Topics) == 0 || l.Topics[0] != paymentSig {
			continue
		}
		out, err := dvpABI.Unpack("Payment", l.Data)
		if err != nil {
			t.Fatalf("unpack Payment: %v", err)
		}
		events = append(events, core.OnChainErc20Event{Commitment: l.Topics[2].Big(), CipherText: out[0].([]byte), EncTxData: out[1].([]byte)})
	}
	notes, err := core.ScanForErc20Notes(bobView.DecapsKey, bobSpend.PublicKey, events)
	if err != nil {
		t.Fatalf("ScanForErc20Notes: %v", err)
	}
	if len(notes) != 1 || notes[0].Amount.Cmp(big.NewInt(60)) != 0 {
		t.Fatalf("Bob's scan of the Payment event found %d notes, want his 60", len(notes))
	}
	t.Logf("payment settled with its bound ciphertexts; Bob's scan found his note of %s", notes[0].Amount)
}
