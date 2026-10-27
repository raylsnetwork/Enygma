package server

// The relayer keeps the opening of every fee note it is paid with: the client
// chooses a fee note's salt and the relayer learns it only from the request,
// so a salt it does not keep leaves the note unspendable.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"enygma_retail_payments/layout"
	"enygma_retail_payments/relayer/config"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

func readFeeNotes(t *testing.T, path string) []feeNoteRecord {
	t.Helper()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("open fee notes: %v", err)
	}
	defer f.Close()
	var out []feeNoteRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r feeNoteRecord
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("parse fee note line %q: %v", sc.Text(), err)
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

// feeNode is a JSON-RPC node that knows every root, has spent no nullifier,
// maps vault ids to vaults as vaults says, accepts transactions and mines them
// at once. With revertCalls set, every other eth_call (a simulation) reverts.
type feeNode struct {
	mu          sync.Mutex
	calls       []string
	revertCalls bool
	rootHistory []byte
	nullifiers  []byte
	vaultById   []byte
	vaults      map[uint64]common.Address
	// unknownRoots and spentNullifiers make rootHistory / nullifiers answer
	// "unknown" / "spent" for those values; every other root is known and
	// every other nullifier unspent.
	unknownRoots    map[uint64]bool
	spentNullifiers map[uint64]bool
	sent            []byte // calldata of the last transaction sent
}

func (n *feeNode) called(method string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, c := range n.calls {
		if c == method {
			return true
		}
	}
	return false
}

func (n *feeNode) serve(t *testing.T) *httptest.Server {
	zero32 := "0x" + strings.Repeat("00", 32)
	header := map[string]interface{}{
		"parentHash": zero32, "sha3Uncles": zero32, "miner": "0x" + strings.Repeat("00", 20),
		"stateRoot": zero32, "transactionsRoot": zero32, "receiptsRoot": zero32,
		"logsBloom": "0x" + strings.Repeat("00", 256), "difficulty": "0x0", "number": "0x1",
		"gasLimit": "0x1c9c380", "gasUsed": "0x0", "timestamp": "0x1", "extraData": "0x",
		"baseFeePerGas": "0x1", "hash": zero32,
	}
	word := func(b byte) string { return "0x" + strings.Repeat("00", 31) + common.Bytes2Hex([]byte{b}) }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		n.mu.Lock()
		n.calls = append(n.calls, req.Method)
		n.mu.Unlock()
		resp := map[string]interface{}{"jsonrpc": "2.0", "id": req.ID}
		switch req.Method {
		case "eth_chainId":
			resp["result"] = "0x539"
		case "eth_blockNumber":
			resp["result"] = "0x1"
		case "eth_getTransactionCount":
			resp["result"] = "0x0"
		case "eth_gasPrice", "eth_maxPriorityFeePerGas":
			resp["result"] = "0x1"
		case "eth_getBlockByNumber":
			resp["result"] = header
		case "eth_call":
			var msg struct {
				Input string `json:"input"`
				Data  string `json:"data"`
			}
			_ = json.Unmarshal(req.Params[0], &msg)
			data := common.FromHex(msg.Input + msg.Data)
			switch {
			case len(data) >= 68 && bytes.Equal(data[:4], n.rootHistory):
				if n.unknownRoots[new(big.Int).SetBytes(data[36:68]).Uint64()] {
					resp["result"] = word(0)
				} else {
					resp["result"] = word(1) // known root
				}
			case len(data) >= 68 && bytes.Equal(data[:4], n.nullifiers):
				if n.spentNullifiers[new(big.Int).SetBytes(data[36:68]).Uint64()] {
					resp["result"] = word(1)
				} else {
					resp["result"] = word(0) // unspent
				}
			case len(data) >= 36 && bytes.Equal(data[:4], n.vaultById):
				addr := n.vaults[new(big.Int).SetBytes(data[4:36]).Uint64()] // zero if unregistered
				resp["result"] = "0x" + common.Bytes2Hex(common.LeftPadBytes(addr.Bytes(), 32))
			case n.revertCalls:
				resp["error"] = map[string]interface{}{"code": 3, "message": "execution reverted", "data": "0x"}
			default:
				resp["result"] = "0x"
			}
		case "eth_sendRawTransaction":
			var raw string
			_ = json.Unmarshal(req.Params[0], &raw)
			var tx types.Transaction
			_ = tx.UnmarshalBinary(common.FromHex(raw))
			n.mu.Lock()
			n.sent = tx.Data()
			n.mu.Unlock()
			resp["result"] = tx.Hash().Hex()
		case "eth_getTransactionReceipt":
			resp["result"] = map[string]interface{}{
				"transactionHash": zero32, "transactionIndex": "0x0", "blockHash": zero32, "blockNumber": "0x2",
				"cumulativeGasUsed": "0x5208", "gasUsed": "0x5208", "status": "0x1",
				"logs": []interface{}{}, "logsBloom": "0x" + strings.Repeat("00", 256), "type": "0x2",
				"effectiveGasPrice": "0x1", "contractAddress": nil,
			}
		}
		out, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// feeFixture is a handler paid with fee notes under feeKey, on a feeNode.
type feeFixture struct {
	h         *Handler
	node      *feeNode
	r         *gin.Engine
	notesAt   string
	vaultAddr common.Address
}

func newFeeFixture(t *testing.T, notesPath string) *feeFixture {
	t.Helper()
	dvpABI, err := loadABIFromFile("../../contracts/abis/EnygmaDvp.json")
	if err != nil {
		t.Skipf("EnygmaDvp ABI missing (run setup.sh): %v", err)
	}
	vaultABI, err := loadABIFromFile("../../contracts/abis/Erc20CoinVault.json")
	if err != nil {
		t.Skipf("Erc20CoinVault ABI missing (run setup.sh): %v", err)
	}
	vaultAddr := common.HexToAddress("0x00000000000000000000000000000000000000a1")
	usdrVaultAddr := common.HexToAddress("0x00000000000000000000000000000000000000b2")
	node := &feeNode{
		rootHistory: vaultABI.Methods["rootHistory"].ID, nullifiers: vaultABI.Methods["nullifiers"].ID,
		vaultById: dvpABI.Methods["vaultById"].ID,
		// 2 is another vault registered on the same EnygmaDvp.
		vaults: map[uint64]common.Address{0: vaultAddr, 2: common.HexToAddress("0x00000000000000000000000000000000000000c7"), 4: usdrVaultAddr},
	}
	client, err := ethclient.Dial(node.serve(t).URL)
	if err != nil {
		t.Fatal(err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	auth.GasLimit = 8_000_000
	feeSpendPubKey, err := poseidon.Hash([]*big.Int{big.NewInt(424242)})
	if err != nil {
		t.Fatal(err)
	}
	f := &feeFixture{node: node, notesAt: notesPath, vaultAddr: vaultAddr}
	f.h = &Handler{
		cfg:            &config.Config{MinFee: big.NewInt(0), FeeNotesPath: notesPath},
		dvpABI:         dvpABI,
		vaultABI:       vaultABI,
		dvpAddr:        common.HexToAddress("0x00000000000000000000000000000000000000d1"),
		vaultAddr:      f.vaultAddr,
		usdrVaultAddr:  usdrVaultAddr,
		auth:           auth,
		client:         client,
		feeSpendPubKey: feeSpendPubKey,
		feeNotes:       newFeeNoteStore(notesPath),
	}
	gin.SetMode(gin.TestMode)
	f.r = gin.New()
	f.r.POST("/relay/payment", f.h.RelayPayment)
	f.r.POST("/relay/payment_relayer_fee", f.h.RelayPaymentRelayerFee)
	f.r.POST("/relay/partial_withdraw", f.h.RelayPartialWithdraw)
	f.r.POST("/relay/payment_usdr_fee", f.h.RelayPaymentUsdrFee)
	return f
}

func (f *feeFixture) feeCmt(t *testing.T, salt, fee, tokenId *big.Int) *big.Int {
	t.Helper()
	c, err := poseidon.Hash([]*big.Int{f.h.feeSpendPubKey, salt, fee, tokenId})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var proof8 = [8]string{"1", "2", "3", "4", "5", "6", "7", "8"}

// paymentSignal is a Payment statement spending nullifier 55 of tree 0
// (root 1) into outputs 6 and 7, bound to vault.
func paymentSignal(vault common.Address) []string {
	s := make([]string, layout.PaymentLen)
	s[layout.PaymentMessageOffset] = "0"
	s[layout.PaymentTreeNumbersOffset] = "0"
	s[layout.PaymentMerkleRootsOffset] = "1"
	s[layout.PaymentNullifiersOffset] = "55"
	s[layout.PaymentCommitmentsOutOffset] = "6"
	s[layout.PaymentCommitmentsOutOffset+1] = "7"
	s[layout.PaymentContractAddressOffset] = new(big.Int).SetBytes(vault.Bytes()).String()
	return s
}

// relayerFeeRequest is a PaymentRelayerFeePublic request with message msg and
// output[0] out0, paying fee 5 with salt to this relayer.
func (f *feeFixture) relayerFeeRequest(t *testing.T, msg string, out0, salt *big.Int) RelayPaymentRelayerFeeRequest {
	fee, tokenId := big.NewInt(5), big.NewInt(0)
	var signal [layout.PaymentRelayerFeeLen]string
	signal[layout.PaymentRelayerFeeMessageOffset] = msg
	signal[layout.PaymentRelayerFeeTreeNumbersOffset] = "0"
	signal[layout.PaymentRelayerFeeMerkleRootsOffset] = "1"
	signal[layout.PaymentRelayerFeeNullifiersOffset] = "55"
	signal[layout.PaymentRelayerFeeCommitmentsOutOffset] = out0.String()
	signal[layout.PaymentRelayerFeeCommitmentsOutOffset+1] = "7"
	signal[layout.PaymentRelayerFeeCommitmentsOutOffset+2] = f.feeCmt(t, salt, fee, tokenId).String()
	signal[layout.PaymentRelayerFeeContractAddressOffset] = new(big.Int).SetBytes(f.vaultAddr.Bytes()).String()
	signal[layout.PaymentRelayerFeeFeeOffset] = fee.String()
	return RelayPaymentRelayerFeeRequest{
		VaultId: "0", Proof: proof8, PublicSignal: signal,
		CipherText: "0x00", EncTxData: "0x00", FeeSalt: salt.String(), TokenId: tokenId.String(),
	}
}

func (f *feeFixture) partialWithdrawRequest(t *testing.T, salt *big.Int) RelayPartialWithdrawRequest {
	recipient := common.HexToAddress("0x00000000000000000000000000000000000000e5")
	amount := big.NewInt(10)
	out0, err := poseidon.Hash([]*big.Int{new(big.Int).SetBytes(recipient.Bytes()), big.NewInt(0), amount, big.NewInt(0)})
	if err != nil {
		t.Fatal(err)
	}
	return RelayPartialWithdrawRequest{
		RelayPaymentRelayerFeeRequest: f.relayerFeeRequest(t, "1", out0, salt),
		Recipient:                     recipient.Hex(),
		Amount:                        amount.String(),
	}
}

// checkKept checks the store holds a submitted record with the fee note's
// opening, then a mined record for the same commitment.
func (f *feeFixture) checkKept(t *testing.T, route, vaultId string, cmt, salt *big.Int) {
	t.Helper()
	notes := readFeeNotes(t, f.notesAt)
	if len(notes) != 2 {
		t.Fatalf("want a submitted and a mined record, got %+v", notes)
	}
	if s := notes[0]; s.Status != "submitted" || s.Route != route || s.VaultId != vaultId ||
		s.Commitment != cmt.String() || s.Salt != salt.String() || s.Amount != "5" || s.TokenId != "0" {
		t.Fatalf("submitted record does not hold the fee note's opening: %+v", s)
	}
	if m := notes[1]; m.Status != "mined" || m.Commitment != cmt.String() || !strings.HasPrefix(m.TxHash, "0x") {
		t.Fatalf("mined record: %+v", m)
	}
}

func TestRelayPaymentRelayerFee_KeepsTheFeeNoteOpening(t *testing.T) {
	f := newFeeFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
	salt := big.NewInt(987654321)
	req := f.relayerFeeRequest(t, "0", big.NewInt(6), salt)
	if w := postJSON(f.r, "/relay/payment_relayer_fee", req); w.Code != http.StatusOK {
		t.Fatalf("relay: got %d: %s", w.Code, w.Body.String())
	}
	f.checkKept(t, "paymentWithRelayerFee", "0", f.feeCmt(t, salt, big.NewInt(5), big.NewInt(0)), salt)
}

func TestRelayPartialWithdraw_KeepsTheFeeNoteOpening(t *testing.T) {
	f := newFeeFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
	salt := big.NewInt(123123123)
	if w := postJSON(f.r, "/relay/partial_withdraw", f.partialWithdrawRequest(t, salt)); w.Code != http.StatusOK {
		t.Fatalf("relay: got %d: %s", w.Code, w.Body.String())
	}
	f.checkKept(t, "partialWithdrawWithRelayerFee", "0", f.feeCmt(t, salt, big.NewInt(5), big.NewInt(0)), salt)
}

func (f *feeFixture) usdrFeeRequest(t *testing.T, salt *big.Int) RelayPaymentUsdrFeeRequest {
	fee, tokenId := big.NewInt(5), big.NewInt(0)
	cmtFee := f.feeCmt(t, salt, fee, tokenId)
	var usdr [layout.UsdrFeeLen]string
	usdr[layout.UsdrFeeMessageOffset] = "0"
	usdr[layout.UsdrFeeTreeNumbersOffset] = "0"
	usdr[layout.UsdrFeeMerkleRootsOffset] = "1"
	usdr[layout.UsdrFeeNullifiersOffset] = "66"
	usdr[layout.UsdrFeeCommitmentsOutOffset] = cmtFee.String()
	usdr[layout.UsdrFeeCommitmentsOutOffset+1] = "8"
	usdr[layout.UsdrFeeContractAddressOffset] = new(big.Int).SetBytes(f.h.usdrVaultAddr.Bytes()).String()
	usdr[layout.UsdrFeeFeeOffset] = fee.String()
	usdr[layout.UsdrFeeTokenIdOffset] = tokenId.String()
	return RelayPaymentUsdrFeeRequest{
		VaultId: "0", Proof: proof8,
		PublicSignal: paymentSignal(f.vaultAddr),
		CipherText:   "0x00", EncTxData: "0x00",
		UsdrVaultId: "4", UsdrProof: proof8,
		UsdrPublicSignal: usdr,
		UsdrCipherText:   "0x00", UsdrEncTxData: "0x00", UsdrFeeSalt: salt.String(),
	}
}

func TestRelayPaymentUsdrFee_KeepsTheFeeNoteOpening(t *testing.T) {
	f := newFeeFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
	salt := big.NewInt(555666777)
	cmtFee := f.feeCmt(t, salt, big.NewInt(5), big.NewInt(0))
	req := f.usdrFeeRequest(t, salt)
	if w := postJSON(f.r, "/relay/payment_usdr_fee", req); w.Code != http.StatusOK {
		t.Fatalf("relay: got %d: %s", w.Code, w.Body.String())
	}
	f.checkKept(t, "paymentWithUsdrFee", "4", cmtFee, salt)
}

// If the opening cannot be kept, the relayer must not relay: it would be paid
// with a note it can never spend.
func TestRelayFeeRoutes_RefuseWhenTheOpeningCannotBeKept(t *testing.T) {
	for _, route := range []string{"/relay/payment_relayer_fee", "/relay/partial_withdraw"} {
		t.Run(route, func(t *testing.T) {
			f := newFeeFixture(t, filepath.Join(t.TempDir(), "missing-dir", "fee_notes.jsonl"))
			salt := big.NewInt(42)
			var req interface{} = f.relayerFeeRequest(t, "0", big.NewInt(6), salt)
			if route == "/relay/partial_withdraw" {
				req = f.partialWithdrawRequest(t, salt)
			}
			if w := postJSON(f.r, route, req); w.Code != http.StatusInternalServerError {
				t.Fatalf("relay with an unwritable fee note store: got %d, want 500: %s", w.Code, w.Body.String())
			}
			if f.node.called("eth_sendRawTransaction") {
				t.Fatal("the relayer sent the transaction although it could not keep the fee note's opening")
			}
			if _, held := f.h.inFlight.Load(nullifierKey(f.vaultAddr, big.NewInt(0), big.NewInt(55))); held {
				t.Fatal("the nullifier stayed claimed after the refusal")
			}
		})
	}
}

// A request the chain would reject is refused before anything is sent or
// recorded, on every route: no gas spent on a reverting transaction, and no
// record of a fee note that will never exist.
func TestRelayRoutes_SendAndRecordNothingWhenTheCallWouldRevert(t *testing.T) {
	cases := map[string]func(f *feeFixture, t *testing.T) interface{}{
		"/relay/payment": func(f *feeFixture, t *testing.T) interface{} {
			return RelayPaymentRequest{VaultId: "0", Proof: proof8,
				PublicSignal: paymentSignal(f.vaultAddr),
				CipherText:   "0x00", EncTxData: "0x00"}
		},
		"/relay/payment_relayer_fee": func(f *feeFixture, t *testing.T) interface{} {
			return f.relayerFeeRequest(t, "0", big.NewInt(6), big.NewInt(42))
		},
		"/relay/partial_withdraw": func(f *feeFixture, t *testing.T) interface{} {
			return f.partialWithdrawRequest(t, big.NewInt(42))
		},
		"/relay/payment_usdr_fee": func(f *feeFixture, t *testing.T) interface{} {
			return f.usdrFeeRequest(t, big.NewInt(42))
		},
	}
	for route, build := range cases {
		t.Run(route, func(t *testing.T) {
			f := newFeeFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
			f.node.revertCalls = true
			if w := postJSON(f.r, route, build(f, t)); w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("relay of a reverting call: got %d, want 422: %s", w.Code, w.Body.String())
			}
			if f.node.called("eth_sendRawTransaction") {
				t.Fatal("sent a call whose simulation reverted")
			}
			if notes := readFeeNotes(t, f.notesAt); len(notes) != 0 {
				t.Fatalf("recorded a fee note for a call that was not sent: %+v", notes)
			}
			held := false
			f.h.inFlight.Range(func(_, _ interface{}) bool { held = true; return false })
			if held {
				t.Fatal("a refused request left a nullifier claimed")
			}
		})
	}
}

// A request naming another vault than the one this relayer checks against is
// refused before anything is claimed, recorded or sent: EnygmaDvp would settle
// it against vaultById(vaultId), not the vault whose roots were checked.
func TestRelayRoutes_RefuseAnotherVault(t *testing.T) {
	cases := map[string]func(f *feeFixture, t *testing.T) (string, interface{}){
		"payment, registered other vault": func(f *feeFixture, t *testing.T) (string, interface{}) {
			return "/relay/payment", RelayPaymentRequest{VaultId: "2", Proof: proof8,
				PublicSignal: paymentSignal(f.vaultAddr),
				CipherText:   "0x00", EncTxData: "0x00"}
		},
		"relayer fee, unregistered vault": func(f *feeFixture, t *testing.T) (string, interface{}) {
			req := f.relayerFeeRequest(t, "0", big.NewInt(6), big.NewInt(42))
			req.VaultId = "9"
			return "/relay/payment_relayer_fee", req
		},
		"partial withdraw, registered other vault": func(f *feeFixture, t *testing.T) (string, interface{}) {
			req := f.partialWithdrawRequest(t, big.NewInt(42))
			req.VaultId = "2"
			return "/relay/partial_withdraw", req
		},
		"usdr fee, main vault as usdr leg": func(f *feeFixture, t *testing.T) (string, interface{}) {
			req := f.usdrFeeRequest(t, big.NewInt(42))
			req.UsdrVaultId = "0"
			return "/relay/payment_usdr_fee", req
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFeeFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
			route, req := build(f, t)
			w := postJSON(f.r, route, req)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "this relayer relays for vault") {
				t.Fatalf("got %d, want 400 naming the vault mismatch: %s", w.Code, w.Body.String())
			}
			if f.node.called("eth_sendRawTransaction") {
				t.Fatal("sent a request for another vault")
			}
			if notes := readFeeNotes(t, f.notesAt); len(notes) != 0 {
				t.Fatalf("recorded a fee note for a refused request: %+v", notes)
			}
			held := false
			f.h.inFlight.Range(func(_, _ interface{}) bool { held = true; return false })
			if held {
				t.Fatal("a refused request left a nullifier claimed")
			}
		})
	}
}

// payment2inSignal is a Payment2in statement spending nullifiers 55 and 56 of
// trees 0 and 1 (roots 1 and 2) into outputs 6 and 7, bound to vault.
func payment2inSignal(vault common.Address) []string {
	s := make([]string, layout.Payment2inLen)
	s[layout.Payment2inMessageOffset] = "0"
	s[layout.Payment2inTreeNumbersOffset] = "0"
	s[layout.Payment2inTreeNumbersOffset+1] = "1"
	s[layout.Payment2inMerkleRootsOffset] = "1"
	s[layout.Payment2inMerkleRootsOffset+1] = "2"
	s[layout.Payment2inNullifiersOffset] = "55"
	s[layout.Payment2inNullifiersOffset+1] = "56"
	s[layout.Payment2inCommitmentsOutOffset] = "6"
	s[layout.Payment2inCommitmentsOutOffset+1] = "7"
	s[layout.Payment2inContractAddressOffset] = new(big.Int).SetBytes(vault.Bytes()).String()
	return s
}

// sentMainReceipt decodes the main leg's receipt from the EnygmaDvp method
// call the relayer sent.
func (f *feeFixture) sentMainReceipt(t *testing.T, method string) proofReceipt {
	t.Helper()
	f.node.mu.Lock()
	data := f.node.sent
	f.node.mu.Unlock()
	m := f.h.dvpABI.Methods[method]
	if len(data) < 4 || !bytes.Equal(data[:4], m.ID) {
		t.Fatalf("no %s transaction was sent", method)
	}
	args, err := m.Inputs.Unpack(data[4:])
	if err != nil {
		t.Fatal(err)
	}
	return *abi.ConvertType(args[0], new(proofReceipt)).(*proofReceipt)
}

// The main leg may be a Payment proof or, so a fee-paying transfer can merge
// two notes, a Payment2in proof; either is sent with its whole statement and
// its own number of inputs.
func TestRelayPaymentUsdrFee_AcceptsAPaymentOrPayment2inMainLeg(t *testing.T) {
	cases := map[string]struct {
		signal                  func(common.Address) []string
		lastNullifier           int
		wantSignals, wantInputs int
	}{
		"Payment":    {paymentSignal, layout.PaymentNullifiersOffset, 7, 1},
		"Payment2in": {payment2inSignal, layout.Payment2inNullifiersOffset + 1, 10, 2},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFeeFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
			req := f.usdrFeeRequest(t, big.NewInt(42))
			req.PublicSignal = tc.signal(f.vaultAddr)
			if w := postJSON(f.r, "/relay/payment_usdr_fee", req); w.Code != http.StatusOK {
				t.Fatalf("relay: got %d: %s", w.Code, w.Body.String())
			}
			r := f.sentMainReceipt(t, "paymentWithUsdrFee")
			if len(r.Statement) != tc.wantSignals || r.Statement[tc.lastNullifier].Cmp(big.NewInt(int64(54+tc.wantInputs))) != 0 ||
				r.NumberOfInputs.Int64() != int64(tc.wantInputs) || r.NumberOfOutputs.Int64() != 2 {
				t.Fatalf("main receipt: %d signals, %s inputs, %s outputs; want %d signals, %d inputs, 2 outputs",
					len(r.Statement), r.NumberOfInputs, r.NumberOfOutputs, tc.wantSignals, tc.wantInputs)
			}
		})
	}
}

// A 2-input main leg is refused before anything is claimed, recorded or sent
// when its second input's root is unknown or its nullifier already spent.
func TestRelayPaymentUsdrFee_ChecksBothInputsOfAPayment2inMainLeg(t *testing.T) {
	cases := map[string]struct {
		unknownRoots, spentNullifiers map[uint64]bool
		want                          string
	}{
		"second root unknown":    {unknownRoots: map[uint64]bool{2: true}, want: "merkle root is not a known vault root"},
		"second nullifier spent": {spentNullifiers: map[uint64]bool{56: true}, want: "nullifier already spent"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFeeFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
			f.node.unknownRoots, f.node.spentNullifiers = tc.unknownRoots, tc.spentNullifiers
			req := f.usdrFeeRequest(t, big.NewInt(42))
			req.PublicSignal = payment2inSignal(f.vaultAddr)
			w := postJSON(f.r, "/relay/payment_usdr_fee", req)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "main input 1: "+tc.want) {
				t.Fatalf("got %d, want 400 naming main input 1: %s", w.Code, w.Body.String())
			}
			f.checkNothingDone(t)
		})
	}
}

// A main leg that is neither a Payment nor a Payment2in statement is refused
// with a 400, before anything is claimed, recorded or sent.
func TestRelayPaymentUsdrFee_RefusesAnotherMainLegLength(t *testing.T) {
	f := newFeeFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
	req := f.usdrFeeRequest(t, big.NewInt(42))
	req.PublicSignal = append(payment2inSignal(f.vaultAddr), "0") // 11 signals
	w := postJSON(f.r, "/relay/payment_usdr_fee", req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "publicSignal has 11 signals") {
		t.Fatalf("got %d, want 400 naming the statement length: %s", w.Code, w.Body.String())
	}
	f.checkNothingDone(t)
}

// checkNothingDone checks a refused request sent nothing, recorded no fee note
// and left no nullifier claimed.
func (f *feeFixture) checkNothingDone(t *testing.T) {
	t.Helper()
	if f.node.called("eth_sendRawTransaction") {
		t.Fatal("sent a refused request")
	}
	if notes := readFeeNotes(t, f.notesAt); len(notes) != 0 {
		t.Fatalf("recorded a fee note for a refused request: %+v", notes)
	}
	held := false
	f.h.inFlight.Range(func(_, _ interface{}) bool { held = true; return false })
	if held {
		t.Fatal("a refused request left a nullifier claimed")
	}
}

func paymentRequest(signal []string) RelayPaymentRequest {
	return RelayPaymentRequest{VaultId: "0", Proof: proof8, PublicSignal: signal, CipherText: "0x00", EncTxData: "0x00"}
}

// A plain payment may be a Payment proof or, so a transfer without a fee can
// merge two notes, a Payment2in proof; either is sent with its whole
// statement and its own number of inputs.
func TestRelayPayment_AcceptsAPaymentOrPayment2in(t *testing.T) {
	cases := map[string]struct {
		signal                  func(common.Address) []string
		lastNullifier           int
		wantSignals, wantInputs int
	}{
		"Payment":    {paymentSignal, layout.PaymentNullifiersOffset, 7, 1},
		"Payment2in": {payment2inSignal, layout.Payment2inNullifiersOffset + 1, 10, 2},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFeeFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
			if w := postJSON(f.r, "/relay/payment", paymentRequest(tc.signal(f.vaultAddr))); w.Code != http.StatusOK {
				t.Fatalf("relay: got %d: %s", w.Code, w.Body.String())
			}
			r := f.sentMainReceipt(t, "payment")
			if len(r.Statement) != tc.wantSignals || r.Statement[tc.lastNullifier].Cmp(big.NewInt(int64(54+tc.wantInputs))) != 0 ||
				r.NumberOfInputs.Int64() != int64(tc.wantInputs) || r.NumberOfOutputs.Int64() != 2 {
				t.Fatalf("receipt: %d signals, %s inputs, %s outputs; want %d signals, %d inputs, 2 outputs",
					len(r.Statement), r.NumberOfInputs, r.NumberOfOutputs, tc.wantSignals, tc.wantInputs)
			}
		})
	}
}

// A Payment2in payment is refused before anything is claimed or sent when its
// second input's root is unknown or its nullifier already spent.
func TestRelayPayment_ChecksBothInputsOfAPayment2in(t *testing.T) {
	cases := map[string]struct {
		unknownRoots, spentNullifiers map[uint64]bool
		want                          string
	}{
		"second root unknown":    {unknownRoots: map[uint64]bool{2: true}, want: "merkle root is not a known vault root"},
		"second nullifier spent": {spentNullifiers: map[uint64]bool{56: true}, want: "nullifier already spent"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFeeFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
			f.node.unknownRoots, f.node.spentNullifiers = tc.unknownRoots, tc.spentNullifiers
			w := postJSON(f.r, "/relay/payment", paymentRequest(payment2inSignal(f.vaultAddr)))
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "input 1: "+tc.want) {
				t.Fatalf("got %d, want 400 naming input 1: %s", w.Code, w.Body.String())
			}
			f.checkNothingDone(t)
		})
	}
}

// A payment that is neither a Payment nor a Payment2in statement is refused
// with a 400, before anything is claimed or sent.
func TestRelayPayment_RefusesAnotherStatementLength(t *testing.T) {
	f := newFeeFixture(t, filepath.Join(t.TempDir(), "fee_notes.jsonl"))
	w := postJSON(f.r, "/relay/payment", paymentRequest(append(payment2inSignal(f.vaultAddr), "0"))) // 11 signals
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "publicSignal has 11 signals") {
		t.Fatalf("got %d, want 400 naming the statement length: %s", w.Code, w.Body.String())
	}
	f.checkNothingDone(t)
}
