package tests

// 25_v2_relayer_fee_note_store_test.go
//
// A relayer fee note is Poseidon(feeSpendPubKey, salt, amount, tokenId) with a
// client-chosen salt that reaches the relayer only in the relay request. The
// relayer used to check the note against it and discard it, so it could never
// spend a fee it earned. It now keeps each note's opening in its fee note store
// (RELAYER_FEE_NOTES_PATH, default relayer/fee_notes.jsonl).
//
// spendFeeNoteFromRelayerStore is the end-to-end check, called from
// TestV2Payment_RelayerFeeAndUsdrFee after each fee relay: it reads the
// opening the relayer kept (not the client's copy of the salt) and spends the
// fee note with it on chain. It needs the relayer's fee spend key, which only
// the relayer holds; the test reads it from RELAYER_FEE_SPEND_PRIVATE_KEY, the
// same variable the relayer is started with.

import (
	"bufio"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"

	core "github.com/raylsnetwork/enygma_dvp/src/core"
)

type relayerFeeNote struct {
	Status     string `json:"status"`
	Route      string `json:"route"`
	VaultId    string `json:"vaultId"`
	Commitment string `json:"commitment"`
	Salt       string `json:"salt"`
	Amount     string `json:"amount"`
	TokenId    string `json:"tokenId"`
	TxHash     string `json:"txHash"`
}

// relayerFeeNotes reads the relayer's fee note store.
func relayerFeeNotes(t *testing.T) []relayerFeeNote {
	t.Helper()
	path := os.Getenv("RELAYER_FEE_NOTES_PATH")
	if path == "" {
		path = "../relayer/fee_notes.jsonl"
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open the relayer's fee note store %s: %v", path, err)
	}
	defer f.Close()
	var out []relayerFeeNote
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var n relayerFeeNote
		if err := json.Unmarshal(sc.Bytes(), &n); err != nil {
			t.Fatalf("bad fee note store line %q: %v", sc.Text(), err)
		}
		out = append(out, n)
	}
	return out
}

func mustBig(t *testing.T, s, what string) *big.Int {
	t.Helper()
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		t.Fatalf("fee note store: bad %s %q", what, s)
	}
	return v
}

// spendFeeNoteFromRelayerStore checks the relayer kept the opening of the fee
// note cmt (paid in the transaction txHash) and that the opening spends it: it
// proves a payment of the note's full value from vault using only the stored
// salt, amount and tokenId and the relayer's fee spend key, submits it, and
// checks the note's nullifier is then spent.
func spendFeeNoteFromRelayerStore(
	t *testing.T,
	ctx context.Context,
	client *ethclient.Client,
	owner *bind.TransactOpts,
	gnarkClient *core.GnarkClient,
	vault *bind.BoundContract,
	vaultAddr common.Address,
	merkleDepth int,
	cmt *big.Int,
	txHash string,
) {
	t.Helper()
	keyStr := os.Getenv("RELAYER_FEE_SPEND_PRIVATE_KEY")
	if keyStr == "" {
		t.Skip("RELAYER_FEE_SPEND_PRIVATE_KEY not set — cannot spend the relayer's fee note")
	}
	feeKey := mustBig(t, keyStr, "RELAYER_FEE_SPEND_PRIVATE_KEY")
	feePub, err := poseidon.Hash([]*big.Int{feeKey})
	if err != nil {
		t.Fatal(err)
	}

	// 1. The relayer kept the opening, and recorded the transaction that paid it.
	var kept *relayerFeeNote
	mined := false
	notes := relayerFeeNotes(t)
	for i := range notes {
		n := notes[i]
		if n.Commitment != cmt.String() {
			continue
		}
		if n.Status == "submitted" {
			kept = &notes[i]
		}
		if n.Status == "mined" && n.TxHash == txHash {
			mined = true
		}
	}
	if kept == nil {
		t.Fatalf("FEE STRANDED: the relayer kept no opening for its fee note %s", cmt)
	}
	if !mined {
		t.Errorf("the relayer's fee note store has no mined record for tx %s", txHash)
	}
	salt, amount, tokenId := mustBig(t, kept.Salt, "salt"), mustBig(t, kept.Amount, "amount"), mustBig(t, kept.TokenId, "tokenId")
	opened, err := core.Erc20CommitmentV2(feePub, salt, amount, tokenId)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Cmp(cmt) != 0 {
		t.Fatalf("the stored opening does not open the fee note: Poseidon(pk, salt, amount, tokenId) = %s, want %s", opened, cmt)
	}

	// 2. The opening spends it: the relayer pays the whole note to itself.
	mt := loadVaultMerkleTree(t, client, vaultAddr, merkleDepth)
	mp, err := mt.GenerateProof(cmt)
	if err != nil {
		t.Fatalf("the fee note is not in the vault's tree: %v", err)
	}
	view, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	relayer := core.KeyPair{PrivateKey: feeKey, PublicKey: feePub}
	keep := new(big.Int).Sub(amount, big.NewInt(1))
	pay, err := gnarkClient.BoundPaymentProof(
		new(big.Int).SetBytes(vaultAddr.Bytes()), big.NewInt(0),
		[]*big.Int{amount}, []core.KeyPair{relayer}, []*big.Int{salt},
		[]*big.Int{keep, big.NewInt(1)}, []*big.Int{feePub, feePub}, [][]byte{view.EncapsKey, view.EncapsKey},
		merkleDepth, []*core.MerkleProof{mp}, []*big.Int{big.NewInt(int64(mp.TreeNumber))}, tokenId,
	)
	if err != nil {
		t.Fatalf("FEE STRANDED: cannot prove a spend of the fee note from the stored opening: %v", err)
	}
	stmt := pay.ContractStatement()
	tx, err := vault.Transact(owner, "transfer", onchainProofReceipt{
		Proof: proofStringsToOnchain(t, pay.Proof), Statement: stmt,
		NumberOfInputs: big.NewInt(1), NumberOfOutputs: big.NewInt(2),
	})
	if err != nil {
		t.Fatalf("FEE STRANDED: spending the fee note with the stored opening was refused: %v", err)
	}
	if r, err := bind.WaitMined(ctx, client, tx); err != nil || r.Status != 1 {
		t.Fatalf("FEE STRANDED: spending the fee note did not succeed (err=%v)", err)
	}
	var out []interface{}
	if err := vault.Call(&bind.CallOpts{}, &out, "nullifiers", big.NewInt(int64(mp.TreeNumber)), stmt[3]); err != nil {
		t.Fatalf("nullifiers(): %v", err)
	}
	if spent, _ := out[0].(bool); !spent {
		t.Fatal("the fee note's nullifier is not spent after the spend")
	}
	t.Logf("  relayer spent its fee note (amount=%s) using only the opening it kept ✓", amount)
}
