package tests

// Can PaymentFeeCircuit mint value?
//
// PaymentFeeCircuit enforces  Σ valuesIn == Σ valuesOut + StFee  in the field,
// range-checks every input and output value, but never range-checks StFee. A
// prover can therefore choose StFee = -d (mod Fr) and spend a note of value v
// into outputs worth v + d. The fee statement slot (index 7) is then a huge
// field element:
//
//   - payment() sends any 1-in/2-out receipt whose statement has 8 elements to
//     the PaymentFee verification key and never looks at the fee;
//   - paymentWithFee() only checks statement[fee] == protocolFee, and
//     protocolFee is an argument the caller supplies.
//
// Two layers now stop it, and the test passes if either does:
//
//   - the circuit range-checks StFee (the prover then refuses the witness);
//   - Erc20CoinVault.checkReceiptConditions rejects a fee-shaped receipt whose
//     StFee is not below MAX_FEE_AMOUNT (FeeOutOfRange). This one needs no new
//     keys, so it also protects a deployment whose PaymentFee key predates the
//     circuit fix.
//
// The attack spends a 10-token note into a 1010-token note through payment(),
// then withdraws 1010 tokens from a vault that received only 10 from the
// attacker, taking the difference from another depositor. A failing sub-test
// message starts with "VULNERABLE:".
//
// TestV2PaymentFee_HonestFeeStillWorks is the control: a normal fee payment with a
// small StFee must still settle.
//
// Prerequisites: the dedicated Payment deployment (see 12_v2_payment_usdr_fee_test.go)
// and the gnark server; the relayer is not needed.
//
// Run:
//   cd test && CC=/usr/bin/clang go test -run TestV2PaymentFee_NoValueInflation -v -timeout 600s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	core "github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// skipUnlessPaymentDeployment skips unless build/payment_receipts.json describes
// the dedicated Payment deployment that is actually on the connected chain.
// Deployment addresses are deterministic, so after a reset that redeploys only the
// main deployment the same addresses hold other contracts; a code check would
// pass wrongly. Only init_payment pins the USDr fee vault, so that is the probe.
func skipUnlessPaymentDeployment(t *testing.T, ctx context.Context, client *ethclient.Client, dvpAddr common.Address) {
	t.Helper()
	dvp := bind.NewBoundContract(dvpAddr, loadOnchainABI(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json"), client, client, client)
	var out []interface{}
	if err := dvp.Call(&bind.CallOpts{Context: ctx}, &out, "usdrFeeVaultSet"); err != nil || len(out) == 0 {
		t.Skipf("the dedicated Payment deployment is not on this chain (usdrFeeVaultSet() unavailable at %s) — skipping", dvpAddr)
	}
	if set, _ := out[0].(bool); !set {
		t.Skipf("the contract at %s is not an initialised Payment deployment — skipping", dvpAddr)
	}
}

// tryPostPaymentGnarkProof is postPaymentGnarkProof without the Fatalf: the gnark
// server refusing a witness is a result this test needs to inspect.
func tryPostPaymentGnarkProof(path string, body interface{}) (*paymentGnarkProofResp, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(paymentGnarkURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gnark server %d: %s", resp.StatusCode, string(raw))
	}
	var out paymentGnarkProofResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, fmt.Errorf("gnark error: %s", out.Error)
	}
	return &out, nil
}

func TestV2PaymentFee_NoValueInflation(t *testing.T) {
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
	defer client.Close()

	receipts := loadPaymentReceipts(t)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	skipUnlessPaymentDeployment(t, ctx, client, dvpAddr)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)

	dvp := bind.NewBoundContract(dvpAddr, loadOnchainABI(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json"), client, client, client)
	vault := bind.NewBoundContract(vaultAddr, loadOnchainABI(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json"), client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, loadOnchainABI(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json"), client, client, client)
	owner := hardhatAuth(t, client)

	const merkleDepth = 8
	tokenId := big.NewInt(0)
	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	fr := core.SNARK_SCALAR_FIELD

	balanceOf := func(a common.Address) *big.Int {
		var out []interface{}
		if err := erc20.Call(&bind.CallOpts{}, &out, "balanceOf", a); err != nil {
			t.Fatalf("balanceOf: %v", err)
		}
		return out[0].(*big.Int)
	}

	// Another depositor's funds sit in the vault.
	depositIntoVault(t, ctx, client, vault, erc20, vaultAddr, owner, big.NewInt(2000), tokenId, merkleDepth)

	// The attacker deposits 10.
	deposit := big.NewInt(10)
	inflated := big.NewInt(1010)
	d := depositIntoVault(t, ctx, client, vault, erc20, vaultAddr, owner, deposit, tokenId, merkleDepth)

	nullifier, err := core.GetNullifierBoundTree(d.spend.PrivateKey, big.NewInt(int64(d.merkleProof.TreeNumber)), d.merkleProof.Indices, merkleDepth, vaultAddrBig)
	if err != nil {
		t.Fatalf("GetNullifierBoundTree: %v", err)
	}
	inflSalt, _ := core.RandomInField()
	changeSalt, _ := core.RandomInField()
	cmtInflated, err := core.Erc20CommitmentV2(d.spend.PublicKey, inflSalt, inflated, tokenId)
	if err != nil {
		t.Fatalf("commitment: %v", err)
	}
	cmtChange, err := core.Erc20CommitmentV2(d.spend.PublicKey, changeSalt, big.NewInt(0), tokenId)
	if err != nil {
		t.Fatalf("commitment: %v", err)
	}

	var pathElements [8]string
	for j, e := range d.merkleProof.Elements {
		if j < 8 {
			pathElements[j] = e.String()
		}
	}
	// 10 == 1010 + StFee  (mod Fr)  =>  StFee = Fr - 1000
	stFee := new(big.Int).Sub(fr, new(big.Int).Sub(inflated, deposit))

	proof, proveErr := tryPostPaymentGnarkProof("/proof/paymentFee", map[string]interface{}{
		"stMessage":            "0",
		"stTreeNumbers":        [1]string{"0"},
		"stMerkleRoots":        [1]string{d.merkleProof.Root.String()},
		"stNullifiers":         [1]string{nullifier.String()},
		"stCommitmentsOut":     [2]string{cmtInflated.String(), cmtChange.String()},
		"stContractAddress":    vaultAddrBig.String(),
		"stFee":                stFee.String(),
		"wtPrivateKeysIn":      [1]string{d.spend.PrivateKey.String()},
		"wtValuesIn":           [1]string{deposit.String()},
		"wtSaltsIn":            [1]string{d.saltBField.String()},
		"wtPathElements":       [1][8]string{pathElements},
		"wtPathIndices":        [1]string{d.merkleProof.Indices.String()},
		"wtTokenId":            tokenId.String(),
		"wtSpendPublicKeysOut": [2]string{d.spend.PublicKey.String(), d.spend.PublicKey.String()},
		"wtValuesOut":          [2]string{inflated.String(), "0"},
		"wtSaltsOut":           [2]string{inflSalt.String(), changeSalt.String()},
	})
	if proveErr != nil {
		t.Logf("SAFE: the prover refused the inflating witness (the circuit range-checks StFee): %.160s", proveErr)
		return
	}
	t.Logf("the prover produced a proof spending a %s-token note into %s tokens (StFee=Fr-%s): the circuit does not range-check StFee",
		deposit, inflated, new(big.Int).Sub(inflated, deposit))

	proofStrs := make([]string, len(proof.Proof))
	for i, v := range proof.Proof {
		proofStrs[i] = v.String()
	}
	receipt := onchainProofReceipt{
		Proof:           proofStringsToOnchain(t, proofStrs),
		Statement:       proof.PublicSignal,
		NumberOfInputs:  big.NewInt(1),
		NumberOfOutputs: big.NewInt(2),
	}
	tx, err := dvp.Transact(owner, "payment", receipt, big.NewInt(0), []byte{}, []byte{})
	if err != nil {
		if !strings.Contains(err.Error(), "FeeOutOfRange") {
			t.Fatalf("payment() rejected the inflating proof, but not with FeeOutOfRange: %v", err)
		}
		t.Logf("SAFE: payment() rejected the inflating PaymentFee proof with FeeOutOfRange (contract guard): %.120s", err)
		return
	}
	mined, err := bind.WaitMined(ctx, client, tx)
	if err != nil || mined.Status != 1 {
		t.Logf("SAFE: payment() reverted on the inflating PaymentFee proof (status=%v err=%v)", mined, err)
		return
	}
	t.Errorf("VULNERABLE: payment() accepted a PaymentFee proof that turns a %s-token note into a %s-token note", deposit, inflated)

	// Spend the inflated note into a withdrawal commitment and take the tokens out.
	recipient := common.HexToAddress("0x000000000000000000000000000000000000bEEF")
	before := balanceOf(recipient)

	mt := loadVaultMerkleTree(t, client, vaultAddr, merkleDepth)
	mp, err := mt.GenerateProof(cmtInflated)
	if err != nil {
		t.Fatalf("GenerateProof (inflated note): %v", err)
	}
	nf2, err := core.GetNullifierBoundTree(d.spend.PrivateKey, big.NewInt(int64(mp.TreeNumber)), mp.Indices, merkleDepth, vaultAddrBig)
	if err != nil {
		t.Fatalf("GetNullifierBoundTree: %v", err)
	}
	wdCmt, err := core.Erc20CommitmentV2(new(big.Int).SetBytes(recipient.Bytes()), big.NewInt(0), inflated, tokenId)
	if err != nil {
		t.Fatalf("commitment: %v", err)
	}
	change2Salt, _ := core.RandomInField()
	change2, err := core.Erc20CommitmentV2(d.spend.PublicKey, change2Salt, big.NewInt(0), tokenId)
	if err != nil {
		t.Fatalf("commitment: %v", err)
	}
	var pe2 [8]string
	for j, e := range mp.Elements {
		if j < 8 {
			pe2[j] = e.String()
		}
	}
	wp := postPaymentGnarkProof(t, "/proof/payment", map[string]interface{}{
		"stMessage":            "0",
		"stTreeNumbers":        [1]string{"0"},
		"stMerkleRoots":        [1]string{mp.Root.String()},
		"stNullifiers":         [1]string{nf2.String()},
		"stCommitmentsOut":     [2]string{wdCmt.String(), change2.String()},
		"stContractAddress":    vaultAddrBig.String(),
		"wtPrivateKeysIn":      [1]string{d.spend.PrivateKey.String()},
		"wtValuesIn":           [1]string{inflated.String()},
		"wtSaltsIn":            [1]string{inflSalt.String()},
		"wtPathElements":       [1][8]string{pe2},
		"wtPathIndices":        [1]string{mp.Indices.String()},
		"wtTokenId":            tokenId.String(),
		"wtSpendPublicKeysOut": [2]string{new(big.Int).SetBytes(recipient.Bytes()).String(), d.spend.PublicKey.String()},
		"wtValuesOut":          [2]string{inflated.String(), "0"},
		"wtSaltsOut":           [2]string{"0", change2Salt.String()},
	})
	wpStrs := make([]string, len(wp.Proof))
	for i, v := range wp.Proof {
		wpStrs[i] = v.String()
	}
	wdReceipt := onchainProofReceipt{
		Proof:           proofStringsToOnchain(t, wpStrs),
		Statement:       wp.PublicSignal,
		NumberOfInputs:  big.NewInt(1),
		NumberOfOutputs: big.NewInt(2),
	}
	wtx, err := vault.Transact(owner, "withdrawV2", []*big.Int{inflated, tokenId}, recipient, wdReceipt)
	if err != nil {
		t.Fatalf("withdrawV2: %v", err)
	}
	if m, err := bind.WaitMined(ctx, client, wtx); err != nil || m.Status != 1 {
		t.Fatalf("withdrawV2 reverted (status=%v err=%v)", m, err)
	}
	got := new(big.Int).Sub(balanceOf(recipient), before)
	t.Errorf("  and withdrawn: recipient received %s tokens although the attacker deposited only %s", got, deposit)
}

// The control: a normal PaymentFee payment (fee absorbed, small StFee) settles
// through paymentWithFee, so the range guard does not break the honest path.
func TestV2PaymentFee_HonestFeeStillWorks(t *testing.T) {
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
	defer client.Close()

	receipts := loadPaymentReceipts(t)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	skipUnlessPaymentDeployment(t, ctx, client, dvpAddr)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)
	dvp := bind.NewBoundContract(dvpAddr, loadOnchainABI(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json"), client, client, client)
	vault := bind.NewBoundContract(vaultAddr, loadOnchainABI(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json"), client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, loadOnchainABI(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json"), client, client, client)
	owner := hardhatAuth(t, client)

	const merkleDepth = 8
	tokenId := big.NewInt(0)
	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())

	deposit, pay, change, fee := big.NewInt(10), big.NewInt(6), big.NewInt(3), big.NewInt(1)
	d := depositIntoVault(t, ctx, client, vault, erc20, vaultAddr, owner, deposit, tokenId, merkleDepth)
	nullifier, err := core.GetNullifierBoundTree(d.spend.PrivateKey, big.NewInt(int64(d.merkleProof.TreeNumber)), d.merkleProof.Indices, merkleDepth, vaultAddrBig)
	if err != nil {
		t.Fatalf("GetNullifierBoundTree: %v", err)
	}
	bob, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	saltBob, _ := core.RandomInField()
	saltChange, _ := core.RandomInField()
	cmtBob, err := core.Erc20CommitmentV2(bob.PublicKey, saltBob, pay, tokenId)
	if err != nil {
		t.Fatal(err)
	}
	cmtChange, err := core.Erc20CommitmentV2(d.spend.PublicKey, saltChange, change, tokenId)
	if err != nil {
		t.Fatal(err)
	}
	var pathElements [8]string
	for j, e := range d.merkleProof.Elements {
		if j < 8 {
			pathElements[j] = e.String()
		}
	}

	proof := postPaymentGnarkProof(t, "/proof/paymentFee", map[string]interface{}{
		"stMessage":            "0",
		"stTreeNumbers":        [1]string{"0"},
		"stMerkleRoots":        [1]string{d.merkleProof.Root.String()},
		"stNullifiers":         [1]string{nullifier.String()},
		"stCommitmentsOut":     [2]string{cmtBob.String(), cmtChange.String()},
		"stContractAddress":    vaultAddrBig.String(),
		"stFee":                fee.String(),
		"wtPrivateKeysIn":      [1]string{d.spend.PrivateKey.String()},
		"wtValuesIn":           [1]string{deposit.String()},
		"wtSaltsIn":            [1]string{d.saltBField.String()},
		"wtPathElements":       [1][8]string{pathElements},
		"wtPathIndices":        [1]string{d.merkleProof.Indices.String()},
		"wtTokenId":            tokenId.String(),
		"wtSpendPublicKeysOut": [2]string{bob.PublicKey.String(), d.spend.PublicKey.String()},
		"wtValuesOut":          [2]string{pay.String(), change.String()},
		"wtSaltsOut":           [2]string{saltBob.String(), saltChange.String()},
	})
	proofStrs := make([]string, len(proof.Proof))
	for i, v := range proof.Proof {
		proofStrs[i] = v.String()
	}
	receipt := onchainProofReceipt{
		Proof:           proofStringsToOnchain(t, proofStrs),
		Statement:       proof.PublicSignal,
		NumberOfInputs:  big.NewInt(1),
		NumberOfOutputs: big.NewInt(2),
	}
	tx, err := dvp.Transact(owner, "paymentWithFee", receipt, big.NewInt(0), fee, []byte{}, []byte{})
	if err != nil {
		t.Fatalf("an honest fee payment (fee=%s) was rejected: %v", fee, err)
	}
	mined, err := bind.WaitMined(ctx, client, tx)
	if err != nil || mined.Status != 1 {
		t.Fatalf("an honest fee payment reverted (status=%v err=%v)", mined, err)
	}
	t.Logf("honest fee payment settled: %s in, %s + %s out, fee %s absorbed", deposit, pay, change, fee)
}
