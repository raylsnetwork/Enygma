package tests

// 18_v2_swaprelayer_onchain_test.go
//
// End-to-end check of SwapRelayer against the real EnygmaDvp, vaults and
// Groth16 proofs (17_ covers the same logic on a mock).
//
//   1. Alice submits her payment leg (ERC-20), the swap expires and a third
//      party cancels it: her note is unlocked.
//   2. Bob's late delivery leg (ERC-721) under that swapId is rejected with
//      SwapClosed and his note is NOT locked; replaying Alice's leg is
//      rejected too.
//   3. A second pair of legs: a leg with a corrupted proof, and a genuine leg
//      under the wrong swapId, are rejected without locking anything; an
//      expiry beyond MAX_SWAP_DURATION is rejected; then both legs settle.
//   4. The settled swapId is closed as well.
//
// The swapId is derived from the legs (SwapRelayer.swapIdOf), so a leg can
// only occupy its own swap's slot.
//
// deploy/init do not deploy SwapRelayer, so the test deploys it, registers it
// as a relayer and registers the (ERC-20, ERC-721) vault pair itself.
//
// Prerequisites: fresh Hardhat node + deploy + init (see test/README.md),
// gnark server on :8081, and `npx hardhat compile` for the SwapRelayer artifact.
//
//	cd test && CC=/usr/bin/clang go test -run TestV2SwapRelayer_OnChain -v -timeout 600s

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"math/big"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// Hardhat accounts 3–5 from enygmadvp.config.json (account 2 is the relayer
// service's key; 0 is the deployer/owner).
var swapRelayerTestKeys = map[string]string{
	"alice":   "7eb9f1b270fe1d052b287322b891df041190f80b92b149b4308d5dee1a791af5",
	"bob":     "678ffea2bbaf2488a3dbcad03a4aa5b54b04f0ab1d135cc659ce74866c590289",
	"mallory": "40217257b8f9ce38193e802494700f987ffe45d1d55f751263ea522cf763d8dd",
}

type onchainSwapRelayer struct {
	t      *testing.T
	client *ethclient.Client
	addr   common.Address
	abi    abi.ABI
	keys   map[string]*ecdsa.PrivateKey
	vaults map[int64]*bind.BoundContract
}

func (r *onchainSwapRelayer) auth(who string) *bind.TransactOpts {
	auth, err := bind.NewKeyedTransactorWithChainID(r.keys[who], big.NewInt(hardhatChainID))
	if err != nil {
		r.t.Fatal(err)
	}
	auth.GasLimit = 6_000_000
	return auth
}

// send simulates the call (to get a revert's custom error), then mines it.
// It returns the custom error name, or "" on success.
func (r *onchainSwapRelayer) send(who, method string, args ...interface{}) (string, *types.Receipt) {
	r.t.Helper()
	data, err := r.abi.Pack(method, args...)
	if err != nil {
		r.t.Fatalf("pack %s: %v", method, err)
	}
	auth := r.auth(who)
	call := ethereum.CallMsg{From: auth.From, To: &r.addr, Gas: auth.GasLimit, Data: data}
	if _, err := r.client.CallContract(context.Background(), call, nil); err != nil {
		return r.revertName(err), nil
	}
	c := bind.NewBoundContract(r.addr, r.abi, r.client, r.client, r.client)
	tx, err := c.RawTransact(auth, data)
	if err != nil {
		r.t.Fatalf("send %s: %v", method, err)
	}
	rcpt, err := bind.WaitMined(context.Background(), r.client, tx)
	if err != nil {
		r.t.Fatalf("wait %s: %v", method, err)
	}
	if rcpt.Status != types.ReceiptStatusSuccessful {
		r.t.Fatalf("%s mined but reverted", method)
	}
	return "", rcpt
}

func (r *onchainSwapRelayer) revertName(err error) string {
	var de rpc.DataError
	if errors.As(err, &de) {
		if s, ok := de.ErrorData().(string); ok {
			raw, _ := hex.DecodeString(strings.TrimPrefix(s, "0x"))
			if len(raw) >= 4 {
				for name, ce := range r.abi.Errors {
					if string(ce.ID[:4]) == string(raw[:4]) {
						return name
					}
				}
			}
		}
	}
	// Hardhat reports custom errors in the message instead of as error data.
	if m := hardhatCustomErr.FindStringSubmatch(err.Error()); m != nil {
		return m[1]
	}
	return err.Error()
}

var hardhatCustomErr = regexp.MustCompile(`custom error '(\w+)\(`)

// nullifierState reports whether the receipt's (single) input nullifier is
// locked and whether it is spent in the given vault.
func (r *onchainSwapRelayer) nullifierState(vaultID int64, rc onchainProofReceipt) (locked, spent bool) {
	r.t.Helper()
	tree, nf := rc.Statement[1], rc.Statement[3]
	var out []interface{}
	if err := r.vaults[vaultID].Call(nil, &out, "isLocked", tree, nf); err != nil {
		r.t.Fatalf("isLocked: %v", err)
	}
	locked = out[0].(bool)
	out = nil
	if err := r.vaults[vaultID].Call(nil, &out, "nullifiers", tree, nf); err != nil {
		r.t.Fatalf("nullifiers: %v", err)
	}
	return locked, out[0].(bool)
}

func (r *onchainSwapRelayer) now() int64 {
	h, err := r.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		r.t.Fatal(err)
	}
	return int64(h.Time)
}

func (r *onchainSwapRelayer) advance(seconds int64) {
	r.t.Helper()
	if err := r.client.Client().Call(nil, "evm_increaseTime", seconds); err != nil {
		r.t.Fatalf("evm_increaseTime: %v", err)
	}
	if err := r.client.Client().Call(nil, "evm_mine"); err != nil {
		r.t.Fatalf("evm_mine: %v", err)
	}
}

func mustTx(t *testing.T, client *ethclient.Client, what string, tx *types.Transaction, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	rcpt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil || rcpt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("%s: mined status=%v err=%v", what, rcpt, err)
	}
}

// buildSwapLegs deposits Alice's 30 ERC-20 and Bob's ERC-721 and returns the
// payment (DvPInitiator) and delivery (DvPDestination) receipts for a swap,
// following TestDvP_SwapViaRelayer.
func buildSwapLegs(t *testing.T, client *ethclient.Client, owner *bind.TransactOpts, receipts map[string]onchainReceiptEntry) (pay, del onchainProofReceipt, commitA, commitB *big.Int) {
	t.Helper()
	erc20VaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	nftVaultAddr := common.HexToAddress(receipts["Erc721CoinVault"].ContractAddress)
	erc20 := bind.NewBoundContract(common.HexToAddress(receipts["ERC20"].ContractAddress),
		loadOnchainABI(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json"), client, client, client)
	erc721 := bind.NewBoundContract(common.HexToAddress(receipts["ERC721"].ContractAddress),
		loadOnchainABI(t, "erc721/contracts/RaylsERC721.sol/RaylsERC721.json"), client, client, client)
	erc20Vault := bind.NewBoundContract(erc20VaultAddr,
		loadOnchainABI(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json"), client, client, client)
	nftVault := bind.NewBoundContract(nftVaultAddr,
		loadOnchainABI(t, "core/contracts/vaults/Erc721CoinVault.sol/Erc721CoinVault.json"), client, client, client)

	gnark := core.NewGnarkClient("http://localhost:8081")
	const depth = 8
	amount, tokenId := big.NewInt(30), big.NewInt(0)
	nftTokenId := big.NewInt(time.Now().UnixNano() % (1 << 32))
	one := big.NewInt(1)

	// Alice's ERC-20 note.
	aliceSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	aliceView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	ss, capsule, err := core.Encapsulate(aliceView.EncapsKey)
	if err != nil {
		t.Fatal(err)
	}
	saltBytes, err := core.DerivePaymentSalt(ss)
	if err != nil {
		t.Fatal(err)
	}
	encKey, err := core.DerivePaymentKey(ss)
	if err != nil {
		t.Fatal(err)
	}
	aliceSalt := core.SaltBToField(saltBytes)
	aliceCmt, err := core.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSalt, amount, tokenId)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := core.EncryptPayload(encKey, tokenId, amount)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := erc20.Transact(owner, "mint", owner.From, amount)
	mustTx(t, client, "ERC20.mint", tx, err)
	tx, err = erc20.Transact(owner, "approve", erc20VaultAddr, amount)
	mustTx(t, client, "ERC20.approve", tx, err)
	tx, err = erc20Vault.Transact(owner, "depositV2", []*big.Int{amount, aliceSpend.PublicKey, aliceSalt, tokenId}, capsule, enc)
	mustTx(t, client, "Erc20CoinVault.depositV2", tx, err)

	// Bob's ERC-721 note.
	bobSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	bobView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	bobNftSalt, err := core.RandomInField()
	if err != nil {
		t.Fatal(err)
	}
	bobNftCmt, err := core.Erc721Commitment(nftTokenId, bobSpend.PublicKey, bobNftSalt)
	if err != nil {
		t.Fatal(err)
	}
	tx, err = erc721.Transact(owner, "mint", owner.From, nftTokenId)
	mustTx(t, client, "ERC721.mint", tx, err)
	tx, err = erc721.Transact(owner, "approve", nftVaultAddr, nftTokenId)
	mustTx(t, client, "ERC721.approve", tx, err)
	tx, err = nftVault.Transact(owner, "deposit", []*big.Int{nftTokenId, bobSpend.PublicKey, bobNftSalt})
	mustTx(t, client, "Erc721CoinVault.deposit", tx, err)

	aliceProof, err := loadVaultMerkleTree(t, client, erc20VaultAddr, depth).GenerateProof(aliceCmt)
	if err != nil {
		t.Fatalf("GenerateProof (Alice): %v", err)
	}
	bobProof, err := loadVaultMerkleTree(t, client, nftVaultAddr, depth).GenerateProof(bobNftCmt)
	if err != nil {
		t.Fatalf("GenerateProof (Bob): %v", err)
	}

	initiator, err := gnark.DvPInitiatorProof(
		core.KeyPair{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey},
		aliceSalt, amount, tokenId,
		bobSpend.PublicKey, bobView.EncapsKey,
		one, nftTokenId,
		big.NewInt(0), aliceProof, depth,
	)
	if err != nil {
		t.Fatalf("DvPInitiatorProof: %v", err)
	}

	ssBob, err := core.Decapsulate(bobView.DecapsKey, initiator.CipherText)
	if err != nil {
		t.Fatal(err)
	}
	saltB, err := core.DerivePaymentSalt(ssBob)
	if err != nil {
		t.Fatal(err)
	}
	saltA, err := core.DeriveDvpSaltInit(ssBob)
	if err != nil {
		t.Fatal(err)
	}
	bobEncKey, err := core.DerivePaymentKey(ssBob)
	if err != nil {
		t.Fatal(err)
	}
	decTokenId, decAmount, err := core.DecryptPayload(bobEncKey, initiator.EncTxData)
	if err != nil {
		t.Fatal(err)
	}
	destination, err := gnark.DvPDestinationProof(
		core.KeyPair{PrivateKey: bobSpend.PrivateKey, PublicKey: bobSpend.PublicKey},
		bobNftSalt, one, nftTokenId,
		aliceSpend.PublicKey, core.SaltBToField(saltA),
		core.SaltBToField(saltB), decAmount, decTokenId,
		initiator.CommitA,
		big.NewInt(0), bobProof, depth,
	)
	if err != nil {
		t.Fatalf("DvPDestinationProof: %v", err)
	}

	pay = onchainProofReceipt{
		Proof:           proofStringsToOnchain(t, initiator.Proof),
		Statement:       initiator.Statement,
		NumberOfInputs:  big.NewInt(int64(initiator.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(initiator.NumberOfOutputs)),
	}
	del = onchainProofReceipt{
		Proof:           proofStringsToOnchain(t, destination.Proof),
		Statement:       destination.Statement,
		NumberOfInputs:  big.NewInt(int64(destination.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(destination.NumberOfOutputs)),
	}
	return pay, del, initiator.CommitA, initiator.CommitB
}

func TestV2SwapRelayer_OnChain(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping")
	}
	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	defer client.Close()

	receipts := loadOnchainReceipts(t)
	owner := hardhatAuth(t, client)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	dvp := bind.NewBoundContract(dvpAddr, loadOnchainABI(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json"), client, client, client)

	// ── Deploy and wire up SwapRelayer ────────────────────────────────────────
	relABI, relBin := loadArtifact(t, "core/contracts/SwapRelayer.sol/SwapRelayer.json")
	relAddr, tx, _, err := bind.DeployContract(owner, relABI, relBin, client, dvpAddr)
	mustTx(t, client, "deploy SwapRelayer", tx, err)
	tx, err = dvp.Transact(owner, "registerRelayer", relAddr)
	mustTx(t, client, "registerRelayer", tx, err)
	tx, err = dvp.Transact(owner, "registerVaultSwapPair", big.NewInt(0), big.NewInt(1))
	mustTx(t, client, "registerVaultSwapPair", tx, err)
	t.Logf("SwapRelayer deployed at %s and registered with EnygmaDvp", relAddr.Hex())

	vaultABI := loadOnchainABI(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	r := &onchainSwapRelayer{
		t: t, client: client, addr: relAddr, abi: relABI,
		keys: map[string]*ecdsa.PrivateKey{},
		vaults: map[int64]*bind.BoundContract{
			0: bind.NewBoundContract(common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress), vaultABI, client, client, client),
			1: bind.NewBoundContract(common.HexToAddress(receipts["Erc721CoinVault"].ContractAddress), vaultABI, client, client, client),
		},
	}
	for name, hexKey := range swapRelayerTestKeys {
		k, err := crypto.HexToECDSA(hexKey)
		if err != nil {
			t.Fatal(err)
		}
		r.keys[name] = k
	}

	t.Log("Building real swap legs (Alice: 30 ERC-20, Bob: ERC-721)…")
	pay, del, _, _ := buildSwapLegs(t, client, owner, receipts)
	ctI, ctII := []byte{0x01}, []byte{0x02}
	vPay, vDel := big.NewInt(0), big.NewInt(1)
	id1 := swapIdOf(pay, del)

	// ── 1. Alice submits, swap expires, someone else cancels ─────────────────
	t.Log("1. Alice submits her payment leg, then a third party cancels it after expiry")
	if e, _ := r.send("alice", "submitReceipt", id1, pay, true, vPay, big.NewInt(r.now()+120), ctI, ctII); e != "" {
		t.Fatalf("Alice's leg: %s", e)
	}
	if locked, _ := r.nullifierState(0, pay); !locked {
		t.Fatal("Alice's note should be locked while her leg is pending")
	}
	r.advance(300)
	if e, _ := r.send("mallory", "cancelSwap", id1); e != "" {
		t.Fatalf("cancelSwap: %s", e)
	}
	if locked, spent := r.nullifierState(0, pay); locked || spent {
		t.Fatalf("after cancel Alice's note: locked=%v spent=%v, want both false", locked, spent)
	}
	t.Log("   cancelled; Alice's note is free again")

	// ── 2. Late legs under the cancelled swapId ───────────────────────────────
	t.Log("2. Bob's late delivery leg under the cancelled swapId")
	if e, _ := r.send("bob", "submitReceipt", id1, del, false, vDel, big.NewInt(r.now()+120), ctI, ctII); e != "SwapClosed" {
		t.Fatalf("Bob's late leg: got %q, want SwapClosed", e)
	}
	if locked, _ := r.nullifierState(1, del); locked {
		t.Fatal("Bob's note was locked by a leg on a cancelled swap")
	}
	t.Log("   rejected with SwapClosed; Bob's note is not locked")
	if e, _ := r.send("mallory", "submitReceipt", id1, pay, true, vPay, big.NewInt(r.now()+120), ctI, ctII); e != "SwapClosed" {
		t.Fatalf("replay of Alice's leg: got %q, want SwapClosed", e)
	}
	if locked, _ := r.nullifierState(0, pay); locked {
		t.Fatal("Alice's note was re-locked by a replay")
	}
	t.Log("   replay of Alice's leg by a third party also rejected")

	// ── 3. A new swap: junk and misfiled legs are rejected, then it settles ──
	t.Log("3. A second swap (new legs)")
	pay2, del2, commitA2, commitB2 := buildSwapLegs(t, client, owner, receipts)
	id2 := swapIdOf(pay2, del2)

	junk := pay2
	junk.Proof.A.X = new(big.Int).Add(pay2.Proof.A.X, big.NewInt(1))
	if e, _ := r.send("mallory", "submitReceipt", id2, junk, true, vPay, big.NewInt(r.now()+600), ctI, ctII); e == "" {
		t.Fatal("a leg with a corrupted proof was accepted")
	} else {
		t.Logf("   leg with a corrupted proof rejected (%s)", e)
	}
	if e, _ := r.send("mallory", "submitReceipt", id1, pay2, true, vPay, big.NewInt(r.now()+600), ctI, ctII); e != "SwapClosed" {
		t.Fatalf("leg under a closed foreign swapId: got %q, want SwapClosed", e)
	}
	otherId := crypto.Keccak256Hash([]byte("not this swap"))
	if e, _ := r.send("mallory", "submitReceipt", otherId, pay2, true, vPay, big.NewInt(r.now()+600), ctI, ctII); e != "SwapIdMismatch" {
		t.Fatalf("leg under a foreign swapId: got %q, want SwapIdMismatch", e)
	}
	if e, _ := r.send("mallory", "submitReceipt", id2, pay2, true, vPay, big.NewInt(r.now()+31*24*3600), ctI, ctII); e != "ExpiryTooFar" {
		t.Fatalf("far expiry: got %q, want ExpiryTooFar", e)
	}
	if locked, _ := r.nullifierState(0, pay2); locked {
		t.Fatal("Alice's second note was locked by a rejected leg")
	}
	t.Log("   junk proof, foreign swapId and 31-day expiry all rejected; nothing locked")

	if e, _ := r.send("alice", "submitReceipt", id2, pay2, true, vPay, big.NewInt(r.now()+600), ctI, ctII); e != "" {
		t.Fatalf("Alice's leg (#2): %s", e)
	}
	e, rcpt := r.send("bob", "submitReceipt", id2, del2, false, vDel, big.NewInt(r.now()+600), ctI, ctII)
	if e != "" {
		t.Fatalf("Bob's leg (#2): %s", e)
	}
	settledSig := relABI.Events["SwapSettled"].ID
	commitmentSig := crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	var settled, gotA, gotB bool
	for _, l := range rcpt.Logs {
		switch {
		case l.Topics[0] == settledSig && l.Topics[1] == id2:
			settled = true
		case l.Topics[0] == commitmentSig && len(l.Topics) >= 3:
			gotA = gotA || l.Topics[2].Big().Cmp(commitA2) == 0
			gotB = gotB || l.Topics[2].Big().Cmp(commitB2) == 0
		}
	}
	if !settled || !gotA || !gotB {
		t.Fatalf("settlement: SwapSettled=%v commitA=%v commitB=%v, want all true", settled, gotA, gotB)
	}
	for _, c := range []struct {
		vault int64
		rc    onchainProofReceipt
		who   string
	}{{0, pay2, "Alice"}, {1, del2, "Bob"}} {
		if locked, spent := r.nullifierState(c.vault, c.rc); locked || !spent {
			t.Fatalf("%s's note after settlement: locked=%v spent=%v, want spent and unlocked", c.who, locked, spent)
		}
	}
	t.Logf("   settled in block %d (gas %d): both notes spent, both new notes inserted", rcpt.BlockNumber, rcpt.GasUsed)

	// ── 4. Settled swapId is closed ───────────────────────────────────────────
	t.Log("4. A leg under the settled swapId")
	if e, _ := r.send("mallory", "submitReceipt", id2, pay2, true, vPay, big.NewInt(r.now()+120), ctI, ctII); e != "SwapClosed" {
		t.Fatalf("leg on settled swapId: got %q, want SwapClosed", e)
	}
	t.Log("   rejected with SwapClosed")
}

// swapIdOf mirrors SwapRelayer.swapIdOf: keccak256(abi.encode(deliveryMessage,
// paymentMessage)), the two legs' statement messages.
func swapIdOf(pay, del onchainProofReceipt) common.Hash {
	return crypto.Keccak256Hash(common.BigToHash(del.Statement[0]).Bytes(), common.BigToHash(pay.Statement[0]).Bytes())
}
