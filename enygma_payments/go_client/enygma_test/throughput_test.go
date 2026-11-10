package enygma_test

// TestPaymentsThroughput measures how many confidential transfers Enygma can
// settle per second. Opt-in: set PAY_MODE.
//
//	PAY_MODE=burst PAY_GROUPS=20 PAY_ROUNDS=3 PAY_EPOCH=1000000 \
//	  CC=/usr/bin/clang go test -run TestPaymentsThroughput -v -timeout 7200s
//	PAY_MODE=loop  PAY_GROUPS=4  PAY_DURATION=120 PAY_EPOCH=30 \
//	  CC=/usr/bin/clang go test -run TestPaymentsThroughput -v -timeout 7200s
//
// Every transfer() names a k=6 anonymity set and must match the current
// on-chain balance of all six accounts plus lastBlockNum, so two transfers
// can only be in flight together when their sets are disjoint. The test
// registers PAY_GROUPS disjoint groups of six banks (6×PAY_GROUPS banks in
// total). In each group bank 0 is the sender, banks 1-4 are recipients and
// bank 5 submits the transaction and is paid the USDr fee.
//
//   - burst: every group proves one transfer against the same state, then all
//     groups submit at once. Repeated PAY_ROUNDS times. Measures the chain's
//     execution ceiling and gas per transfer at this bank count.
//   - loop: every group runs read state → prove → submit → wait, repeatedly,
//     for PAY_DURATION seconds. Measures sustained transfers/s, latency, and
//     how often in-flight transfers are invalidated.
//
// Needs a node at ENYGMA_CHAIN_URL that produces blocks on its own clock and
// the gnark server on :8080. Contracts are deployed fresh by the test.

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

const (
	tpTransferAmt = 1
	tpMintAmt     = 1_000_000
	tpUsdrMintAmt = 1_000_000
)

type tpGroup struct {
	ids       [nBanks]int64
	keys      [nBanks]*ecdsa.PrivateKey
	addrs     [nBanks]common.Address
	sk        *big.Int // sender (slot 0) spend key
	prevR     *big.Int
	prevV     int64
	usdrPrevR *big.Int
	usdrPrevV int64
	nonce     uint64 // submitter (slot 5) nonce
}

type tpProved struct {
	deltas     []enygma.IEnygmaPoint
	proof      enygma.IEnygmaProof
	usdrDeltas []enygma.IEnygmaPoint
	usdrProof  enygma.IEnygmaUsdrProof
	txRand     []*big.Int
	usdrTxRand []*big.Int
	proveTime  time.Duration
}

func tpEnv(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func tpPercentile(d []time.Duration, p int) time.Duration {
	if len(d) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(a, b int) bool { return s[a] < s[b] })
	i := len(s) * p / 100
	if i >= len(s) {
		i = len(s) - 1
	}
	return s[i]
}

// tpRevertReason replays a failed call and names the custom error it hit.
func tpRevertReason(ctx context.Context, client *ethclient.Client, a abi.ABI, msg ethereum.CallMsg, block *big.Int) string {
	_, err := client.CallContract(ctx, msg, block)
	if err == nil {
		return "no revert on replay"
	}
	var de rpc.DataError
	if errors.As(err, &de) {
		if s, ok := de.ErrorData().(string); ok {
			data, _ := hexutil.Decode(s)
			if len(data) >= 4 {
				for name, e := range a.Errors {
					if bytes.Equal(e.ID[:4], data[:4]) {
						return name
					}
				}
				return fmt.Sprintf("0x%x", data[:4])
			}
		}
	}
	if strings.Contains(err.Error(), "out of gas") {
		return "out of gas"
	}
	return "empty revert"
}

func TestPaymentsThroughput(t *testing.T) {
	mode := os.Getenv("PAY_MODE")
	if mode != "burst" && mode != "loop" {
		t.Skip("set PAY_MODE=burst or PAY_MODE=loop to run the throughput benchmark")
	}
	if !chainAvailable() || !tcpAvailable("127.0.0.1:8080") {
		t.Skip("needs a node at ENYGMA_CHAIN_URL and the gnark server on :8080")
	}
	nGroups := tpEnv("PAY_GROUPS", 4)
	epoch := tpEnv("PAY_EPOCH", 30)
	workers := tpEnv("PAY_WORKERS", 4)
	nTotal := nGroups * nBanks

	ctx := context.Background()
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	cid := big.NewInt(chainID)
	signer := ethtypes.LatestSignerForChainID(cid)
	var clientVersion string
	_ = client.Client().Call(&clientVersion, "web3_clientVersion")

	ownerKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	ownerNonce, err := client.PendingNonceAt(ctx, ownerAddr)
	if err != nil {
		t.Fatal(err)
	}
	ownerAuth := func() *bind.TransactOpts {
		auth, _ := bind.NewKeyedTransactorWithChainID(ownerKey, cid)
		auth.Nonce = new(big.Int).SetUint64(ownerNonce)
		ownerNonce++
		return auth
	}

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	abiStr, _ := artifactJSON(t, artifactBase+"/Enygma.sol/Enygma.json")
	enygmaABI, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		t.Fatal(err)
	}
	enygmaAddr := deployFromArtifact(t, client, ownerAuth(), artifactBase+"/Enygma.sol/Enygma.json", big.NewInt(int64(epoch)))
	verifierAddr := deployFromArtifact(t, client, ownerAuth(), artifactBase+"/EnygmaVerifier.sol/Verifier.json")
	usdrVerifierAddr := deployFromArtifact(t, client, ownerAuth(), artifactBase+"/UsdrVerifier.sol/Verifier.json")
	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("node %q: %d groups (%d banks), epoch %d blocks, mode %s", clientVersion, nGroups, nTotal, epoch, mode)

	// ── Pipelined setup: sign everything with explicit nonces, send, then wait.
	pack := func(method string, args ...interface{}) []byte {
		data, err := enygmaABI.Pack(method, args...)
		if err != nil {
			t.Fatalf("pack %s: %v", method, err)
		}
		return data
	}
	feeCap := big.NewInt(100_000_000_000)
	tip := big.NewInt(1_000_000_000)
	sign := func(key *ecdsa.PrivateKey, nonce uint64, to common.Address, value *big.Int, gas uint64, data []byte) *ethtypes.Transaction {
		tx, err := ethtypes.SignNewTx(key, signer, &ethtypes.DynamicFeeTx{
			ChainID: cid, Nonce: nonce, GasTipCap: tip, GasFeeCap: feeCap,
			Gas: gas, To: &to, Value: value, Data: data,
		})
		if err != nil {
			t.Fatal(err)
		}
		return tx
	}
	waitAll := func(label string, txs []*ethtypes.Transaction) []*ethtypes.Receipt {
		t.Helper()
		out := make([]*ethtypes.Receipt, len(txs))
		for i, tx := range txs {
			for {
				r, err := client.TransactionReceipt(ctx, tx.Hash())
				if err == nil {
					out[i] = r
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		return out
	}
	sendAndWait := func(label string, txs []*ethtypes.Transaction) {
		t.Helper()
		for _, tx := range txs {
			if err := client.SendTransaction(ctx, tx); err != nil {
				t.Fatalf("%s: send: %v", label, err)
			}
		}
		for i, r := range waitAll(label, txs) {
			if r.Status != 1 {
				from, _ := ethtypes.Sender(signer, txs[i])
				reason := tpRevertReason(ctx, client, enygmaABI, ethereum.CallMsg{From: from, To: txs[i].To(), Data: txs[i].Data(), Gas: txs[i].Gas()}, new(big.Int).Sub(r.BlockNumber, big.NewInt(1)))
				t.Fatalf("%s: tx %d reverted (%s)", label, i, reason)
			}
		}
	}
	ownerTx := func(gas uint64, data []byte) *ethtypes.Transaction {
		tx := sign(ownerKey, ownerNonce, enygmaAddr, nil, gas, data)
		ownerNonce++
		return tx
	}

	setupStart := time.Now()
	sendAndWait("init", []*ethtypes.Transaction{
		ownerTx(500_000, pack("initialize")),
		ownerTx(500_000, pack("addVerifier", verifierAddr)),
		ownerTx(500_000, pack("addUsdrVerifier", usdrVerifierAddr)),
		ownerTx(500_000, pack("setUsdrFixedFee", big.NewInt(usdrFeeAmt))),
	})

	groups := make([]*tpGroup, nGroups)
	var fund []*ethtypes.Transaction
	for g := range groups {
		gr := &tpGroup{
			sk:        big.NewInt(int64(1_000_003 + 7919*g)),
			prevR:     big.NewInt(senderPrevR),
			prevV:     tpMintAmt,
			usdrPrevR: big.NewInt(usdrPrevR),
			usdrPrevV: tpUsdrMintAmt,
		}
		for i := 0; i < nBanks; i++ {
			gr.ids[i] = int64(g*nBanks + i + 1)
			k, _ := crypto.GenerateKey()
			gr.keys[i] = k
			gr.addrs[i] = crypto.PubkeyToAddress(k.PublicKey)
			eth := new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18))
			fund = append(fund, sign(ownerKey, ownerNonce, gr.addrs[i], eth, 21000, nil))
			ownerNonce++
		}
		groups[g] = gr
	}
	sendAndWait("fund", fund)

	// Spend keys: the sender's is gr.sk; the others only need to be distinct.
	pkOf := func(sk *big.Int) *big.Int {
		pk, _ := poseidon.Hash([]*big.Int{sk, sk})
		return pk.Mod(pk, curveP)
	}
	otherSk := func(g, i int) *big.Int { return big.NewInt(int64(5_000_011 + 13*(g*nBanks+i))) }
	var reg []*ethtypes.Transaction
	for g, gr := range groups {
		for i := 0; i < nBanks; i++ {
			sk := otherSk(g, i)
			r := big.NewInt(senderPrevR)
			if i == 0 {
				sk = gr.sk
				r = big.NewInt(senderRegR)
			}
			cx, cy := regCommit(r)
			reg = append(reg, ownerTx(3_000_000, pack("registerAccount", gr.addrs[i], big.NewInt(gr.ids[i]), pkOf(sk), cx, cy, []byte{})))
		}
	}
	sendAndWait("register", reg)

	var usdr []*ethtypes.Transaction
	ux, uy := regCommit(big.NewInt(usdrPrevR))
	for _, gr := range groups {
		for i := 0; i < nBanks; i++ {
			usdr = append(usdr, ownerTx(500_000, pack("initializeUsdrBalance", big.NewInt(gr.ids[i]), ux, uy)))
		}
	}
	sendAndWait("usdr init", usdr)

	// Mints run check() over every account, so their cost grows with nTotal.
	var mints []*ethtypes.Transaction
	mx, my := mintCommitPt(big.NewInt(tpMintAmt), big.NewInt(senderMintR))
	for _, gr := range groups {
		mints = append(mints,
			ownerTx(60_000_000, pack("mintSupply", big.NewInt(tpMintAmt), big.NewInt(gr.ids[0]), mx, my)),
			ownerTx(60_000_000, pack("mintUsdrSupply", big.NewInt(tpUsdrMintAmt), big.NewInt(gr.ids[0]))))
	}
	sendAndWait("mint", mints)

	// Fingerprints depend only on the recipients' shared secrets, so every
	// directed pair in a group is confirmed once.
	initSecrets := make([]*big.Int, nBanks)
	copy(initSecrets, baseSecrets)
	initSecrets[0] = big.NewInt(1)
	fp := fingerPrintGen(initSecrets, 0)
	var fps []*ethtypes.Transaction
	for _, gr := range groups {
		for i := 0; i < nBanks; i++ {
			n := uint64(0)
			for j := 0; j < nBanks; j++ {
				if i == j {
					continue
				}
				fps = append(fps, sign(gr.keys[i], n, enygmaAddr, nil, 300_000, pack("registerFingerprint", big.NewInt(gr.ids[j]), fp[i][j])))
				n++
			}
			if i == nBanks-1 {
				gr.nonce = n
			}
		}
	}
	sendAndWait("fingerprints", fps)
	t.Logf("setup: %d setup txs in %v", 4+len(fund)+len(reg)+len(usdr)+len(mints)+len(fps), time.Since(setupStart).Round(time.Second))

	// ── Proving ─────────────────────────────────────────────────────────────
	keyStrs := func(keys []*big.Int, gr *tpGroup) []string {
		out := make([]string, nBanks)
		for i := 0; i < nBanks; i++ {
			out[i] = keys[gr.ids[i]].String()
		}
		return out
	}
	commitStrs := func(bals []enygma.IEnygmaPoint, gr *tpGroup) [][]string {
		out := make([][]string, nBanks)
		for i := 0; i < nBanks; i++ {
			b := bals[gr.ids[i]]
			out[i] = []string{b.C1.String(), b.C2.String()}
		}
		return out
	}
	pointStrs := func(pts []enygma.IEnygmaPoint) [][]string {
		out := make([][]string, len(pts))
		for i, p := range pts {
			out[i] = []string{p.C1.String(), p.C2.String()}
		}
		return out
	}
	strs := func(vals []*big.Int) []string {
		out := make([]string, len(vals))
		for i, v := range vals {
			out[i] = v.String()
		}
		return out
	}
	kIdx := make([]*big.Int, nBanks)
	for i := range kIdx {
		kIdx[i] = big.NewInt(int64(i))
	}
	domain := expectedDomainId(enygmaAddr)
	httpClient := &http.Client{Timeout: 10 * time.Minute}
	postProof := func(url string, body interface{}, nSignals int) ([]*big.Int, []*big.Int, error) {
		raw, _ := json.Marshal(body)
		resp, err := httpClient.Post(url, "application/json", bytes.NewReader(raw))
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return nil, nil, fmt.Errorf("%s: %d %s", url, resp.StatusCode, b)
		}
		var pr struct {
			Proof        []*big.Int `json:"proof"`
			PublicSignal []*big.Int `json:"publicSignal"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
			return nil, nil, err
		}
		if len(pr.Proof) != 8 || len(pr.PublicSignal) != nSignals {
			return nil, nil, fmt.Errorf("%s: sizes proof=%d signal=%d", url, len(pr.Proof), len(pr.PublicSignal))
		}
		return pr.Proof, pr.PublicSignal, nil
	}

	// prove builds the main and USDr proofs for one transfer of tpTransferAmt
	// from slot 0 to slot 1, paying usdrFeeAmt to slot 5, against the given state.
	prove := func(gr *tpGroup, blockNum *big.Int, bals, usdrBals []enygma.IEnygmaPoint, keys []*big.Int) (*tpProved, error) {
		start := time.Now()
		secrets := make([]*big.Int, nBanks)
		copy(secrets, baseSecrets)
		s0, _ := poseidon.Hash([]*big.Int{gr.prevR, gr.sk})
		secrets[0] = s0.Mod(s0, curveP)
		nullifier, _ := poseidon.Hash([]*big.Int{secrets[0], blockNum})
		txVals := []*big.Int{negMod(big.NewInt(tpTransferAmt)), big.NewInt(tpTransferAmt), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)}
		txCommit, txRand := genCommitmentAndRandom(0, big.NewInt(tpTransferAmt), txVals, nullifier, secrets)
		mainReq := map[string]interface{}{
			"fingerprint_shared_secrets":   fp2Strs(fingerPrintGen(secrets, 0)),
			"public_keys":                  keyStrs(keys, gr),
			"previous_commits":             commitStrs(bals, gr),
			"tx_commits":                   pointStrs(txCommit),
			"block_number":                 blockNum.String(),
			"anonymity_set":                strs(kIdx),
			"message_tags":                 strs(tagMessageGen(0, secrets, nullifier)),
			"nullifier":                    nullifier.String(),
			"sender_id":                    "0",
			"shared_secrets":               strs(secrets),
			"secret_key":                   gr.sk.String(),
			"previous_sender_balance":      strconv.FormatInt(gr.prevV, 10),
			"previous_sender_random_value": gr.prevR.String(),
			"tx_values":                    strs(txVals),
			"tx_random_values":             strs(txRand),
			"sender_tx_value":              strconv.Itoa(tpTransferAmt),
			"domain_id":                    domain.String(),
		}

		usdrSecrets := make([]*big.Int, nBanks)
		copy(usdrSecrets, baseSecrets)
		u0, _ := poseidon.Hash([]*big.Int{gr.usdrPrevR, gr.sk})
		usdrSecrets[0] = u0.Mod(u0, curveP)
		usdrNullifier, _ := poseidon.Hash([]*big.Int{usdrSecrets[0], blockNum})
		usdrVals := []*big.Int{negMod(big.NewInt(usdrFeeAmt)), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(usdrFeeAmt)}
		usdrCommit, usdrRand := genCommitmentAndRandomUsdr(0, big.NewInt(usdrFeeAmt), usdrVals, usdrNullifier, usdrSecrets)
		usdrReq := map[string]interface{}{
			"fingerprint_shared_secrets":   fp2Strs(fingerPrintGen(usdrSecrets, 0)),
			"public_keys":                  keyStrs(keys, gr),
			"previous_commits":             commitStrs(usdrBals, gr),
			"tx_commits":                   pointStrs(usdrCommit),
			"block_number":                 blockNum.String(),
			"anonymity_set":                strs(kIdx),
			"message_tags":                 strs(tagMessageGenUsdr(0, usdrSecrets, usdrNullifier)),
			"nullifier":                    usdrNullifier.String(),
			"sender_id":                    "0",
			"shared_secrets":               strs(usdrSecrets),
			"secret_key":                   gr.sk.String(),
			"previous_sender_balance":      strconv.FormatInt(gr.usdrPrevV, 10),
			"previous_sender_random_value": gr.usdrPrevR.String(),
			"tx_values":                    strs(usdrVals),
			"tx_random_values":             strs(usdrRand),
			"sender_tx_value":              strconv.Itoa(usdrFeeAmt),
			"domain_id":                    domain.String(),
			"fee_recipient_key":            keys[gr.ids[5]].String(),
		}

		// A client proves both legs at once.
		var mainProof, mainSig, usdrProofV, usdrSig []*big.Int
		var mainErr, usdrErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); mainProof, mainSig, mainErr = postProof(gnarkURL, mainReq, 81) }()
		go func() { defer wg.Done(); usdrProofV, usdrSig, usdrErr = postProof(gnarkUsdrURL, usdrReq, 83) }()
		wg.Wait()
		if mainErr != nil {
			return nil, mainErr
		}
		if usdrErr != nil {
			return nil, usdrErr
		}
		const txOff = 54
		p := &tpProved{txRand: txRand, usdrTxRand: usdrRand, proveTime: time.Since(start)}
		for i := 0; i < nBanks; i++ {
			p.deltas = append(p.deltas, enygma.IEnygmaPoint{C1: mainSig[txOff+2*i], C2: mainSig[txOff+2*i+1]})
			p.usdrDeltas = append(p.usdrDeltas, enygma.IEnygmaPoint{C1: usdrSig[txOff+2*i], C2: usdrSig[txOff+2*i+1]})
		}
		copy(p.proof.Proof[:], mainProof)
		copy(p.proof.PublicSignal[:], mainSig)
		copy(p.usdrProof.Proof[:], usdrProofV)
		copy(p.usdrProof.PublicSignal[:], usdrSig)
		return p, nil
	}
	readState := func() (*big.Int, []enygma.IEnygmaPoint, []enygma.IEnygmaPoint, []*big.Int) {
		opts := &bind.CallOpts{Context: ctx}
		bn, err := instance.GetBlckHash(opts)
		if err != nil {
			t.Fatal(err)
		}
		pv, err := instance.GetPublicValues(opts, big.NewInt(int64(nTotal+1)))
		if err != nil {
			t.Fatal(err)
		}
		uv, err := instance.GetUsdrPublicValues(opts, big.NewInt(int64(nTotal+1)))
		if err != nil {
			t.Fatal(err)
		}
		return bn, pv.Balances, uv.Balances, pv.Keys
	}
	transferData := func(gr *tpGroup, p *tpProved) []byte {
		ids := make([]*big.Int, nBanks)
		for i := range ids {
			ids[i] = big.NewInt(gr.ids[i])
		}
		return pack("transfer", p.deltas, p.proof, p.usdrDeltas, p.usdrProof, ids, "")
	}
	applySuccess := func(gr *tpGroup, p *tpProved) {
		gr.prevR = new(big.Int).Mod(new(big.Int).Add(gr.prevR, p.txRand[0]), curveP)
		gr.prevV -= tpTransferAmt
		gr.usdrPrevR = new(big.Int).Mod(new(big.Int).Add(gr.usdrPrevR, p.usdrTxRand[0]), curveP)
		gr.usdrPrevV -= usdrFeeAmt
	}
	// gasFor estimates the transfer and adds 30%; it returns the revert reason
	// instead when the transfer would already fail against the current state.
	gasFor := func(gr *tpGroup, data []byte) (uint64, string) {
		msg := ethereum.CallMsg{From: gr.addrs[5], To: &enygmaAddr, Data: data}
		g, err := client.EstimateGas(ctx, msg)
		if err != nil {
			return 0, tpRevertReason(ctx, client, enygmaABI, msg, nil)
		}
		return g * 13 / 10, ""
	}

	// ── Block watcher: records when each block first became visible. ───────
	type seen struct {
		num uint64
		at  time.Time
		txs int
		gas uint64
	}
	var wmu sync.Mutex
	var blocks []seen
	stopWatch := make(chan struct{})
	watchDone := make(chan struct{})
	head, _ := client.BlockNumber(ctx)
	go func() {
		defer close(watchDone)
		next := head + 1
		for {
			select {
			case <-stopWatch:
				return
			case <-time.After(20 * time.Millisecond):
			}
			for {
				var b *struct {
					GasUsed      hexutil.Uint64 `json:"gasUsed"`
					Transactions []common.Hash  `json:"transactions"`
				}
				if err := client.Client().Call(&b, "eth_getBlockByNumber", hexutil.EncodeUint64(next), false); err != nil || b == nil {
					break
				}
				wmu.Lock()
				blocks = append(blocks, seen{next, time.Now(), len(b.Transactions), uint64(b.GasUsed)})
				wmu.Unlock()
				next++
			}
		}
	}()
	defer func() { close(stopWatch); <-watchDone }()
	blockSeen := func(n uint64) time.Time {
		wmu.Lock()
		defer wmu.Unlock()
		for _, b := range blocks {
			if b.num == n {
				return b.at
			}
		}
		return time.Time{}
	}

	if mode == "burst" {
		rounds := tpEnv("PAY_ROUNDS", 3)
		var allProve []time.Duration
		var gasAll []uint64
		for round := 1; round <= rounds; round++ {
			bn, bals, usdrBals, keys := readState()
			proved := make([]*tpProved, nGroups)
			jobs := make(chan int)
			var wg sync.WaitGroup
			var perr error
			var pmu sync.Mutex
			proveStart := time.Now()
			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for g := range jobs {
						p, err := prove(groups[g], bn, bals, usdrBals, keys)
						pmu.Lock()
						if err != nil && perr == nil {
							perr = err
						}
						proved[g] = p
						pmu.Unlock()
					}
				}()
			}
			for g := range groups {
				jobs <- g
			}
			close(jobs)
			wg.Wait()
			if perr != nil {
				t.Fatalf("round %d: prove: %v", round, perr)
			}
			proveWall := time.Since(proveStart)
			for _, p := range proved {
				allProve = append(allProve, p.proveTime)
			}

			txs := make([]*ethtypes.Transaction, nGroups)
			for g, gr := range groups {
				data := transferData(gr, proved[g])
				gas, reason := gasFor(gr, data)
				if reason != "" {
					t.Fatalf("round %d group %d: transfer would revert before sending: %s", round, g, reason)
				}
				txs[g] = sign(gr.keys[5], gr.nonce, enygmaAddr, nil, gas, data)
				gr.nonce++
			}
			sendStart := time.Now()
			for _, tx := range txs {
				if err := client.SendTransaction(ctx, tx); err != nil {
					t.Fatalf("round %d: send: %v", round, err)
				}
			}
			rcpts := waitAll("transfer", txs)
			var last time.Time
			okN := 0
			blockSet := map[uint64]int{}
			reasons := map[string]int{}
			var gasMin, gasMax, gasSum uint64
			gasMin = ^uint64(0)
			for g, r := range rcpts {
				blockSet[r.BlockNumber.Uint64()]++
				if seenAt := blockSeen(r.BlockNumber.Uint64()); seenAt.After(last) {
					last = seenAt
				}
				if r.Status == 1 {
					okN++
					applySuccess(groups[g], proved[g])
					gasSum += r.GasUsed
					gasAll = append(gasAll, r.GasUsed)
					gasMin = min(gasMin, r.GasUsed)
					gasMax = max(gasMax, r.GasUsed)
				} else {
					reasons[tpRevertReason(ctx, client, enygmaABI, ethereum.CallMsg{From: groups[g].addrs[5], To: &enygmaAddr, Data: txs[g].Data(), Gas: txs[g].Gas()}, new(big.Int).Sub(r.BlockNumber, big.NewInt(1)))]++
				}
			}
			if last.IsZero() {
				last = time.Now()
			}
			avg := uint64(0)
			if okN > 0 {
				avg = gasSum / uint64(okN)
			}
			t.Logf("round %d: proved %d transfers (%d proofs) in %v → %.2f transfers/s proving | %d/%d mined OK in %d block(s), %v after send | gas avg %d min %d max %d | failures %v",
				round, nGroups, 2*nGroups, proveWall.Round(10*time.Millisecond), float64(nGroups)/proveWall.Seconds(),
				okN, nGroups, len(blockSet), last.Sub(sendStart).Round(10*time.Millisecond), avg, gasMin, gasMax, reasons)
		}
		sort.Slice(gasAll, func(a, b int) bool { return gasAll[a] < gasAll[b] })
		if len(gasAll) > 0 {
			t.Logf("SUMMARY burst: %d banks, prove p50 %v (both legs, %d concurrent), gas p50 %d (%.1f transfers per 500M block)",
				nTotal, tpPercentile(allProve, 50).Round(10*time.Millisecond), workers,
				gasAll[len(gasAll)/2], 500e6/float64(gasAll[len(gasAll)/2]))
		}
		return
	}

	// ── loop: each group runs its own closed loop. ────────────────────────
	duration := time.Duration(tpEnv("PAY_DURATION", 120)) * time.Second
	type result struct {
		ok       bool
		reason   string
		latency  time.Duration
		prove    time.Duration
		gas      uint64
		finished time.Time
	}
	var rmu sync.Mutex
	var results []result
	start := time.Now()
	deadline := start.Add(duration)
	var lwg sync.WaitGroup
	sem := make(chan struct{}, workers) // bounds concurrent proving on the one server
	for g := range groups {
		lwg.Add(1)
		go func(gr *tpGroup) {
			defer lwg.Done()
			for time.Now().Before(deadline) {
				begin := time.Now()
				bn, bals, usdrBals, keys := readState()
				sem <- struct{}{}
				p, err := prove(gr, bn, bals, usdrBals, keys)
				<-sem
				if err != nil {
					t.Errorf("prove: %v", err)
					return
				}
				data := transferData(gr, p)
				gas, reason := gasFor(gr, data)
				if reason != "" {
					rmu.Lock()
					results = append(results, result{reason: "before send: " + reason, prove: p.proveTime, finished: time.Now()})
					rmu.Unlock()
					continue
				}
				tx := sign(gr.keys[5], gr.nonce, enygmaAddr, nil, gas, data)
				gr.nonce++
				if err := client.SendTransaction(ctx, tx); err != nil {
					t.Errorf("send: %v", err)
					return
				}
				r := waitAll("transfer", []*ethtypes.Transaction{tx})[0]
				res := result{prove: p.proveTime, latency: time.Since(begin), gas: r.GasUsed, finished: time.Now()}
				if r.Status == 1 {
					res.ok = true
					applySuccess(gr, p)
				} else {
					res.reason = "on chain: " + tpRevertReason(ctx, client, enygmaABI, ethereum.CallMsg{From: gr.addrs[5], To: &enygmaAddr, Data: data, Gas: tx.Gas()}, new(big.Int).Sub(r.BlockNumber, big.NewInt(1)))
				}
				rmu.Lock()
				results = append(results, res)
				rmu.Unlock()
			}
		}(groups[g])
	}
	lwg.Wait()
	elapsed := time.Since(start)
	okN := 0
	var lat, pr []time.Duration
	var gasOK []uint64
	reasons := map[string]int{}
	for _, r := range results {
		pr = append(pr, r.prove)
		if r.ok {
			okN++
			lat = append(lat, r.latency)
			gasOK = append(gasOK, r.gas)
		} else {
			reasons[r.reason]++
		}
	}
	sort.Slice(gasOK, func(a, b int) bool { return gasOK[a] < gasOK[b] })
	gasLine := ""
	if len(gasOK) > 0 {
		gasLine = fmt.Sprintf("gas p50 %d max %d", gasOK[len(gasOK)/2], gasOK[len(gasOK)-1])
	}
	t.Logf("SUMMARY loop: %d groups (%d banks), epoch %d blocks, %v: %d transfers confirmed → %.2f transfers/s | attempts %d, failed %d %v | latency p50 %v p95 %v | prove (both legs) p50 %v p95 %v | %s",
		nGroups, nTotal, epoch, elapsed.Round(time.Second), okN, float64(okN)/elapsed.Seconds(),
		len(results), len(results)-okN, reasons,
		tpPercentile(lat, 50).Round(10*time.Millisecond), tpPercentile(lat, 95).Round(10*time.Millisecond),
		tpPercentile(pr, 50).Round(10*time.Millisecond), tpPercentile(pr, 95).Round(10*time.Millisecond), gasLine)
}
