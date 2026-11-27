package tests

// The channel forms of the fee variants (#452): channelPaymentWithFee,
// channelPaymentWithRelayerFee and channelPaymentWithUsdrFee each settle a
// payment over a tag channel as channelPayment does (test 17), the proof's
// StCiphertextHash binding block_n, the tag t and the sealed note ctxt, and
// each keeps its variant's fee checks.
//
// Needs the retail stack (Hardhat :8545, gnark :8082; the relayer :8090 for
// the second test), on the keys and contracts of this change.

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
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

// channelFeePaid is a PaymentFee or PaymentRelayerFee channel payment,
// proved and ready to settle.
type channelFeePaid struct {
	note    *tags.ChannelPayment
	receipt onchainProofReceipt
	proof   []string
	signal  []*big.Int
	payeeCm *big.Int // C_dest
	feeSalt *big.Int // the relayer fee note's salt (PaymentRelayerFee only)
	feeCm   *big.Int // the relayer fee note (PaymentRelayerFee only)
	amount  *big.Int
}

// proveChannelFeePayment prepares a channel payment of amount to payee over
// ss, tagged for block, and proves it with circuit (pwPaymentFee, whose fee
// is burned, or pwPaymentRelayerFeePublic, whose fee is a third output note
// to relayerPk), spending in with change back to in's owner.
func proveChannelFeePayment(t *testing.T, e *pwEnv, circuit pwCircuit, ss []byte, block uint64, payee *big.Int, in pwNote, amount, change, fee, relayerPk *big.Int) channelFeePaid {
	t.Helper()
	note, err := tags.PrepareChannelPayment(ss, block, payee, amount, e.tokenId)
	if err != nil {
		t.Fatalf("PrepareChannelPayment: %v", err)
	}
	mt := rebuildVaultTree(t, e.client, e.vaultAddr, nil)
	mp, err := mt.GenerateProof(in.commit)
	if err != nil {
		t.Fatalf("GenerateProof: %v", err)
	}
	treeNumber := big.NewInt(int64(mp.TreeNumber))
	nullifier, err := core.GetNullifierWithTree(in.spend.PrivateKey, treeNumber, mp.Indices, partialWithdrawDepth)
	if err != nil {
		t.Fatalf("GetNullifierWithTree: %v", err)
	}
	var path [partialWithdrawDepth]string
	for j, el := range mp.Elements {
		if j < partialWithdrawDepth {
			path[j] = el.String()
		}
	}
	payeeCm, err := core.Erc20CommitmentV2(payee, note.Salt, amount, e.tokenId)
	if err != nil {
		t.Fatalf("payee commitment: %v", err)
	}
	// The circuits require the change salt Poseidon(sk, nullifier).
	changeSalt, err := core.WithdrawChangeSalt(in.spend.PrivateKey, nullifier)
	if err != nil {
		t.Fatalf("WithdrawChangeSalt: %v", err)
	}
	changeCm, err := core.Erc20CommitmentV2(in.spend.PublicKey, changeSalt, change, e.tokenId)
	if err != nil {
		t.Fatalf("change commitment: %v", err)
	}
	commitments := []string{payeeCm.String(), changeCm.String()}
	pks := []string{payee.String(), in.spend.PublicKey.String()}
	values := []string{amount.String(), change.String()}
	salts := []string{note.Salt.String(), changeSalt.String()}

	endpoint := "/proof/paymentFee"
	var feeSalt, feeCm *big.Int
	if circuit == pwPaymentRelayerFeePublic {
		endpoint = "/proof/paymentRelayerFeePublic"
		if feeSalt, err = core.RandomInField(); err != nil {
			t.Fatal(err)
		}
		if feeCm, err = core.Erc20CommitmentV2(relayerPk, feeSalt, fee, e.tokenId); err != nil {
			t.Fatalf("relayer fee commitment: %v", err)
		}
		commitments, pks = append(commitments, feeCm.String()), append(pks, relayerPk.String())
		values, salts = append(values, fee.String()), append(salts, feeSalt.String())
	}
	proof, err := tryPostPaymentGnarkProof(endpoint, map[string]interface{}{
		"stMessage":            pwPaymentMessage,
		"stTreeNumbers":        []string{treeNumber.String()},
		"stMerkleRoots":        []string{mp.Root.String()},
		"stNullifiers":         []string{nullifier.String()},
		"stCommitmentsOut":     commitments,
		"stContractAddress":    vaultInt(e.vaultAddr).String(),
		"stCiphertextHash":     core.ChannelPaymentCiphertextHash(block, note.Tag, note.Ctxt).String(),
		"stFee":                fee.String(),
		"wtPrivateKeysIn":      []string{in.spend.PrivateKey.String()},
		"wtValuesIn":           []string{in.value.String()},
		"wtSaltsIn":            []string{in.salt.String()},
		"wtPathElements":       [][partialWithdrawDepth]string{path},
		"wtPathIndices":        []string{mp.Indices.String()},
		"wtTokenId":            e.tokenId.String(),
		"wtSpendPublicKeysOut": pks,
		"wtValuesOut":          values,
		"wtSaltsOut":           salts,
	})
	if err != nil {
		t.Fatalf("prove %s: %v", endpoint, err)
	}
	proofStrs := make([]string, len(proof.Proof))
	for i, v := range proof.Proof {
		proofStrs[i] = v.String()
	}
	return channelFeePaid{
		note: note,
		receipt: onchainProofReceipt{
			Proof:           proofStringsToOnchain(t, proofStrs),
			Statement:       proof.PublicSignal,
			NumberOfInputs:  big.NewInt(1),
			NumberOfOutputs: big.NewInt(int64(len(commitments))),
		},
		proof: proofStrs, signal: proof.PublicSignal,
		payeeCm: payeeCm, feeSalt: feeSalt, feeCm: feeCm, amount: amount,
	}
}

// setFixedFee has the owner set one of EnygmaDvp's fixed fees (setter) to
// fee, putting back what getter read before when the test ends: the fees are
// the node's, and later tests run on run.sh's.
func setFixedFee(t *testing.T, e *pwEnv, dvp *bind.BoundContract, getter, setter string, fee *big.Int) {
	t.Helper()
	var before []interface{}
	if err := dvp.Call(&bind.CallOpts{Context: e.ctx}, &before, getter); err != nil {
		t.Fatalf("%s: %v", getter, err)
	}
	set := func(v *big.Int) error {
		tx, err := dvp.Transact(e.owner, setter, v)
		if err != nil {
			return err
		}
		r, err := bind.WaitMined(e.ctx, e.client, tx)
		if err != nil {
			return err
		}
		if r.Status != types.ReceiptStatusSuccessful {
			return fmt.Errorf("%s reverted", setter)
		}
		return nil
	}
	if err := set(fee); err != nil {
		t.Fatalf("%s(%s): %v", setter, fee, err)
	}
	t.Cleanup(func() {
		if err := set(before[0].(*big.Int)); err != nil {
			t.Errorf("restore %s: %v", getter, err)
		}
	})
}

// feeVariantEnv is what both tests below share: the stack's contracts, the
// fixed fees they set, Bob's key and a tag channel secret to him.
type feeVariantEnv struct {
	*pwEnv
	dvpAddr              common.Address
	dvp                  *bind.BoundContract
	usdrVault, usdrErc20 *bind.BoundContract
	usdrVaultAddr        common.Address
	usdrVaultId          *big.Int
	relayerFee, usdrFee  *big.Int
	ss                   []byte
	bob, alice           *core.SpendKeyPair
	aliceView            *core.ViewKeyPair
}

func newFeeVariantEnv(t *testing.T) *feeVariantEnv {
	t.Helper()
	e := newPartialWithdrawEnv(t)
	receipts := loadOnchainReceipts(t)
	f := &feeVariantEnv{
		pwEnv:         e,
		dvpAddr:       common.HexToAddress(receipts["EnygmaDvp"].ContractAddress),
		usdrVaultAddr: common.HexToAddress(receipts["UsdrCoinVault"].ContractAddress),
		relayerFee:    big.NewInt(3),
		usdrFee:       big.NewInt(4),
		ss:            make([]byte, 32),
	}
	f.dvp = bind.NewBoundContract(f.dvpAddr, loadOnchainABI(t, "EnygmaDvp"), e.client, e.client, e.client)
	f.usdrVault = bind.NewBoundContract(f.usdrVaultAddr, e.vaultABI, e.client, e.client, e.client)
	f.usdrErc20 = bind.NewBoundContract(common.HexToAddress(receipts["UsdrERC20"].ContractAddress), loadOnchainABI(t, "RaylsERC20"), e.client, e.client, e.client)
	var out []interface{}
	if err := f.dvp.Call(&bind.CallOpts{Context: e.ctx}, &out, "usdrFeeVaultId"); err != nil {
		t.Fatalf("usdrFeeVaultId: %v", err)
	}
	f.usdrVaultId = out[0].(*big.Int)
	setFixedFee(t, e, f.dvp, "relayerFixedFeeAmount", "setRelayerFixedFee", f.relayerFee)
	setFixedFee(t, e, f.dvp, "usdrFixedFeeAmount", "setUsdrFixedFee", f.usdrFee)
	if _, err := rand.Read(f.ss); err != nil {
		t.Fatal(err)
	}
	var err error
	if f.bob, err = core.NewSpendKeyPair(); err != nil {
		t.Fatal(err)
	}
	if f.alice, err = core.NewSpendKeyPair(); err != nil {
		t.Fatal(err)
	}
	if f.aliceView, err = core.NewViewKeyPair(); err != nil {
		t.Fatal(err)
	}
	return f
}

// scanBob has Bob, holding a decoy channel and the payer's, find the
// channel payments tagged for block, and checks he opens each, in order,
// to its amount and commitment.
func (f *feeVariantEnv) scanBob(t *testing.T, block uint64, wantAmounts, wantCms []*big.Int) []tags.FoundChannelPayment {
	t.Helper()
	decoy := make([]byte, 32)
	if _, err := rand.Read(decoy); err != nil {
		t.Fatal(err)
	}
	channels := []tags.Channel{{SharedSecret: decoy, PkSpend: f.bob.PublicKey}, {SharedSecret: f.ss, PkSpend: f.bob.PublicKey}}
	found, err := tags.ScanChannelPayments(f.ctx, f.client, f.dvpAddr, block, channels)
	if err != nil {
		t.Fatalf("ScanChannelPayments: %v", err)
	}
	if len(found) != len(wantAmounts) {
		t.Fatalf("Bob's scan of block_n found %d payments, want %d", len(found), len(wantAmounts))
	}
	for i, fp := range found {
		note, err := fp.Open()
		if err != nil {
			t.Fatalf("Bob cannot open payment %d: %v", i, err)
		}
		if note.Amount.Cmp(wantAmounts[i]) != 0 || fp.Commitment.Cmp(wantCms[i]) != 0 {
			t.Errorf("payment %d opens to %s at %s, want %s at %s", i, note.Amount, fp.Commitment, wantAmounts[i], wantCms[i])
		}
	}
	return found
}

func TestRetailErc20_ChannelPaymentFeeVariants(t *testing.T) {
	f := newFeeVariantEnv(t)
	relayer, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Every note is deposited before block_n is read: each deposit mines a
	// block.
	noteFee := depositFor(t, f.pwEnv, f.alice, f.aliceView, big.NewInt(100))
	noteRelayer := depositFor(t, f.pwEnv, f.alice, f.aliceView, big.NewInt(100))
	noteUsdrMain := depositFor(t, f.pwEnv, f.alice, f.aliceView, big.NewInt(100))
	noteOther := depositFor(t, f.pwEnv, f.alice, f.aliceView, big.NewInt(100))
	usdrIn := depositUsdrFeeInput(t, f.ctx, f.client, f.usdrVault, f.usdrErc20, f.usdrVaultAddr, f.owner, f.usdrFee, f.tokenId)

	head, err := f.client.BlockNumber(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	blockN := head + 1
	const protocolFee = 2
	pFee := proveChannelFeePayment(t, f.pwEnv, pwPaymentFee, f.ss, blockN, f.bob.PublicKey, noteFee, big.NewInt(60), big.NewInt(38), big.NewInt(protocolFee), nil)
	pRelayer := proveChannelFeePayment(t, f.pwEnv, pwPaymentRelayerFeePublic, f.ss, blockN, f.bob.PublicKey, noteRelayer,
		big.NewInt(60), new(big.Int).Sub(big.NewInt(40), f.relayerFee), f.relayerFee, relayer.PublicKey)
	pUsdr := proveChannelPayment(t, f.pwEnv, f.ss, blockN, f.bob.PublicKey, []pwNote{noteUsdrMain}, big.NewInt(60), big.NewInt(40))
	// Another channel payment, never settled: the fee leg must not settle
	// next to it.
	pOther := proveChannelPayment(t, f.pwEnv, f.ss, blockN, f.bob.PublicKey, []pwNote{noteOther}, big.NewInt(60), big.NewInt(40))
	// The USDr fee leg binds its main leg's statement, so it is proved after it.
	usdr := buildUsdrFeeWitness(t, f.client, f.usdrVaultAddr, usdrIn, pUsdr.res.ContractStatement(), f.usdrFee, relayer.PublicKey, f.tokenId)
	usdrReceipt := onchainProofReceipt{
		Proof:           proofStringsToOnchain(t, usdr.proof[:]),
		Statement:       decimals(t, usdr.publicSignal[:]),
		NumberOfInputs:  big.NewInt(layout.UsdrFeeNullifiersSize),
		NumberOfOutputs: big.NewInt(layout.UsdrFeeCommitmentsOutSize),
	}

	block := func(b uint64) *big.Int { return new(big.Int).SetUint64(b) }
	vault := big.NewInt(0)
	variants := []struct {
		name, method string
		note         *tags.ChannelPayment
		args         func(b uint64, tag [32]byte, ctxt []byte) []interface{}
	}{
		{"with_fee", "channelPaymentWithFee", pFee.note, func(b uint64, tag [32]byte, ctxt []byte) []interface{} {
			return []interface{}{pFee.receipt, vault, big.NewInt(protocolFee), block(b), tag, ctxt}
		}},
		{"with_relayer_fee", "channelPaymentWithRelayerFee", pRelayer.note, func(b uint64, tag [32]byte, ctxt []byte) []interface{} {
			return []interface{}{pRelayer.receipt, vault, block(b), tag, ctxt}
		}},
		{"with_usdr_fee", "channelPaymentWithUsdrFee", pUsdr.note, func(b uint64, tag [32]byte, ctxt []byte) []interface{} {
			return []interface{}{pUsdr.receipt, vault, block(b), tag, ctxt, usdrReceipt, f.usdrVaultId, usdr.ctxtFee, usdr.ctxtChange}
		}},
	}

	// None of these settles, and none mines a block: each is a call against
	// the pending block, block_n.
	for _, v := range variants {
		t.Run(v.name+"/other_tag", func(t *testing.T) {
			tag := v.note.Tag
			tag[31] ^= 1
			expectPendingRevert(t, f.pwEnv, f.dvp, "CiphertextHashMismatch", v.method, v.args(blockN, tag, v.note.Ctxt)...)
		})
		t.Run(v.name+"/other_ctxt", func(t *testing.T) {
			ctxt := append([]byte{}, v.note.Ctxt...)
			ctxt[len(ctxt)-1] ^= 1
			expectPendingRevert(t, f.pwEnv, f.dvp, "CiphertextHashMismatch", v.method, v.args(blockN, v.note.Tag, ctxt)...)
		})
		t.Run(v.name+"/other_block", func(t *testing.T) {
			// block_n - 1 is inside the window too, so only the binding refuses it.
			expectPendingRevert(t, f.pwEnv, f.dvp, "CiphertextHashMismatch", v.method, v.args(blockN-1, v.note.Tag, v.note.Ctxt)...)
		})
	}

	// Each variant's own checks, before its proof is verified.
	n := boundTo(blockN, pFee.note)
	t.Run("with_fee/fee_mismatch", func(t *testing.T) {
		expectPendingRevert(t, f.pwEnv, f.dvp, "fee mismatch", "channelPaymentWithFee", pFee.receipt, vault, big.NewInt(protocolFee+1), n.block, n.tag, n.ctxt)
	})
	t.Run("with_fee/payment_receipt", func(t *testing.T) {
		expectPendingRevert(t, f.pwEnv, f.dvp, "InvalidStatementSize", "channelPaymentWithFee", pUsdr.receipt, vault, big.NewInt(protocolFee), n.block, n.tag, n.ctxt)
	})
	n = boundTo(blockN, pRelayer.note)
	t.Run("with_relayer_fee/fee_not_fixed", func(t *testing.T) {
		r := pRelayer.receipt
		r.Statement = append([]*big.Int{}, r.Statement...)
		r.Statement[layout.PaymentRelayerFeeFeeOffset] = new(big.Int).Add(f.relayerFee, big.NewInt(1))
		expectPendingRevert(t, f.pwEnv, f.dvp, "InvalidRelayerFee", "channelPaymentWithRelayerFee", r, vault, n.block, n.tag, n.ctxt)
	})
	t.Run("with_relayer_fee/fee_receipt", func(t *testing.T) {
		expectPendingRevert(t, f.pwEnv, f.dvp, "InvalidStatementSize", "channelPaymentWithRelayerFee", pFee.receipt, vault, n.block, n.tag, n.ctxt)
	})
	n = boundTo(blockN, pUsdr.note)
	t.Run("with_usdr_fee/other_usdr_vault", func(t *testing.T) {
		expectPendingRevert(t, f.pwEnv, f.dvp, "InvalidUsdrVault", "channelPaymentWithUsdrFee", pUsdr.receipt, vault, n.block, n.tag, n.ctxt, usdrReceipt, vault, usdr.ctxtFee, usdr.ctxtChange)
	})
	t.Run("with_usdr_fee/usdr_fee_not_fixed", func(t *testing.T) {
		r := usdrReceipt
		r.Statement = append([]*big.Int{}, r.Statement...)
		r.Statement[layout.UsdrFeeFeeOffset] = new(big.Int).Add(f.usdrFee, big.NewInt(1))
		expectPendingRevert(t, f.pwEnv, f.dvp, "InvalidUsdrFee", "channelPaymentWithUsdrFee", pUsdr.receipt, vault, n.block, n.tag, n.ctxt, r, f.usdrVaultId, usdr.ctxtFee, usdr.ctxtChange)
	})
	t.Run("with_usdr_fee/fee_receipt", func(t *testing.T) {
		expectPendingRevert(t, f.pwEnv, f.dvp, "InvalidStatementSize", "channelPaymentWithUsdrFee", pFee.receipt, vault, n.block, n.tag, n.ctxt, usdrReceipt, f.usdrVaultId, usdr.ctxtFee, usdr.ctxtChange)
	})
	t.Run("with_usdr_fee/usdr_ciphertexts", func(t *testing.T) {
		// The USDr leg keeps its own binding, to its fee note's ciphertexts.
		expectPendingRevert(t, f.pwEnv, f.dvp, "CiphertextHashMismatch", "channelPaymentWithUsdrFee", pUsdr.receipt, vault, n.block, n.tag, n.ctxt, usdrReceipt, f.usdrVaultId, []byte{1}, usdr.ctxtChange)
	})
	t.Run("with_usdr_fee/fee_leg_of_another_payment", func(t *testing.T) {
		// The other payment settles on its own (a call: nothing is mined),
		// but the fee leg, bound to pUsdr's statement, is refused next to it.
		o := boundTo(blockN, pOther.note)
		var out []interface{}
		if err := f.dvp.Call(&bind.CallOpts{Context: f.ctx, Pending: true, From: f.owner.From}, &out, "channelPayment", pOther.receipt, vault, o.block, o.tag, o.ctxt); err != nil {
			t.Fatalf("the other payment does not settle on its own: %v", err)
		}
		expectPendingRevert(t, f.pwEnv, f.dvp, "CiphertextHashMismatch", "channelPaymentWithUsdrFee", pOther.receipt, vault, o.block, o.tag, o.ctxt, usdrReceipt, f.usdrVaultId, usdr.ctxtFee, usdr.ctxtChange)
	})

	// All three settle, each in one transaction, in block_n, block_n + 1 and
	// block_n + 2.
	var mined []*types.Receipt
	for _, v := range variants {
		tx, err := f.dvp.Transact(f.owner, v.method, v.args(blockN, v.note.Tag, v.note.Ctxt)...)
		if err != nil {
			t.Fatalf("%s: %v", v.method, err)
		}
		r, err := bind.WaitMined(f.ctx, f.client, tx)
		if err != nil || r.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("%s did not settle (err=%v)", v.method, err)
		}
		mined = append(mined, r)
	}
	if got := mined[0].BlockNumber.Uint64(); got != blockN {
		t.Fatalf("the first payment landed in block %d, want block_n = %d", got, blockN)
	}

	// The fee notes are in: the relayer's PaymentRelayerFee output 2 in the
	// main vault, its USDr fee note in the USDr vault.
	if _, err := rebuildVaultTree(t, f.client, f.vaultAddr, nil).GenerateProof(pRelayer.feeCm); err != nil {
		t.Errorf("the relayer's fee note is not in the vault: %v", err)
	}
	if _, err := rebuildVaultTree(t, f.client, f.usdrVaultAddr, nil).GenerateProof(usdr.cmtFee); err != nil {
		t.Errorf("the relayer's USDr fee note is not in the USDr vault: %v", err)
	}

	found := f.scanBob(t, blockN, []*big.Int{big.NewInt(60), big.NewInt(60), big.NewInt(60)},
		[]*big.Int{pFee.payeeCm, pRelayer.payeeCm, pUsdr.res.CommitmentsOut()[0]})
	for i, fp := range found {
		if fp.TxHash != mined[i].TxHash {
			t.Errorf("payment %d was found in tx %s, settled in %s", i, fp.TxHash, mined[i].TxHash)
		}
	}
	t.Logf("three channel fee payments for block %d settled; Bob found and opened all three by tag", blockN)
}

// boundFields is a channel payment's block, tag and ctxt as call arguments.
type boundFields struct {
	block *big.Int
	tag   [32]byte
	ctxt  []byte
}

func boundTo(block uint64, note *tags.ChannelPayment) boundFields {
	return boundFields{new(big.Int).SetUint64(block), note.Tag, note.Ctxt}
}

// decimals parses decimal strings, as the relay requests carry signals.
func decimals(t *testing.T, s []string) []*big.Int {
	t.Helper()
	out := make([]*big.Int, len(s))
	for i, v := range s {
		n, ok := new(big.Int).SetString(v, 10)
		if !ok {
			t.Fatalf("not a decimal: %q", v)
		}
		out[i] = n
	}
	return out
}

func decimalStrings(v []*big.Int) []string {
	out := make([]string, len(v))
	for i, n := range v {
		out[i] = n.String()
	}
	return out
}

// postChannelRoute posts body to the relayer's path and returns its answer.
func postChannelRoute(t *testing.T, path string, body interface{}) (relayPaymentResponse, int) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, relayerURL+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+relayerAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	var out relayPaymentResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out, resp.StatusCode
}

// The relayer's POST /relay/channel_payment_relayer_fee and
// /relay/channel_payment_usdr_fee settle the channel forms of its two fee
// routes, each as one transaction from its own address, keeping its fee
// note, and refuse a tag other than the bound one before sending anything.
func TestRetailErc20_ChannelPaymentFeeVariantsViaRelayer(t *testing.T) {
	f := newFeeVariantEnv(t)
	if !serverAvailable("localhost:8090") {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}
	info := fetchRelayerInfo(t)
	relayerPk, ok := new(big.Int).SetString(info.FeeSpendPubKey, 10)
	if !ok {
		t.Skip("relayer is not configured with RELAYER_FEE_SPEND_PRIVATE_KEY — skipping")
	}

	noteRelayer := depositFor(t, f.pwEnv, f.alice, f.aliceView, big.NewInt(50))
	noteUsdrMain := depositFor(t, f.pwEnv, f.alice, f.aliceView, big.NewInt(50))
	usdrIn := depositUsdrFeeInput(t, f.ctx, f.client, f.usdrVault, f.usdrErc20, f.usdrVaultAddr, f.owner, f.usdrFee, f.tokenId)
	var usdr usdrFeeWitness // proved by the usdr_fee route, after its main leg

	type relayerFeeReq struct {
		VaultId      string    `json:"vaultId"`
		Proof        [8]string `json:"proof"`
		PublicSignal []string  `json:"publicSignal"`
		BlockNumber  uint64    `json:"blockNumber"`
		Tag          string    `json:"tag"`
		Ctxt         string    `json:"ctxt"`
		FeeSalt      string    `json:"feeSalt"`
		TokenId      string    `json:"tokenId"`
	}
	type usdrFeeReq struct {
		VaultId          string    `json:"vaultId"`
		Proof            [8]string `json:"proof"`
		PublicSignal     []string  `json:"publicSignal"`
		BlockNumber      uint64    `json:"blockNumber"`
		Tag              string    `json:"tag"`
		Ctxt             string    `json:"ctxt"`
		UsdrVaultId      string    `json:"usdrVaultId"`
		UsdrProof        [8]string `json:"usdrProof"`
		UsdrPublicSignal []string  `json:"usdrPublicSignal"`
		UsdrCipherText   string    `json:"usdrCipherText"`
		UsdrEncTxData    string    `json:"usdrEncTxData"`
		UsdrFeeSalt      string    `json:"usdrFeeSalt"`
	}
	var relayerFeeCm *big.Int // the relayer's fee note, PaymentRelayerFee's output 2
	// proved is one route's payment, proved for block_n: its request with a
	// given tag, its bound tag, and Bob's commitment.
	type proved struct {
		req     func(tag [32]byte) interface{}
		tag     [32]byte
		payeeCm *big.Int
	}
	routes := []struct {
		name, path string
		prove      func(blockN uint64) proved
	}{
		{"relayer_fee", "/relay/channel_payment_relayer_fee", func(blockN uint64) proved {
			p := proveChannelFeePayment(t, f.pwEnv, pwPaymentRelayerFeePublic, f.ss, blockN, f.bob.PublicKey, noteRelayer,
				big.NewInt(35), new(big.Int).Sub(big.NewInt(15), f.relayerFee), f.relayerFee, relayerPk)
			relayerFeeCm = p.feeCm
			var proof [8]string
			copy(proof[:], p.proof)
			return proved{func(tag [32]byte) interface{} {
				return relayerFeeReq{VaultId: "0", Proof: proof, PublicSignal: decimalStrings(p.signal), BlockNumber: blockN,
					Tag: hexutil.Encode(tag[:]), Ctxt: hexutil.Encode(p.note.Ctxt), FeeSalt: p.feeSalt.String(), TokenId: f.tokenId.String()}
			}, p.note.Tag, p.payeeCm}
		}},
		{"usdr_fee", "/relay/channel_payment_usdr_fee", func(blockN uint64) proved {
			p := proveChannelPayment(t, f.pwEnv, f.ss, blockN, f.bob.PublicKey, []pwNote{noteUsdrMain}, big.NewInt(35), big.NewInt(15))
			// The USDr fee leg binds its main leg's statement.
			usdr = buildUsdrFeeWitness(t, f.client, f.usdrVaultAddr, usdrIn, p.res.ContractStatement(), f.usdrFee, relayerPk, f.tokenId)
			var proof [8]string
			copy(proof[:], p.res.Proof)
			return proved{func(tag [32]byte) interface{} {
				return usdrFeeReq{VaultId: "0", Proof: proof, PublicSignal: decimalStrings(p.res.Statement), BlockNumber: blockN,
					Tag: hexutil.Encode(tag[:]), Ctxt: hexutil.Encode(p.note.Ctxt),
					UsdrVaultId: f.usdrVaultId.String(), UsdrProof: usdr.proof, UsdrPublicSignal: usdr.publicSignal[:],
					UsdrCipherText: hexutil.Encode(usdr.ctxtFee), UsdrEncTxData: hexutil.Encode(usdr.ctxtChange), UsdrFeeSalt: usdr.feeSalt.String()}
			}, p.note.Tag, p.res.CommitmentsOut()[0]}
		}},
	}

	// Each route's payment is tagged for the block after the head, the one
	// its transaction lands in: only a simulation against the pending block
	// sees it inside its window.
	for _, r := range routes {
		head, err := f.client.BlockNumber(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		blockN := head + 1
		p := r.prove(blockN)
		t.Run(r.name+"/other_tag", func(t *testing.T) {
			tag := p.tag
			tag[0] ^= 1
			out, status := postChannelRoute(t, r.path, p.req(tag))
			if status != http.StatusUnprocessableEntity || !strings.Contains(out.Error, "CiphertextHashMismatch") {
				t.Fatalf("got %d %q; want 422 naming CiphertextHashMismatch", status, out.Error)
			}
			if now, _ := f.client.BlockNumber(f.ctx); now != head {
				t.Fatalf("the refused request mined a block (head %d, was %d)", now, head)
			}
		})
		out, status := postChannelRoute(t, r.path, p.req(p.tag))
		if status != http.StatusOK {
			t.Fatalf("POST %s: %d %s", r.path, status, out.Error)
		}
		if out.BlockNumber != blockN {
			t.Fatalf("%s landed in block %d, want block_n = %d", r.path, out.BlockNumber, blockN)
		}
		if got := txSender(t, f.client, out.TxHash); got != common.HexToAddress(info.RelayerAddr) {
			t.Errorf("%s was sent by %s, want the relayer %s", r.path, got.Hex(), info.RelayerAddr)
		}
		found := f.scanBob(t, blockN, []*big.Int{big.NewInt(35)}, []*big.Int{p.payeeCm})
		if found[0].TxHash != common.HexToHash(out.TxHash) {
			t.Errorf("%s: Bob found the payment in tx %s, relayed as %s", r.name, found[0].TxHash, out.TxHash)
		}
	}
	if _, err := rebuildVaultTree(t, f.client, f.vaultAddr, nil).GenerateProof(relayerFeeCm); err != nil {
		t.Errorf("the relayer's fee note is not in the vault: %v", err)
	}
	if _, err := rebuildVaultTree(t, f.client, f.usdrVaultAddr, nil).GenerateProof(usdr.cmtFee); err != nil {
		t.Errorf("the relayer's USDr fee note is not in the USDr vault: %v", err)
	}
}
