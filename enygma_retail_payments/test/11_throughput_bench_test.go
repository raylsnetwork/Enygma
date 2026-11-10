package tests

// Throughput benchmark for retail payment() — opt-in, set RP_BENCH_N.
//
//   RP_BENCH_N=40 RP_BENCH_WORKERS=4 CC=/usr/bin/clang go test -run TestRetailThroughput -v -timeout 1200s
//
// Measures, on a fresh node with < 255 - 3N leaves in the vault tree:
//   1. proof latency with one request at a time (prover-bound TPS of one server)
//   2. proof rate with RP_BENCH_WORKERS concurrent requests to the same server
//   3. gas per payment, with all N payments mined in a single block
//      (automine off) to confirm concurrent payments against shared roots succeed.

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func TestRetailThroughput(t *testing.T) {
	n := envInt("RP_BENCH_N", 0)
	if n == 0 {
		t.Skip("set RP_BENCH_N to run the throughput benchmark")
	}
	workers := envInt("RP_BENCH_WORKERS", 4)
	if !chainAvailable() || !serverAvailable("localhost:8082") {
		t.Skip("needs Hardhat on :8545 and gnark server on :8082")
	}
	ctx := context.Background()
	client := mustDial(t)
	defer client.Close()

	receipts := loadOnchainReceipts(t)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	vault := bind.NewBoundContract(vaultAddr, loadOnchainABI(t, "Erc20CoinVault"), client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, loadOnchainABI(t, "RaylsERC20"), client, client, client)
	dvp := bind.NewBoundContract(dvpAddr, loadOnchainABI(t, "EnygmaDvp"), client, client, client)
	alice := hardhatAuth(t, client)

	gnarkClient := rpcore.NewPaymentClient("")
	const depth = 8
	tokenId := big.NewInt(0)
	depositAmt, payAmt, changeAmt := big.NewInt(40), big.NewInt(30), big.NewInt(10)

	aliceSpend, _ := rpcore.NewSpendKeyPair()
	aliceView, _ := rpcore.NewViewKeyPair()
	bobSpend, _ := rpcore.NewSpendKeyPair()
	bobView, _ := rpcore.NewViewKeyPair()

	wait := func(tx *types.Transaction, err error) *types.Receipt {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		r, err := bind.WaitMined(ctx, client, tx)
		if err != nil || r.Status != 1 {
			t.Fatalf("tx %s failed: %v", tx.Hash(), err)
		}
		return r
	}
	total := new(big.Int).Mul(depositAmt, big.NewInt(int64(n)))
	wait(erc20.Transact(alice, "mint", alice.From, total))
	wait(erc20.Transact(alice, "approve", vaultAddr, total))

	// ── Deposits: N independent notes so the payments do not depend on each other.
	salts := make([]*big.Int, n)
	cmts := make([]*big.Int, n)
	var depositGas uint64
	for i := 0; i < n; i++ {
		ss, capsule, err := rpcore.Encapsulate(aliceView.EncapsKey)
		if err != nil {
			t.Fatal(err)
		}
		saltB, _ := rpcore.DerivePaymentSalt(ss)
		encKey, _ := rpcore.DerivePaymentKey(ss)
		salts[i] = rpcore.SaltBToField(saltB)
		cmts[i], _ = rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, salts[i], depositAmt, tokenId)
		payload, _ := rpcore.EncryptPayload(encKey, tokenId, depositAmt)
		r := wait(vault.Transact(alice, "depositV2",
			[]*big.Int{depositAmt, aliceSpend.PublicKey, salts[i], tokenId}, capsule, payload))
		depositGas += r.GasUsed
	}
	mt := loadVaultMerkleTree(t, client, vaultAddr, depth)
	paths := make([]*rpcore.MerkleProof, n)
	for i := range paths {
		p, err := mt.GenerateProof(cmts[i])
		if err != nil {
			t.Fatal(err)
		}
		paths[i] = p
	}
	t.Logf("deposits: %d, avg gas %d", n, depositGas/uint64(n))

	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	prove := func(i int) (*rpcore.PaymentResult, time.Duration) {
		start := time.Now()
		res, err := gnarkClient.BoundPaymentProof(
			vaultAddrBig, big.NewInt(0),
			[]*big.Int{depositAmt},
			[]rpcore.KeyPair{{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey}},
			[]*big.Int{salts[i]},
			[]*big.Int{payAmt, changeAmt},
			[]*big.Int{bobSpend.PublicKey, aliceSpend.PublicKey},
			[][]byte{bobView.EncapsKey, aliceView.EncapsKey},
			depth, []*rpcore.MerkleProof{paths[i]}, []*big.Int{big.NewInt(0)}, tokenId,
		)
		if err != nil {
			t.Errorf("proof %d: %v", i, err)
		}
		return res, time.Since(start)
	}

	// ── 1. Sequential proving (one client, one request in flight).
	results := make([]*rpcore.PaymentResult, n)
	seqN := n / 2
	lat := make([]time.Duration, 0, seqN)
	seqStart := time.Now()
	for i := 0; i < seqN; i++ {
		res, d := prove(i)
		results[i] = res
		lat = append(lat, d)
	}
	seqWall := time.Since(seqStart)
	sort.Slice(lat, func(a, b int) bool { return lat[a] < lat[b] })
	t.Logf("sequential: %d proofs in %v → %.2f proofs/s (p50 %v, p95 %v, max %v)",
		seqN, seqWall.Round(time.Millisecond), float64(seqN)/seqWall.Seconds(),
		lat[len(lat)/2].Round(time.Millisecond), lat[len(lat)*95/100].Round(time.Millisecond),
		lat[len(lat)-1].Round(time.Millisecond))

	// ── 2. Concurrent proving against the same server.
	jobs := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	conLat := []time.Duration{}
	conStart := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				res, d := prove(i)
				mu.Lock()
				results[i] = res
				conLat = append(conLat, d)
				mu.Unlock()
			}
		}()
	}
	for i := seqN; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	conWall := time.Since(conStart)
	sort.Slice(conLat, func(a, b int) bool { return conLat[a] < conLat[b] })
	t.Logf("concurrent (%d workers): %d proofs in %v → %.2f proofs/s (p50 %v, max %v)",
		workers, len(conLat), conWall.Round(time.Millisecond), float64(len(conLat))/conWall.Seconds(),
		conLat[len(conLat)/2].Round(time.Millisecond), conLat[len(conLat)-1].Round(time.Millisecond))
	if t.Failed() {
		t.FailNow()
	}

	// ── 3. Submit all N payments into one block.
	if err := client.Client().Call(nil, "evm_setAutomine", false); err != nil {
		t.Fatalf("evm_setAutomine: %v", err)
	}
	defer client.Client().Call(nil, "evm_setAutomine", true)

	nonce, err := client.PendingNonceAt(ctx, alice.From)
	if err != nil {
		t.Fatal(err)
	}
	txs := make([]*types.Transaction, n)
	for i, res := range results {
		sp := proofStringsToOnchain(t, res.Proof)
		receipt := ProofReceipt{
			Proof: SnarkProof{
				A: G1Point{X: sp.A.X, Y: sp.A.Y},
				B: G2Point{X: sp.B.X, Y: sp.B.Y},
				C: G1Point{X: sp.C.X, Y: sp.C.Y},
			},
			Statement:       res.ContractStatement(),
			NumberOfInputs:  big.NewInt(int64(res.NumberOfInputs)),
			NumberOfOutputs: big.NewInt(int64(res.NumberOfOutputs)),
		}
		opts := *alice
		opts.Nonce = new(big.Int).SetUint64(nonce + uint64(i))
		opts.GasLimit = 2_000_000
		tx, err := dvp.Transact(&opts, "payment", receipt, big.NewInt(0), res.CipherText, res.EncTxData)
		if err != nil {
			t.Fatalf("send payment %d: %v", i, err)
		}
		txs[i] = tx
	}
	var mineRes interface{}
	if err := client.Client().Call(&mineRes, "evm_mine"); err != nil {
		t.Fatalf("evm_mine: %v", err)
	}

	var gasSum, gasMin, gasMax uint64
	gasMin = ^uint64(0)
	blocks := map[uint64]bool{}
	for i, tx := range txs {
		r, err := client.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			t.Fatalf("receipt %d: %v", i, err)
		}
		if r.Status != 1 {
			t.Fatalf("payment %d reverted", i)
		}
		blocks[r.BlockNumber.Uint64()] = true
		gasSum += r.GasUsed
		if r.GasUsed < gasMin {
			gasMin = r.GasUsed
		}
		if r.GasUsed > gasMax {
			gasMax = r.GasUsed
		}
	}
	avg := gasSum / uint64(n)
	t.Logf("on-chain: %d payments mined in %d block(s), all succeeded; gas avg %d (min %d, max %d)",
		n, len(blocks), avg, gasMin, gasMax)
	fmt.Printf("BENCH n=%d seq_tps=%.3f con_tps=%.3f workers=%d gas_avg=%d gas_min=%d gas_max=%d deposit_gas=%d\n",
		n, float64(seqN)/seqWall.Seconds(), float64(len(conLat))/conWall.Seconds(), workers,
		avg, gasMin, gasMax, depositGas/uint64(n))
}

// TestRetailScanRate measures how many Payment events a recipient can
// trial-decrypt per second (ML-KEM decapsulation + HKDF + AES-GCM), i.e. the
// chain TPS a single wallet can keep up with. Opt-in via RP_BENCH_N.
func TestRetailScanRate(t *testing.T) {
	if envInt("RP_BENCH_N", 0) == 0 {
		t.Skip("set RP_BENCH_N to run the throughput benchmark")
	}
	const m = 2000
	other, _ := rpcore.NewViewKeyPair()
	me, _ := rpcore.NewViewKeyPair()
	mySpend, _ := rpcore.NewSpendKeyPair()
	events := make([]dvpcore.OnChainErc20Event, m)
	for i := range events {
		ss, capsule, err := rpcore.Encapsulate(other.EncapsKey)
		if err != nil {
			t.Fatal(err)
		}
		key, _ := rpcore.DerivePaymentKey(ss)
		payload, _ := rpcore.EncryptPayload(key, big.NewInt(0), big.NewInt(1))
		events[i] = dvpcore.OnChainErc20Event{Commitment: big.NewInt(int64(i)), CipherText: capsule, EncTxData: payload}
	}
	start := time.Now()
	notes, err := dvpcore.ScanForErc20Notes(me.DecapsKey, mySpend.PublicKey, events)
	d := time.Since(start)
	if err != nil || len(notes) != 0 {
		t.Fatalf("scan: %v (%d notes)", err, len(notes))
	}
	t.Logf("scan: %d foreign events in %v → %.0f events/s per core (%v/event)",
		m, d.Round(time.Millisecond), float64(m)/d.Seconds(), (d / m).Round(time.Microsecond))
}

// loadVaultMerkleTreeByTx rebuilds the vault's local Merkle tree, inserting
// each transaction's Commitment logs as one batch so tree rollover lands
// exactly where Merkle.sol puts it (insertLeaves rolls per call, not per leaf).
func loadVaultMerkleTreeByTx(t *testing.T, client *ethclient.Client, vaultAddr common.Address, depth int) *rpcore.MerkleTree {
	t.Helper()
	sig := crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	logs, err := client.FilterLogs(context.Background(), ethereum.FilterQuery{
		Addresses: []common.Address{vaultAddr},
		Topics:    [][]common.Hash{{sig}},
	})
	if err != nil {
		t.Fatalf("FilterLogs: %v", err)
	}
	mt := rpcore.NewMerkleTree(depth)
	var batch []*big.Int
	var cur common.Hash
	for _, l := range logs {
		if l.TxHash != cur && len(batch) > 0 {
			mt.InsertLeaves(batch)
			batch = nil
		}
		cur = l.TxHash
		batch = append(batch, l.Topics[2].Big())
	}
	if len(batch) > 0 {
		mt.InsertLeaves(batch)
	}
	return mt
}

// timedMine mines one block and returns how long the node took to produce it.
func timedMine(t *testing.T, client *ethclient.Client) time.Duration {
	t.Helper()
	start := time.Now()
	if err := client.Client().Call(nil, "evm_mine"); err != nil {
		t.Fatalf("evm_mine: %v", err)
	}
	return time.Since(start)
}

// TestRetailChainTPS measures how many payment() calls the node itself can
// execute per second: per round it deposits N notes, proves N payments, sends
// all N with automine off, and times the single block that executes them
// against an empty block. Opt-in via RP_CHAIN_N (payments per block).
//
//	RP_CHAIN_N=100 RP_CHAIN_ROUNDS=3 CC=/usr/bin/clang go test -run TestRetailChainTPS -v -timeout 3600s
func TestRetailChainTPS(t *testing.T) {
	n := envInt("RP_CHAIN_N", 0)
	if n == 0 {
		t.Skip("set RP_CHAIN_N to run the on-chain TPS benchmark")
	}
	rounds := envInt("RP_CHAIN_ROUNDS", 3)
	workers := envInt("RP_BENCH_WORKERS", 4)
	if !chainAvailable() || !serverAvailable("localhost:8082") {
		t.Skip("needs a node on :8545 and gnark server on :8082")
	}
	ctx := context.Background()
	client := mustDial(t)
	defer client.Close()

	var clientVersion string
	_ = client.Client().Call(&clientVersion, "web3_clientVersion")

	receipts := loadOnchainReceipts(t)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	vault := bind.NewBoundContract(vaultAddr, loadOnchainABI(t, "Erc20CoinVault"), client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, loadOnchainABI(t, "RaylsERC20"), client, client, client)
	dvp := bind.NewBoundContract(dvpAddr, loadOnchainABI(t, "EnygmaDvp"), client, client, client)
	alice := hardhatAuth(t, client)

	gnarkClient := rpcore.NewPaymentClient("")
	const depth = 8
	tokenId := big.NewInt(0)
	depositAmt, payAmt, changeAmt := big.NewInt(40), big.NewInt(30), big.NewInt(10)
	aliceSpend, _ := rpcore.NewSpendKeyPair()
	aliceView, _ := rpcore.NewViewKeyPair()
	bobSpend, _ := rpcore.NewSpendKeyPair()
	bobView, _ := rpcore.NewViewKeyPair()
	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())

	wait := func(tx *types.Transaction, err error) *types.Receipt {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		r, err := bind.WaitMined(ctx, client, tx)
		if err != nil || r.Status != 1 {
			t.Fatalf("tx %s failed: %v", tx.Hash(), err)
		}
		return r
	}
	total := new(big.Int).Mul(depositAmt, big.NewInt(int64(n*rounds)))
	wait(erc20.Transact(alice, "mint", alice.From, total))
	wait(erc20.Transact(alice, "approve", vaultAddr, total))

	var allPayments int
	var allExec time.Duration
	var allGas uint64
	for round := 1; round <= rounds; round++ {
		// Deposits (automine on).
		salts := make([]*big.Int, n)
		cmts := make([]*big.Int, n)
		for i := 0; i < n; i++ {
			ss, capsule, err := rpcore.Encapsulate(aliceView.EncapsKey)
			if err != nil {
				t.Fatal(err)
			}
			saltB, _ := rpcore.DerivePaymentSalt(ss)
			encKey, _ := rpcore.DerivePaymentKey(ss)
			salts[i] = rpcore.SaltBToField(saltB)
			cmts[i], _ = rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, salts[i], depositAmt, tokenId)
			payload, _ := rpcore.EncryptPayload(encKey, tokenId, depositAmt)
			wait(vault.Transact(alice, "depositV2",
				[]*big.Int{depositAmt, aliceSpend.PublicKey, salts[i], tokenId}, capsule, payload))
		}
		mt := loadVaultMerkleTreeByTx(t, client, vaultAddr, depth)

		// Proofs (not timed here — see TestRetailThroughput).
		results := make([]*rpcore.PaymentResult, n)
		jobs := make(chan int)
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range jobs {
					path, err := mt.GenerateProof(cmts[i])
					if err != nil {
						t.Errorf("merkle proof %d: %v", i, err)
						continue
					}
					res, err := gnarkClient.BoundPaymentProof(
						vaultAddrBig, big.NewInt(0),
						[]*big.Int{depositAmt},
						[]rpcore.KeyPair{{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey}},
						[]*big.Int{salts[i]},
						[]*big.Int{payAmt, changeAmt},
						[]*big.Int{bobSpend.PublicKey, aliceSpend.PublicKey},
						[][]byte{bobView.EncapsKey, aliceView.EncapsKey},
						depth, []*rpcore.MerkleProof{path},
						[]*big.Int{big.NewInt(int64(path.TreeNumber))}, tokenId,
					)
					if err != nil {
						t.Errorf("proof %d: %v", i, err)
						continue
					}
					results[i] = res
				}
			}()
		}
		for i := 0; i < n; i++ {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		if t.Failed() {
			t.FailNow()
		}

		// Send all N with automine off, then time the block that executes them.
		if err := client.Client().Call(nil, "evm_setAutomine", false); err != nil {
			t.Fatalf("evm_setAutomine: %v", err)
		}
		empty := timedMine(t, client)
		nonce, err := client.PendingNonceAt(ctx, alice.From)
		if err != nil {
			t.Fatal(err)
		}
		txs := make([]*types.Transaction, n)
		for i, res := range results {
			sp := proofStringsToOnchain(t, res.Proof)
			receipt := ProofReceipt{
				Proof: SnarkProof{
					A: G1Point{X: sp.A.X, Y: sp.A.Y},
					B: G2Point{X: sp.B.X, Y: sp.B.Y},
					C: G1Point{X: sp.C.X, Y: sp.C.Y},
				},
				Statement:       res.ContractStatement(),
				NumberOfInputs:  big.NewInt(int64(res.NumberOfInputs)),
				NumberOfOutputs: big.NewInt(int64(res.NumberOfOutputs)),
			}
			opts := *alice
			opts.Nonce = new(big.Int).SetUint64(nonce + uint64(i))
			opts.GasLimit = 2_000_000
			tx, err := dvp.Transact(&opts, "payment", receipt, big.NewInt(0), res.CipherText, res.EncTxData)
			if err != nil {
				t.Fatalf("send payment %d: %v", i, err)
			}
			txs[i] = tx
		}
		full := timedMine(t, client)
		if err := client.Client().Call(nil, "evm_setAutomine", true); err != nil {
			t.Fatalf("evm_setAutomine: %v", err)
		}

		var gas uint64
		var block uint64
		for i, tx := range txs {
			r, err := client.TransactionReceipt(ctx, tx.Hash())
			if err != nil || r.Status != 1 {
				t.Fatalf("round %d payment %d failed: %v", round, i, err)
			}
			if i == 0 {
				block = r.BlockNumber.Uint64()
			} else if r.BlockNumber.Uint64() != block {
				t.Fatalf("round %d: payments split across blocks", round)
			}
			gas += r.GasUsed
		}
		exec := full - empty
		t.Logf("round %d: %d payments in block %d, %.1f Mgas, block %v (empty %v) → exec %v/payment → %.0f payments/s",
			round, n, block, float64(gas)/1e6, full.Round(time.Millisecond), empty.Round(time.Millisecond),
			(exec / time.Duration(n)).Round(10*time.Microsecond), float64(n)/exec.Seconds())
		allPayments += n
		allExec += exec
		allGas += gas
	}
	t.Logf("node %q: %d payments, avg gas %d, exec %v/payment → %.0f payments/s, %.0f Mgas/s",
		clientVersion, allPayments, allGas/uint64(allPayments),
		(allExec / time.Duration(allPayments)).Round(10*time.Microsecond),
		float64(allPayments)/allExec.Seconds(), float64(allGas)/1e6/allExec.Seconds())
}
