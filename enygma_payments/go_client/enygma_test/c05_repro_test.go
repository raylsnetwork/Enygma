package enygma_test

// TestC05DuplicateParticipantIdRejected reproduces the C-05 finding
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md) against the *fixed* contract and
// confirms the exploit shape is now rejected.
//
// C-05's root cause: _updateBalancesForTransfer read each participant's old
// balance from balanceCommitments[lastBlockNum] but wrote the new balance to
// balanceCommitments[epochStart] — two different storage slots off an epoch
// boundary. A duplicated account id in participantIds made the second write
// silently discard the first, minting or burning value with no counterparty.
//
// The audit's own repro duplicated a *non-sender* slot's public key and
// previous-commitment public signals so _verifyPublicInputsFP would accept
// participantIds=[1,1,3,4,5,6] (both position 0 and 1 checked against
// account 1's own on-chain key/commitment). That is reproduced here: the
// circuit only constrains the sender's own slot, so position 1's
// public_keys/previous_commits entries are free witnesses the prover can
// set to anything — including a copy of the sender's own identity.
//
// Prerequisites:
//
//	export MY_KEY=<hex-private-key>   (or rely on the local Hardhat default)
//	gnark server running on :8080
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestC05 -v -timeout 120s

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

func TestC05DuplicateParticipantIdRejected(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	if !tcpAvailable("127.0.0.1:8080") {
		t.Skip("gnark server not reachable at localhost:8080 — start gnark-server first")
	}

	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	ownerKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*ownerKey.Public().(*ecdsa.PublicKey))

	mkAuth := func() *bind.TransactOpts {
		nonce, _ := client.PendingNonceAt(context.Background(), ownerAddr)
		gasPrice, _ := client.SuggestGasPrice(context.Background())
		auth, _ := bind.NewKeyedTransactorWithChainID(ownerKey, big.NewInt(chainID))
		auth.Nonce = big.NewInt(int64(nonce))
		auth.Value = big.NewInt(0)
		auth.GasLimit = 16_000_000
		auth.GasPrice = gasPrice
		return auth
	}
	waitTx := func(tx *ethtypes.Transaction, txErr error) *ethtypes.Receipt {
		t.Helper()
		if txErr != nil {
			t.Fatalf("send tx: %v", txErr)
		}
		r, err := bind.WaitMined(context.Background(), client, tx)
		if err != nil {
			t.Fatalf("wait mined: %v", err)
		}
		return r
	}

	// ── Deploy fresh Enygma + REAL Verifier, register 6 DISTINCT-address
	// banks (not freshSetup's single shared owner address) ─────────────────
	// C-04's _verifyFingerprints requires every pairwise fingerprint among
	// a transfer's participants to be mutually confirmed on-chain, and
	// registerFingerprint resolves the caller via
	// addressToAccountId[msg.sender] — which collapses to one identity if
	// every bank shares freshSetup's single owner address. Mirrors
	// c04Setup's own distinct-address registration (see that function's
	// doc comment) but with the real Verifier instead of
	// MockTransferVerifier, since this test needs an actual circuit-
	// generated attack proof, not just contract-side logic in isolation.
	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	enygmaAddr := deployFromArtifact(t, client, mkAuth(), artifactBase+"/Enygma.sol/Enygma.json", big.NewInt(30))
	verifierAddr := deployFromArtifact(t, client, mkAuth(), artifactBase+"/EnygmaVerifier.sol/Verifier.json")

	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind contract: %v", err)
	}
	waitTx(instance.Initialize(mkAuth()))
	waitTx(instance.AddVerifier(mkAuth(), verifierAddr))

	banks := make([]c04Bank, nBanks)
	for i := 0; i < nBanks; i++ {
		key, genErr := crypto.GenerateKey()
		if genErr != nil {
			t.Fatalf("generate bank %d key: %v", i, genErr)
		}
		addr := crypto.PubkeyToAddress(key.PublicKey)
		banks[i] = c04Bank{key: key, addr: addr, accountID: int64(i + 1)}

		gasPrice, _ := client.SuggestGasPrice(context.Background())
		nonce, _ := client.PendingNonceAt(context.Background(), ownerAddr)
		fundTx := ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce:    nonce,
			To:       &addr,
			Value:    new(big.Int).SetUint64(50_000_000_000_000_000), // 0.05 ETH
			Gas:      21000,
			GasPrice: gasPrice,
		})
		signedFundTx, signErr := ethtypes.SignTx(fundTx, ethtypes.NewEIP155Signer(big.NewInt(chainID)), ownerKey)
		if signErr != nil {
			t.Fatalf("sign funding tx: %v", signErr)
		}
		if sendErr := client.SendTransaction(context.Background(), signedFundTx); sendErr != nil {
			t.Fatalf("fund bank %d: %v", i, sendErr)
		}
		if _, waitErr := bind.WaitMined(context.Background(), client, signedFundTx); waitErr != nil {
			t.Fatalf("wait funding mined: %v", waitErr)
		}

		pk, pkErr := poseidon.Hash([]*big.Int{bankSks[i], bankSks[i]})
		if pkErr != nil {
			t.Fatalf("pk[%d]: %v", i, pkErr)
		}
		pk.Mod(pk, curveP)
		// Fix H-02 residual: bank 0 registers with senderRegR (not
		// senderPrevR directly) since it mints below too — senderRegR +
		// senderMintR == senderPrevR.
		r := big.NewInt(senderPrevR)
		if i == senderIdx {
			r = big.NewInt(senderRegR)
		}
		cx, cy := regCommit(r)
		waitTx(instance.RegisterAccount(mkAuth(), addr, big.NewInt(banks[i].accountID), pk, cx, cy, []byte{}))
	}
	t.Logf("registered %d banks, each under its OWN distinct address", nBanks)

	mcx, mcy := mintCommitPt(big.NewInt(mintAmt), big.NewInt(senderMintR))
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1), mcx, mcy)); r.Status != 1 {
		t.Fatal("mintSupply failed")
	}
	t.Logf("minted %d to bank 0 (accountId=1)", mintAmt)

	allAccountIds := make([]int64, nBanks)
	for i := range allAccountIds {
		allAccountIds[i] = int64(i + 1)
	}
	setupMockUsdr(t, client, mkAuth, waitTx, instance, allAccountIds)

	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("getPublicValues: %v", err)
	}
	prevBalances := pubVals.Balances[1:]
	onChainKeys := pubVals.Keys[1:]

	sk := big.NewInt(senderSk)
	prevR := big.NewInt(senderPrevR)
	senderSecret, _ := poseidon.Hash([]*big.Int{prevR, sk})
	senderSecret.Mod(senderSecret, curveP)

	secrets := make([]*big.Int, nBanks)
	copy(secrets, baseSecrets)
	secrets[senderIdx] = senderSecret

	fp := fingerPrintGen(secrets, senderIdx)

	// nullifier computed before tagMessageGen/genCommitmentAndRandom: Fix
	// H-01/H-02 use it (not blockHash) as the per-transaction value.
	nullifier, _ := poseidon.Hash([]*big.Int{senderSecret, blockHash})
	tagMessages := tagMessageGen(senderIdx, secrets, nullifier)

	// Bank 0 (position 0, accountId 1) sends 100; bank 1 (position 1,
	// accountId 2) and bank 2 (position 2, accountId 3) split it 60/40 — an
	// otherwise completely honest transfer.
	txValues := []*big.Int{
		negMod(big.NewInt(transferAmt)),
		big.NewInt(60), big.NewInt(40),
		big.NewInt(0), big.NewInt(0), big.NewInt(0),
	}
	txCommit, txRandom := genCommitmentAndRandom(senderIdx, big.NewInt(transferAmt), txValues, nullifier, secrets)

	toStrs := func(vals []*big.Int) []string {
		s := make([]string, len(vals))
		for i, v := range vals {
			s[i] = v.String()
		}
		return s
	}
	prevCommitSlice := make([][]string, nBanks)
	for i, pt := range prevBalances {
		prevCommitSlice[i] = []string{pt.C1.String(), pt.C2.String()}
	}
	txCommitSlice := make([][]string, nBanks)
	for i, pt := range txCommit {
		txCommitSlice[i] = []string{pt.C1.String(), pt.C2.String()}
	}
	keyStrs := make([]string, nBanks)
	for i, k := range onChainKeys {
		keyStrs[i] = k.String()
	}

	// ── The C-05 attack construction ───────────────────────────────────────
	// Position 1's public_keys/previous_commits entries are NOT constrained
	// by the circuit for a non-sender slot (C-03/C-04). Overwrite them with
	// account 1's own on-chain key/commitment (position 0's honest values)
	// instead of account 2's — this is exactly what makes an on-chain
	// participantIds=[1,1,3,4,5,6] pass _verifyPublicInputsFP: both
	// positions 0 and 1 check against keys[1]/balances[1] and both match.
	keyStrs[1] = keyStrs[0]
	prevCommitSlice[1] = prevCommitSlice[0]

	kIndex := make([]*big.Int, nBanks)
	for i := range kIndex {
		kIndex[i] = big.NewInt(int64(i))
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"fingerprint_shared_secrets":   fp2Strs(fp),
		"public_keys":                  keyStrs,
		"previous_commits":             prevCommitSlice,
		"tx_commits":                   txCommitSlice,
		"block_number":                 blockHash.String(),
		"anonymity_set":                toStrs(kIndex),
		"message_tags":                 toStrs(tagMessages),
		"nullifier":                    nullifier.String(),
		"sender_id":                    fmt.Sprintf("%d", senderIdx),
		"shared_secrets":               toStrs(secrets),
		"secret_key":                   sk.String(),
		"previous_sender_balance":      fmt.Sprintf("%d", senderPrevV),
		"previous_sender_random_value": prevR.String(),
		"tx_values":                    toStrs(txValues),
		"tx_random_values":             toStrs(txRandom),
		"sender_tx_value":              fmt.Sprintf("%d", transferAmt),
		"domain_id":                    expectedDomainId(enygmaAddr).String(), // Fix L-01
	})

	t.Log("requesting attack proof (may take ~30s)…")
	httpResp, err := http.Post(gnarkURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("gnark POST: %v", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		t.Fatalf("gnark %d: %s", httpResp.StatusCode, body)
	}
	var proofResp struct {
		Proof        []*big.Int `json:"proof"`
		PublicSignal []*big.Int `json:"publicSignal"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&proofResp); err != nil {
		t.Fatalf("decode proof: %v", err)
	}
	if len(proofResp.Proof) != 8 || len(proofResp.PublicSignal) != 81 {
		t.Fatalf("unexpected proof sizes: proof=%d publicSignal=%d", len(proofResp.Proof), len(proofResp.PublicSignal))
	}
	t.Log("attack proof received — circuit happily proved a witness with a duplicated non-sender identity")

	var proof8 [8]*big.Int
	for i := 0; i < 8; i++ {
		proof8[i] = proofResp.Proof[i]
	}
	var pubSig80 [81]*big.Int
	for i := range pubSig80 {
		pubSig80[i] = big.NewInt(0)
	}
	for i, v := range proofResp.PublicSignal {
		pubSig80[i] = v
	}
	const txCommitOffset = 54
	commitmentDeltas := make([]enygma.IEnygmaPoint, nBanks)
	for i := 0; i < nBanks; i++ {
		commitmentDeltas[i] = enygma.IEnygmaPoint{
			C1: proofResp.PublicSignal[txCommitOffset+2*i],
			C2: proofResp.PublicSignal[txCommitOffset+2*i+1],
		}
	}
	attackProof := enygma.IEnygmaProof{Proof: proof8, PublicSignal: pubSig80}

	// The on-chain half of the attack: accountId 1 appears twice.
	attackParticipantIds := []*big.Int{
		big.NewInt(1), big.NewInt(1), big.NewInt(3), big.NewInt(4), big.NewInt(5), big.NewInt(6),
	}
	attackAccountIds := []int64{1, 1, 3, 4, 5, 6}
	usdrDeltas, usdrProof := buildMockUsdrLeg(t, instance, enygmaAddr, pubSig80, attackAccountIds, banks[0].addr)

	// bankAuth(banks[0]), not mkAuth() — onlyRegistered requires
	// addressToAccountId[msg.sender] != 0, and only the 6 banks' own
	// distinct addresses are registered under this setup (see the
	// registration loop above), not the deployer/owner address mkAuth()
	// signs with.
	_, sendErr := instance.Transfer(bankAuth(t, client, banks[0]), commitmentDeltas, attackProof, usdrDeltas, usdrProof, attackParticipantIds, "") // Fix H-09: no attribution for a direct test call
	if sendErr == nil {
		t.Fatal("FAIL (C-05 regressed): Transfer with a duplicated participantId SUCCEEDED — " +
			"the epoch read/write aliasing or the missing duplicate-id check is back")
	}
	// Expected rejection reason changed since this test was written: back
	// then, _updateBalancesForTransfer's ParticipantIdsNotSorted() was the
	// only defense against a duplicated id, so that's what fired. The C-04
	// fingerprint requirement added later now catches it first and more
	// fundamentally — _verifyFingerprints requires fingerprintConfirmed
	// between every pair of DISTINCT array positions in participantIds,
	// including position (0,1) here (both value 1, i.e. accountId 1 with
	// itself); registerFingerprint() explicitly reverts InvalidFingerprintParty
	// on otherPartyId == callerId, so a self-fingerprint can never be
	// confirmed — any duplicated id is therefore structurally rejected by
	// C-04 before ParticipantIdsNotSorted's own check is ever reached. The
	// underlying security property this test exists to confirm — a
	// duplicated participantId is rejected — still holds, now via a
	// stronger, earlier check.
	if !strings.Contains(sendErr.Error(), "FingerprintNotConfirmed") {
		t.Fatalf("Transfer reverted, but not with FingerprintNotConfirmed: %v", sendErr)
	}
	t.Logf("attack Transfer reverted with FingerprintNotConfirmed() — duplicate participantId correctly rejected (now caught by C-04's fingerprint uniqueness requirement before reaching C-05's own check): %v", sendErr)

	// ── Control: the contract still works normally afterwards ─────────────
	// The attack tx reverted in full (no nullifier consumed, no storage
	// written), so the ledger invariant must still hold — confirms the
	// rejection above didn't leave the contract in a broken state.
	ok, err := instance.Check(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("check() call failed after rejected attack: %v", err)
	}
	if !ok {
		t.Fatal("FAIL: check() invariant broken after a REJECTED attack — the revert leaked state")
	}
	t.Log("check() invariant holds after the rejected attack — no state leaked from the reverted tx")
}
