package enygma_test

// Shared mock types and test helpers for the relayer handler and integration tests.

import (
	"context"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"enygma_payments_relayer/config"
	contracts "enygma_payments_relayer/contracts"
	"enygma_payments_relayer/server"
	"enygma_payments_relayer/store"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// testAPIKey is the Bearer token used across all relayer tests.
const testAPIKey = "test-bearer-token"

// hardhat #0 — well-known deterministic test key; never use in production.
const hardhatKey0 = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// ── Mock Ethereum contract ────────────────────────────────────────────────────

// mockContract implements server.EnygmaContract.
// Transfer/TransferWithFee return the configured tx/err pair. Simulate*
// return simErr (nil by default, so existing tests reach Transfer/
// TransferWithFee exactly as before). transferCalls counts how many times
// the real (gas-spending) call was invoked — used both to assert a failed
// dry-run short-circuits before broadcast, and to assert the gas-bump retry
// resubmits exactly once (not zero, not an unbounded loop).
type mockContract struct {
	tx  *types.Transaction
	err error

	simErr error

	transferCalled bool // true if either Transfer or TransferWithFee was invoked
	transferCalls  int  // count of either Transfer or TransferWithFee invocations
	simulateCalled bool

	// Per-method counters, additive to the shared ones above, so a test can
	// assert exactly which underlying contract method the handler invoked
	// (RelayTransfer -> Transfer vs RelayTransferFee -> TransferWithFee)
	// instead of only that "some" transfer method fired — the shared
	// counters alone can't catch RelayTransferFee being wired to the wrong
	// method (or vice versa).
	transferOnlyCalls        int
	transferWithFeeOnlyCalls int
}

func (m *mockContract) Transfer(_ *bind.TransactOpts, _ []contracts.IEnygmaPoint, _ contracts.IEnygmaProof, _ []*big.Int) (*types.Transaction, error) {
	m.transferCalled = true
	m.transferCalls++
	m.transferOnlyCalls++
	return m.tx, m.err
}

func (m *mockContract) TransferWithFee(_ *bind.TransactOpts, _ []contracts.IEnygmaPoint, _ contracts.IEnygmaFeeProof, _ []*big.Int) (*types.Transaction, error) {
	m.transferCalled = true
	m.transferCalls++
	m.transferWithFeeOnlyCalls++
	return m.tx, m.err
}

func (m *mockContract) SimulateTransfer(_ *bind.CallOpts, _ []contracts.IEnygmaPoint, _ contracts.IEnygmaProof, _ []*big.Int) error {
	m.simulateCalled = true
	return m.simErr
}

func (m *mockContract) SimulateTransferWithFee(_ *bind.CallOpts, _ []contracts.IEnygmaPoint, _ contracts.IEnygmaFeeProof, _ []*big.Int) error {
	m.simulateCalled = true
	return m.simErr
}

// ── Mock proof verifier ───────────────────────────────────────────────────────

// mockVerifier implements server.ProofVerifier. Returns err (nil by default,
// so existing tests — whose proof/publicSignal fixtures are synthetic filler
// values, not real Groth16 proofs — pass local verification exactly as
// before real cryptographic checking existed).
type mockVerifier struct {
	err error
}

func (m *mockVerifier) Verify(_ [8]*big.Int, _ []*big.Int) error {
	return m.err
}

// ── Mock chain backend (server.ChainBackend) ─────────────────────────────────

// mockMiner implements server.ChainBackend.
//
// TransactionReceipt supports simulating a transaction that isn't mined yet:
// while notFoundCalls > 0, it returns ethereum.NotFound (decrementing) —
// matching what bind.WaitMined actually polls for — before falling back to
// the configured receipt/err. This is how the gas-bump-retry tests force a
// deterministic "stuck, then recovers" or "stuck on both attempts" sequence
// without waiting on real chain timing.
type mockMiner struct {
	receipt       *types.Receipt
	err           error
	notFoundCalls int
	receiptCalls  int

	blockNum    uint64
	blockErr    error
	balance     *big.Int
	balanceErr  error
	gasPrice    *big.Int
	gasPriceErr error
	nonce       uint64
	nonceErr    error
}

func (m *mockMiner) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	m.receiptCalls++
	if m.notFoundCalls > 0 {
		m.notFoundCalls--
		return nil, ethereum.NotFound
	}
	return m.receipt, m.err
}

func (m *mockMiner) CodeAt(_ context.Context, _ common.Address, _ *big.Int) ([]byte, error) {
	return []byte("ok"), nil
}

func (m *mockMiner) BlockNumber(_ context.Context) (uint64, error) {
	return m.blockNum, m.blockErr
}

func (m *mockMiner) BalanceAt(_ context.Context, _ common.Address, _ *big.Int) (*big.Int, error) {
	if m.balance == nil {
		return big.NewInt(0), m.balanceErr
	}
	return m.balance, m.balanceErr
}

func (m *mockMiner) SuggestGasPrice(_ context.Context) (*big.Int, error) {
	if m.gasPrice == nil {
		return big.NewInt(1_000_000_000), m.gasPriceErr // 1 gwei default
	}
	return m.gasPrice, m.gasPriceErr
}

func (m *mockMiner) PendingNonceAt(_ context.Context, _ common.Address) (uint64, error) {
	return m.nonce, m.nonceErr
}

// ── Transaction/receipt factories ────────────────────────────────────────────

// dummyTx returns a minimal non-nil *types.Transaction for mocks.
func dummyTx() *types.Transaction {
	to := common.Address{}
	return types.NewTx(&types.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1e9),
		Gas:      21000,
		To:       &to,
		Value:    big.NewInt(0),
	})
}

// successReceipt returns a mined receipt with Status=1 for the given tx.
func successReceipt(tx *types.Transaction) *types.Receipt {
	return &types.Receipt{
		Status:      types.ReceiptStatusSuccessful,
		TxHash:      tx.Hash(),
		BlockNumber: big.NewInt(42),
		GasUsed:     21000,
	}
}

// ── Handler factory ──────────────────────────────────────────────────────────

// testHandlerOpts configures newTestHandlerOpts; zero-valued fields fall
// back to sane test defaults (see newTestHandlerOpts).
type testHandlerOpts struct {
	minFee     *big.Int
	vTransfer  server.ProofVerifier
	vFee       server.ProofVerifier
	rps        float64
	burst      int
	staleness  time.Duration
	txTimeout  time.Duration
	minBalance *big.Int
}

// testStores tracks every store newTestStore opens (dir + handle) so
// TestMain can close and remove them all after the package's tests finish.
// newTestStore is called from shared handler-factory helpers with no
// *testing.T of their own to register per-test cleanup against, so a single
// package-level sweep is the low-friction way to avoid leaking a bbolt file
// handle and a temp directory per call across the whole suite (~50 sites).
var (
	testStoresMu sync.Mutex
	testStores   []struct {
		dir string
		st  *store.Store
	}
)

// newTestStore opens a bbolt store in a fresh temp directory — real
// persistence machinery, isolated per call, not a mock. bbolt operations
// are microseconds, so this adds no meaningful test latency.
func newTestStore() *store.Store {
	dir, err := os.MkdirTemp("", "relayer-test-*")
	if err != nil {
		panic("newTestStore: " + err.Error())
	}
	st, err := store.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		panic("newTestStore: " + err.Error())
	}
	testStoresMu.Lock()
	testStores = append(testStores, struct {
		dir string
		st  *store.Store
	}{dir, st})
	testStoresMu.Unlock()
	return st
}

// TestMain runs the package's tests, then closes and removes every bbolt
// store newTestStore opened along the way — see testStores.
func TestMain(m *testing.M) {
	code := m.Run()
	testStoresMu.Lock()
	for _, ts := range testStores {
		ts.st.Close()
		os.RemoveAll(ts.dir)
	}
	testStoresMu.Unlock()
	os.Exit(code)
}

// newTestHandlerOpts builds a Handler plus its backing store — the store is
// returned so tests can seed or inspect dedup state directly (persistence
// and idempotent-replay tests); most tests can ignore the second value.
func newTestHandlerOpts(c *mockContract, m *mockMiner, o testHandlerOpts) (*server.Handler, *store.Store) {
	if o.minFee == nil {
		o.minFee = big.NewInt(0)
	}
	if o.vTransfer == nil {
		o.vTransfer = &mockVerifier{}
	}
	if o.vFee == nil {
		o.vFee = &mockVerifier{}
	}
	if o.staleness == 0 {
		o.staleness = 5 * time.Minute
	}
	if o.txTimeout == 0 {
		o.txTimeout = 2 * time.Second
	}
	if o.minBalance == nil {
		o.minBalance = big.NewInt(0)
	}

	privKey, _ := crypto.HexToECDSA(hardhatKey0)
	auth, _ := bind.NewKeyedTransactorWithChainID(privKey, big.NewInt(1337))
	cfg := &config.Config{
		APIKey:         testAPIKey,
		ChainID:        big.NewInt(1337),
		GasLimit:       300_000_000,
		MinFee:         o.minFee,
		RateLimitRPS:   o.rps,
		RateLimitBurst: o.burst,
		DedupStaleness: o.staleness,
		MinBalanceWei:  o.minBalance,
	}
	st := newTestStore()
	h := server.NewHandlerWithDeps(server.HandlerDeps{
		Cfg:          cfg,
		ContractAddr: "0x1234567890123456789012345678901234567890",
		Auth:         auth,
		Client:       m,
		Instance:     c,
		VTransfer:    o.vTransfer,
		VFee:         o.vFee,
		Store:        st,
		Nonces:       server.NewNonceManagerFromStart(0),
		TxTimeout:    o.txTimeout,
	})
	return h, st
}

// newTestHandler creates a Handler backed by mock contract + mock miner.
// MinFee defaults to 0 (disabled), local proof verification always passes,
// rate limiting is disabled — use the other constructors for those.
func newTestHandler(c *mockContract, m *mockMiner) *server.Handler {
	h, _ := newTestHandlerOpts(c, m, testHandlerOpts{})
	return h
}

// newTestHandlerWithMinFee is like newTestHandler but sets a specific relayer MinFee.
func newTestHandlerWithMinFee(c *mockContract, m *mockMiner, minFee *big.Int) *server.Handler {
	h, _ := newTestHandlerOpts(c, m, testHandlerOpts{minFee: minFee})
	return h
}

// newTestHandlerWithVerifiers is like newTestHandler but injects specific
// (possibly failing) local proof verifiers for the transfer/fee endpoints.
func newTestHandlerWithVerifiers(c *mockContract, m *mockMiner, minFee *big.Int, vTransfer, vFee server.ProofVerifier) *server.Handler {
	h, _ := newTestHandlerOpts(c, m, testHandlerOpts{minFee: minFee, vTransfer: vTransfer, vFee: vFee})
	return h
}

// newTestHandlerWithRateLimit is like newTestHandlerWithVerifiers but also
// sets a specific per-caller rate limit (rps <= 0 disables it).
func newTestHandlerWithRateLimit(c *mockContract, m *mockMiner, minFee *big.Int, vTransfer, vFee server.ProofVerifier, rps float64, burst int) *server.Handler {
	h, _ := newTestHandlerOpts(c, m, testHandlerOpts{minFee: minFee, vTransfer: vTransfer, vFee: vFee, rps: rps, burst: burst})
	return h
}

// newTestHandlerWithStore is like newTestHandler but also returns the
// backing store (for dedup-persistence / idempotent-replay tests) and takes
// an explicit staleness window.
func newTestHandlerWithStore(c *mockContract, m *mockMiner, staleness time.Duration) (*server.Handler, *store.Store) {
	return newTestHandlerOpts(c, m, testHandlerOpts{staleness: staleness})
}

// newTestHandlerWithMinBalance is like newTestHandler but sets a readiness
// balance floor (RELAYER_MIN_BALANCE_WEI equivalent).
func newTestHandlerWithMinBalance(c *mockContract, m *mockMiner, minBalance *big.Int) *server.Handler {
	h, _ := newTestHandlerOpts(c, m, testHandlerOpts{minBalance: minBalance})
	return h
}

// newTestHandlerWithTxTimeout is like newTestHandler but sets a short
// per-attempt mined-wait budget — for gas-bump-retry tests, which need a
// tx-timeout on the order of a couple of poll intervals (bind.WaitMined
// polls once per second), not the real 45s default.
func newTestHandlerWithTxTimeout(c *mockContract, m *mockMiner, txTimeout time.Duration) *server.Handler {
	h, _ := newTestHandlerOpts(c, m, testHandlerOpts{txTimeout: txTimeout})
	return h
}

// ── Valid request body ────────────────────────────────────────────────────────

func validTransferBody() server.RelayTransferRequest {
	var proof [8]string
	for i := range proof {
		proof[i] = big.NewInt(int64(i + 1)).String()
	}
	pubSig := make([]string, 25)
	for i := range pubSig {
		pubSig[i] = big.NewInt(int64(i + 100)).String()
	}
	return server.RelayTransferRequest{
		Proof:        proof,
		PublicSignal: pubSig,
		Commitments:  [][]string{{"1", "2"}, {"3", "4"}, {"5", "6"}, {"7", "8"}, {"9", "10"}, {"11", "12"}},
		KIndex:       []int64{1, 2, 3, 4, 5, 6},
	}
}

// validTransferFeeBody returns a well-formed RelayTransferFeeRequest with a
// fee of 20 at publicSignal[50] (the enygma_fee circuit's public Fee slot).
func validTransferFeeBody() server.RelayTransferFeeRequest {
	var proof [8]string
	for i := range proof {
		proof[i] = big.NewInt(int64(i + 1)).String()
	}
	pubSig := make([]string, 54)
	for i := range pubSig {
		pubSig[i] = big.NewInt(int64(i + 100)).String()
	}
	pubSig[50] = "20" // Fee
	return server.RelayTransferFeeRequest{
		Proof:        proof,
		PublicSignal: pubSig,
		Commitments:  [][]string{{"1", "2"}, {"3", "4"}, {"5", "6"}, {"7", "8"}, {"9", "10"}, {"11", "12"}},
		KIndex:       []int64{1, 2, 3, 4, 5, 6},
	}
}
