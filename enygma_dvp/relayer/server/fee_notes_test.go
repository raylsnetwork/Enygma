package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"enygma_dvp/relayer/config"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

func readFeeNotes(t *testing.T, path string) []feeNoteRecord {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fee notes: %v", err)
	}
	defer f.Close()
	var out []feeNoteRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r feeNoteRecord
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("bad fee note line %q: %v", sc.Text(), err)
		}
		out = append(out, r)
	}
	return out
}

func TestFeeNoteStore_AppendsOpeningsReadableOnlyByTheRelayer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fee_notes.jsonl")
	s := newFeeNoteStore(path)
	if err := s.recordSubmitted("paymentWithRelayerFee", big.NewInt(0), big.NewInt(111), big.NewInt(222), big.NewInt(5), big.NewInt(7)); err != nil {
		t.Fatal(err)
	}
	if err := s.recordMined("paymentWithRelayerFee", big.NewInt(0), big.NewInt(111), "0xabc", 9); err != nil {
		t.Fatal(err)
	}
	notes := readFeeNotes(t, path)
	if len(notes) != 2 || notes[0].Status != "submitted" || notes[0].Salt != "222" || notes[0].Commitment != "111" ||
		notes[0].Amount != "5" || notes[0].TokenId != "7" || notes[1].Status != "mined" || notes[1].TxHash != "0xabc" {
		t.Fatalf("unexpected records: %+v", notes)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("fee note store permissions %o, want 600", perm)
	}
}

// feeRelayFixture is a /relay/payment_relayer_fee request whose fee note is
// addressed to the handler's fee key, against a node that accepts it.
type feeRelayFixture struct {
	h       *Handler
	node    *mockNode
	r       *gin.Engine
	body    []byte
	salt    *big.Int
	feeCmt  *big.Int
	notesAt string
}

func newFeeRelayFixture(t *testing.T, notesPath string) *feeRelayFixture {
	t.Helper()
	dvpABI, err := loadABIFromArtifact("../../artifacts/contracts/core/contracts/EnygmaDvp.sol/EnygmaDvp.json")
	if err != nil {
		t.Skipf("EnygmaDvp artifact missing (run `npx hardhat compile`): %v", err)
	}
	vaultABI, err := loadABIFromArtifact("../../artifacts/contracts/core/contracts/vaults/AbstractCoinVault.sol/AbstractCoinVault.json")
	if err != nil {
		t.Skipf("AbstractCoinVault artifact missing (run `npx hardhat compile`): %v", err)
	}
	vaultAddr := common.HexToAddress("0x00000000000000000000000000000000000000a1")
	word := func(b []byte) string { return "0x" + common.Bytes2Hex(common.LeftPadBytes(b, 32)) }
	node := &mockNode{callReply: func(data []byte) (string, bool) {
		if len(data) < 4 {
			return "", false
		}
		switch {
		case bytes.Equal(data[:4], dvpABI.Methods["vaultById"].ID):
			return word(vaultAddr.Bytes()), true
		case bytes.Equal(data[:4], vaultABI.Methods["rootHistory"].ID):
			return word([]byte{1}), true // known root
		case bytes.Equal(data[:4], vaultABI.Methods["nullifiers"].ID):
			return word([]byte{0}), true // unspent
		}
		return "", false
	}}
	srv := node.serve(t)
	t.Cleanup(srv.Close)
	h := newTestHandler(t, srv.URL, 8_000_000)
	h.dvpABI, h.vaultABI = dvpABI, vaultABI

	feeKey := big.NewInt(424242)
	h.feeSpendPubKey, err = poseidon.Hash([]*big.Int{feeKey})
	if err != nil {
		t.Fatal(err)
	}
	h.cfg = &config.Config{MinFee: big.NewInt(0), FeeNotesPath: notesPath}
	h.feeNotes = newFeeNoteStore(notesPath)

	salt, fee, tokenId := big.NewInt(987654321), big.NewInt(5), big.NewInt(0)
	feeCmt, err := poseidon.Hash([]*big.Int{h.feeSpendPubKey, salt, fee, tokenId})
	if err != nil {
		t.Fatal(err)
	}
	// [msg, treeNum0, root0, nf0, cmtBob, cmtChange, cmtRelayer, contractAddr, fee]
	signal := []string{"0", "0", "1", "5", "6", "7", feeCmt.String(), new(big.Int).SetBytes(vaultAddr.Bytes()).String(), fee.String()}
	body, _ := json.Marshal(RelayPaymentRelayerFeeRequest{
		VaultId: "0",
		Receipt: ReceiptPayload{
			Proof:        [8]string{"1", "2", "3", "4", "5", "6", "7", "8"},
			PublicSignal: signal, NumberOfInputs: 1, NumberOfOutputs: 3,
		},
		CipherText: "0x00", EncTxData: "0x00", FeeSalt: salt.String(), TokenId: tokenId.String(),
	})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/relay/payment_relayer_fee", h.RelayPaymentRelayerFee)
	return &feeRelayFixture{h: h, node: node, r: r, body: body, salt: salt, feeCmt: feeCmt, notesAt: notesPath}
}

func (f *feeRelayFixture) post() *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	f.r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/relay/payment_relayer_fee", bytes.NewReader(f.body)))
	return w
}

// The fee note's salt reaches the relayer only through the request; it must be
// kept, or the note the relayer is paid with can never be spent.
func TestRelayPaymentRelayerFee_KeepsTheFeeNoteOpening(t *testing.T) {
	f := newFeeRelayFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
	if w := f.post(); w.Code != http.StatusOK {
		t.Fatalf("relay: got %d: %s", w.Code, w.Body.String())
	}
	notes := readFeeNotes(t, f.notesAt)
	if len(notes) != 2 {
		t.Fatalf("want a submitted and a mined record, got %+v", notes)
	}
	if notes[0].Status != "submitted" || notes[0].Salt != f.salt.String() || notes[0].Commitment != f.feeCmt.String() ||
		notes[0].Amount != "5" || notes[0].TokenId != "0" || notes[0].VaultId != "0" {
		t.Fatalf("submitted record does not hold the fee note's opening: %+v", notes[0])
	}
	if notes[1].Status != "mined" || notes[1].Commitment != f.feeCmt.String() || !strings.HasPrefix(notes[1].TxHash, "0x") {
		t.Fatalf("mined record: %+v", notes[1])
	}
}

// If the opening cannot be kept, the relayer must not relay: it would be paid
// with a note it can never spend.
func TestRelayPaymentRelayerFee_RefusesWhenTheOpeningCannotBeKept(t *testing.T) {
	f := newFeeRelayFixture(t, filepath.Join(t.TempDir(), "missing-dir", "fee_notes.jsonl"))
	if w := f.post(); w.Code != http.StatusInternalServerError {
		t.Fatalf("relay with an unwritable fee note store: got %d, want 500: %s", w.Code, w.Body.String())
	}
	if f.node.called("eth_sendRawTransaction") {
		t.Fatal("the relayer sent the transaction although it could not keep the fee note's opening")
	}
	nfKey := common.HexToAddress("0x00000000000000000000000000000000000000a1").Hex() + ":0:5" // vault:tree:nullifier, as claimNullifiers keys it
	if _, held := f.h.inFlight.Load(nfKey); held {
		t.Fatal("the nullifier stayed claimed after the refusal")
	}
}
