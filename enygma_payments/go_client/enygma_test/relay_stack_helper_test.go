package enygma_test

// Shared setup for the tests that relay a transfer through a relayer against
// the deploy_direct.py contract (TestFullTransactionFlow, TestEpochIntervalFlow,
// TestBlockNumberMismatch). A relayed transfer needs, beyond the main proof:
//
//   - every pair of participants to have confirmed each other's fingerprint
//     (C-04), which each bank does from its own address, so every bank needs
//     its own registered, funded key;
//   - a USDr fee proof paying the relayer, which the relayer checks before
//     relaying (Config.VerifyFeeSlot), so the relayer must be one of the
//     participants: it runs as bank relayFeeSlot.
//
// TestSequentialTransfers sets this up inline for its own fresh deployment;
// this is the same setup, factored out.

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
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

const (
	// relayFeeSlot is the bank the test-local relayer runs as, and so the
	// participant the USDr fee is paid to. It must not be the sender.
	relayFeeSlot = nBanks - 1
	// relayStackPort is the test-local relayer's port, distinct from a
	// relayer the developer may already run on :8082 and from
	// TestSequentialTransfers' :8084.
	relayStackPort = "8085"
)

// relayBanks holds one registered key per bank: bank 0 (the sender) is the
// owner, the others are generated and funded.
type relayBanks struct {
	addrs []common.Address
	keys  []*ecdsa.PrivateKey
	auth  func(i int) *bind.TransactOpts
}

// newRelayBanks generates and funds a key for every bank but the sender, whose
// address is the owner's.
func newRelayBanks(t *testing.T, client *ethclient.Client, ownerKey *ecdsa.PrivateKey, mkAuth func() *bind.TransactOpts) relayBanks {
	t.Helper()
	ctx := context.Background()
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	b := relayBanks{addrs: make([]common.Address, nBanks), keys: make([]*ecdsa.PrivateKey, nBanks)}
	b.addrs[senderIdx], b.keys[senderIdx] = ownerAddr, ownerKey
	for i := 0; i < nBanks; i++ {
		if i == senderIdx {
			continue
		}
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatalf("generate bank %d key: %v", i, err)
		}
		b.keys[i], b.addrs[i] = key, crypto.PubkeyToAddress(key.PublicKey)

		gasPrice, _ := client.SuggestGasPrice(ctx)
		nonce, _ := client.PendingNonceAt(ctx, ownerAddr)
		fundTx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce: nonce, To: &b.addrs[i], Value: big.NewInt(50_000_000_000_000_000), // 0.05 ETH
			Gas: 21000, GasPrice: gasPrice,
		}), ethtypes.NewEIP155Signer(big.NewInt(chainID)), ownerKey)
		if err != nil {
			t.Fatalf("sign funding tx: %v", err)
		}
		if err := client.SendTransaction(ctx, fundTx); err != nil {
			t.Fatalf("fund bank %d: %v", i, err)
		}
		if _, err := bind.WaitMined(ctx, client, fundTx); err != nil {
			t.Fatalf("wait funding bank %d: %v", i, err)
		}
	}
	b.auth = func(i int) *bind.TransactOpts {
		if i == senderIdx {
			return mkAuth()
		}
		nonce, _ := client.PendingNonceAt(ctx, b.addrs[i])
		gasPrice, _ := client.SuggestGasPrice(ctx)
		auth, _ := bind.NewKeyedTransactorWithChainID(b.keys[i], big.NewInt(chainID))
		auth.Nonce = big.NewInt(int64(nonce))
		auth.Value = big.NewInt(0)
		auth.GasLimit = 16_000_000
		auth.GasPrice = gasPrice
		return auth
	}
	return b
}

// senderSecrets is the shared-secret vector the main proof uses for the
// sender's first transfer (Com(senderPrevV, senderPrevR)).
func senderSecrets() []*big.Int {
	s, _ := poseidon.Hash([]*big.Int{big.NewInt(senderPrevR), big.NewInt(senderSk)})
	s.Mod(s, curveP)
	secrets := make([]*big.Int, nBanks)
	copy(secrets, baseSecrets)
	secrets[senderIdx] = s
	return secrets
}

// confirmFingerprints has every bank confirm the fingerprint of every other,
// as the main proof built from secrets will present them.
func confirmFingerprints(t *testing.T, instance *enygma.Enygma, banks relayBanks,
	waitTx func(*ethtypes.Transaction, error) *ethtypes.Receipt, secrets []*big.Int) {
	t.Helper()
	fp := fingerPrintGen(secrets, senderIdx)
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			if i == j {
				continue
			}
			if r := waitTx(instance.RegisterFingerprint(banks.auth(i), big.NewInt(int64(j+1)), fp[i][j])); r.Status != 1 {
				t.Fatalf("registerFingerprint %d→%d reverted", i, j)
			}
		}
	}
	t.Logf("all %d directed pairwise fingerprints confirmed", nBanks*(nBanks-1))
}

// setupUsdrLedger registers the USDr verifier and fixed fee, initializes every
// bank's USDr balance to Com(0, usdrPrevR) and mints usdrMintAmt to the sender,
// so the sender's USDr balance opens as (usdrPrevV, usdrPrevR).
func setupUsdrLedger(t *testing.T, client *ethclient.Client, instance *enygma.Enygma,
	mkAuth func() *bind.TransactOpts, waitTx func(*ethtypes.Transaction, error) *ethtypes.Receipt) {
	t.Helper()
	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	usdrVerifier := deployFromArtifact(t, client, mkAuth(), artifactBase+"/UsdrVerifier.sol/Verifier.json")
	ok := func(what string, r *ethtypes.Receipt) {
		if r.Status != 1 {
			t.Fatalf("%s reverted", what)
		}
	}
	ok("addUsdrVerifier", waitTx(instance.AddUsdrVerifier(mkAuth(), usdrVerifier)))
	ok("setUsdrFixedFee", waitTx(instance.SetUsdrFixedFee(mkAuth(), big.NewInt(usdrFeeAmt))))
	cx, cy := regCommit(big.NewInt(usdrPrevR))
	for i := 0; i < nBanks; i++ {
		ok(fmt.Sprintf("initializeUsdrBalance(%d)", i+1), waitTx(instance.InitializeUsdrBalance(mkAuth(), big.NewInt(int64(i+1)), cx, cy)))
	}
	ok("mintUsdrSupply", waitTx(instance.MintUsdrSupply(mkAuth(), big.NewInt(usdrMintAmt), big.NewInt(senderIdx+1))))
	t.Logf("USDr ledger ready: fee %d, %d USDr minted to bank %d", usdrFeeAmt, usdrMintAmt, senderIdx)
}

// startTestRelayer runs the relayer binary as bank relayFeeSlot against
// enygmaAddr on relayStackPort, and returns its URL. It is stopped when the
// test ends.
func startTestRelayer(t *testing.T, enygmaAddr common.Address, banks relayBanks) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	relayerDir, _ := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", "..", "relayer"))
	relayerBin := filepath.Join(relayerDir, "relayer_bin")
	if _, err := os.Stat(relayerBin); os.IsNotExist(err) {
		t.Fatalf("relayer binary not found: %s\nRun: cd enygma_payments/relayer && ./run.sh", relayerBin)
	}
	// macOS enforces code signing: ad-hoc sign so the binary can run.
	if out, err := exec.Command("codesign", "--force", "--deep", "--sign", "-", relayerBin).CombinedOutput(); err != nil {
		t.Logf("codesign warning (non-fatal): %v — %s", err, out)
	}
	if tcpAvailable("127.0.0.1:" + relayStackPort) {
		t.Fatalf("port %s is already in use; the test-local relayer needs it", relayStackPort)
	}

	cmd := exec.Command(relayerBin)
	cmd.Dir = relayerDir
	cmd.Env = append(os.Environ(),
		"RELAYER_RPC_URL="+chainURL,
		fmt.Sprintf("RELAYER_CHAIN_ID=%d", chainID),
		"RELAYER_PRIVATE_KEY="+hex.EncodeToString(crypto.FromECDSA(banks.keys[relayFeeSlot])),
		"RELAYER_API_KEY="+relayerKey,
		"RELAYER_GAS_LIMIT=10000000",
		"RELAYER_CONTRACT_ADDR="+enygmaAddr.Hex(),
		"RELAYER_PORT="+relayStackPort,
	)
	logFile, _ := os.CreateTemp("", "relayer-output-*.txt")
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		t.Fatalf("start relayer: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		if data, err := os.ReadFile(logFile.Name()); err == nil && len(data) > 0 && t.Failed() {
			t.Logf("relayer output:\n%s", data)
		}
		os.Remove(logFile.Name())
	})
	for i := 0; !tcpAvailable("127.0.0.1:" + relayStackPort); i++ {
		if i == 20 {
			t.Fatal("test-local relayer did not start within 20s")
		}
		time.Sleep(time.Second)
	}
	t.Logf("test-local relayer on :%s as bank %d → %s", relayStackPort, relayFeeSlot, enygmaAddr.Hex())
	return "http://127.0.0.1:" + relayStackPort
}

// usdrFeeLeg is the USDr half of a relayed transfer: the fee proof, its
// commitment deltas, and the opening of the relayer's fee note.
type usdrFeeLeg struct {
	Proof         [8]string
	PublicSignal  []string
	Commitments   [][]string
	FeeRandomness string
}

// buildUsdrFeeLeg proves the sender paying usdrFeeAmt USDr to relayFeeSlot at
// blockHash, from the sender's first USDr balance (usdrPrevV, usdrPrevR).
// usdrPrevBals are the participants' current USDr balances.
func buildUsdrFeeLeg(t *testing.T, blockHash *big.Int, usdrPrevBals []enygma.IEnygmaPoint,
	onChainKeys []*big.Int, enygmaAddr common.Address) usdrFeeLeg {
	t.Helper()
	sk := big.NewInt(senderSk)
	usdrSenderSecret, _ := poseidon.Hash([]*big.Int{big.NewInt(usdrPrevR), sk})
	usdrSenderSecret.Mod(usdrSenderSecret, curveP)
	secrets := make([]*big.Int, nBanks)
	copy(secrets, baseSecrets)
	secrets[senderIdx] = usdrSenderSecret

	nullifier, _ := poseidon.Hash([]*big.Int{usdrSenderSecret, blockHash})
	txValues := make([]*big.Int, nBanks)
	for i := range txValues {
		txValues[i] = big.NewInt(0)
	}
	txValues[senderIdx] = negMod(big.NewInt(usdrFeeAmt))
	txValues[relayFeeSlot] = big.NewInt(usdrFeeAmt)
	txCommit, txRand := genCommitmentAndRandomUsdr(senderIdx, big.NewInt(usdrFeeAmt), txValues, nullifier, secrets)

	strs := func(vals []*big.Int) []string {
		s := make([]string, len(vals))
		for i, v := range vals {
			s[i] = v.String()
		}
		return s
	}
	points := func(pts []enygma.IEnygmaPoint) [][]string {
		s := make([][]string, len(pts))
		for i, p := range pts {
			s[i] = []string{p.C1.String(), p.C2.String()}
		}
		return s
	}
	anonymitySet := make([]*big.Int, nBanks)
	for i := range anonymitySet {
		anonymitySet[i] = big.NewInt(int64(i))
	}
	keyStrs := strs(onChainKeys)
	body, _ := json.Marshal(map[string]interface{}{
		"fingerprint_shared_secrets":   fp2Strs(fingerPrintGen(secrets, senderIdx)),
		"public_keys":                  keyStrs,
		"previous_commits":             points(usdrPrevBals),
		"tx_commits":                   points(txCommit),
		"block_number":                 blockHash.String(),
		"anonymity_set":                strs(anonymitySet),
		"message_tags":                 strs(tagMessageGenUsdr(senderIdx, secrets, nullifier)),
		"nullifier":                    nullifier.String(),
		"sender_id":                    fmt.Sprintf("%d", senderIdx),
		"shared_secrets":               strs(secrets),
		"secret_key":                   sk.String(),
		"previous_sender_balance":      fmt.Sprintf("%d", usdrPrevV),
		"previous_sender_random_value": fmt.Sprintf("%d", usdrPrevR),
		"tx_values":                    strs(txValues),
		"tx_random_values":             strs(txRand),
		"sender_tx_value":              fmt.Sprintf("%d", usdrFeeAmt),
		"domain_id":                    expectedDomainId(enygmaAddr).String(),
		"fee_recipient_key":            keyStrs[relayFeeSlot],
	})

	resp, err := http.Post(gnarkUsdrURL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("gnark usdr POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("gnark usdr %d: %s", resp.StatusCode, raw)
	}
	var proof struct {
		Proof        []*big.Int `json:"proof"`
		PublicSignal []*big.Int `json:"publicSignal"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&proof); err != nil {
		t.Fatalf("decode usdr proof: %v", err)
	}
	if len(proof.Proof) != 8 || len(proof.PublicSignal) != 83 {
		t.Fatalf("unexpected usdr proof sizes: proof=%d publicSignal=%d", len(proof.Proof), len(proof.PublicSignal))
	}

	leg := usdrFeeLeg{PublicSignal: strs(proof.PublicSignal), FeeRandomness: txRand[relayFeeSlot].String()}
	for i := 0; i < 8; i++ {
		leg.Proof[i] = proof.Proof[i].String()
	}
	// TX_COMMIT_OFFSET = 36 (FingerPrint 6×6) + 6 (pks) + 12 (prevCommit) = 54.
	const txCommitOffset = 54
	leg.Commitments = make([][]string, nBanks)
	for i := 0; i < nBanks; i++ {
		leg.Commitments[i] = []string{proof.PublicSignal[txCommitOffset+2*i].String(), proof.PublicSignal[txCommitOffset+2*i+1].String()}
	}
	t.Log("USDr fee proof received")
	return leg
}

// usdrBalances returns the participants' current USDr balances (accounts 1..nBanks).
func usdrBalances(t *testing.T, instance *enygma.Enygma) []enygma.IEnygmaPoint {
	t.Helper()
	bals := make([]enygma.IEnygmaPoint, nBanks)
	for i := range bals {
		b, err := instance.GetUsdrBalance(&bind.CallOpts{}, big.NewInt(int64(i+1)))
		if err != nil {
			t.Fatalf("getUsdrBalance(%d): %v", i+1, err)
		}
		bals[i] = enygma.IEnygmaPoint{C1: b.X, C2: b.Y}
	}
	return bals
}
