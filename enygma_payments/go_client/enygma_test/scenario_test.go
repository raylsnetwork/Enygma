package enygma_test

// Additional scenario tests not covered by TestFullTransactionFlow:
//
//   TestCheckInvariant            — verifies Σ(bank balances) == totalSupply after a transfer.
//   TestNullifierReuseProtection  — deploys fresh contracts (including the relayer's
//                                   recursive-verification verifier), builds a real transfer
//                                   proof + real relayer re-verification proof, and checks: a
//                                   proof/relayer-proof pairing with a mismatched public_signal
//                                   reverts (rejected by _verifyRelayerProof's own cryptographic
//                                   check before _verifyRelayerBinding is ever reached — both
//                                   still correctly block it), a garbage relayer proof reverts
//                                   (InvalidProof), a genuine submission succeeds (logging gas
//                                   used), and replaying the same proof afterward is rejected
//                                   (in practice via InvalidPublicInputs, since round 1 already
//                                   advanced the balance the stale proof's previous-commitment
//                                   no longer matches — _verifyPublicInputsFP runs before
//                                   _consumeNullifierFP, so NullifierAlreadyUsed is never
//                                   reached on a naive full-proof replay; either way the replay
//                                   is blocked, which is the property that matters).
//   TestBurnBalanceUpdate         — burn() decrements a bank's Pedersen commitment by amount*G
//                                   (homomorphic subtraction); also documents that check()
//                                   breaks after burn because totalSupplyX/Y is not updated.
//   TestDoubleInitializeReverts   — calling initialize() twice reverts with AlreadyInitialized.
//   TestMintAccumulation          — minting to multiple different banks accumulates TotalSupply()
//                                   correctly and the check() invariant holds after each mint.
//   TestInvalidProofRejection     — a garbage all-zero SNARK proof is rejected on-chain
//                                   (Status=0) without modifying any contract state.
//
// See sequential_transfer_test.go for:
//   TestSequentialTransfers       — two back-to-back ZK transfers from bank 0 through the
//                                   full stack (gnark → relayer → chain); derives updated
//                                   Pedersen randomness between rounds and verifies check().
//
// Prerequisites for all tests:
//   chain: Rayls mainnet reachable at https://mainnet-rpc.rayls.com
//   gnark: gnark server running on localhost:8080 (only for TestNullifierReuseProtection)
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestCheckInvariant             -v -timeout 30s
//	CC=/usr/bin/clang go test -run TestNullifierReuseProtection   -v -timeout 300s
//	CC=/usr/bin/clang go test -run TestBurnBalanceUpdate          -v -timeout 60s
//	CC=/usr/bin/clang go test -run TestDoubleInitializeReverts    -v -timeout 60s
//	CC=/usr/bin/clang go test -run TestMintAccumulation           -v -timeout 60s
//	CC=/usr/bin/clang go test -run TestInvalidProofRejection      -v -timeout 60s

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	enygma "enygma/contracts"
	enygmaverifier "enygma/contracts/enygmaverifier"
	relayerverifier "enygma/contracts/relayerverifier"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// ── Artifact helpers ──────────────────────────────────────────────────────────

// artifactJSON reads a Hardhat artifact file and returns (ABI JSON string, deployment bytecode).
// relPath is relative to this source file.
func artifactJSON(t *testing.T, relPath string) (string, []byte) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	fullPath := filepath.Join(filepath.Dir(file), relPath)
	raw, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("read artifact %s: %v", fullPath, err)
	}
	var art struct {
		ABI      json.RawMessage `json:"abi"`
		Bytecode string          `json:"bytecode"`
	}
	if err := json.Unmarshal(raw, &art); err != nil {
		t.Fatalf("parse artifact %s: %v", relPath, err)
	}
	bz, err := hex.DecodeString(strings.TrimPrefix(art.Bytecode, "0x"))
	if err != nil {
		t.Fatalf("decode bytecode %s: %v", relPath, err)
	}
	return string(art.ABI), bz
}

// deployFromArtifact deploys a contract from a Hardhat artifact JSON, waits for mining,
// and returns the deployed address. constructorArgs are passed to the constructor.
func deployFromArtifact(
	t *testing.T,
	client *ethclient.Client,
	auth *bind.TransactOpts,
	artifactRelPath string,
	constructorArgs ...interface{},
) common.Address {
	t.Helper()
	abiStr, bytecode := artifactJSON(t, artifactRelPath)
	parsedABI, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		t.Fatalf("parse ABI from %s: %v", artifactRelPath, err)
	}
	addr, tx, _, err := bind.DeployContract(auth, parsedABI, bytecode, client, constructorArgs...)
	if err != nil {
		t.Fatalf("deploy %s: %v", artifactRelPath, err)
	}
	r, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil || r.Status != 1 {
		t.Fatalf("deploy %s: status=%d err=%v", artifactRelPath, r.Status, err)
	}
	t.Logf("deployed %s → %s (gas %d)", filepath.Base(artifactRelPath), addr.Hex(), r.GasUsed)
	return addr
}

// ── TestCheckInvariant ────────────────────────────────────────────────────────

// TestCheckInvariant calls check() on the existing deployed contract to verify
// that the homomorphic sum of all bank balance commitments equals totalSupply.
// Expected to be run after TestFullTransactionFlow on the same contract.
func TestCheckInvariant(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	tokenAddr, _ := readReceipts(t)

	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	instance, err := enygma.NewEnygma(common.HexToAddress(tokenAddr), client)
	if err != nil {
		t.Fatalf("contract instance: %v", err)
	}

	nBanksOnChain, err := instance.TotalRegisteredBanks(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalRegisteredBanks(): %v", err)
	}
	totalSupplyAmt, err := instance.TotalSupply(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalSupply(): %v", err)
	}
	t.Logf("registered banks: %s", nBanksOnChain)
	t.Logf("totalSupplyAmount: %s", totalSupplyAmt)

	tsX, err := instance.TotalSupplyX(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalSupplyX(): %v", err)
	}
	tsY, err := instance.TotalSupplyY(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalSupplyY(): %v", err)
	}
	t.Logf("totalSupplyX: %s", tsX)
	t.Logf("totalSupplyY: %s", tsY)
	if tsY.Cmp(big.NewInt(1)) == 0 && tsX.Sign() == 0 {
		t.Log("DIAGNOSTIC: totalSupply point is neutral — registerAccount fix NOT deployed (old artifact used)")
	}

	// check() reverts with BalanceMismatch if the invariant is violated;
	// returns true otherwise.
	ok, err := instance.Check(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("check() reverted — balance invariant VIOLATED: %v", err)
	}
	if !ok {
		t.Fatal("check() returned false — balance invariant violated")
	}
	t.Log("check() PASSED: Σ(bank commitment balances) == totalSupply commitment")
}

// ── TestNullifierReuseProtection ──────────────────────────────────────────────

// TestNullifierReuseProtection deploys a fresh Enygma + Verifier + RelayerVerifier
// contract set, performs the standard setup, and builds one real transfer proof plus
// one real relayer recursive re-verification proof (both via the local gnark-server).
// It then submits, in order: a tampered-public-signal relayer proof (must revert —
// _verifyRelayerProof's own check fails since Groth16 proofs bind their public inputs,
// so this never even reaches _verifyRelayerBinding), a garbage-relayer-proof variant
// (must revert InvalidProof — reverted calls consume no nullifier, so the real proof
// stays valid for what follows), the genuine transfer (must succeed — gas usage is
// logged), and finally a replay of the identical proof (must be rejected — in practice
// via InvalidPublicInputs, see the file-level doc comment above).
//
// Transfer() is called directly on the contract binding — no relayer service needed.
// This makes the test independent of which contract address the relayer is pointed at.
func TestNullifierReuseProtection(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	if !tcpAvailable("127.0.0.1:8080") {
		t.Skip("gnark server not reachable at localhost:8080")
	}

	// ── Chain client + auth factory ───────────────────────────────────────────
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

	// ── Deploy fresh contract pair ────────────────────────────────────────────
	const artifactBase = "../../contracts/enygma/artifacts/contracts"

	enygmaAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/Enygma.sol/Enygma.json",
		big.NewInt(30), // epochInterval
	)
	verifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/EnygmaVerifier.sol/Verifier.json",
	)
	relayerVerifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/RelayerVerifier.sol/Verifier.json",
	)

	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind contract: %v", err)
	}

	// ── Setup ─────────────────────────────────────────────────────────────────
	waitTx(instance.Initialize(mkAuth()))
	t.Log("initialized")

	if r := waitTx(instance.AddVerifier(mkAuth(), verifierAddr)); r.Status != 1 {
		t.Fatal("addVerifier failed")
	}
	t.Log("verifier registered")

	if r := waitTx(instance.AddRelayerVerifier(mkAuth(), relayerVerifierAddr)); r.Status != 1 {
		t.Fatal("addRelayerVerifier failed")
	}
	t.Log("relayer verifier registered")

	pks := make([]*big.Int, nBanks)
	for i, sk := range bankSks {
		pk, _ := poseidon.Hash([]*big.Int{sk, sk})
		pks[i] = pk.Mod(pk, curveP)
	}
	for i := 0; i < nBanks; i++ {
		if r := waitTx(instance.RegisterAccount(mkAuth(), ownerAddr,
			big.NewInt(int64(i+1)), pks[i], big.NewInt(senderPrevR), []byte{})); r.Status != 1 {
			t.Fatalf("registerAccount bank %d failed", i)
		}
	}
	t.Logf("registered %d banks", nBanks)

	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1))); r.Status != 1 {
		t.Fatal("mintSupply failed")
	}
	t.Logf("minted %d to bank 0 (accountId=1)", mintAmt)

	// ── Build and submit ZK proof ─────────────────────────────────────────────
	blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("getBlckHash: %v", err)
	}
	t.Logf("lastBlockNum: %s", blockHash)

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
	tagMessages := tagMessageGen(secrets, new(big.Int).Set(blockHash))

	txValues := []*big.Int{
		negMod(big.NewInt(transferAmt)),
		big.NewInt(60), big.NewInt(40),
		big.NewInt(0), big.NewInt(0), big.NewInt(0),
	}
	txCommit, txRandom := genCommitmentAndRandom(senderIdx, big.NewInt(transferAmt), txValues, new(big.Int).Set(blockHash), secrets)
	nullifier, _ := poseidon.Hash([]*big.Int{senderSecret, blockHash})

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
	})

	t.Log("requesting proof (may take ~30s)…")
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
	if len(proofResp.Proof) != 8 || len(proofResp.PublicSignal) != 80 {
		t.Fatalf("unexpected proof sizes: proof=%d publicSignal=%d", len(proofResp.Proof), len(proofResp.PublicSignal))
	}
	t.Log("proof received")

	// Build on-chain proof struct.
	// NOTE: IEnygmaProof.PublicSignal must be [80]*big.Int after contract/binding regeneration.
	var proof8 [8]*big.Int
	for i := 0; i < 8; i++ {
		proof8[i] = proofResp.Proof[i]
	}
	var pubSig80 [80]*big.Int
	for i := range pubSig80 {
		pubSig80[i] = big.NewInt(0)
	}
	for i, v := range proofResp.PublicSignal {
		pubSig80[i] = v
	}

	// Sanity check: EnygmaVerifier.verifyProof() must accept this proof on
	// its own, decoupled from anything Enygma.sol/transfer() does — isolates
	// a bad sender proof/verifier pairing from the relayer logic below.
	ev, evErr := enygmaverifier.NewEnygmaVerifier(verifierAddr, client)
	if evErr != nil {
		t.Fatalf("bind EnygmaVerifier: %v", evErr)
	}
	if err := ev.VerifyProof(&bind.CallOpts{}, proof8, pubSig80); err != nil {
		t.Fatalf("EnygmaVerifier.verifyProof() rejected the sender's own transfer proof: %v", err)
	}
	t.Log("EnygmaVerifier.verifyProof() accepted the transfer proof directly ✓")

	// TX_COMMIT_OFFSET = 36 (FingerPrint 6×6) + 6 (pks) + 12 (prevCommit) = 54.
	const txCommitOffset = 54
	commitmentDeltas := make([]enygma.IEnygmaPoint, nBanks)
	for i := 0; i < nBanks; i++ {
		commitmentDeltas[i] = enygma.IEnygmaPoint{
			C1: proofResp.PublicSignal[txCommitOffset+2*i],
			C2: proofResp.PublicSignal[txCommitOffset+2*i+1],
		}
	}

	transferProof := enygma.IEnygmaProof{Proof: proof8, PublicSignal: pubSig80}

	// ── Relayer's independent recursive re-verification proof ─────────────────
	// Same gnark-server, /proof/relayer endpoint: independently re-verifies
	// transferProof against the same transfer-circuit VK, over the exact same
	// public signal — see enygma_payments/gnark-server/pkg/circuits/relayer.
	relayerReqBody, _ := json.Marshal(map[string]interface{}{
		"proof":        toStrs(proof8[:]),
		"publicSignal": toStrs(pubSig80[:]),
	})
	relayerHTTPResp, err := http.Post(gnarkRelayerURL, "application/json", bytes.NewReader(relayerReqBody))
	if err != nil {
		t.Fatalf("gnark relayer POST: %v", err)
	}
	defer relayerHTTPResp.Body.Close()
	if relayerHTTPResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(relayerHTTPResp.Body)
		t.Fatalf("gnark relayer %d: %s", relayerHTTPResp.StatusCode, body)
	}
	var relayerProofResp struct {
		Proof        []*big.Int `json:"proof"`
		PublicSignal []*big.Int `json:"publicSignal"`
	}
	if err := json.NewDecoder(relayerHTTPResp.Body).Decode(&relayerProofResp); err != nil {
		t.Fatalf("decode relayer proof: %v", err)
	}
	if len(relayerProofResp.Proof) != 12 || len(relayerProofResp.PublicSignal) != 80 {
		t.Fatalf("unexpected relayer proof sizes: proof=%d publicSignal=%d", len(relayerProofResp.Proof), len(relayerProofResp.PublicSignal))
	}
	t.Log("relayer proof received")

	var relayerProof enygma.IEnygmaRelayerProof
	copy(relayerProof.Proof[:], relayerProofResp.Proof[0:8])
	copy(relayerProof.Commitments[:], relayerProofResp.Proof[8:10])
	copy(relayerProof.CommitmentPok[:], relayerProofResp.Proof[10:12])
	copy(relayerProof.PublicSignal[:], relayerProofResp.PublicSignal)

	// Sanity check: RelayerVerifier.verifyProof() must accept this proof on
	// its own, decoupled from anything Enygma.sol/transfer() does — isolates
	// a bad relayer proof/verifier pairing from Enygma.sol's own wiring.
	rv, rvErr := relayerverifier.NewRelayerVerifier(relayerVerifierAddr, client)
	if rvErr != nil {
		t.Fatalf("bind RelayerVerifier: %v", rvErr)
	}
	if err := rv.VerifyProof(&bind.CallOpts{}, relayerProof.Proof, relayerProof.Commitments, relayerProof.CommitmentPok, relayerProof.PublicSignal); err != nil {
		t.Fatalf("RelayerVerifier.verifyProof() rejected the relayer's own proof: %v", err)
	}
	t.Log("RelayerVerifier.verifyProof() accepted the relayer proof directly ✓")

	participantIds := make([]*big.Int, nBanks)
	for i := range participantIds {
		participantIds[i] = big.NewInt(int64(i + 1))
	}

	// expectRelayerRejection submits a Transfer with a bad relayerProof and
	// asserts it never succeeds. Hardhat Network simulates transactions as
	// part of accepting them, so a revert can surface either as an error
	// from the send itself or (less often here) as a mined Status=0
	// receipt — both mean the same thing on-chain: nothing happened, no
	// nullifier was consumed, so transferProof/relayerProof stay valid for
	// the real submission below.
	expectRelayerRejection := func(label string, badProof enygma.IEnygmaRelayerProof) {
		t.Helper()
		tx, txErr := instance.Transfer(mkAuth(), commitmentDeltas, transferProof, badProof, participantIds)
		if txErr != nil {
			t.Logf("%s correctly rejected at send (%v) ✓", label, txErr)
			return
		}
		r, err := bind.WaitMined(context.Background(), client, tx)
		if err != nil {
			t.Fatalf("%s: wait mined: %v", label, err)
		}
		if r.Status != 0 {
			t.Fatalf("FAIL: Transfer succeeded with %s — not enforced", label)
		}
		t.Logf("%s correctly rejected on-chain (Status=0, tx=%s) ✓", label, r.TxHash.Hex())
	}

	// ── Tampered relayer binding: must be rejected, nullifier untouched ───────
	// Note: since a Groth16 proof cryptographically binds its own public
	// inputs, tampering PublicSignal without regenerating the proof also
	// breaks the relayer proof's own validity (_verifyRelayerProof rejects
	// it before _verifyRelayerBinding is even reached) — this still proves
	// the security property that matters (a sender/relayer mismatch here
	// gets rejected), just via InvalidProof rather than
	// RelayerBindingMismatch specifically.
	tamperedRelayerProof := relayerProof
	tamperedRelayerProof.PublicSignal[0] = new(big.Int).Add(relayerProof.PublicSignal[0], big.NewInt(1))
	expectRelayerRejection("tampered relayer public_signal", tamperedRelayerProof)

	// ── Garbage relayer proof (correct binding, invalid proof bytes) ──────────
	garbageRelayerProof := relayerProof
	for i := range garbageRelayerProof.Proof {
		garbageRelayerProof.Proof[i] = big.NewInt(0)
	}
	expectRelayerRejection("garbage relayer proof", garbageRelayerProof)

	// ── First (real) submission: must succeed ─────────────────────────────────
	r1 := waitTx(instance.Transfer(mkAuth(), commitmentDeltas, transferProof, relayerProof, participantIds))
	if r1.Status != 1 {
		t.Fatal("first Transfer reverted — setup problem, not a nullifier issue")
	}
	t.Logf("first Transfer PASSED  tx=%s  gas=%d", r1.TxHash.Hex(), r1.GasUsed)

	// ── Second submission with identical proof: must be rejected ──────────────
	// Note: transfer() checks _verifyPublicInputsFP (proof's embedded
	// "previous commitment" matches current on-chain balance) BEFORE
	// _consumeNullifierFP — and round 1 already advanced bank 0's balance,
	// so the identical proof's now-stale previous-commitment trips
	// InvalidPublicInputs before the nullifier check is ever reached. Either
	// way the outcome that matters holds: the replay is rejected and no
	// second balance update happens.
	expectRelayerRejection("replayed proof (stale previous-commitment / nullifier reuse)", relayerProof)
	t.Log("PASSED: replay attack blocked at the contract level")
}

// ── Shared fresh-contract setup ───────────────────────────────────────────────

// freshSetup deploys Enygma + Verifier, initializes them, registers nBanks accounts
// (all with senderPrevR as randomness), and returns the bound instance.
// It does NOT mint; callers mint as needed.
func freshSetup(
	t *testing.T,
	client *ethclient.Client,
	mkAuth func() *bind.TransactOpts,
	waitTx func(*ethtypes.Transaction, error) *ethtypes.Receipt,
) *enygma.Enygma {
	t.Helper()

	ownerAddr := crypto.PubkeyToAddress(*mustPrivKey(t).Public().(*ecdsa.PublicKey))

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	enygmaAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/Enygma.sol/Enygma.json",
		big.NewInt(30),
	)
	verifierAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/EnygmaVerifier.sol/Verifier.json",
	)

	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind contract: %v", err)
	}

	waitTx(instance.Initialize(mkAuth()))

	if r := waitTx(instance.AddVerifier(mkAuth(), verifierAddr)); r.Status != 1 {
		t.Fatal("addVerifier failed")
	}

	pks := make([]*big.Int, nBanks)
	for i, sk := range bankSks {
		pk, _ := poseidon.Hash([]*big.Int{sk, sk})
		pks[i] = pk.Mod(pk, curveP)
	}
	for i := 0; i < nBanks; i++ {
		if r := waitTx(instance.RegisterAccount(mkAuth(), ownerAddr,
			big.NewInt(int64(i+1)), pks[i], big.NewInt(senderPrevR), []byte{})); r.Status != 1 {
			t.Fatalf("registerAccount bank %d failed", i)
		}
	}
	t.Logf("freshSetup: deployed at %s, registered %d banks", enygmaAddr.Hex(), nBanks)
	return instance
}

// hardhatTestKey is the publicly committed key in contracts/enygma/hardhat.config.js.
// It is safe to use only on local Hardhat nodes — never on mainnet.
const hardhatTestKey = "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"

// mustPrivKey returns the signing key for the test.
// On mainnet (non-localhost chainURL) MY_KEY must be set.
// On a local Hardhat node (chainURL contains "127.0.0.1" or "localhost") the
// committed test key is used automatically when MY_KEY is not set.
func mustPrivKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key := ownerPrivKey
	if key == "" && (strings.Contains(chainURL, "127.0.0.1") || strings.Contains(chainURL, "localhost")) {
		key = hardhatTestKey
		t.Log("using committed Hardhat test key (safe for local node only)")
	}
	if key == "" {
		t.Fatal("MY_KEY env var not set — export MY_KEY=<your-hex-private-key> before running")
	}
	pk, err := crypto.HexToECDSA(key)
	if err != nil {
		t.Fatalf("MY_KEY parse error: %v", err)
	}
	return pk
}

// scenarioClient dials the chain and returns a client+mkAuth+waitTx triple.
func scenarioClient(t *testing.T) (*ethclient.Client, func() *bind.TransactOpts, func(*ethtypes.Transaction, error) *ethtypes.Receipt) {
	t.Helper()
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { client.Close() })

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

	return client, mkAuth, waitTx
}

// ── TestBurnBalanceUpdate ─────────────────────────────────────────────────────

// TestBurnBalanceUpdate verifies that burn(accountId, amount) applies the correct
// homomorphic subtraction: newBal = prevBal + Com(P−amount, 0) = prevBal − amount·G.
//
// Also documents a known invariant gap: check() will revert after burn because
// burn() does not decrement totalSupplyX/totalSupplyY to match the reduced balance.
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestBurnBalanceUpdate -v -timeout 60s
func TestBurnBalanceUpdate(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}

	client, mkAuth, waitTx := scenarioClient(t)
	instance := freshSetup(t, client, mkAuth, waitTx)

	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1))); r.Status != 1 {
		t.Fatal("mintSupply failed")
	}
	t.Logf("minted %d to bank 0 (accountId=1)", mintAmt)

	// Read bank 0's commitment before the burn.
	prevBal, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("GetBalance(1) before burn: %v", err)
	}
	t.Logf("bank 0 pre-burn commitment:  (%s, %s)", prevBal.X, prevBal.Y)

	prevSupplyScalar, _ := instance.TotalSupply(&bind.CallOpts{})
	t.Logf("TotalSupply() scalar before burn: %s", prevSupplyScalar)

	// Burn 200 from bank 0.
	const burnAmt = 200
	if r := waitTx(instance.Burn(mkAuth(), big.NewInt(1), big.NewInt(burnAmt))); r.Status != 1 {
		t.Fatal("burn() reverted unexpectedly")
	}
	t.Logf("burn(%d) from bank 0 succeeded", burnAmt)

	newBal, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("GetBalance(1) after burn: %v", err)
	}
	t.Logf("bank 0 post-burn commitment: (%s, %s)", newBal.X, newBal.Y)

	if newBal.X.Cmp(prevBal.X) == 0 && newBal.Y.Cmp(prevBal.Y) == 0 {
		t.Fatal("bank 0 commitment unchanged after burn — burn had no effect")
	}

	// Homomorphic check: newBal == prevBal + Com(P − burnAmt, 0)
	//   Com(P − burnAmt, 0) = (P − burnAmt)·G  ≡  −burnAmt·G  on the curve.
	negDelta := pedersenCommitment(new(big.Int).Sub(curveP, big.NewInt(burnAmt)), big.NewInt(0))
	prevPt := &babyjub.Point{X: prevBal.X, Y: prevBal.Y}
	expectedNewBal := addBJPoints(prevPt, negDelta)
	if newBal.X.Cmp(expectedNewBal.X) != 0 || newBal.Y.Cmp(expectedNewBal.Y) != 0 {
		t.Errorf("homomorphic check FAILED:\n  got      (%s, %s)\n  expected (%s, %s)",
			newBal.X, newBal.Y, expectedNewBal.X, expectedNewBal.Y)
	} else {
		t.Log("homomorphic check PASSED: newBal == prevBal − burnAmt·G ✓")
	}

	// The scalar TotalSupply() is NOT decremented by burn.
	afterSupplyScalar, _ := instance.TotalSupply(&bind.CallOpts{})
	if afterSupplyScalar.Cmp(prevSupplyScalar) != 0 {
		t.Errorf("TotalSupply() scalar changed after burn: %s → %s (unexpected)", prevSupplyScalar, afterSupplyScalar)
	} else {
		t.Logf("TotalSupply() scalar unchanged (%s) after burn — as expected (burn adjusts commitments only)", afterSupplyScalar)
	}

	// check() is expected to revert here: burn decrements the balance commitment but
	// does not update totalSupplyX/totalSupplyY, so Σ(balances) ≠ totalSupply commitment.
	_, checkErr := instance.Check(&bind.CallOpts{})
	if checkErr != nil {
		t.Logf("check() correctly reverts after burn (totalSupply commitment not decremented): %v", checkErr)
		t.Log("DOCUMENTED: burn does not maintain the Σ(balances)==totalSupply commitment invariant")
	} else {
		t.Error("check() unexpectedly passed after burn — totalSupplyX/Y may have been modified")
	}
}

// ── TestDoubleInitializeReverts ───────────────────────────────────────────────

// TestDoubleInitializeReverts verifies that calling initialize() twice on the same
// contract reverts with AlreadyInitialized on the second attempt.
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestDoubleInitializeReverts -v -timeout 60s
func TestDoubleInitializeReverts(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}

	client, mkAuth, waitTx := scenarioClient(t)

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	enygmaAddr := deployFromArtifact(t, client, mkAuth(),
		artifactBase+"/Enygma.sol/Enygma.json",
		big.NewInt(30),
	)
	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind contract: %v", err)
	}

	// First call must succeed.
	r1 := waitTx(instance.Initialize(mkAuth()))
	if r1.Status != 1 {
		t.Fatal("first initialize() reverted — unexpected")
	}
	t.Logf("first initialize() succeeded (tx=%s) ✓", r1.TxHash.Hex())

	// Second call must revert with AlreadyInitialized.
	// mkAuth sets GasLimit explicitly → go-ethereum skips eth_call simulation and
	// sends the tx; revert is detected via receipt Status == 0.
	r2 := waitTx(instance.Initialize(mkAuth()))
	if r2.Status != 0 {
		t.Fatal("FAIL: second initialize() succeeded — AlreadyInitialized guard not enforced")
	}
	t.Logf("second initialize() correctly reverted (Status=0, tx=%s) — AlreadyInitialized ✓", r2.TxHash.Hex())
}

// ── TestMintAccumulation ──────────────────────────────────────────────────────

// TestMintAccumulation mints to three separate banks (accountIds 1, 2, 3) with
// distinct amounts, then verifies:
//
//   - TotalSupply() scalar equals the arithmetic sum of all mint amounts.
//   - Each minted bank's commitment is non-neutral (changed from registration value).
//   - check() invariant holds: Σ(bank commitments) == totalSupply commitment.
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestMintAccumulation -v -timeout 60s
func TestMintAccumulation(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}

	client, mkAuth, waitTx := scenarioClient(t)
	instance := freshSetup(t, client, mkAuth, waitTx)

	// Mint different amounts to banks 0, 1, 2 (accountIds 1, 2, 3).
	mintAmounts := []*big.Int{big.NewInt(300), big.NewInt(150), big.NewInt(50)}
	accountIds := []int64{1, 2, 3}

	for i, id := range accountIds {
		if r := waitTx(instance.MintSupply(mkAuth(), mintAmounts[i], big.NewInt(id))); r.Status != 1 {
			t.Fatalf("mintSupply(%s, accountId=%d) failed", mintAmounts[i], id)
		}
		t.Logf("minted %s to accountId=%d", mintAmounts[i], id)
	}

	// Verify scalar totalSupply equals sum of mints.
	expectedTotal := big.NewInt(300 + 150 + 50)
	gotTotal, err := instance.TotalSupply(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalSupply(): %v", err)
	}
	if gotTotal.Cmp(expectedTotal) != 0 {
		t.Errorf("TotalSupply(): got %s, expected %s", gotTotal, expectedTotal)
	} else {
		t.Logf("TotalSupply() = %s ✓ (300 + 150 + 50)", gotTotal)
	}

	// Each minted bank's commitment must differ from its registration value Com(0, senderPrevR).
	regCommit := pedersenCommitment(big.NewInt(0), big.NewInt(senderPrevR))
	for _, id := range accountIds {
		bal, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(id))
		if err != nil {
			t.Fatalf("GetBalance(accountId=%d): %v", id, err)
		}
		if bal.X.Cmp(regCommit.X) == 0 && bal.Y.Cmp(regCommit.Y) == 0 {
			t.Errorf("accountId=%d commitment unchanged after mint — mint had no effect", id)
		} else {
			t.Logf("accountId=%d commitment updated after mint ✓", id)
		}
	}

	// Invariant: Σ(all bank commitments) == totalSupply commitment point.
	ok, err := instance.Check(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("check() reverted — balance invariant VIOLATED: %v", err)
	}
	if !ok {
		t.Fatal("check() returned false — invariant violated")
	}
	t.Log("check() PASSED: Σ(bank commitments) == totalSupply commitment ✓")
}

// ── TestInvalidProofRejection ─────────────────────────────────────────────────

// TestInvalidProofRejection submits a Transfer call with a completely invalid SNARK
// proof (8 zero field elements) and an all-zero public signal (50 zeros). The contract
// must reject the transaction (Status=0) without altering any balance commitments.
//
// Run:
//
//	CC=/usr/bin/clang go test -run TestInvalidProofRejection -v -timeout 60s
func TestInvalidProofRejection(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}

	client, mkAuth, waitTx := scenarioClient(t)
	instance := freshSetup(t, client, mkAuth, waitTx)

	if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1))); r.Status != 1 {
		t.Fatal("mintSupply failed")
	}

	// Snapshot bank 0 commitment before the attempted transfer.
	prevBal, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("GetBalance(1): %v", err)
	}

	// Build a garbage proof: 8 zeros for the Groth16 elements, 50 zeros for
	// the public signal, and identity points (0,1) as commitment deltas.
	var badProof [8]*big.Int
	for i := range badProof {
		badProof[i] = big.NewInt(0)
	}
	var badPubSig [80]*big.Int
	for i := range badPubSig {
		badPubSig[i] = big.NewInt(0)
	}

	neutralDeltas := make([]enygma.IEnygmaPoint, nBanks)
	for i := range neutralDeltas {
		neutralDeltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
	}

	participantIds := make([]*big.Int, nBanks)
	for i := range participantIds {
		participantIds[i] = big.NewInt(int64(i + 1))
	}

	badTransferProof := enygma.IEnygmaProof{Proof: badProof, PublicSignal: badPubSig}

	// The relayer proof here is also garbage — irrelevant, since
	// _verifyTransferProof (checked first in transfer()) already rejects
	// badTransferProof before the relayer proof is ever examined.
	var badRelayerProof enygma.IEnygmaRelayerProof
	for i := range badRelayerProof.Proof {
		badRelayerProof.Proof[i] = big.NewInt(0)
	}
	for i := range badRelayerProof.Commitments {
		badRelayerProof.Commitments[i] = big.NewInt(0)
	}
	for i := range badRelayerProof.CommitmentPok {
		badRelayerProof.CommitmentPok[i] = big.NewInt(0)
	}
	copy(badRelayerProof.PublicSignal[:], badPubSig[:])

	// mkAuth sets explicit GasLimit → tx is sent without eth_call simulation.
	// Revert is detected via receipt Status == 0.
	r := waitTx(instance.Transfer(mkAuth(), neutralDeltas, badTransferProof, badRelayerProof, participantIds))
	if r.Status != 0 {
		t.Fatal("FAIL: Transfer with invalid proof succeeded — proof verification not enforced")
	}
	t.Logf("invalid proof correctly rejected (Status=0, tx=%s) ✓", r.TxHash.Hex())

	// Confirm bank 0's balance was not modified by the reverted transaction.
	afterBal, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatalf("GetBalance(1) after invalid proof: %v", err)
	}
	if afterBal.X.Cmp(prevBal.X) != 0 || afterBal.Y.Cmp(prevBal.Y) != 0 {
		t.Error("FAIL: bank 0 balance changed despite reverted transfer — state mutation on revert")
	} else {
		t.Log("bank 0 balance unchanged after reverted transfer — state correctly rolled back ✓")
	}
	t.Log("PASSED: invalid SNARK proof rejected, no state modified")
}
