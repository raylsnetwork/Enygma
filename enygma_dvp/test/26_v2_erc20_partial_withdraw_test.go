package tests

// Partial withdrawal: take part of a note out of the vault and keep the rest.
//
// Erc20CoinVault.partialWithdraw spends a note through the plain Payment circuit
// (1-in/2-out, VK slot 0):
//
//   output[0] = Poseidon4(uint160(recipient), 0, amount, tokenId) — paid out
//   output[1] = the change note, inserted into the vault's Merkle tree
//
// TestV2Erc20PartialWithdraw_FullFlow walks one deposit of 100 down to zero:
//
//   1. deposit 100
//   2. partialWithdraw 10  → recipient +10, the 100 note is spent, a 90 note is
//      inserted and its EncryptedNote decrypts to 90 with the owner's view key
//   3. partialWithdraw 30 from the 90 note → recipient +30, a 60 note
//   4. withdrawV2 the 60 note in full → recipient has received all 100
//
// and checks the vault's token balance and the spent nullifiers at every step.
//
// TestV2Erc20PartialWithdraw_TwoInputs merges two notes (60 + 40) through the
// 2-input Payment circuit (VK slot 1): 10 is withdrawn, one change note of 90
// remains, and both input notes are spent.
//
// TestV2Erc20PartialWithdraw_RejectsFeeCircuits makes genuine PaymentFee and
// PaymentRelayerFeePublic proofs shaped as withdrawals, confirms each one
// verifies against its own VK, and checks that partialWithdraw still refuses
// both: their fee would leave the note outside the withdrawal and the change.
//
// TestV2Erc20PartialWithdraw_Rejections checks that partialWithdraw refuses:
// a replayed receipt, a different recipient or amount than the proof commits
// to, a zero amount, a statement of the wrong shape, and that the prover itself
// refuses to withdraw more than the note holds.
//
// Prerequisites: the dedicated Payment deployment (see 12_v2_payment_usdr_fee_test.go)
// and the gnark server on :8081; the relayer is not needed.
//
// Run:
//   cd test && CC=/usr/bin/clang go test -run TestV2Erc20PartialWithdraw -v -timeout 600s

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	core "github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

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
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping")
	}
	ctx := context.Background()
	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	t.Cleanup(client.Close)

	receipts := loadPaymentReceipts(t)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	skipUnlessPaymentDeployment(t, ctx, client, dvpAddr)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)

	vaultABI := loadOnchainABI(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	if _, ok := vaultABI.Methods["partialWithdraw"]; !ok {
		t.Fatalf("Erc20CoinVault ABI has no partialWithdraw — recompile the contracts")
	}
	return &pwEnv{
		ctx:       ctx,
		client:    client,
		vault:     bind.NewBoundContract(vaultAddr, vaultABI, client, client, client),
		erc20:     bind.NewBoundContract(erc20Addr, loadOnchainABI(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json"), client, client, client),
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
	receipts := loadPaymentReceipts(t)
	verifier := bind.NewBoundContract(common.HexToAddress(receipts["Verifier"].ContractAddress),
		loadOnchainABI(t, "core/contracts/Verifier.sol/Verifier.json"), e.client, e.client, e.client)
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
			if got := lg.Topics[3].Big(); got.Cmp(s.change.commit) != 0 {
				t.Errorf("PartialWithdrawal changeCommitment = %s, want %s", got, s.change.commit)
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
			saltB, _ := core.DerivePaymentSalt(ss)
			encKey, _ := core.DerivePaymentKey(ss)
			tokenId, amount, err := core.DecryptPayload(encKey, vals[1].([]byte))
			if err != nil {
				t.Fatalf("DecryptPayload change note: %v", err)
			}
			if amount.Cmp(s.change.value) != 0 || tokenId.Cmp(e.tokenId) != 0 {
				t.Errorf("change note decrypts to amount=%s tokenId=%s, want %s/%s", amount, tokenId, s.change.value, e.tokenId)
			}
			if core.SaltBToField(saltB).Cmp(s.change.salt) != 0 {
				t.Errorf("change note salt from the event does not match the committed salt")
			}
		case "Nullifier":
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
	d := depositIntoVault(t, e.ctx, e.client, e.vault, e.erc20, e.vaultAddr, e.owner, amount, e.tokenId, partialWithdrawDepth)
	return pwNote{spend: d.spend, view: d.view, salt: d.saltBField, value: amount, commit: d.commitment}
}

func TestV2Erc20PartialWithdraw_FullFlow(t *testing.T) {
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

func TestV2Erc20PartialWithdraw_Rejections(t *testing.T) {
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

func TestV2Erc20PartialWithdraw_TwoInputs(t *testing.T) {
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

func TestV2Erc20PartialWithdraw_RejectsFeeCircuits(t *testing.T) {
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

// deploySwapRelayer deploys a SwapRelayer for the Payment deployment's
// EnygmaDvp (the dedicated deployment does not include one).
func deploySwapRelayer(t *testing.T, e *pwEnv, dvpAddr common.Address) *bind.BoundContract {
	t.Helper()
	data, err := os.ReadFile("../artifacts/contracts/core/contracts/SwapRelayer.sol/SwapRelayer.json")
	if err != nil {
		t.Fatalf("read SwapRelayer artifact: %v", err)
	}
	var artifact struct {
		Bytecode string `json:"bytecode"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("parse SwapRelayer artifact: %v", err)
	}
	srABI := loadOnchainABI(t, "core/contracts/SwapRelayer.sol/SwapRelayer.json")
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
func TestV2Erc20PartialWithdraw_NoCrossSettlement(t *testing.T) {
	e := newPartialWithdrawEnv(t)
	recipient := common.HexToAddress("0x0000000000000000000000000000000000005e84")
	receipts := loadPaymentReceipts(t)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	dvp := bind.NewBoundContract(dvpAddr, loadOnchainABI(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json"), e.client, e.client, e.client)

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
