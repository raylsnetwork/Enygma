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

	enygma "enygma/contracts"

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

	privKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey))

	mkAuth := func() *bind.TransactOpts {
		nonce, _ := client.PendingNonceAt(context.Background(), ownerAddr)
		gasPrice, _ := client.SuggestGasPrice(context.Background())
		auth, _ := bind.NewKeyedTransactorWithChainID(privKey, big.NewInt(chainID))
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

	instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx)
	// Fix H-02 residual: senderMintR, not r=0 — senderRegR + senderMintR
	// == senderPrevR (used directly below), see freshSetup's comment.
	mcx, mcy := mintCommitPt(big.NewInt(mintAmt), big.NewInt(senderMintR))
	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1), mcx, mcy)); r.Status != 1 {
		t.Fatal("mintSupply failed")
	}
	t.Logf("minted %d to bank 0 (accountId=1)", mintAmt)

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

	// Hardhat preflights the call before broadcasting, so a revert surfaces
	// as a send error here (not a mined Status=0 receipt) — assert on the
	// custom error selector directly. ParticipantIdsNotSorted() = 0xf170f72d.
	_, sendErr := instance.Transfer(mkAuth(), commitmentDeltas, attackProof, attackParticipantIds, "") // Fix H-09: no attribution for a direct test call
	if sendErr == nil {
		t.Fatal("FAIL (C-05 regressed): Transfer with a duplicated participantId SUCCEEDED — " +
			"the epoch read/write aliasing or the missing duplicate-id check is back")
	}
	const wantSelector = "0xf170f72d" // ParticipantIdsNotSorted()
	if !strings.Contains(sendErr.Error(), wantSelector) {
		t.Fatalf("Transfer reverted, but not with ParticipantIdsNotSorted (%s): %v", wantSelector, sendErr)
	}
	t.Logf("attack Transfer reverted with ParticipantIdsNotSorted() — duplicate participantId correctly rejected: %v", sendErr)

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
