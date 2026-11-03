package tests

// On-chain trade agreement (Idea 1): the DvP swap runs with no off-chain step.
//
// Alice swaps 50 USDT (ERC20) for Bob's NFT ticket (ERC721):
//
//   1. Both register spend + ML-KEM view keys in UserRegistry.
//   2. Alice encapsulates to Bob's view key and publishes the encrypted offer
//      (terms + deadline + her pkSpend) with DvpAgreementRegistry.openOffer,
//      hiding Bob in an anonymity set of registered users.
//   3. Bob scans OfferOpened events, opens the offer with his view key.
//      An unrelated view key cannot.
//   4. Bob accepts with publishReply under t = Poseidon(block, pkSpend_A, ss).
//   5. Alice scans ReplyPublished events, matches the tag, reads Accept.
//   6. Alice derives salt_B / salt_A from (ss, offerId), proves DvPInitiator
//      and submits the first leg with the offered deadline.
//   7. Bob derives the same salts, finds SwapInitiated by COMMIT_B, checks it
//      matches the offer, proves DvPDestination and settles.
//
// Nothing passes between Alice and Bob except through the chain.
//
// Prerequisites: Hardhat node, deploy + init (deploy now also deploys
// UserRegistry and DvpAgreementRegistry), gnark server on :8081.
//
// Run:
//   cd test && CC=/usr/bin/clang go test -run TestV2DvP_OnChainAgreement -v -timeout 600s

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// fundedAccount creates a fresh key funded with 10 ETH from Hardhat account[0],
// so Alice and Bob register and publish from their own addresses.
func fundedAccount(t *testing.T, tc *dvpTestContext) (*ecdsa.PrivateKey, *bind.TransactOpts) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	to := crypto.PubkeyToAddress(key.PublicKey)
	nonce, err := tc.client.PendingNonceAt(tc.ctx, tc.auth.From)
	if err != nil {
		t.Fatal(err)
	}
	gasPrice, err := tc.client.SuggestGasPrice(tc.ctx)
	if err != nil {
		t.Fatal(err)
	}
	value := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18))
	tx := types.NewTransaction(nonce, to, value, 21000, gasPrice, nil)
	signed, err := tc.auth.Signer(tc.auth.From, tx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tc.client.SendTransaction(tc.ctx, signed); err != nil {
		t.Fatalf("fund %s: %v", to.Hex(), err)
	}
	if _, err := bind.WaitMined(tc.ctx, tc.client, signed); err != nil {
		t.Fatal(err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(hardhatChainID))
	if err != nil {
		t.Fatal(err)
	}
	auth.GasLimit = 6_000_000
	return key, auth
}

func mustMine(t *testing.T, ctx context.Context, client *ethclient.Client, tx *types.Transaction, err error, what string) *types.Receipt {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	r, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		t.Fatalf("%s: wait: %v", what, err)
	}
	if r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("%s: reverted", what)
	}
	return r
}

func TestV2DvP_OnChainAgreement(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping on-chain test")
	}

	tc := newDvpTestContext(t)
	defer tc.client.Close()
	ctx := tc.ctx

	receipts := loadOnchainReceipts(t)
	agreementAddr := common.HexToAddress(receipts["DvpAgreementRegistry"].ContractAddress)
	userRegAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	if agreementAddr == (common.Address{}) || userRegAddr == (common.Address{}) {
		t.Fatal("receipts.json has no DvpAgreementRegistry / UserRegistry — rerun deploy")
	}
	userReg := bind.NewBoundContract(userRegAddr,
		loadOnchainABI(t, "core/contracts/UserRegistry.sol/UserRegistry.json"), tc.client, tc.client, tc.client)

	// Notes: Alice holds 50 USDT, Bob holds a ticket. A time-based tokenId keeps
	// reruns on the same node from colliding with an already-minted NFT.
	nftTokenId := time.Now().UnixNano() % (1 << 32)
	d := dvpSetupDeposits(t, tc, nftTokenId)

	// ── 1. Registration ───────────────────────────────────────────────────────
	_, aliceAuth := fundedAccount(t, tc)
	_, bobAuth := fundedAccount(t, tc)
	tx, err := userReg.Transact(aliceAuth, "register", d.aliceSpend.PublicKey, d.aliceView.EncapsKey)
	mustMine(t, ctx, tc.client, tx, err, "Alice register")
	tx, err = userReg.Transact(bobAuth, "register", d.bobSpend.PublicKey, d.bobView.EncapsKey)
	mustMine(t, ctx, tc.client, tx, err, "Bob register")

	callOne := func(method string, args ...interface{}) []interface{} {
		var out []interface{}
		if err := userReg.Call(&bind.CallOpts{Context: ctx}, &out, method, args...); err != nil {
			t.Fatalf("UserRegistry.%s: %v", method, err)
		}
		return out
	}
	userCount := int(callOne("getUserCount")[0].(*big.Int).Int64())
	bobIdx := int(callOne("getUserIndex", bobAuth.From)[0].(uint32))
	bobKeys := callOne("getKeys", bobAuth.From)
	bobPkSpend, bobPkView := bobKeys[0].(*big.Int), bobKeys[1].([]byte)
	t.Logf("registered: %d users, Bob at index %d", userCount, bobIdx)

	// ── 2. Alice publishes the offer ──────────────────────────────────────────
	offerId, err := core.NewOfferId()
	if err != nil {
		t.Fatal(err)
	}
	deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(3600))
	offer := &core.AgreementOffer{
		OfferId:     offerId,
		GiveVaultId: big.NewInt(0), GiveTokenId: d.erc20TokenId, GiveAmount: d.erc20Amount,
		TakeVaultId: big.NewInt(1), TakeTokenId: d.nftTokenId, TakeAmount: d.nftAmount,
		Deadline:         deadline,
		InitiatorPkSpend: d.aliceSpend.PublicKey,
	}
	ssAlice, c1, c2, err := core.PrepareAgreementOffer(bobPkView, offer)
	if err != nil {
		t.Fatal(err)
	}
	bitmap, err := core.BuildAnonymityBitmap(userCount, bobIdx, 3)
	if err != nil {
		t.Fatal(err)
	}
	offerRcpt, err := core.PublishAgreementOffer(ctx, tc.client, aliceAuth, agreementAddr, c1, c2, bitmap)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("offer published (block %d, gas %d)", offerRcpt.BlockNumber, offerRcpt.GasUsed)

	// ── 3. Bob finds the offer ────────────────────────────────────────────────
	onchainOffers, err := core.FetchAgreementOffers(ctx, tc.client, agreementAddr, offerRcpt.BlockNumber.Uint64(), nil)
	if err != nil {
		t.Fatal(err)
	}
	eveView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if got := core.ScanAgreementOffers(eveView.DecapsKey, -1, onchainOffers); len(got) != 0 {
		t.Fatalf("an unrelated view key opened %d offers", len(got))
	}
	var mine *core.FoundAgreementOffer
	for _, f := range core.ScanAgreementOffers(d.bobView.DecapsKey, bobIdx, onchainOffers) {
		if f.Offer.OfferId == offerId {
			f := f
			mine = &f
		}
	}
	if mine == nil {
		t.Fatal("Bob did not find Alice's offer")
	}
	got := mine.Offer
	if got.GiveAmount.Cmp(d.erc20Amount) != 0 || got.TakeTokenId.Cmp(d.nftTokenId) != 0 || got.Deadline.Cmp(deadline) != 0 {
		t.Fatalf("Bob read different terms: %+v", got)
	}
	t.Logf("Bob opened the offer: give %s of token %s for token %s, deadline %s",
		got.GiveAmount, got.GiveTokenId, got.TakeTokenId, got.Deadline)

	// ── 4. Bob accepts under a private tag ────────────────────────────────────
	head, err := tc.client.BlockNumber(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := core.DeriveAgreementTagWindow(head, 4, got.InitiatorPkSpend, mine.SharedSecret)
	if err != nil {
		t.Fatal(err)
	}
	accept, err := core.EncryptAgreementReply(mine.SharedSecret, mine.C1, got.InitiatorPkSpend,
		&core.AgreementReply{Kind: core.ReplyAccept, OfferId: got.OfferId})
	if err != nil {
		t.Fatal(err)
	}
	replyRcpt, err := core.PublishAgreementReply(ctx, tc.client, bobAuth, agreementAddr, head, tags, accept)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("reply published (block %d, gas %d)", replyRcpt.BlockNumber, replyRcpt.GasUsed)

	// ── 5. Alice reads the acceptance ─────────────────────────────────────────
	onchainReplies, err := core.FetchAgreementReplies(ctx, tc.client, agreementAddr, head, nil)
	if err != nil {
		t.Fatal(err)
	}
	replies, err := core.ScanAgreementReplies(d.aliceSpend.PublicKey,
		[]core.AgreementChannel{{SharedSecret: ssAlice, C1: c1}}, onchainReplies)
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 1 || replies[0].Reply.Kind != core.ReplyAccept || replies[0].Reply.OfferId != offerId {
		t.Fatalf("Alice expected one Accept, found %+v", replies)
	}
	// Bob scanning for replies addressed to himself finds none.
	if r, _ := core.ScanAgreementReplies(bobPkSpend,
		[]core.AgreementChannel{{SharedSecret: mine.SharedSecret, C1: mine.C1}}, onchainReplies); len(r) != 0 {
		t.Fatal("Bob matched his own reply")
	}
	t.Log("Alice read Bob's acceptance")

	// ── 6. Alice starts the swap ──────────────────────────────────────────────
	saltB, saltA, err := core.DeriveAgreementSwapSalts(ssAlice, offerId)
	if err != nil {
		t.Fatal(err)
	}
	aliceResult, err := tc.gnarkClient.DvPInitiatorProofFromSalts(
		core.KeyPair{PrivateKey: d.aliceSpend.PrivateKey, PublicKey: d.aliceSpend.PublicKey},
		d.aliceSaltField, offer.GiveAmount, offer.GiveTokenId,
		bobPkSpend, saltB,
		offer.TakeAmount, offer.TakeTokenId, saltA,
		big.NewInt(0), d.aliceMerkleProof, tc.merkleDepth, vaultInt(tc.erc721VaultAddr),
	)
	if err != nil {
		t.Fatalf("Alice DvPInitiatorProofFromSalts: %v", err)
	}
	aliceReceipt := onchainProofReceipt{
		Proof:           proofStringsToOnchain(t, aliceResult.Proof),
		Statement:       aliceResult.Statement,
		NumberOfInputs:  big.NewInt(int64(aliceResult.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(aliceResult.NumberOfOutputs)),
	}
	tx, err = tc.dvp.Transact(tc.auth, "submitPartialSettlement", aliceReceipt,
		offer.GiveVaultId, big.NewInt(0), offer.Deadline)
	aliceLeg := mustMine(t, ctx, tc.client, tx, err, "Alice submitPartialSettlement")
	t.Logf("Alice leg mined (block %d, gas %d)", aliceLeg.BlockNumber, aliceLeg.GasUsed)

	// ── 7. Bob verifies and settles ───────────────────────────────────────────
	bobCommitB, bobCommitA, err := core.AgreementCommitments(mine.SharedSecret, got, bobPkSpend)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := core.FindPendingSwap(ctx, tc.client, dvpAddr, bobCommitB, offerRcpt.BlockNumber.Uint64())
	if err != nil {
		t.Fatal(err)
	}
	if pending == nil {
		t.Fatal("Bob found no SwapInitiated for the agreed COMMIT_B")
	}
	if err := core.CheckPendingSwapMatchesOffer(pending, mine.SharedSecret, got, bobPkSpend); err != nil {
		t.Fatalf("pending swap does not match the accepted offer: %v", err)
	}
	t.Log("Bob confirmed Alice's leg matches the accepted offer")

	bobSaltB, bobSaltA, err := core.DeriveAgreementSwapSalts(mine.SharedSecret, got.OfferId)
	if err != nil {
		t.Fatal(err)
	}
	bobResult, err := tc.gnarkClient.DvPDestinationProof(
		core.KeyPair{PrivateKey: d.bobSpend.PrivateKey, PublicKey: d.bobSpend.PublicKey},
		d.bobTicketSalt, got.TakeAmount, got.TakeTokenId,
		got.InitiatorPkSpend, bobSaltA, bobSaltB,
		got.GiveAmount, got.GiveTokenId,
		bobCommitA, big.NewInt(0), d.bobMerkleProof, tc.merkleDepth, vaultInt(tc.erc20VaultAddr),
	)
	if err != nil {
		t.Fatalf("Bob DvPDestinationProof: %v", err)
	}
	bobReceipt := onchainProofReceipt{
		Proof:           proofStringsToOnchain(t, bobResult.Proof),
		Statement:       bobResult.Statement,
		NumberOfInputs:  big.NewInt(int64(bobResult.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(bobResult.NumberOfOutputs)),
	}
	tx, err = tc.dvp.Transact(tc.auth, "submitPartialSettlement", bobReceipt,
		got.TakeVaultId, big.NewInt(1), big.NewInt(0))
	bobLeg := mustMine(t, ctx, tc.client, tx, err, "Bob submitPartialSettlement")

	foundA, foundB := false, false
	for _, l := range bobLeg.Logs {
		if l.Topics[0] != tc.commitmentSig {
			continue
		}
		switch l.Topics[2].Big().String() {
		case bobCommitA.String():
			foundA = true
		case bobCommitB.String():
			foundB = true
		}
	}
	if !foundA || !foundB {
		t.Fatalf("settlement did not insert both commitments (A=%v B=%v)", foundA, foundB)
	}
	t.Logf("swap settled (block %d, gas %d): Bob holds 50 USDT, Alice holds ticket %d",
		bobLeg.BlockNumber, bobLeg.GasUsed, nftTokenId)
}
