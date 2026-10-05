package tests

// 17_v2_swaprelayer_closed_swap_test.go
//
// SwapRelayer must treat a settled or cancelled swapId as closed. Before the
// fix, cancelSwap/settlement deleted the PendingSwap record outright, so a late
// leg under the same swapId looked like the first leg of a new swap: it was
// accepted, its nullifiers were locked, and it could never settle.
//
// Runs on go-ethereum's simulated backend against SwapRelayerDvpMock (the four
// EnygmaDvp functions SwapRelayer calls), so no Hardhat node or gnark server
// is needed. Requires `npx hardhat compile` for the two artifacts.

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/rpc"
)

type swapRelayerEnv struct {
	t       *testing.T
	backend *simulated.Backend
	relayer common.Address
	mock    common.Address
	relABI  abi.ABI
	mockABI abi.ABI
	keys    map[string]*ecdsa.PrivateKey
}

func loadArtifact(t *testing.T, rel string) (abi.ABI, []byte) {
	t.Helper()
	data, err := os.ReadFile("../artifacts/contracts/" + rel)
	if err != nil {
		t.Skipf("artifact %s missing (run `npx hardhat compile`): %v", rel, err)
	}
	var a struct {
		ABI      json.RawMessage `json:"abi"`
		Bytecode string          `json:"bytecode"`
	}
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	parsed, err := abi.JSON(strings.NewReader(string(a.ABI)))
	if err != nil {
		t.Fatalf("parse ABI %s: %v", rel, err)
	}
	return parsed, common.FromHex(a.Bytecode)
}

func newSwapRelayerEnv(t *testing.T) *swapRelayerEnv {
	t.Helper()
	keys := map[string]*ecdsa.PrivateKey{}
	alloc := types.GenesisAlloc{}
	for _, name := range []string{"deployer", "alice", "bob", "mallory"} {
		k, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		keys[name] = k
		alloc[crypto.PubkeyToAddress(k.PublicKey)] = types.Account{Balance: new(big.Int).Lsh(big.NewInt(1), 100)}
	}
	backend := simulated.NewBackend(alloc)
	t.Cleanup(func() { backend.Close() })

	env := &swapRelayerEnv{t: t, backend: backend, keys: keys}
	var mockBin, relBin []byte
	env.mockABI, mockBin = loadArtifact(t, "test/SwapRelayerDvpMock.sol/SwapRelayerDvpMock.json")
	env.relABI, relBin = loadArtifact(t, "core/contracts/SwapRelayer.sol/SwapRelayer.json")

	env.mock = env.deploy(env.mockABI, mockBin)
	env.relayer = env.deploy(env.relABI, relBin, env.mock)
	return env
}

func (e *swapRelayerEnv) auth(who string) *bind.TransactOpts {
	auth, err := bind.NewKeyedTransactorWithChainID(e.keys[who], big.NewInt(1337))
	if err != nil {
		e.t.Fatal(err)
	}
	auth.GasLimit = 3_000_000
	return auth
}

func (e *swapRelayerEnv) deploy(a abi.ABI, bin []byte, args ...interface{}) common.Address {
	e.t.Helper()
	addr, _, _, err := bind.DeployContract(e.auth("deployer"), a, bin, e.backend.Client(), args...)
	if err != nil {
		e.t.Fatalf("deploy: %v", err)
	}
	e.backend.Commit()
	return addr
}

func (e *swapRelayerEnv) now() uint64 {
	h, err := e.backend.Client().HeaderByNumber(context.Background(), nil)
	if err != nil {
		e.t.Fatal(err)
	}
	return h.Time
}

// send simulates the call first so a revert comes back with its error data,
// then mines it. It returns the custom error name ("" on success).
func (e *swapRelayerEnv) send(who, method string, args ...interface{}) string {
	e.t.Helper()
	data, err := e.relABI.Pack(method, args...)
	if err != nil {
		e.t.Fatalf("pack %s: %v", method, err)
	}
	auth := e.auth(who)
	call := ethereum.CallMsg{From: auth.From, To: &e.relayer, Gas: auth.GasLimit, Data: data}
	if _, err := e.backend.Client().CallContract(context.Background(), call, nil); err != nil {
		return e.revertName(err)
	}
	c := bind.NewBoundContract(e.relayer, e.relABI, e.backend.Client(), e.backend.Client(), e.backend.Client())
	tx, err := c.RawTransact(auth, data)
	if err != nil {
		e.t.Fatalf("send %s: %v", method, err)
	}
	e.backend.Commit()
	rcpt, err := e.backend.Client().TransactionReceipt(context.Background(), tx.Hash())
	if err != nil {
		e.t.Fatalf("receipt %s: %v", method, err)
	}
	if rcpt.Status != types.ReceiptStatusSuccessful {
		e.t.Fatalf("%s mined but reverted", method)
	}
	return ""
}

func (e *swapRelayerEnv) revertName(err error) string {
	var de rpc.DataError
	if errors.As(err, &de) {
		if s, ok := de.ErrorData().(string); ok {
			raw, _ := hex.DecodeString(strings.TrimPrefix(s, "0x"))
			if len(raw) >= 4 {
				for name, ce := range e.relABI.Errors {
					if string(ce.ID[:4]) == string(raw[:4]) {
						return name
					}
				}
			}
		}
	}
	return err.Error()
}

func (e *swapRelayerEnv) locked(vaultID int64, nf *big.Int) bool {
	e.t.Helper()
	var out []interface{}
	c := bind.NewBoundContract(e.mock, e.mockABI, e.backend.Client(), nil, nil)
	if err := c.Call(nil, &out, "locked", big.NewInt(vaultID), nf); err != nil {
		e.t.Fatalf("locked: %v", err)
	}
	return out[0].(bool)
}

func (e *swapRelayerEnv) swapCount() int64 {
	e.t.Helper()
	var out []interface{}
	c := bind.NewBoundContract(e.mock, e.mockABI, e.backend.Client(), nil, nil)
	if err := c.Call(nil, &out, "swapCount"); err != nil {
		e.t.Fatalf("swapCount: %v", err)
	}
	return out[0].(*big.Int).Int64()
}

// fakeReceipt builds a 1-input/1-output receipt; only the nullifier matters
// to SwapRelayer (the proof itself is checked by EnygmaDvp.swap()).
func fakeReceipt(nullifier int64) onchainProofReceipt {
	z := big.NewInt(0)
	return onchainProofReceipt{
		Proof: onchainSnarkProof{
			A: onchainG1Point{X: z, Y: z},
			B: onchainG2Point{X: [2]*big.Int{z, z}, Y: [2]*big.Int{z, z}},
			C: onchainG1Point{X: z, Y: z},
		},
		Statement:       []*big.Int{big.NewInt(1), z, big.NewInt(2), big.NewInt(nullifier), big.NewInt(3)},
		NumberOfInputs:  big.NewInt(1),
		NumberOfOutputs: big.NewInt(1),
	}
}

const (
	paymentVault  = 0
	deliveryVault = 1
)

func TestV2SwapRelayer_ClosedSwapId(t *testing.T) {
	ctx := []byte{0x01}

	t.Run("late leg after cancel is rejected", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		id := crypto.Keccak256Hash([]byte("cancelled-swap"))
		alice, bob := fakeReceipt(101), fakeReceipt(202)

		expiry := big.NewInt(int64(env.now() + 3600))
		if r := env.send("alice", "submitReceipt", id, alice, true, big.NewInt(paymentVault), expiry, ctx, ctx); r != "" {
			t.Fatalf("Alice's leg: %s", r)
		}
		if err := env.backend.AdjustTime(2 * time.Hour); err != nil {
			t.Fatal(err)
		}
		env.backend.Commit()
		if r := env.send("alice", "cancelSwap", id); r != "" {
			t.Fatalf("cancelSwap: %s", r)
		}
		if env.locked(paymentVault, alice.Statement[3]) {
			t.Fatal("Alice's nullifier still locked after cancel")
		}

		// Bob's late B-side leg under the same swapId.
		lateExpiry := big.NewInt(int64(env.now() + 3600))
		if r := env.send("bob", "submitReceipt", id, bob, false, big.NewInt(deliveryVault), lateExpiry, ctx, ctx); r != "SwapClosed" {
			t.Fatalf("late leg: got %q, want SwapClosed", r)
		}
		if env.locked(deliveryVault, bob.Statement[3]) {
			t.Fatal("Bob's nullifier locked by a leg on a cancelled swap")
		}

		// Nor can anyone replay Alice's published leg to re-lock her note.
		if r := env.send("mallory", "submitReceipt", id, alice, true, big.NewInt(paymentVault), lateExpiry, ctx, ctx); r != "SwapClosed" {
			t.Fatalf("replayed leg: got %q, want SwapClosed", r)
		}
		if env.locked(paymentVault, alice.Statement[3]) {
			t.Fatal("Alice's nullifier re-locked by a replay")
		}
	})

	t.Run("settled swapId cannot be reused", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		id := crypto.Keccak256Hash([]byte("settled-swap"))
		expiry := big.NewInt(int64(env.now() + 3600))

		if r := env.send("alice", "submitReceipt", id, fakeReceipt(101), true, big.NewInt(paymentVault), expiry, ctx, ctx); r != "" {
			t.Fatalf("Alice's leg: %s", r)
		}
		if r := env.send("bob", "submitReceipt", id, fakeReceipt(202), false, big.NewInt(deliveryVault), expiry, ctx, ctx); r != "" {
			t.Fatalf("Bob's leg: %s", r)
		}
		if n := env.swapCount(); n != 1 {
			t.Fatalf("swapCount = %d, want 1", n)
		}

		if r := env.send("mallory", "submitReceipt", id, fakeReceipt(303), true, big.NewInt(paymentVault), expiry, ctx, ctx); r != "SwapClosed" {
			t.Fatalf("reuse of settled swapId: got %q, want SwapClosed", r)
		}
		if env.locked(paymentVault, big.NewInt(303)) {
			t.Fatal("nullifier locked under a settled swapId")
		}
	})

	t.Run("expiry beyond MAX_SWAP_DURATION is rejected", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		id := crypto.Keccak256Hash([]byte("far-expiry"))
		far := big.NewInt(int64(env.now() + 31*24*3600))

		if r := env.send("mallory", "submitReceipt", id, fakeReceipt(101), true, big.NewInt(paymentVault), far, ctx, ctx); r != "ExpiryTooFar" {
			t.Fatalf("far expiry: got %q, want ExpiryTooFar", r)
		}
		if env.locked(paymentVault, big.NewInt(101)) {
			t.Fatal("nullifier locked despite rejected expiry")
		}
	})
}
