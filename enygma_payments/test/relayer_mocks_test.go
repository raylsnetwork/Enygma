package enygma_test

// Shared mock types and test helpers for the relayer handler and integration tests.

import (
	"context"
	"math/big"

	"enygma_payments_relayer/config"
	contracts "enygma_payments_relayer/contracts"
	"enygma_payments_relayer/server"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// testAPIKey is the Bearer token used across all relayer tests.
const testAPIKey = "test-bearer-token"

// testAPIKeys is the Fix H-06 per-bank credential map used across all
// relayer tests — testAPIKey is issued to a single bank, "bank-test".
var testAPIKeys = map[string]string{testAPIKey: "bank-test"}

// hardhat #0 — well-known deterministic test key; never use in production.
const hardhatKey0 = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// ── Mock Ethereum contract ────────────────────────────────────────────────────

// mockContract implements server.EnygmaContract.
// Transfer returns the configured tx/err pair. gotBankTag captures
// whatever bankTag the handler actually passed through (Fix H-09), so a
// test can assert on it directly rather than only on the HTTP outcome.
type mockContract struct {
	tx         *types.Transaction
	err        error
	gotBankTag string
}

func (m *mockContract) Transfer(_ *bind.TransactOpts, _ []contracts.IEnygmaPoint, _ contracts.IEnygmaProof, _ []*big.Int, bankTag string) (*types.Transaction, error) {
	m.gotBankTag = bankTag
	return m.tx, m.err
}

func (m *mockContract) TransferWithFee(_ *bind.TransactOpts, _ []contracts.IEnygmaPoint, _ contracts.IEnygmaFeeProof, _ []*big.Int, bankTag string) (*types.Transaction, error) {
	m.gotBankTag = bankTag
	return m.tx, m.err
}

// ── Mock miner (bind.DeployBackend) ──────────────────────────────────────────

// mockMiner implements bind.DeployBackend for use with bind.WaitMined.
type mockMiner struct {
	receipt *types.Receipt
	err     error
}

func (m *mockMiner) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	return m.receipt, m.err
}
func (m *mockMiner) CodeAt(_ context.Context, _ common.Address, _ *big.Int) ([]byte, error) {
	return []byte("ok"), nil
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

// newTestHandler creates a Handler backed by mock contract + mock miner.
func newTestHandler(c *mockContract, m *mockMiner) *server.Handler {
	privKey, _ := crypto.HexToECDSA(hardhatKey0)
	auth, _ := bind.NewKeyedTransactorWithChainID(privKey, big.NewInt(1337))
	cfg := &config.Config{
		APIKeys:  testAPIKeys,
		ChainID:  big.NewInt(1337),
		GasLimit: 300_000_000,
	}
	return server.NewHandlerWithDeps(cfg, "0x1234567890123456789012345678901234567890", auth, m, c)
}

// ── Valid request body ────────────────────────────────────────────────────────

// transferPublicSignalLen is the enygma circuit's exact public-signal
// arity (FingerPrint 6×6 layout + the Fix L-01 domain separator).
// Fix L-05: RelayTransfer now requires exactly this many elements —
// no more, no less — rather than accepting anything up to it and
// zero-padding the rest.
const transferPublicSignalLen = 81

func validTransferBody() server.RelayTransferRequest {
	var proof [8]string
	for i := range proof {
		proof[i] = big.NewInt(int64(i + 1)).String()
	}
	pubSig := make([]string, transferPublicSignalLen)
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
