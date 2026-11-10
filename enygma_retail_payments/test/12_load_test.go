package tests

// Steady-state load test for retail payment() — opt-in, set RP_LOAD.
//
//   RP_LOAD=direct|relayer|both CC=/usr/bin/clang go test -run TestRetailLoad -v -timeout 7200s
//
// Unlike 11_throughput_bench_test.go (one sender, one hand-mined block), this
// runs against a node that produces blocks on its own clock (e.g. geth --dev
// --dev.period 1) and offers payments at a fixed rate:
//   - RP_LOAD_USERS distinct users (own ECDSA account + spend/view keys);
//     notes are owned by random users and pay random other users random amounts.
//   - Proofs are generated up front (one prover server would otherwise cap the
//     offered rate at ~14/s; see TestRetailThroughput for prover numbers).
//   - "direct": each payment is a signed tx from the note owner's account, sent
//     open-loop at each rate in RP_LOAD_RATES (payments/s) for RP_LOAD_STEP_SEC.
//   - "relayer": each payment is POSTed to the relayer on :8090 at each rate in
//     RP_LOAD_RELAY_RATES, one simulated client IP per user (X-Forwarded-For).
// Reported per step: confirmed payments/s, inclusion latency, reverts, and
// payments per block.

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	mrand "math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type loadUser struct {
	key   *ecdsa.PrivateKey
	addr  common.Address
	spend *rpcore.SpendKeyPair
	view  *rpcore.ViewKeyPair
	nonce uint64
}

type loadPayment struct {
	owner int
	res   *rpcore.PaymentResult
}

type blockLite struct {
	Number       hexutil.Uint64 `json:"number"`
	Timestamp    hexutil.Uint64 `json:"timestamp"`
	GasUsed      hexutil.Uint64 `json:"gasUsed"`
	Transactions []common.Hash  `json:"transactions"`
}

type seenBlock struct {
	number uint64
	seen   time.Time
	txs    int
	gas    uint64
}

// blockWatcher polls the head and records when each block (and each tx in it)
// first became visible to a client.
type blockWatcher struct {
	mu     sync.Mutex
	txSeen map[common.Hash]time.Time
	txBlk  map[common.Hash]uint64
	blocks []seenBlock
	stop   chan struct{}
	done   chan struct{}
}

func startBlockWatcher(t *testing.T, client *ethclient.Client) *blockWatcher {
	w := &blockWatcher{
		txSeen: map[common.Hash]time.Time{},
		txBlk:  map[common.Hash]uint64{},
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	head, err := client.BlockNumber(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(w.done)
		next := head + 1
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-w.stop:
				return
			case <-tick.C:
			}
			for {
				var b *blockLite
				err := client.Client().Call(&b, "eth_getBlockByNumber", hexutil.EncodeUint64(next), false)
				if err != nil || b == nil {
					break
				}
				now := time.Now()
				w.mu.Lock()
				for _, h := range b.Transactions {
					w.txSeen[h] = now
					w.txBlk[h] = next
				}
				w.blocks = append(w.blocks, seenBlock{next, now, len(b.Transactions), uint64(b.GasUsed)})
				w.mu.Unlock()
				next++
			}
		}
	}()
	return w
}

func (w *blockWatcher) close() { close(w.stop); <-w.done }

func (w *blockWatcher) seen(h common.Hash) (time.Time, uint64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	ts, ok := w.txSeen[h]
	return ts, w.txBlk[h], ok
}

func (w *blockWatcher) blocksBetween(from, to time.Time) []seenBlock {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []seenBlock
	for _, b := range w.blocks {
		if !b.seen.Before(from) && !b.seen.After(to) {
			out = append(out, b)
		}
	}
	return out
}

func parseRates(env, def string) []int {
	s := os.Getenv(env)
	if s == "" {
		s = def
	}
	var out []int
	for _, f := range strings.Split(s, ",") {
		if v, err := strconv.Atoi(strings.TrimSpace(f)); err == nil && v > 0 {
			out = append(out, v)
		}
	}
	return out
}

func percentile(d []time.Duration, p int) time.Duration {
	if len(d) == 0 {
		return 0
	}
	sort.Slice(d, func(a, b int) bool { return d[a] < d[b] })
	i := len(d) * p / 100
	if i >= len(d) {
		i = len(d) - 1
	}
	return d[i]
}

func toProofReceipt(t *testing.T, res *rpcore.PaymentResult) ProofReceipt {
	sp := proofStringsToOnchain(t, res.Proof)
	return ProofReceipt{
		Proof: SnarkProof{
			A: G1Point{X: sp.A.X, Y: sp.A.Y},
			B: G2Point{X: sp.B.X, Y: sp.B.Y},
			C: G1Point{X: sp.C.X, Y: sp.C.Y},
		},
		Statement:       res.ContractStatement(),
		NumberOfInputs:  big.NewInt(int64(res.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(res.NumberOfOutputs)),
	}
}

// waitIncluded blocks until every hash is seen by the watcher or timeout.
func waitIncluded(w *blockWatcher, hashes []common.Hash, timeout time.Duration) int {
	deadline := time.Now().Add(timeout)
	for {
		n := 0
		for _, h := range hashes {
			if _, _, ok := w.seen(h); ok {
				n++
			}
		}
		if n == len(hashes) || time.Now().After(deadline) {
			return n
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestRetailLoad(t *testing.T) {
	mode := os.Getenv("RP_LOAD")
	if mode == "" {
		t.Skip("set RP_LOAD=direct|relayer|both to run the load test")
	}
	doDirect := mode == "direct" || mode == "both"
	doRelay := mode == "relayer" || mode == "both"
	nUsers := envInt("RP_LOAD_USERS", 100)
	stepSec := envInt("RP_LOAD_STEP_SEC", 10)
	workers := envInt("RP_BENCH_WORKERS", 6)
	// payment() costs ~800k gas when the vault's next leaf index is even and
	// ~1.01M when it is odd (insertLeaves hashes more levels), so a 1M limit
	// reverts every payment once the index turns odd — and a reverted payment
	// never advances the index.
	payGas := uint64(envInt("RP_LOAD_GAS", 1_200_000))
	var directRates, relayRates []int
	if doDirect {
		directRates = parseRates("RP_LOAD_RATES", "100,200,300,400")
	}
	if doRelay {
		relayRates = parseRates("RP_LOAD_RELAY_RATES", "10,20,30")
	}
	if !chainAvailable() || !serverAvailable("localhost:8082") {
		t.Skip("needs a node on :8545 and gnark server on :8082")
	}
	if doRelay && !serverAvailable("localhost:8090") {
		t.Skip("relayer mode needs the relayer on :8090")
	}

	ctx := context.Background()
	client := mustDial(t)
	defer client.Close()
	chainID, err := client.ChainID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	signer := types.LatestSignerForChainID(chainID)
	var clientVersion string
	_ = client.Client().Call(&clientVersion, "web3_clientVersion")
	t.Logf("node: %s, users: %d, step: %ds, direct rates %v, relayer rates %v",
		clientVersion, nUsers, stepSec, directRates, relayRates)

	receipts := loadOnchainReceipts(t)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	vaultABI := loadOnchainABI(t, "Erc20CoinVault")
	erc20ABI := loadOnchainABI(t, "RaylsERC20")
	dvpABI := loadOnchainABI(t, "EnygmaDvp")

	aliceKey, err := crypto.HexToECDSA(hardhatPrivKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	aliceAddr := crypto.PubkeyToAddress(aliceKey.PublicKey)
	aliceNonce, err := client.PendingNonceAt(ctx, aliceAddr)
	if err != nil {
		t.Fatal(err)
	}

	// fees returns EIP-1559 caps with wide headroom: under sustained load
	// the base fee climbs 12.5% per block that is over half full.
	fees := func() (*big.Int, *big.Int) {
		h, err := client.HeaderByNumber(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		tip := big.NewInt(1_000_000_000)
		feeCap := new(big.Int).Mul(h.BaseFee, big.NewInt(100))
		feeCap.Add(feeCap, tip)
		if feeCap.Cmp(big.NewInt(50_000_000_000)) < 0 {
			feeCap = big.NewInt(50_000_000_000)
		}
		return tip, feeCap
	}
	signTx := func(key *ecdsa.PrivateKey, nonce uint64, to common.Address, value *big.Int, gas uint64, data []byte) *types.Transaction {
		tip, feeCap := fees()
		tx, err := types.SignNewTx(key, signer, &types.DynamicFeeTx{
			ChainID: chainID, Nonce: nonce, GasTipCap: tip, GasFeeCap: feeCap,
			Gas: gas, To: &to, Value: value, Data: data,
		})
		if err != nil {
			t.Fatal(err)
		}
		return tx
	}
	// sendAndWait sends txs in order and waits for every receipt to succeed.
	sendAndWait := func(label string, txs []*types.Transaction) {
		t.Helper()
		for _, tx := range txs {
			if err := client.SendTransaction(ctx, tx); err != nil {
				t.Fatalf("%s: send: %v", label, err)
			}
		}
		for _, tx := range txs {
			for {
				r, err := client.TransactionReceipt(ctx, tx.Hash())
				if err == nil {
					if r.Status != 1 {
						t.Fatalf("%s: tx %s reverted", label, tx.Hash())
					}
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
	pack := func(a abi.ABI, method string, args ...interface{}) []byte {
		data, err := a.Pack(method, args...)
		if err != nil {
			t.Fatalf("pack %s: %v", method, err)
		}
		return data
	}

	// ── Users ────────────────────────────────────────────────────────────────
	users := make([]*loadUser, nUsers)
	var fund []*types.Transaction
	for i := range users {
		k, _ := crypto.GenerateKey()
		sp, _ := rpcore.NewSpendKeyPair()
		vw, _ := rpcore.NewViewKeyPair()
		users[i] = &loadUser{key: k, addr: crypto.PubkeyToAddress(k.PublicKey), spend: sp, view: vw}
		eth := new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18))
		fund = append(fund, signTx(aliceKey, aliceNonce, users[i].addr, eth, 21000, nil))
		aliceNonce++
	}
	sendAndWait("fund users", fund)

	// ── Notes: the issuer (alice) deposits one note per payment, owned by a
	// random user and encrypted to that user's view key.
	nDirect, nRelay := 0, 0
	for _, r := range directRates {
		nDirect += r * stepSec
	}
	for _, r := range relayRates {
		nRelay += r * stepSec
	}
	total := nDirect + nRelay
	rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	tokenId := big.NewInt(0)
	owners := make([]int, total)
	amounts := make([]*big.Int, total)
	salts := make([]*big.Int, total)
	cmts := make([]*big.Int, total)
	supply := new(big.Int)
	for i := range owners {
		owners[i] = rng.Intn(nUsers)
		amounts[i] = big.NewInt(int64(100 + rng.Intn(901)))
		supply.Add(supply, amounts[i])
	}
	sendAndWait("mint+approve", []*types.Transaction{
		signTx(aliceKey, aliceNonce, erc20Addr, nil, 200_000, pack(erc20ABI, "mint", aliceAddr, supply)),
		signTx(aliceKey, aliceNonce+1, erc20Addr, nil, 200_000, pack(erc20ABI, "approve", vaultAddr, supply)),
	})
	aliceNonce += 2

	depStart := time.Now()
	const chunk = 1000
	for start := 0; start < total; start += chunk {
		end := min(start+chunk, total)
		var txs []*types.Transaction
		for i := start; i < end; i++ {
			u := users[owners[i]]
			ss, capsule, err := rpcore.Encapsulate(u.view.EncapsKey)
			if err != nil {
				t.Fatal(err)
			}
			saltB, _ := rpcore.DerivePaymentSalt(ss)
			encKey, _ := rpcore.DerivePaymentKey(ss)
			salts[i] = rpcore.SaltBToField(saltB)
			cmts[i], _ = rpcore.Erc20CommitmentV2(u.spend.PublicKey, salts[i], amounts[i], tokenId)
			payload, _ := rpcore.EncryptPayload(encKey, tokenId, amounts[i])
			data := pack(vaultABI, "depositV2",
				[]*big.Int{amounts[i], u.spend.PublicKey, salts[i], tokenId}, capsule, payload)
			txs = append(txs, signTx(aliceKey, aliceNonce, vaultAddr, nil, 1_000_000, data))
			aliceNonce++
		}
		sendAndWait("deposits", txs)
	}
	t.Logf("setup: %d notes deposited in %v", total, time.Since(depStart).Round(time.Second))

	// ── Merkle paths and proofs ─────────────────────────────────────────────
	mt := loadVaultMerkleTreeByTx(t, client, vaultAddr, 8)
	paths := make([]*rpcore.MerkleProof, total)
	for i := range paths {
		p, err := mt.GenerateProof(cmts[i])
		if err != nil {
			t.Fatalf("merkle proof %d: %v", i, err)
		}
		paths[i] = p
	}
	gnarkClient := rpcore.NewPaymentClient("")
	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	payments := make([]loadPayment, total)
	recips := make([]int, total)
	pays := make([]*big.Int, total)
	for i := range recips {
		recips[i] = (owners[i] + 1 + rng.Intn(nUsers-1)) % nUsers
		pays[i] = big.NewInt(1 + rng.Int63n(amounts[i].Int64()-1))
	}
	proveStart := time.Now()
	jobs := make(chan int)
	var wg sync.WaitGroup
	var failed sync.Once
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				o, r := users[owners[i]], users[recips[i]]
				change := new(big.Int).Sub(amounts[i], pays[i])
				res, err := gnarkClient.BoundPaymentProof(
					vaultAddrBig, big.NewInt(0),
					[]*big.Int{amounts[i]},
					[]rpcore.KeyPair{{PrivateKey: o.spend.PrivateKey, PublicKey: o.spend.PublicKey}},
					[]*big.Int{salts[i]},
					[]*big.Int{pays[i], change},
					[]*big.Int{r.spend.PublicKey, o.spend.PublicKey},
					[][]byte{r.view.EncapsKey, o.view.EncapsKey},
					8, []*rpcore.MerkleProof{paths[i]},
					[]*big.Int{big.NewInt(int64(paths[i].TreeNumber))}, tokenId,
				)
				if err != nil {
					failed.Do(func() { t.Errorf("proof %d: %v", i, err) })
					continue
				}
				payments[i] = loadPayment{owner: owners[i], res: res}
			}
		}()
	}
	for i := 0; i < total; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}
	pd := time.Since(proveStart)
	t.Logf("setup: %d proofs in %v (%.1f proofs/s, %d workers)", total, pd.Round(time.Second),
		float64(total)/pd.Seconds(), workers)

	watcher := startBlockWatcher(t, client)
	defer watcher.close()

	report := func(label string, rate, sent, ok int, lat []time.Duration, first, last time.Time, extra string) {
		blocks := watcher.blocksBetween(first, last)
		maxTx, sumTx := 0, 0
		var maxGas uint64
		for _, b := range blocks {
			sumTx += b.txs
			maxTx = max(maxTx, b.txs)
			maxGas = max(maxGas, b.gas)
		}
		var interval time.Duration
		if len(blocks) > 1 {
			interval = blocks[len(blocks)-1].seen.Sub(blocks[0].seen) / time.Duration(len(blocks)-1)
		}
		tps := 0.0
		if ok > 0 {
			tps = float64(ok) / last.Sub(first).Seconds()
		}
		t.Logf("%s offered %3d/s: sent %d, confirmed %d → %6.1f payments/s | latency p50 %v p95 %v max %v | %d blocks, avg interval %v, max %d tx (%.0f Mgas) per block%s",
			label, rate, sent, ok, tps,
			percentile(lat, 50).Round(10*time.Millisecond), percentile(lat, 95).Round(10*time.Millisecond),
			percentile(lat, 100).Round(10*time.Millisecond),
			len(blocks), interval.Round(10*time.Millisecond), maxTx, float64(maxGas)/1e6, extra)
	}

	next := 0
	// ── Direct: note owners send their own payment() txs open-loop. ─────────
	for _, rate := range directRates {
		n := rate * stepSec
		batch := payments[next : next+n]
		next += n
		txs := make([]*types.Transaction, n)
		for j, p := range batch {
			u := users[p.owner]
			data := pack(dvpABI, "payment", toProofReceipt(t, p.res), big.NewInt(0), p.res.CipherText, p.res.EncTxData)
			txs[j] = signTx(u.key, u.nonce, dvpAddr, nil, payGas, data)
			u.nonce++
		}
		sendTimes := make([]time.Time, n)
		sendErrs := 0
		var mu sync.Mutex
		var swg sync.WaitGroup
		sem := make(chan struct{}, 256)
		start := time.Now()
		for j := range txs {
			time.Sleep(time.Until(start.Add(time.Duration(j) * time.Second / time.Duration(rate))))
			sem <- struct{}{}
			swg.Add(1)
			go func(j int) {
				defer swg.Done()
				defer func() { <-sem }()
				ts := time.Now()
				err := client.SendTransaction(ctx, txs[j])
				mu.Lock()
				sendTimes[j] = ts
				if err != nil {
					sendErrs++
					if sendErrs == 1 {
						t.Logf("send error: %v", err)
					}
				}
				mu.Unlock()
			}(j)
		}
		swg.Wait()
		hashes := make([]common.Hash, n)
		for j, tx := range txs {
			hashes[j] = tx.Hash()
		}
		waitIncluded(watcher, hashes, 120*time.Second)

		var lat []time.Duration
		var last time.Time
		blockSet := map[uint64]bool{}
		ok := 0
		for j, h := range hashes {
			if ts, bn, seen := watcher.seen(h); seen {
				ok++
				lat = append(lat, ts.Sub(sendTimes[j]))
				if ts.After(last) {
					last = ts
				}
				blockSet[bn] = true
			}
		}
		// Reverts: check each block's receipts for our txs.
		ours := map[common.Hash]bool{}
		for _, h := range hashes {
			ours[h] = true
		}
		reverted := 0
		var gasSum uint64
		for bn := range blockSet {
			var rs []*types.Receipt
			if err := client.Client().Call(&rs, "eth_getBlockReceipts", hexutil.EncodeUint64(bn)); err != nil {
				t.Fatalf("eth_getBlockReceipts: %v", err)
			}
			for _, r := range rs {
				if ours[r.TxHash] {
					gasSum += r.GasUsed
					if r.Status != 1 {
						reverted++
					}
				}
			}
		}
		extra := fmt.Sprintf(" | reverted %d, send errors %d, avg gas %d", reverted, sendErrs, gasSum/uint64(max(ok, 1)))
		report("direct ", rate, n-sendErrs, ok-reverted, lat, start, last, extra)
	}

	// ── Relayer: every payment goes through POST /relay/payment. ────────────
	httpClient := &http.Client{
		Timeout:   180 * time.Second,
		Transport: &http.Transport{MaxIdleConnsPerHost: 1024, MaxConnsPerHost: 0},
	}
	for _, rate := range relayRates {
		n := rate * stepSec
		batch := payments[next : next+n]
		next += n
		bodies := make([][]byte, n)
		for j, p := range batch {
			b, err := json.Marshal(buildRelayRequest(t, p.res, "0"))
			if err != nil {
				t.Fatal(err)
			}
			bodies[j] = b
		}
		var mu sync.Mutex
		var lat []time.Duration
		codes := map[int]int{}
		var last time.Time
		ok := 0
		var swg sync.WaitGroup
		start := time.Now()
		for j := range bodies {
			time.Sleep(time.Until(start.Add(time.Duration(j) * time.Second / time.Duration(rate))))
			swg.Add(1)
			go func(j int) {
				defer swg.Done()
				owner := batch[j].owner
				req, _ := http.NewRequest(http.MethodPost, relayerURL+"/relay/payment", bytes.NewReader(bodies[j]))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+relayerAPIKey)
				req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.%d.%d.%d", owner>>16&255, owner>>8&255, owner&255+1))
				ts := time.Now()
				resp, err := httpClient.Do(req)
				code := 0
				if err == nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					code = resp.StatusCode
				}
				done := time.Now()
				mu.Lock()
				codes[code]++
				if code == http.StatusOK {
					ok++
					lat = append(lat, done.Sub(ts))
					if done.After(last) {
						last = done
					}
				}
				mu.Unlock()
			}(j)
		}
		swg.Wait()
		extra := fmt.Sprintf(" | HTTP status counts %v (latency = request until relayer returns the mined receipt)", codes)
		report("relayer", rate, n, ok, lat, start, last, extra)
	}
}
