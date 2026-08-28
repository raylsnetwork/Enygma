package enygma_test

// TestH09Item2_BankSelfSubmission reproduces and verifies H-09 item 2 of 5
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, Medium/register-High): the relayer
// is the sole possible transaction submitter — every registerAccount call
// binds a bank's identity to the OWNER's address by convention, not the
// contract's own design — so it could silently censor any submission with
// nothing on-chain to detect it and no fallback for the affected bank.
//
// registerAccount already takes an arbitrary addr parameter (never bound
// to msg.sender), and onlyRegistered only checks "is msg.sender SOME
// registered participant" — not "is msg.sender the specific bank this
// proof moves value for" (that's the ZK proof's own job, by design, since
// msg.sender was never meant to identify the real sender within the
// k-anonymity set — see H-01/H-02). So nothing in the contract actually
// requires the relayer: a bank registered under its own address can
// always submit transfer() directly.
//
// This test proves that end to end: registers 6 banks under 6 DISTINCT
// addresses (mirroring cmd/register_bank's exact derivation —
// publicKey = Poseidon(sk,sk) mod P, initial commitment = Com(0, regR)),
// confirms the C-04 pairwise fingerprint registry for the FULL k×k matrix
// (not just the sender's column — a real circuit only derives the
// sender's own column; every other cell is a free wire the prover must
// still fill to match the on-chain registry, which — per
// demo/main.go's own loadFingerprintMatrix() comment — is exactly what a
// real multi-party anonymity set requires), requests a REAL proof from
// the live gnark server, and submits transfer() DIRECTLY with the
// sender's own key. No relayer process is started anywhere in this test.
//
// Run (needs a live gnark server on :8080):
//
//	CC=/usr/bin/clang go test -run TestH09Item2 -v -timeout 120s

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"testing"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// h09item2ProofResponse mirrors the gnark server's /proof/enygma response shape.
type h09item2ProofResponse struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}

func postProofRequest(t *testing.T, reqBody map[string]interface{}) h09item2ProofResponse {
	t.Helper()
	data, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	t.Log("requesting proof (may take ~30s)...")
	httpResp, err := http.Post(gnarkURL, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gnark POST: %v", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		t.Fatalf("gnark %d: %s", httpResp.StatusCode, body)
	}
	var resp h09item2ProofResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		t.Fatalf("decode proof: %v", err)
	}
	return resp
}

func TestH09Item2_BankSelfSubmission(t *testing.T) {
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
	ownerAuth := func() *bind.TransactOpts {
		nonce, _ := client.PendingNonceAt(context.Background(), ownerAddr)
		gasPrice, _ := client.SuggestGasPrice(context.Background())
		auth, _ := bind.NewKeyedTransactorWithChainID(ownerKey, big.NewInt(chainID))
		auth.Nonce = big.NewInt(int64(nonce))
		auth.GasLimit = 16_000_000
		auth.GasPrice = gasPrice
		return auth
	}
	waitTx := func(tx *ethtypes.Transaction, txErr error) *ethtypes.Receipt {
		t.Helper()
		if txErr != nil {
			t.Fatalf("send: %v", txErr)
		}
		r, err := bind.WaitMined(context.Background(), client, tx)
		if err != nil || r.Status != 1 {
			t.Fatalf("mine: status=%v err=%v", r, err)
		}
		return r
	}

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	enygmaAddr := deployFromArtifact(t, client, ownerAuth(), artifactBase+"/Enygma.sol/Enygma.json", big.NewInt(30))
	verifierAddr := deployFromArtifact(t, client, ownerAuth(), artifactBase+"/EnygmaVerifier.sol/Verifier.json")

	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	waitTx(instance.Initialize(ownerAuth()))
	waitTx(instance.AddVerifier(ownerAuth(), verifierAddr))

	// ── Register 6 banks under 6 DISTINCT addresses (Fix H-09 item 2) ──────
	// Mirrors cmd/register_bank exactly: owner calls registerAccount with
	// each bank's OWN address; publicKey/commitment derived the same way.
	type selfBank struct {
		key  *ecdsa.PrivateKey
		addr common.Address
	}
	sks := []*big.Int{big.NewInt(senderSk), big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4), big.NewInt(5)}
	banks := make([]selfBank, nBanks)
	for i := 0; i < nBanks; i++ {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatalf("gen key %d: %v", i, err)
		}
		addr := crypto.PubkeyToAddress(key.PublicKey)
		banks[i] = selfBank{key: key, addr: addr}

		gasPrice, _ := client.SuggestGasPrice(context.Background())
		nonce, _ := client.PendingNonceAt(context.Background(), ownerAddr)
		fundTx := ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce: nonce, To: &addr, Value: new(big.Int).SetUint64(50_000_000_000_000_000), Gas: 21000, GasPrice: gasPrice,
		})
		signed, err := ethtypes.SignTx(fundTx, ethtypes.NewEIP155Signer(big.NewInt(chainID)), ownerKey)
		if err != nil {
			t.Fatalf("sign fund %d: %v", i, err)
		}
		if err := client.SendTransaction(context.Background(), signed); err != nil {
			t.Fatalf("fund %d: %v", i, err)
		}
		bind.WaitMined(context.Background(), client, signed)

		pkHash, _ := poseidon.Hash([]*big.Int{sks[i], sks[i]})
		pk := new(big.Int).Mod(pkHash, curveP)
		regR := big.NewInt(int64(senderPrevR + i))
		cx, cy := regCommit(regR)
		waitTx(instance.RegisterAccount(ownerAuth(), addr, big.NewInt(int64(i+1)), pk, cx, cy, []byte{}))
	}
	t.Logf("registered %d banks, each under its OWN distinct address", nBanks)

	mcx, mcy := mintCommitPt(big.NewInt(mintAmt), big.NewInt(0))
	waitTx(instance.MintSupply(ownerAuth(), big.NewInt(mintAmt), big.NewInt(1), mcx, mcy))

	bankAuth := func(b selfBank) *bind.TransactOpts {
		nonce, _ := client.PendingNonceAt(context.Background(), b.addr)
		gasPrice, _ := client.SuggestGasPrice(context.Background())
		auth, _ := bind.NewKeyedTransactorWithChainID(b.key, big.NewInt(chainID))
		auth.Nonce = big.NewInt(int64(nonce))
		auth.GasLimit = 16_000_000
		auth.GasPrice = gasPrice
		return auth
	}

	// ── Confirm the FULL pairwise fingerprint matrix ────────────────────────
	// See this test's doc comment: a real circuit only derives the
	// sender's own column; every other cell must still match the registry
	// exactly, and registerFingerprint cannot confirm a value of 0 (its
	// "no claim submitted" sentinel), so every pair needs some agreed
	// nonzero value even where the circuit itself doesn't constrain it.
	secrets := make([]*big.Int, nBanks)
	copy(secrets, baseSecrets)
	senderSecretRaw, _ := poseidon.Hash([]*big.Int{big.NewInt(senderPrevR), big.NewInt(senderSk)})
	secrets[senderIdx] = new(big.Int).Mod(senderSecretRaw, curveP)

	fp := fingerPrintGen(secrets, senderIdx) // fills the sender's own column
	type pair struct{ i, j int }
	agreed := make(map[pair]*big.Int)
	for i := 0; i < nBanks; i++ {
		for j := i + 1; j < nBanks; j++ {
			var v *big.Int
			switch senderIdx {
			case i:
				v = fp[j][senderIdx]
			case j:
				v = fp[i][senderIdx]
			default:
				v = big.NewInt(int64(10000*(i+1) + (j + 1))) // arbitrary nonzero: free wire off the sender's column
			}
			agreed[pair{i, j}] = v
			waitTx(instance.RegisterFingerprint(bankAuth(banks[i]), big.NewInt(int64(j+1)), v))
			waitTx(instance.RegisterFingerprint(bankAuth(banks[j]), big.NewInt(int64(i+1)), v))
		}
	}
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			if i == j {
				continue
			}
			lo, hi := i, j
			if lo > hi {
				lo, hi = hi, lo
			}
			fp[i][j] = agreed[pair{lo, hi}]
		}
	}
	t.Log("all 15 pairwise fingerprints confirmed; full matrix echoed for the request")

	// ── Build a real proof via the live gnark server ────────────────────────
	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("GetBlckHash: %v", err)
	}
	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatalf("GetPublicValues: %v", err)
	}
	keys := pubVals.Keys[1:]
	prevBalances := pubVals.Balances[1:]

	txValues := []*big.Int{
		negMod(big.NewInt(transferAmt)),
		big.NewInt(60), big.NewInt(40),
		big.NewInt(0), big.NewInt(0), big.NewInt(0),
	}
	nullifier, _ := poseidon.Hash([]*big.Int{secrets[senderIdx], blockHash})
	tagMessages := tagMessageGen(senderIdx, secrets, nullifier)
	txCommit, txRandom := genCommitmentAndRandom(senderIdx, big.NewInt(transferAmt), txValues, nullifier, secrets)

	toStrs := func(vals []*big.Int) []string {
		s := make([]string, len(vals))
		for i, v := range vals {
			s[i] = v.String()
		}
		return s
	}
	kIndex := make([]*big.Int, nBanks)
	for i := range kIndex {
		kIndex[i] = big.NewInt(int64(i))
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
	for i, k := range keys {
		keyStrs[i] = k.String()
	}

	reqBody := map[string]interface{}{
		"fingerprint_shared_secrets":   fp2Strs(fp),
		"public_keys":                  keyStrs,
		"previous_commits":             prevCommitSlice,
		"tx_commits":                   txCommitSlice,
		"block_number":                 blockHash.String(),
		"anonymity_set":                toStrs(kIndex),
		"message_tags":                 toStrs(tagMessages),
		"nullifier":                    nullifier.String(),
		"sender_id":                    "0",
		"shared_secrets":               toStrs(secrets),
		"secret_key":                   sks[senderIdx].String(),
		"previous_sender_balance":      "500",
		"previous_sender_random_value": "67890",
		"tx_values":                    toStrs(txValues),
		"tx_random_values":             toStrs(txRandom),
		"sender_tx_value":              "100",
		"domain_id":                    expectedDomainId(enygmaAddr).String(),
	}

	proofResp := postProofRequest(t, reqBody)
	if len(proofResp.Proof) != 8 || len(proofResp.PublicSignal) != 81 {
		t.Fatalf("unexpected proof sizes: proof=%d publicSignal=%d", len(proofResp.Proof), len(proofResp.PublicSignal))
	}
	t.Log("real proof received")

	var proof8 [8]*big.Int
	for i := 0; i < 8; i++ {
		proof8[i] = proofResp.Proof[i]
	}
	var pubSig81 [81]*big.Int
	for i, v := range proofResp.PublicSignal {
		pubSig81[i] = v
	}
	transferProof := enygma.IEnygmaProof{Proof: proof8, PublicSignal: pubSig81}

	participantIds := make([]*big.Int, nBanks)
	for i := range participantIds {
		participantIds[i] = big.NewInt(int64(i + 1))
	}

	balBefore, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("GetBalance before: %v", err)
	}

	// ── The actual H-09 item 2 assertion: submit DIRECTLY, no relayer ──────
	t.Log("submitting Transfer directly as bank 0's own key — no relayer process was ever started in this test")
	tx, sendErr := instance.Transfer(bankAuth(banks[senderIdx]), txCommit, transferProof, participantIds, "")
	if sendErr != nil {
		t.Fatalf("FAIL (H-09 item 2 regressed): direct self-submission reverted: %v", sendErr)
	}
	receipt := waitTx(tx, nil)
	t.Logf("direct self-submission succeeded: tx=%s gas=%d", receipt.TxHash.Hex(), receipt.GasUsed)

	balAfter, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("GetBalance after: %v", err)
	}
	if balBefore.X.Cmp(balAfter.X) == 0 && balBefore.Y.Cmp(balAfter.Y) == 0 {
		t.Fatal("FAIL: bank 0's balance did not change after the direct transfer")
	}

	ok, err := instance.Check(&bind.CallOpts{})
	if err != nil || !ok {
		t.Fatalf("check() invariant broken: ok=%v err=%v", ok, err)
	}
	t.Log("check() invariant holds — a bank registered under its own address submitted a real transfer with ZERO relayer involvement ✓")
}
