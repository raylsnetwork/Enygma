package tests

// Partial withdrawal on the retail deployment: take part of a note out of the
// vault and keep the rest, directly or through the relayer.
//
// The retail deployment runs enygma_dvp's contracts (Erc20CoinVault,
// EnygmaDvp) with this project's own Payment-family circuits and keys, so this
// file checks Erc20CoinVault.partialWithdraw, partialWithdrawWithRelayerFee and
// withdrawV2 against the retail gnark server (:8082) and relayer (:8090). It is
// the retail port of enygma_dvp's test/26 and test/27:
//
//   - TestRetailErc20_PartialWithdraw_FullFlow: deposit 100, withdraw 10 (a
//     change note of 90), withdraw 30 from it (60), withdrawV2 the 60
//   - _TwoInputs: 60 + 40 through Payment2in → 10 withdrawn, one note of 90
//   - _Rejections: wrong recipient or amount, zero amount, a payment
//     message, a fee-shaped statement, overdraws the prover refuses, replay
//   - _RejectsFeeCircuits: genuine PaymentFee and PaymentRelayerFeePublic
//     proofs refused by partialWithdraw
//   - _NoCrossSettlement: a withdrawal receipt (StMessage WITHDRAW_MESSAGE)
//     is refused by transfer, transferV2, payment, paymentWithRelayerFee,
//     submitPartialSettlement and SwapRelayer.submitReceipt; a payment
//     receipt is refused by the withdrawal paths; withdrawV2 keeps the change
//   - TestRetailRelayer_PartialWithdraw_FullFlow / _Rejections: the same
//     through POST /relay/partial_withdraw, the relayer paid from the note
//
// Prerequisites: a fresh deployment from setup.sh (it copies enygma_dvp's
// built contracts into contracts/abis), the retail gnark server, and for the
// relayer tests the retail relayer with RELAYER_FEE_SPEND_PRIVATE_KEY=123456789
// and RELAYER_MIN_FEE=1:
//
//	cd relayer && RELAYER_PRIVATE_KEY=9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda \ // gitleaks:allow (public Hardhat demo account)
//	  RELAYER_API_KEY=test-api-key-dev-only RELAYER_FEE_SPEND_PRIVATE_KEY=123456789 \
//	  RELAYER_MIN_FEE=1 go run .
//
// Run:
//
//	cd test && CC=/usr/bin/clang go test -run 'PartialWithdraw' -v -timeout 600s

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"testing"

	core "github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// ── retail adapters for the helpers the enygma_dvp versions use ──────────────

// tryPostPaymentGnarkProof posts to the retail gnark server and returns its
// error instead of failing, so callers can check refused witnesses.
func tryPostPaymentGnarkProof(path string, body interface{}) (*proofResponse, error) {
	return postProof(gnarkURL+path, body)
}

// vaultInt is a vault address as the integer a proof's StContractAddress carries.
func vaultInt(vault common.Address) *big.Int {
	return new(big.Int).SetBytes(vault.Bytes())
}

// txSender recovers the sender of a mined transaction.
func txSender(t *testing.T, client *ethclient.Client, txHashHex string) common.Address {
	t.Helper()
	tx, _, err := client.TransactionByHash(context.Background(), common.HexToHash(txHashHex))
	if err != nil {
		t.Fatalf("TransactionByHash: %v", err)
	}
	sender, err := types.Sender(types.LatestSignerForChainID(big.NewInt(hardhatChainID)), tx)
	if err != nil {
		t.Fatalf("types.Sender: %v", err)
	}
	return sender
}

// postToRetailRelayer posts body to path on the retail relayer.
func postToRetailRelayer(t *testing.T, path string, body interface{}) (*relayPaymentRelayerFeeResponse, int) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, relayerURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+relayerAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http.Do: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out relayPaymentRelayerFeeResponse
	_ = json.Unmarshal(raw, &out)
	return &out, resp.StatusCode
}

const partialWithdrawDepth = 8

// pwEnv is the deployment a partial-withdraw test runs against.
type pwEnv struct {
	ctx       context.Context
	client    *ethclient.Client
	vault     *bind.BoundContract
	erc20     *bind.BoundContract
	vaultABI  abi.ABI
	vaultAddr common.Address
	owner     *bind.TransactOpts
	tokenId   *big.Int
}

func newPartialWithdrawEnv(t *testing.T) *pwEnv {
	t.Helper()
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8082") {
		t.Skip("gnark server not running on localhost:8082 — skipping")
	}
	ctx := context.Background()
	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	t.Cleanup(client.Close)

	receipts := loadOnchainReceipts(t)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)

	vaultABI := loadOnchainABI(t, "Erc20CoinVault")
	if _, ok := vaultABI.Methods["partialWithdraw"]; !ok {
		t.Fatalf("contracts/abis/Erc20CoinVault.json has no partialWithdraw — run setup.sh")
	}
	return &pwEnv{
		ctx:       ctx,
		client:    client,
		vault:     bind.NewBoundContract(vaultAddr, vaultABI, client, client, client),
		erc20:     bind.NewBoundContract(erc20Addr, loadOnchainABI(t, "RaylsERC20"), client, client, client),
		vaultABI:  vaultABI,
		vaultAddr: vaultAddr,
		owner:     hardhatAuth(t, client),
		tokenId:   big.NewInt(0),
	}
}

func (e *pwEnv) balanceOf(t *testing.T, a common.Address) *big.Int {
	t.Helper()
	var out []interface{}
	if err := e.erc20.Call(&bind.CallOpts{Context: e.ctx}, &out, "balanceOf", a); err != nil {
		t.Fatalf("balanceOf: %v", err)
	}
	return out[0].(*big.Int)
}

func (e *pwEnv) nullifierSpent(t *testing.T, treeNumber int, nullifier *big.Int) bool {
	t.Helper()
	var out []interface{}
	if err := e.vault.Call(&bind.CallOpts{Context: e.ctx}, &out, "isValidNullifier", big.NewInt(int64(treeNumber)), nullifier); err != nil {
		t.Fatalf("isValidNullifier: %v", err)
	}
	return out[0].(bool)
}

// pwNote is a note the test owns: its opening and where it sits in the tree.
type pwNote struct {
	spend  *core.SpendKeyPair
	view   *core.ViewKeyPair
	salt   *big.Int
	value  *big.Int
	commit *big.Int
}

// pwSpend is a partial-withdraw proof and the change note it creates.
type pwSpend struct {
	receipt     onchainProofReceipt
	change      pwNote
	ctI, ctII   []byte
	treeNumbers []int
	nullifiers  []*big.Int
	amount      *big.Int
	recipient   common.Address
	// proofStrs is the raw proof, for submitting through the relayer.
	proofStrs []string
	// feeSalt/feeCommit open the relayer fee note (PaymentRelayerFeePublic only).
	feeSalt, feeCommit *big.Int
}

// pwCircuit selects the gnark circuit a withdrawal proof is made with.
type pwCircuit int

const (
	pwPayment                 pwCircuit = iota // 1-in/2-out or 2-in/2-out: what partialWithdraw takes
	pwPaymentFee                               // 1-in/2-out plus a public, burned StFee
	pwPaymentRelayerFeePublic                  // 1-in/3-out, output[2] is a relayer fee note
)

// proveWithdraw builds a Payment proof spending `in` into a withdrawal of
// `amount` to `recipient` and a change note of `changeValue`, addressed back to
// the same owner and encrypted to their view key the way a wallet would.
// It returns the gnark server's error instead of failing when the witness is
// refused, so callers can test that.
func (e *pwEnv) proveWithdraw(t *testing.T, in pwNote, amount, changeValue *big.Int, recipient common.Address) (*pwSpend, error) {
	t.Helper()
	return e.proveWithdrawWith(t, pwPayment, []pwNote{in}, amount, changeValue, nil, nil, recipient)
}

// proveWithdrawWith is proveWithdraw for one or two input notes (all owned by
// ins[0]'s keys for the change note) and any of the three Payment-family
// circuits. fee is PaymentFee's burned StFee or PaymentRelayerFeePublic's fee
// note value; it is ignored for pwPayment. relayerPk owns the
// PaymentRelayerFeePublic fee note (nil: a fresh key).
func (e *pwEnv) proveWithdrawWith(t *testing.T, circuit pwCircuit, ins []pwNote, amount, changeValue, fee, relayerPk *big.Int, recipient common.Address) (*pwSpend, error) {
	t.Helper()
	// The withdrawal message, except PaymentFee: it is never a withdrawal and
	// its circuit only takes a payment's.
	message := pwWithdrawMessage
	if circuit == pwPaymentFee {
		message = pwPaymentMessage
	}
	return e.proveWithdrawMsg(t, circuit, message, ins, amount, changeValue, fee, relayerPk, recipient)
}

// StMessage values of the Payment-family circuits: WITHDRAW_MESSAGE in
// IEnygmaDvp.sol for a withdrawal, 0 for a payment.
const (
	pwPaymentMessage  = "0"
	pwWithdrawMessage = "1"
)

// proveWithdrawMsg is proveWithdrawWith with the proof's StMessage chosen by
// the caller.
func (e *pwEnv) proveWithdrawMsg(t *testing.T, circuit pwCircuit, message string, ins []pwNote, amount, changeValue, fee, relayerPk *big.Int, recipient common.Address) (*pwSpend, error) {
	t.Helper()
	return e.proveWithdrawFull(t, circuit, message, ins, amount, changeValue, fee, relayerPk, nil, recipient)
}

// proveWithdrawFull is proveWithdrawMsg with the change note's salt chosen by
// the caller (nil: the salt a wallet uses — WithdrawChangeSalt for a
// withdrawal, the KEM-derived one for a payment).
func (e *pwEnv) proveWithdrawFull(t *testing.T, circuit pwCircuit, message string, ins []pwNote, amount, changeValue, fee, relayerPk, changeSaltOverride *big.Int, recipient common.Address) (*pwSpend, error) {
	t.Helper()
	nIn := len(ins)
	mt := loadVaultMerkleTree(t, e.client, e.vaultAddr, partialWithdrawDepth)

	trees := make([]string, nIn)
	roots := make([]string, nIn)
	nullifierStrs := make([]string, nIn)
	keys := make([]string, nIn)
	values := make([]string, nIn)
	salts := make([]string, nIn)
	paths := make([][partialWithdrawDepth]string, nIn)
	indices := make([]string, nIn)
	treeNumbers := make([]int, nIn)
	nullifiers := make([]*big.Int, nIn)
	for i, in := range ins {
		mp, err := mt.GenerateProof(in.commit)
		if err != nil {
			t.Fatalf("GenerateProof (input %d): %v", i, err)
		}
		treeNumber := big.NewInt(int64(mp.TreeNumber))
		nullifier, err := core.GetNullifierWithTree(in.spend.PrivateKey, treeNumber, mp.Indices, partialWithdrawDepth)
		if err != nil {
			t.Fatalf("GetNullifierWithTree (input %d): %v", i, err)
		}
		for j, el := range mp.Elements {
			if j < partialWithdrawDepth {
				paths[i][j] = el.String()
			}
		}
		trees[i], roots[i], nullifierStrs[i] = treeNumber.String(), mp.Root.String(), nullifier.String()
		keys[i], values[i], salts[i] = in.spend.PrivateKey.String(), in.value.String(), in.salt.String()
		indices[i] = mp.Indices.String()
		treeNumbers[i], nullifiers[i] = mp.TreeNumber, nullifier
	}
	owner := ins[0]

	recipientInt := new(big.Int).SetBytes(recipient.Bytes())
	wdCmt, err := core.Erc20CommitmentV2(recipientInt, big.NewInt(0), amount, e.tokenId)
	if err != nil {
		t.Fatalf("withdrawal commitment: %v", err)
	}

	// Change note: encapsulate to the owner's own view key, like a transfer to self.
	ss, ctI, err := core.Encapsulate(owner.view.EncapsKey)
	if err != nil {
		t.Fatalf("Encapsulate: %v", err)
	}
	saltB, err := core.DerivePaymentSalt(ss)
	if err != nil {
		t.Fatalf("DerivePaymentSalt: %v", err)
	}
	encKey, err := core.DerivePaymentKey(ss)
	if err != nil {
		t.Fatalf("DerivePaymentKey: %v", err)
	}
	changeSalt := core.SaltBToField(saltB)
	if message == pwWithdrawMessage {
		// The circuit requires a withdrawal's change salt to be recoverable
		// from the owner's key: Poseidon(sk, nullifier of input 0). The
		// EncryptedNote below is then only a convenience.
		changeSalt, err = core.WithdrawChangeSalt(owner.spend.PrivateKey, nullifiers[0])
		if err != nil {
			t.Fatalf("WithdrawChangeSalt: %v", err)
		}
	}
	if changeSaltOverride != nil {
		changeSalt = changeSaltOverride
	}
	ctII, err := core.EncryptPayload(encKey, e.tokenId, changeValue)
	if err != nil {
		t.Fatalf("EncryptPayload: %v", err)
	}
	changeCmt, err := core.Erc20CommitmentV2(owner.spend.PublicKey, changeSalt, changeValue, e.tokenId)
	if err != nil {
		t.Fatalf("change commitment: %v", err)
	}

	commitments := []string{wdCmt.String(), changeCmt.String()}
	outPks := []string{recipientInt.String(), owner.spend.PublicKey.String()}
	outValues := []string{amount.String(), changeValue.String()}
	outSalts := []string{"0", changeSalt.String()}

	var feeSalt, feeCommit *big.Int
	endpoint := "/proof/payment"
	if nIn == 2 {
		endpoint = "/proof/payment2in"
	}
	body := map[string]interface{}{
		"stMessage":         message,
		"stTreeNumbers":     trees,
		"stMerkleRoots":     roots,
		"stNullifiers":      nullifierStrs,
		"stContractAddress": vaultInt(e.vaultAddr).String(),
		"wtPrivateKeysIn":   keys,
		"wtValuesIn":        values,
		"wtSaltsIn":         salts,
		"wtPathElements":    paths,
		"wtPathIndices":     indices,
		"wtTokenId":         e.tokenId.String(),
	}
	switch circuit {
	case pwPaymentFee:
		endpoint = "/proof/paymentFee"
		body["stFee"] = fee.String()
	case pwPaymentRelayerFeePublic:
		// A relayer's fee note as the third output.
		endpoint = "/proof/paymentRelayerFeePublic"
		if relayerPk == nil {
			relayer, err := core.NewSpendKeyPair()
			if err != nil {
				t.Fatalf("NewSpendKeyPair (relayer): %v", err)
			}
			relayerPk = relayer.PublicKey
		}
		relayerSalt, _ := core.RandomInField()
		relayerCmt, err := core.Erc20CommitmentV2(relayerPk, relayerSalt, fee, e.tokenId)
		if err != nil {
			t.Fatalf("relayer commitment: %v", err)
		}
		feeSalt, feeCommit = relayerSalt, relayerCmt
		commitments = append(commitments, relayerCmt.String())
		outPks = append(outPks, relayerPk.String())
		outValues = append(outValues, fee.String())
		outSalts = append(outSalts, relayerSalt.String())
		body["stFee"] = fee.String()
	}
	body["stCommitmentsOut"] = commitments
	body["wtSpendPublicKeysOut"] = outPks
	body["wtValuesOut"] = outValues
	body["wtSaltsOut"] = outSalts

	proof, err := tryPostPaymentGnarkProof(endpoint, body)
	if err != nil {
		return nil, err
	}
	proofStrs := make([]string, len(proof.Proof))
	for i, v := range proof.Proof {
		proofStrs[i] = v.String()
	}
	// The handlers return [msg, tree0, root0, null0, tree1, root1, null1, ...];
	// the verifier and the vault take the inputs grouped:
	// [msg, trees..., roots..., nullifiers..., ...]. Same for one input.
	sig := proof.PublicSignal
	statement := []*big.Int{sig[0]}
	for field := 0; field < 3; field++ {
		for i := 0; i < nIn; i++ {
			statement = append(statement, sig[1+3*i+field])
		}
	}
	statement = append(statement, sig[1+3*nIn:]...)
	return &pwSpend{
		receipt: onchainProofReceipt{
			Proof:           proofStringsToOnchain(t, proofStrs),
			Statement:       statement,
			NumberOfInputs:  big.NewInt(int64(nIn)),
			NumberOfOutputs: big.NewInt(int64(len(commitments))),
		},
		change:      pwNote{spend: owner.spend, view: owner.view, salt: changeSalt, value: changeValue, commit: changeCmt},
		ctI:         ctI,
		ctII:        ctII,
		treeNumbers: treeNumbers,
		nullifiers:  nullifiers,
		amount:      amount,
		recipient:   recipient,
		proofStrs:   proofStrs,
		feeSalt:     feeSalt,
		feeCommit:   feeCommit,
	}, nil
}

func (e *pwEnv) mustProveWithdraw(t *testing.T, in pwNote, amount, changeValue *big.Int, recipient common.Address) *pwSpend {
	t.Helper()
	s, err := e.proveWithdraw(t, in, amount, changeValue, recipient)
	if err != nil {
		t.Fatalf("gnark /proof/payment: %v", err)
	}
	return s
}

// verifiesAt reports whether the deployment's Verifier accepts the receipt's
// proof under the VK in slot vkID: a proof that verifies is a genuine proof of
// that circuit, so a partialWithdraw rejection of it is the vault's doing.
func (e *pwEnv) verifiesAt(t *testing.T, vkID int64, r onchainProofReceipt) bool {
	t.Helper()
	receipts := loadOnchainReceipts(t)
	verifier := bind.NewBoundContract(common.HexToAddress(receipts["Verifier"].ContractAddress),
		loadOnchainABI(t, "Verifier"), e.client, e.client, e.client)
	var out []interface{}
	if err := verifier.Call(&bind.CallOpts{Context: e.ctx}, &out, "verifyProof", big.NewInt(vkID), r.Proof, r.Statement); err != nil {
		t.Fatalf("Verifier.verifyProof(%d): %v", vkID, err)
	}
	return out[0].(bool)
}

// depositFor deposits amount into a note owned by an existing spend/view key pair.
func depositFor(t *testing.T, e *pwEnv, spend *core.SpendKeyPair, view *core.ViewKeyPair, amount *big.Int) pwNote {
	t.Helper()
	mint, err := e.erc20.Transact(e.owner, "mint", e.owner.From, amount)
	if err != nil {
		t.Fatalf("ERC20.mint: %v", err)
	}
	if _, err := bind.WaitMined(e.ctx, e.client, mint); err != nil {
		t.Fatalf("wait mint: %v", err)
	}
	approve, err := e.erc20.Transact(e.owner, "approve", e.vaultAddr, amount)
	if err != nil {
		t.Fatalf("ERC20.approve: %v", err)
	}
	if _, err := bind.WaitMined(e.ctx, e.client, approve); err != nil {
		t.Fatalf("wait approve: %v", err)
	}
	ss, capsule, err := core.Encapsulate(view.EncapsKey)
	if err != nil {
		t.Fatalf("Encapsulate: %v", err)
	}
	saltB, _ := core.DerivePaymentSalt(ss)
	encKey, _ := core.DerivePaymentKey(ss)
	salt := core.SaltBToField(saltB)
	commit, err := core.Erc20CommitmentV2(spend.PublicKey, salt, amount, e.tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2: %v", err)
	}
	ct, err := core.EncryptPayload(encKey, e.tokenId, amount)
	if err != nil {
		t.Fatalf("EncryptPayload: %v", err)
	}
	tx, err := e.vault.Transact(e.owner, "depositV2", []*big.Int{amount, spend.PublicKey, salt, e.tokenId}, capsule, ct)
	if err != nil {
		t.Fatalf("depositV2: %v", err)
	}
	if m, err := bind.WaitMined(e.ctx, e.client, tx); err != nil || m.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("depositV2 reverted (receipt=%v err=%v)", m, err)
	}
	return pwNote{spend: spend, view: view, salt: salt, value: amount, commit: commit}
}

// submitPartialWithdraw sends partialWithdraw and returns the mined receipt.
func (e *pwEnv) submitPartialWithdraw(t *testing.T, amount *big.Int, recipient common.Address, s *pwSpend) *types.Receipt {
	t.Helper()
	tx, err := e.vault.Transact(e.owner, "partialWithdraw",
		[]*big.Int{amount, e.tokenId}, recipient, s.receipt, s.ctI, s.ctII)
	if err != nil {
		t.Fatalf("partialWithdraw: %v", err)
	}
	mined, err := bind.WaitMined(e.ctx, e.client, tx)
	if err != nil || mined.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("partialWithdraw reverted (receipt=%v err=%v)", mined, err)
	}
	return mined
}

// expectRevert sends partialWithdraw and requires it to be rejected with want.
func (e *pwEnv) expectRevert(t *testing.T, want string, amount *big.Int, recipient common.Address, receipt onchainProofReceipt, ctI, ctII []byte) {
	t.Helper()
	tx, err := e.vault.Transact(e.owner, "partialWithdraw", []*big.Int{amount, e.tokenId}, recipient, receipt, ctI, ctII)
	if err == nil {
		mined, werr := bind.WaitMined(e.ctx, e.client, tx)
		if werr == nil && mined.Status == types.ReceiptStatusSuccessful {
			t.Fatalf("partialWithdraw was accepted; want a %s revert", want)
		}
		t.Fatalf("partialWithdraw reverted on-chain, but the reason could not be checked against %s (err=%v)", want, werr)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("partialWithdraw rejected with %q; want %s", err, want)
	}
	t.Logf("rejected as expected: %s", want)
}

// checkSettled verifies one partialWithdraw's on-chain effects: the events it
// emitted, that the change note decrypts to its value for the owner, and that
// it is now in the vault's Merkle tree.
func (e *pwEnv) checkSettled(t *testing.T, mined *types.Receipt, s *pwSpend) {
	t.Helper()
	var sawPW, sawNote, sawNullifier bool
	var firstNullifier, eventChange *big.Int
	for _, lg := range mined.Logs {
		if lg.Address != e.vaultAddr || len(lg.Topics) == 0 {
			continue
		}
		ev, err := e.vaultABI.EventByID(lg.Topics[0])
		if err != nil {
			continue
		}
		switch ev.Name {
		case "PartialWithdrawal":
			sawPW = true
			if got := common.BytesToAddress(lg.Topics[2].Bytes()); got != s.recipient {
				t.Errorf("PartialWithdrawal recipient = %s, want %s", got.Hex(), s.recipient.Hex())
			}
			eventChange = lg.Topics[3].Big()
			if eventChange.Cmp(s.change.commit) != 0 {
				t.Errorf("PartialWithdrawal changeCommitment = %s, want %s", eventChange, s.change.commit)
			}
			vals, err := ev.Inputs.NonIndexed().Unpack(lg.Data)
			if err != nil {
				t.Fatalf("unpack PartialWithdrawal: %v", err)
			}
			if got := vals[0].(*big.Int); got.Cmp(s.amount) != 0 {
				t.Errorf("PartialWithdrawal amount = %s, want %s", got, s.amount)
			}
		case "EncryptedNote":
			if lg.Topics[2].Big().Cmp(s.change.commit) != 0 {
				t.Errorf("EncryptedNote for %s, want the change note %s", lg.Topics[2].Big(), s.change.commit)
			}
			sawNote = true
			vals, err := ev.Inputs.NonIndexed().Unpack(lg.Data)
			if err != nil {
				t.Fatalf("unpack EncryptedNote: %v", err)
			}
			// The owner's wallet rediscovers the change note from the event alone.
			ss, err := core.Decapsulate(s.change.view.DecapsKey, vals[0].([]byte))
			if err != nil {
				t.Fatalf("Decapsulate change note: %v", err)
			}
			encKey, _ := core.DerivePaymentKey(ss)
			tokenId, amount, err := core.DecryptPayload(encKey, vals[1].([]byte))
			if err != nil {
				t.Fatalf("DecryptPayload change note: %v", err)
			}
			if amount.Cmp(s.change.value) != 0 || tokenId.Cmp(e.tokenId) != 0 {
				t.Errorf("change note decrypts to amount=%s tokenId=%s, want %s/%s", amount, tokenId, s.change.value, e.tokenId)
			}
		case "Nullifier":
			if !sawNullifier {
				firstNullifier = lg.Topics[3].Big()
			}
			sawNullifier = true
		case "Commitment":
			if lg.Topics[2].Big().Cmp(s.change.commit) != 0 {
				t.Errorf("Commitment event for %s; only the change note %s should be inserted", lg.Topics[2].Big(), s.change.commit)
			}
		}
	}
	if !sawPW || !sawNote || !sawNullifier {
		t.Errorf("missing events: PartialWithdrawal=%v EncryptedNote=%v Nullifier=%v", sawPW, sawNote, sawNullifier)
	}
	// The owner can rebuild the change note from its key and the events
	// alone, without the EncryptedNote.
	if firstNullifier != nil && eventChange != nil {
		_, rebuilt, err := core.WithdrawChangeNote(s.change.spend.PrivateKey, firstNullifier, s.change.value, e.tokenId)
		if err != nil {
			t.Fatalf("WithdrawChangeNote: %v", err)
		}
		if rebuilt.Cmp(eventChange) != 0 {
			t.Errorf("the change note rebuilt from the owner's key and the Nullifier event (%s) is not the event's changeCommitment %s", rebuilt, eventChange)
		}
	}
	for i := range s.nullifiers {
		if !e.nullifierSpent(t, s.treeNumbers[i], s.nullifiers[i]) {
			t.Errorf("input %d's nullifier is not marked spent", i)
		}
	}
	mt := loadVaultMerkleTree(t, e.client, e.vaultAddr, partialWithdrawDepth)
	if _, err := mt.GenerateProof(s.change.commit); err != nil {
		t.Errorf("the change note is not in the vault's Merkle tree: %v", err)
	}
}

func newDepositNote(t *testing.T, e *pwEnv, amount *big.Int) pwNote {
	t.Helper()
	spend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	view, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair: %v", err)
	}
	return depositFor(t, e, spend, view, amount)
}

func TestRetailErc20_PartialWithdraw_FullFlow(t *testing.T) {
	e := newPartialWithdrawEnv(t)

	recipient := common.HexToAddress("0x0000000000000000000000000000000000005e7d")
	recipientStart := e.balanceOf(t, recipient)
	vaultStart := e.balanceOf(t, e.vaultAddr)

	// 1. Deposit 100.
	note100 := newDepositNote(t, e, big.NewInt(100))
	if got := new(big.Int).Sub(e.balanceOf(t, e.vaultAddr), vaultStart); got.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("vault received %s on deposit, want 100", got)
	}
	t.Logf("deposited a note of 100 (commitment %s)", note100.commit)

	// 2. Withdraw 10 of it; the note of 100 becomes a note of 90.
	s1 := e.mustProveWithdraw(t, note100, big.NewInt(10), big.NewInt(90), recipient)
	m1 := e.submitPartialWithdraw(t, big.NewInt(10), recipient, s1)
	e.checkSettled(t, m1, s1)
	if got := new(big.Int).Sub(e.balanceOf(t, recipient), recipientStart); got.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("recipient received %s after the first partial withdraw, want 10", got)
	}
	if got := new(big.Int).Sub(e.balanceOf(t, e.vaultAddr), vaultStart); got.Cmp(big.NewInt(90)) != 0 {
		t.Fatalf("vault holds %s of this deposit after withdrawing 10, want 90", got)
	}
	t.Logf("withdrew 10 (gas %d): the note of 100 is spent, a note of 90 is in the tree", m1.GasUsed)

	// 3. The 90 note is a normal note: withdraw 30 more, leaving 60.
	s2 := e.mustProveWithdraw(t, s1.change, big.NewInt(30), big.NewInt(60), recipient)
	m2 := e.submitPartialWithdraw(t, big.NewInt(30), recipient, s2)
	e.checkSettled(t, m2, s2)
	if got := new(big.Int).Sub(e.balanceOf(t, recipient), recipientStart); got.Cmp(big.NewInt(40)) != 0 {
		t.Fatalf("recipient received %s after the second partial withdraw, want 40", got)
	}
	t.Logf("withdrew 30 from the note of 90: a note of 60 is in the tree")

	// 4. Withdraw the remaining 60 in full with withdrawV2.
	s3 := e.mustProveWithdraw(t, s2.change, big.NewInt(60), big.NewInt(0), recipient)
	tx, err := e.vault.Transact(e.owner, "withdrawV2", []*big.Int{big.NewInt(60), e.tokenId}, recipient, s3.receipt)
	if err != nil {
		t.Fatalf("withdrawV2: %v", err)
	}
	if mined, err := bind.WaitMined(e.ctx, e.client, tx); err != nil || mined.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("withdrawV2 reverted (receipt=%v err=%v)", mined, err)
	}
	if !e.nullifierSpent(t, s3.treeNumbers[0], s3.nullifiers[0]) {
		t.Errorf("the 60 note's nullifier is not marked spent after withdrawV2")
	}
	if got := new(big.Int).Sub(e.balanceOf(t, recipient), recipientStart); got.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("recipient received %s in total, want the full 100", got)
	}
	if got := e.balanceOf(t, e.vaultAddr); got.Cmp(vaultStart) != 0 {
		t.Fatalf("vault balance is %s, want it back at %s", got, vaultStart)
	}
	t.Logf("withdrew the last 60: recipient received 100 in total (10 + 30 + 60), vault back to %s", vaultStart)
}

func TestRetailErc20_PartialWithdraw_Rejections(t *testing.T) {
	e := newPartialWithdrawEnv(t)
	recipient := common.HexToAddress("0x0000000000000000000000000000000000005e7e")
	attacker := common.HexToAddress("0x000000000000000000000000000000000000bad0")

	note := newDepositNote(t, e, big.NewInt(100))
	s := e.mustProveWithdraw(t, note, big.NewInt(10), big.NewInt(90), recipient)

	t.Run("different_recipient", func(t *testing.T) {
		// Someone copying the receipt from the mempool cannot redirect the funds.
		e.expectRevert(t, "InvalidOpening", big.NewInt(10), attacker, s.receipt, s.ctI, s.ctII)
	})
	t.Run("larger_amount", func(t *testing.T) {
		e.expectRevert(t, "InvalidOpening", big.NewInt(20), recipient, s.receipt, s.ctI, s.ctII)
	})
	t.Run("zero_amount", func(t *testing.T) {
		e.expectRevert(t, "ZeroWithdrawAmount", big.NewInt(0), recipient, s.receipt, s.ctI, s.ctII)
	})
	t.Run("fee_shaped_statement", func(t *testing.T) {
		// An 8-element statement is the PaymentFee shape, whose fee leaves the
		// input outside the two outputs.
		bad := s.receipt
		bad.Statement = append(append([]*big.Int{}, s.receipt.Statement...), big.NewInt(1))
		e.expectRevert(t, "InvalidStatmentSize", big.NewInt(10), recipient, bad, s.ctI, s.ctII)
	})
	t.Run("payment_message", func(t *testing.T) {
		// StMessage 0 marks a payment proof, not a withdrawal.
		bad := s.receipt
		bad.Statement = append([]*big.Int{}, s.receipt.Statement...)
		bad.Statement[0] = big.NewInt(0)
		e.expectRevert(t, "InvalidPaymentMessage", big.NewInt(10), recipient, bad, s.ctI, s.ctII)
	})
	t.Run("overdraw_refused_by_prover", func(t *testing.T) {
		// 100 in, 110 out and a "negative" change of -10 (mod Fr): the circuit's
		// range check on output values refuses the witness.
		neg := new(big.Int).Sub(core.SNARK_SCALAR_FIELD, big.NewInt(10))
		if _, err := e.proveWithdraw(t, note, big.NewInt(110), neg, recipient); err == nil {
			t.Fatalf("the prover produced a proof withdrawing 110 from a note of 100")
		} else {
			t.Logf("prover refused the overdraw: %.120s", err)
		}
	})
	t.Run("sum_mismatch_refused_by_prover", func(t *testing.T) {
		// 10 out + 95 change from a note of 100.
		if _, err := e.proveWithdraw(t, note, big.NewInt(10), big.NewInt(95), recipient); err == nil {
			t.Fatalf("the prover produced a proof whose outputs exceed the note")
		} else {
			t.Logf("prover refused 10 + 95 from 100: %.120s", err)
		}
	})

	// The honest submission settles, and then the same receipt cannot settle twice.
	e.submitPartialWithdraw(t, big.NewInt(10), recipient, s)
	t.Run("replay", func(t *testing.T) {
		e.expectRevert(t, "InvalidNullifier", big.NewInt(10), recipient, s.receipt, s.ctI, s.ctII)
	})
	t.Run("full_withdraw_of_spent_note", func(t *testing.T) {
		// withdrawV2 shares the nullifier set, so the spent 100 note is gone there too.
		tx, err := e.vault.Transact(e.owner, "withdrawV2", []*big.Int{big.NewInt(10), e.tokenId}, recipient, s.receipt)
		if err == nil {
			if mined, werr := bind.WaitMined(e.ctx, e.client, tx); werr == nil && mined.Status == types.ReceiptStatusSuccessful {
				t.Fatalf("withdrawV2 re-spent a note already spent by partialWithdraw")
			}
			return
		}
		if !strings.Contains(err.Error(), "InvalidNullifier") {
			t.Fatalf("withdrawV2 rejected with %q; want InvalidNullifier", err)
		}
	})
}

func TestRetailErc20_PartialWithdraw_TwoInputs(t *testing.T) {
	e := newPartialWithdrawEnv(t)
	recipient := common.HexToAddress("0x0000000000000000000000000000000000005e7f")
	recipientStart := e.balanceOf(t, recipient)
	vaultStart := e.balanceOf(t, e.vaultAddr)

	spend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	view, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair: %v", err)
	}
	note60 := depositFor(t, e, spend, view, big.NewInt(60))
	note40 := depositFor(t, e, spend, view, big.NewInt(40))
	t.Logf("deposited two notes for one owner: 60 and 40")

	s, err := e.proveWithdrawWith(t, pwPayment, []pwNote{note60, note40}, big.NewInt(10), big.NewInt(90), nil, nil, recipient)
	if err != nil {
		t.Fatalf("gnark /proof/payment2in: %v", err)
	}
	if n := len(s.receipt.Statement); n != 10 {
		t.Fatalf("2-input statement has %d elements, want 10", n)
	}
	if !e.verifiesAt(t, 1, s.receipt) {
		t.Fatalf("the 2-input proof does not verify against VK slot 1 (Payment2in)")
	}
	m := e.submitPartialWithdraw(t, big.NewInt(10), recipient, s)
	e.checkSettled(t, m, s)

	if got := new(big.Int).Sub(e.balanceOf(t, recipient), recipientStart); got.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("recipient received %s, want 10", got)
	}
	if got := new(big.Int).Sub(e.balanceOf(t, e.vaultAddr), vaultStart); got.Cmp(big.NewInt(90)) != 0 {
		t.Fatalf("vault holds %s of the 100 deposited, want 90", got)
	}
	t.Logf("withdrew 10 from 60 + 40 (gas %d): both notes spent, one note of 90 in the tree", m.GasUsed)

	// The merged 90 note spends like any other.
	s2 := e.mustProveWithdraw(t, s.change, big.NewInt(90), big.NewInt(0), recipient)
	tx, err := e.vault.Transact(e.owner, "withdrawV2", []*big.Int{big.NewInt(90), e.tokenId}, recipient, s2.receipt)
	if err != nil {
		t.Fatalf("withdrawV2 of the merged note: %v", err)
	}
	if mined, err := bind.WaitMined(e.ctx, e.client, tx); err != nil || mined.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("withdrawV2 of the merged note reverted (receipt=%v err=%v)", mined, err)
	}
	if got := new(big.Int).Sub(e.balanceOf(t, recipient), recipientStart); got.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("recipient received %s in total, want 100", got)
	}
	t.Logf("withdrew the merged 90: recipient received 100 in total")
}

func TestRetailErc20_PartialWithdraw_RejectsFeeCircuits(t *testing.T) {
	e := newPartialWithdrawEnv(t)
	recipient := common.HexToAddress("0x0000000000000000000000000000000000005e80")
	note := newDepositNote(t, e, big.NewInt(100))

	t.Run("PaymentFee", func(t *testing.T) {
		// 100 = 10 withdrawn + 89 change + 1 burned fee.
		s, err := e.proveWithdrawWith(t, pwPaymentFee, []pwNote{note}, big.NewInt(10), big.NewInt(89), big.NewInt(1), nil, recipient)
		if err != nil {
			t.Fatalf("gnark /proof/paymentFee: %v", err)
		}
		if n := len(s.receipt.Statement); n != 8 {
			t.Fatalf("PaymentFee statement has %d elements, want 8", n)
		}
		if !e.verifiesAt(t, 2, s.receipt) {
			t.Fatalf("the PaymentFee proof does not verify against VK slot 2 — the test would prove nothing")
		}
		e.expectRevert(t, "InvalidStatmentSize", big.NewInt(10), recipient, s.receipt, s.ctI, s.ctII)
	})

	t.Run("PaymentRelayerFeePublic", func(t *testing.T) {
		// 100 = 10 withdrawn + 85 change + 5 relayer fee note.
		s, err := e.proveWithdrawWith(t, pwPaymentRelayerFeePublic, []pwNote{note}, big.NewInt(10), big.NewInt(85), big.NewInt(5), nil, recipient)
		if err != nil {
			t.Fatalf("gnark /proof/paymentRelayerFeePublic: %v", err)
		}
		if n := len(s.receipt.Statement); n != 9 {
			t.Fatalf("PaymentRelayerFeePublic statement has %d elements, want 9", n)
		}
		if !e.verifiesAt(t, 3, s.receipt) {
			t.Fatalf("the PaymentRelayerFeePublic proof does not verify against VK slot 3 — the test would prove nothing")
		}
		// As produced: three outputs.
		e.expectRevert(t, "InvalidNumberOfOutputs", big.NewInt(10), recipient, s.receipt, s.ctI, s.ctII)
		// Relabelled as two outputs so the fee note would be skipped: the
		// statement length gives it away.
		relabelled := s.receipt
		relabelled.NumberOfOutputs = big.NewInt(2)
		e.expectRevert(t, "InvalidStatmentSize", big.NewInt(10), recipient, relabelled, s.ctI, s.ctII)
	})

	// Neither attempt spent the note: an honest partial withdrawal still works.
	s := e.mustProveWithdraw(t, note, big.NewInt(10), big.NewInt(90), recipient)
	e.submitPartialWithdraw(t, big.NewInt(10), recipient, s)
	t.Logf("the note was untouched by the rejected fee proofs and withdrew normally")
}

// expectCallRevert sends method on contract and requires it to be rejected
// with want before anything is mined.
func expectCallRevert(t *testing.T, e *pwEnv, contract *bind.BoundContract, want, method string, args ...interface{}) {
	t.Helper()
	tx, err := contract.Transact(e.owner, method, args...)
	if err == nil {
		mined, werr := bind.WaitMined(e.ctx, e.client, tx)
		if werr == nil && mined.Status == types.ReceiptStatusSuccessful {
			t.Fatalf("%s accepted the receipt; want a %s revert", method, want)
		}
		t.Fatalf("%s reverted on-chain, but the reason could not be checked against %s (err=%v)", method, want, werr)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("%s rejected with %q; want %s", method, err, want)
	}
	t.Logf("%s refused the receipt: %s", method, want)
}

// deploySwapRelayer deploys a SwapRelayer for this deployment's EnygmaDvp
// (retail does not deploy one), from enygma_dvp's build of the contract.
func deploySwapRelayer(t *testing.T, e *pwEnv, dvpAddr common.Address) *bind.BoundContract {
	t.Helper()
	data, err := os.ReadFile("../../enygma_dvp/artifacts/contracts/core/contracts/SwapRelayer.sol/SwapRelayer.json")
	if os.IsNotExist(err) {
		t.Skip("enygma_dvp's contracts are not compiled (no SwapRelayer artifact) — skipping the SwapRelayer check")
	}
	if err != nil {
		t.Fatalf("read SwapRelayer artifact: %v", err)
	}
	var artifact struct {
		ABI      json.RawMessage `json:"abi"`
		Bytecode string          `json:"bytecode"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("parse SwapRelayer artifact: %v", err)
	}
	srABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		t.Fatalf("parse SwapRelayer ABI: %v", err)
	}
	_, tx, sr, err := bind.DeployContract(e.owner, srABI, common.FromHex(artifact.Bytecode), e.client, dvpAddr)
	if err != nil {
		t.Fatalf("deploy SwapRelayer: %v", err)
	}
	if _, err := bind.WaitMined(e.ctx, e.client, tx); err != nil {
		t.Fatalf("wait SwapRelayer deploy: %v", err)
	}
	return sr
}

// TestV2Erc20PartialWithdraw_NoCrossSettlement is the regression test for a
// withdrawal receipt being settled by a different function first. Payment and
// withdrawal proofs used to share StMessage 0, so anyone could take a pending
// partialWithdraw receipt and settle it through vault.transfer (the withdrawn
// amount became a note owned by an address: locked forever) or withdrawV2 (the
// change note was dropped). Now a withdrawal proof carries WITHDRAW_MESSAGE:
// every payment and DvP path refuses it, withdrawV2 keeps its change note, and
// the withdrawal paths refuse a payment proof.
func TestRetailErc20_PartialWithdraw_NoCrossSettlement(t *testing.T) {
	e := newPartialWithdrawEnv(t)
	recipient := common.HexToAddress("0x0000000000000000000000000000000000005e84")
	receipts := loadOnchainReceipts(t)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	dvp := bind.NewBoundContract(dvpAddr, loadOnchainABI(t, "EnygmaDvp"), e.client, e.client, e.client)

	note := newDepositNote(t, e, big.NewInt(100))
	s := e.mustProveWithdraw(t, note, big.NewInt(10), big.NewInt(90), recipient)
	if s.receipt.Statement[0].Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("withdrawal proof carries StMessage %s, want WITHDRAW_MESSAGE (1)", s.receipt.Statement[0])
	}
	if !e.verifiesAt(t, 0, s.receipt) {
		t.Fatalf("the withdrawal proof does not verify against VK slot 0 — the regenerated Payment key is not registered")
	}

	t.Run("payment_paths_refuse_a_withdrawal", func(t *testing.T) {
		expectCallRevert(t, e, e.vault, "InvalidPaymentMessage", "transfer", s.receipt)
		expectCallRevert(t, e, e.vault, "InvalidPaymentMessage", "transferV2", s.receipt, [][]byte{{1}, {1}}, [][]byte{{1}, {1}})
		expectCallRevert(t, e, dvp, "InvalidPaymentMessage", "payment", s.receipt, big.NewInt(0), []byte{}, []byte{})
	})
	t.Run("dvp_paths_refuse_a_withdrawal", func(t *testing.T) {
		expectCallRevert(t, e, dvp, "InvalidPartialProofReceipt", "submitPartialSettlement", s.receipt, big.NewInt(0), big.NewInt(0), big.NewInt(0))
		sr := deploySwapRelayer(t, e, dvpAddr)
		expectCallRevert(t, e, sr, "WithdrawalReceipt", "submitReceipt", [32]byte{1}, s.receipt, true, big.NewInt(0), big.NewInt(0), []byte{}, []byte{})
	})
	t.Run("withdrawal_paths_refuse_a_payment", func(t *testing.T) {
		pay, err := e.proveWithdrawMsg(t, pwPayment, pwPaymentMessage, []pwNote{note}, big.NewInt(10), big.NewInt(90), nil, nil, recipient)
		if err != nil {
			t.Fatalf("gnark /proof/payment (message 0): %v", err)
		}
		expectCallRevert(t, e, e.vault, "InvalidPaymentMessage", "partialWithdraw", []*big.Int{big.NewInt(10), e.tokenId}, recipient, pay.receipt, pay.ctI, pay.ctII)
		expectCallRevert(t, e, e.vault, "InvalidPaymentMessage", "withdrawV2", []*big.Int{big.NewInt(10), e.tokenId}, recipient, pay.receipt)
	})
	t.Run("circuit_refuses_other_messages", func(t *testing.T) {
		if _, err := e.proveWithdrawMsg(t, pwPayment, "2", []pwNote{note}, big.NewInt(10), big.NewInt(90), nil, nil, recipient); err == nil {
			t.Fatalf("the Payment circuit proved StMessage 2")
		} else {
			t.Logf("prover refused StMessage 2: %.100s", err)
		}
	})
	t.Run("relayer_fee_withdrawal_refused_by_payment_paths", func(t *testing.T) {
		rs, err := e.proveWithdrawWith(t, pwPaymentRelayerFeePublic, []pwNote{note}, big.NewInt(10), big.NewInt(89), big.NewInt(1), nil, recipient)
		if err != nil {
			t.Fatalf("gnark /proof/paymentRelayerFeePublic: %v", err)
		}
		expectCallRevert(t, e, e.vault, "InvalidPaymentMessage", "transfer", rs.receipt)
		expectCallRevert(t, e, dvp, "InvalidPaymentMessage", "paymentWithRelayerFee", rs.receipt, big.NewInt(0), []byte{}, []byte{})
		expectCallRevert(t, e, e.vault, "InvalidNumberOfOutputs", "withdrawV2", []*big.Int{big.NewInt(10), e.tokenId}, recipient, rs.receipt)
	})

	// Settling the withdrawal through withdrawV2 instead of partialWithdraw
	// (what a front-runner could do) no longer loses the change: the 90 note
	// is inserted and spendable.
	before := e.balanceOf(t, recipient)
	tx, err := e.vault.Transact(e.owner, "withdrawV2", []*big.Int{big.NewInt(10), e.tokenId}, recipient, s.receipt)
	if err != nil {
		t.Fatalf("withdrawV2: %v", err)
	}
	if m, err := bind.WaitMined(e.ctx, e.client, tx); err != nil || m.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("withdrawV2 reverted (receipt=%v err=%v)", m, err)
	}
	mt := loadVaultMerkleTree(t, e.client, e.vaultAddr, partialWithdrawDepth)
	if _, err := mt.GenerateProof(s.change.commit); err != nil {
		t.Fatalf("withdrawV2 dropped the change note: %v", err)
	}
	rest := e.mustProveWithdraw(t, s.change, big.NewInt(90), big.NewInt(0), recipient)
	e.submitPartialWithdraw(t, big.NewInt(90), recipient, rest)
	if got := new(big.Int).Sub(e.balanceOf(t, recipient), before); got.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("recipient received %s, want 100 (10 through withdrawV2, then the 90 change)", got)
	}
	t.Logf("withdrawV2 kept the change note: 10 withdrawn, then the 90 change withdrawn in full")
}

// relayerFeeSpendKey is the RELAYER_FEE_SPEND_PRIVATE_KEY the relayer is
// started with for these tests.
var relayerFeeSpendKey = big.NewInt(123456789)

// relayPartialWithdrawReq is the retail relayer's POST /relay/partial_withdraw
// body: the /relay/payment_relayer_fee fields plus recipient and amount.
type relayPartialWithdrawReq struct {
	relayPaymentRelayerFeeRequest
	Recipient string `json:"recipient"`
	Amount    string `json:"amount"`
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
	if !serverAvailable("localhost:8090") {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}
	info := fetchRelayerInfo(t)
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
		vaultId:     "0", // the main payment-token vault (see scripts/cmd/init)
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
	// The retail route takes exactly 9 public signals; a shorter statement
	// leaves the rest empty, which the relayer refuses.
	var sig [9]string
	for i, v := range s.receipt.Statement {
		if i < len(sig) {
			sig[i] = v.String()
		}
	}
	feeSalt := "1"
	if s.feeSalt != nil {
		feeSalt = s.feeSalt.String()
	}
	return relayPartialWithdrawReq{
		relayPaymentRelayerFeeRequest: relayPaymentRelayerFeeRequest{
			VaultId:      r.vaultId,
			Proof:        proof,
			PublicSignal: sig,
			CipherText:   "0x" + hex.EncodeToString(s.ctI),
			EncTxData:    "0x" + hex.EncodeToString(s.ctII),
			FeeSalt:      feeSalt,
			TokenId:      r.tokenId.String(),
		},
		Recipient: recipient.Hex(),
		Amount:    amount.String(),
	}
}

// relay posts a partial withdrawal and requires it to settle.
func (r *relayerEnv) relay(t *testing.T, s *pwSpend) *types.Receipt {
	t.Helper()
	resp, status := postToRetailRelayer(t, "/relay/partial_withdraw", r.request(s, s.recipient, s.amount))
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
	if from := txSender(t, r.client, resp.TxHash); from != r.relayerAddr {
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
	resp, status := postToRetailRelayer(t, "/relay/partial_withdraw", req)
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

func TestRetailRelayer_PartialWithdraw_FullFlow(t *testing.T) {
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

func TestRetailRelayer_PartialWithdraw_Rejections(t *testing.T) {
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
		// Its 7-element statement does not fill the route's 9 signals.
		r.expectRelayRefused(t, r.request(plain, recipient, big.NewInt(10)), http.StatusBadRequest, "parse")
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

// TestRetailErc20_PartialWithdraw_RecoverChangeWithoutCiphertext is the regression test for a
// pending withdrawal's change note being made undiscoverable. Its
// EncryptedNote ciphertexts are not bound by the proof, so whoever settles the
// receipt first can replace them (here with junk) or omit them (withdrawV2).
// The circuit now requires a withdrawal's change salt to be
// Poseidon(sk, nullifier0), so the owner rebuilds the note from its spend key
// and the transaction's events alone, and spends it.
func TestRetailErc20_PartialWithdraw_RecoverChangeWithoutCiphertext(t *testing.T) {
	e := newPartialWithdrawEnv(t)
	recipient := common.HexToAddress("0x0000000000000000000000000000000000005e85")

	t.Run("prover_requires_the_recoverable_salt", func(t *testing.T) {
		note := newDepositNote(t, e, big.NewInt(100))
		random, _ := core.RandomInField()
		if _, err := e.proveWithdrawFull(t, pwPayment, pwWithdrawMessage, []pwNote{note}, big.NewInt(10), big.NewInt(90), nil, nil, random, recipient); err == nil {
			t.Fatalf("the Payment circuit proved a withdrawal whose change salt is not Poseidon(sk, nullifier0)")
		} else {
			t.Logf("Payment prover refused a random change salt: %.100s", err)
		}
		if _, err := e.proveWithdrawFull(t, pwPaymentRelayerFeePublic, pwWithdrawMessage, []pwNote{note}, big.NewInt(10), big.NewInt(89), big.NewInt(1), nil, random, recipient); err == nil {
			t.Fatalf("the PaymentRelayerFeePublic circuit proved a withdrawal whose change salt is not Poseidon(sk, nullifier0)")
		} else {
			t.Logf("PaymentRelayerFeePublic prover refused a random change salt: %.100s", err)
		}
		// A payment's change salt stays free (the KEM-derived one is used).
		if _, err := e.proveWithdrawMsg(t, pwPayment, pwPaymentMessage, []pwNote{note}, big.NewInt(10), big.NewInt(90), nil, nil, recipient); err != nil {
			t.Fatalf("a payment proof with a KEM-derived change salt was refused: %v", err)
		}
	})

	for _, via := range []string{"partialWithdraw_with_junk_ciphertexts", "withdrawV2"} {
		t.Run(via, func(t *testing.T) {
			note := newDepositNote(t, e, big.NewInt(100))
			s := e.mustProveWithdraw(t, note, big.NewInt(10), big.NewInt(90), recipient)
			before := e.balanceOf(t, recipient)

			// Someone else settles the owner's receipt first, without usable
			// discovery data.
			var tx *types.Transaction
			var err error
			if via == "withdrawV2" {
				tx, err = e.vault.Transact(e.owner, "withdrawV2", []*big.Int{big.NewInt(10), e.tokenId}, recipient, s.receipt)
			} else {
				tx, err = e.vault.Transact(e.owner, "partialWithdraw", []*big.Int{big.NewInt(10), e.tokenId}, recipient, s.receipt, []byte{0xde, 0xad}, []byte{0xbe, 0xef})
			}
			if err != nil {
				t.Fatalf("%s: %v", via, err)
			}
			mined, err := bind.WaitMined(e.ctx, e.client, tx)
			if err != nil || mined.Status != types.ReceiptStatusSuccessful {
				t.Fatalf("%s reverted (receipt=%v err=%v)", via, mined, err)
			}

			// The owner knows only its spend key and the note it held (100).
			// From the transaction: the first Nullifier event and the
			// PartialWithdrawal amount and changeCommitment.
			var nullifier0, amount, changeCmt *big.Int
			for _, lg := range mined.Logs {
				if lg.Address != e.vaultAddr || len(lg.Topics) == 0 {
					continue
				}
				ev, err := e.vaultABI.EventByID(lg.Topics[0])
				if err != nil {
					continue
				}
				switch ev.Name {
				case "Nullifier":
					if nullifier0 == nil {
						nullifier0 = lg.Topics[3].Big()
					}
				case "PartialWithdrawal":
					changeCmt = lg.Topics[3].Big()
					vals, err := ev.Inputs.NonIndexed().Unpack(lg.Data)
					if err != nil {
						t.Fatalf("unpack PartialWithdrawal: %v", err)
					}
					amount = vals[0].(*big.Int)
				case "EncryptedNote":
					vals, err := ev.Inputs.NonIndexed().Unpack(lg.Data)
					if err != nil {
						t.Fatalf("unpack EncryptedNote: %v", err)
					}
					if _, err := core.Decapsulate(note.view.DecapsKey, vals[0].([]byte)); err == nil {
						t.Fatalf("the replaced EncryptedNote still decapsulates — the junk did not take effect")
					}
				}
			}
			if nullifier0 == nil || amount == nil || changeCmt == nil {
				t.Fatalf("missing events: nullifier=%v amount=%v changeCommitment=%v", nullifier0, amount, changeCmt)
			}
			changeValue := new(big.Int).Sub(note.value, amount)
			salt, rebuilt, err := core.WithdrawChangeNote(note.spend.PrivateKey, nullifier0, changeValue, e.tokenId)
			if err != nil {
				t.Fatalf("WithdrawChangeNote: %v", err)
			}
			if rebuilt.Cmp(changeCmt) != 0 {
				t.Fatalf("rebuilt change note %s is not the settled changeCommitment %s", rebuilt, changeCmt)
			}

			// The rebuilt note is spendable: withdraw all 90.
			recovered := pwNote{spend: note.spend, view: note.view, salt: salt, value: changeValue, commit: rebuilt}
			rest := e.mustProveWithdraw(t, recovered, changeValue, big.NewInt(0), recipient)
			e.submitPartialWithdraw(t, changeValue, recipient, rest)
			if got := new(big.Int).Sub(e.balanceOf(t, recipient), before); got.Cmp(big.NewInt(100)) != 0 {
				t.Fatalf("recipient received %s, want 100 (10, then the recovered 90)", got)
			}
			t.Logf("%s: change note of %s rebuilt from the key and events, then withdrawn", via, changeValue)
		})
	}
}
