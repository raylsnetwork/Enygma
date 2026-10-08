package tests

// 17_v2_swaprelayer_closed_swap_test.go
//
// SwapRelayer must treat a settled or cancelled swapId as closed, and must only
// let a genuine leg occupy the slot of its own swap. Before the
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
				for _, a := range []abi.ABI{e.relABI, e.mockABI} {
					for name, ce := range a.Errors {
						if string(ce.ID[:4]) == string(raw[:4]) {
							return name
						}
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

// leg builds a 1-input/1-output receipt for one side of a swap. A leg's
// message is its own side's and its first output is the counterparty's, as
// SwapRelayer.swapIdOf reads them. bad marks the proof as invalid for the mock.
func leg(own, other, nullifier int64, bad bool) onchainProofReceipt {
	z := big.NewInt(0)
	ax := z
	if bad {
		ax = big.NewInt(0xbad) // SwapRelayerDvpMock.BAD_PROOF
	}
	return onchainProofReceipt{
		Proof: onchainSnarkProof{
			A: onchainG1Point{X: ax, Y: z},
			B: onchainG2Point{X: [2]*big.Int{z, z}, Y: [2]*big.Int{z, z}},
			C: onchainG1Point{X: z, Y: z},
		},
		Statement:       []*big.Int{big.NewInt(own), z, big.NewInt(2), big.NewInt(nullifier), big.NewInt(other)},
		NumberOfInputs:  big.NewInt(1),
		NumberOfOutputs: big.NewInt(1),
	}
}

// swapPair returns matching payment and delivery legs and their swapId,
// keccak256(abi.encode(deliveryMessage, paymentMessage)).
func swapPair(payMsg, delMsg, payNf, delNf int64) (pay, del onchainProofReceipt, id common.Hash) {
	pay = leg(payMsg, delMsg, payNf, false)
	del = leg(delMsg, payMsg, delNf, false)
	id = crypto.Keccak256Hash(common.BigToHash(big.NewInt(delMsg)).Bytes(), common.BigToHash(big.NewInt(payMsg)).Bytes())
	return pay, del, id
}

const (
	paymentVault  = 0
	deliveryVault = 1
)

func TestV2SwapRelayer_ClosedSwapId(t *testing.T) {
	ctx := []byte{0x01}
	pv, dv := big.NewInt(paymentVault), big.NewInt(deliveryVault)

	t.Run("late leg after cancel is rejected", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		alice, bob, id := swapPair(11, 12, 101, 202)

		expiry := big.NewInt(int64(env.now() + 3600))
		if r := env.send("alice", "submitReceipt", id, alice, true, pv, expiry, ctx, ctx); r != "" {
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
		if r := env.send("bob", "submitReceipt", id, bob, false, dv, lateExpiry, ctx, ctx); r != "SwapClosed" {
			t.Fatalf("late leg: got %q, want SwapClosed", r)
		}
		if env.locked(deliveryVault, bob.Statement[3]) {
			t.Fatal("Bob's nullifier locked by a leg on a cancelled swap")
		}

		// Nor can anyone replay Alice's published leg to re-lock her note.
		if r := env.send("mallory", "submitReceipt", id, alice, true, pv, lateExpiry, ctx, ctx); r != "SwapClosed" {
			t.Fatalf("replayed leg: got %q, want SwapClosed", r)
		}
		if env.locked(paymentVault, alice.Statement[3]) {
			t.Fatal("Alice's nullifier re-locked by a replay")
		}
	})

	t.Run("settled swapId cannot be reused", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		alice, bob, id := swapPair(21, 22, 101, 202)
		expiry := big.NewInt(int64(env.now() + 3600))

		if r := env.send("alice", "submitReceipt", id, alice, true, pv, expiry, ctx, ctx); r != "" {
			t.Fatalf("Alice's leg: %s", r)
		}
		if r := env.send("bob", "submitReceipt", id, bob, false, dv, expiry, ctx, ctx); r != "" {
			t.Fatalf("Bob's leg: %s", r)
		}
		if n := env.swapCount(); n != 1 {
			t.Fatalf("swapCount = %d, want 1", n)
		}

		// Another payment leg for the same swap (same messages, other note).
		again := leg(21, 22, 303, false)
		if r := env.send("mallory", "submitReceipt", id, again, true, pv, expiry, ctx, ctx); r != "SwapClosed" {
			t.Fatalf("reuse of settled swapId: got %q, want SwapClosed", r)
		}
		if env.locked(paymentVault, big.NewInt(303)) {
			t.Fatal("nullifier locked under a settled swapId")
		}
	})

	t.Run("expiry beyond MAX_SWAP_DURATION is rejected", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		alice, _, id := swapPair(31, 32, 101, 202)
		far := big.NewInt(int64(env.now() + 31*24*3600))

		if r := env.send("mallory", "submitReceipt", id, alice, true, pv, far, ctx, ctx); r != "ExpiryTooFar" {
			t.Fatalf("far expiry: got %q, want ExpiryTooFar", r)
		}
		if env.locked(paymentVault, alice.Statement[3]) {
			t.Fatal("nullifier locked despite rejected expiry")
		}
	})

	// The first submitter (possibly a front-runner holding Alice's public
	// leg) picks the expiry, and anyone can cancel once it passes: one under
	// MIN_SWAP_DURATION would let them cancel before Bob can deliver.
	t.Run("expiry under MIN_SWAP_DURATION is rejected", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		alice, _, id := swapPair(33, 34, 101, 202)
		soon := big.NewInt(int64(env.now() + 60))

		if r := env.send("mallory", "submitReceipt", id, alice, true, pv, soon, ctx, ctx); r != "ExpiryTooSoon" {
			t.Fatalf("VULNERABLE: 1-minute expiry: got %q, want ExpiryTooSoon", r)
		}
		if env.locked(paymentVault, alice.Statement[3]) {
			t.Fatal("nullifier locked despite rejected expiry")
		}
	})

	t.Run("a leg with an invalid proof cannot occupy a swap", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		alice, _, id := swapPair(41, 42, 101, 202)
		expiry := big.NewInt(int64(env.now() + 3600))

		// Mallory front-runs Alice with a junk leg for the same swap that
		// would lock someone's nullifier (here 999).
		junk := leg(41, 42, 999, true)
		if r := env.send("mallory", "submitReceipt", id, junk, true, pv, expiry, ctx, ctx); r != "InvalidProof" {
			t.Fatalf("junk leg: got %q, want InvalidProof", r)
		}
		if env.locked(paymentVault, big.NewInt(999)) {
			t.Fatal("junk leg locked a nullifier")
		}
		// The slot is still free for Alice's genuine leg.
		if r := env.send("alice", "submitReceipt", id, alice, true, pv, expiry, ctx, ctx); r != "" {
			t.Fatalf("Alice's leg after the junk attempt: %s", r)
		}
	})

	t.Run("a leg cannot be filed under another swap's id", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		alice, _, _ := swapPair(51, 52, 101, 202)
		_, _, otherId := swapPair(61, 62, 303, 404)
		expiry := big.NewInt(int64(env.now() + 3600))

		if r := env.send("mallory", "submitReceipt", otherId, alice, true, pv, expiry, ctx, ctx); r != "SwapIdMismatch" {
			t.Fatalf("leg under a foreign swapId: got %q, want SwapIdMismatch", r)
		}
		if env.locked(paymentVault, alice.Statement[3]) {
			t.Fatal("nullifier locked under a foreign swapId")
		}
	})

	t.Run("anyone can cancel an expired leg", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		alice, _, id := swapPair(71, 72, 101, 202)
		expiry := big.NewInt(int64(env.now() + 3600))

		// Mallory replays Alice's leg before she does, so Mallory is the
		// recorded submitter; Alice must still be able to release her note.
		if r := env.send("mallory", "submitReceipt", id, alice, true, pv, expiry, ctx, ctx); r != "" {
			t.Fatalf("replayed leg: %s", r)
		}
		if r := env.send("alice", "cancelSwap", id); r != "SwapNotExpiredYet" {
			t.Fatalf("cancel before expiry: got %q, want SwapNotExpiredYet", r)
		}
		if err := env.backend.AdjustTime(2 * time.Hour); err != nil {
			t.Fatal(err)
		}
		env.backend.Commit()
		if r := env.send("alice", "cancelSwap", id); r != "" {
			t.Fatalf("Alice cancelling the leg Mallory submitted: %s", r)
		}
		if env.locked(paymentVault, alice.Statement[3]) {
			t.Fatal("Alice's nullifier still locked after cancel")
		}
	})
	// The same leg has a second ("mirror") swapId when filed with the
	// isPayment flag flipped: swapIdOf orders the messages by the flag.
	mirrorOf := func(payMsg, delMsg int64) common.Hash {
		return crypto.Keccak256Hash(common.BigToHash(big.NewInt(payMsg)).Bytes(), common.BigToHash(big.NewInt(delMsg)).Bytes())
	}

	t.Run("a leg filed with the flag flipped is rejected", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		alice, _, id := swapPair(81, 82, 101, 202)
		expiry := big.NewInt(int64(env.now() + 29*24*3600))

		// Mallory front-runs Alice, filing her payment leg as a delivery leg.
		if r := env.send("mallory", "submitReceipt", mirrorOf(81, 82), alice, false, pv, expiry, ctx, ctx); r != "LegTypeMismatch" {
			t.Fatalf("payment leg filed as delivery: got %q, want LegTypeMismatch", r)
		}
		if env.locked(paymentVault, alice.Statement[3]) {
			t.Fatal("the flipped leg locked Alice's note")
		}
		// Alice's own submission still goes through.
		if r := env.send("alice", "submitReceipt", id, alice, true, pv, big.NewInt(int64(env.now()+3600)), ctx, ctx); r != "" {
			t.Fatalf("Alice's own leg after the attempt: %s", r)
		}
	})

	t.Run("the mirror of a cancelled swap is closed too", func(t *testing.T) {
		env := newSwapRelayerEnv(t)
		alice, _, id := swapPair(91, 92, 101, 202)
		if r := env.send("alice", "submitReceipt", id, alice, true, pv, big.NewInt(int64(env.now()+3600)), ctx, ctx); r != "" {
			t.Fatalf("Alice's leg: %s", r)
		}
		if err := env.backend.AdjustTime(2 * time.Hour); err != nil {
			t.Fatal(err)
		}
		env.backend.Commit()
		if r := env.send("alice", "cancelSwap", id); r != "" {
			t.Fatalf("cancelSwap: %s", r)
		}
		lateExpiry := big.NewInt(int64(env.now() + 29*24*3600))
		if r := env.send("mallory", "submitReceipt", mirrorOf(91, 92), alice, false, pv, lateExpiry, ctx, ctx); r != "SwapClosed" {
			t.Fatalf("replay under the mirror id: got %q, want SwapClosed", r)
		}
		if env.locked(paymentVault, alice.Statement[3]) {
			t.Fatal("the replay re-locked Alice's note")
		}
	})
}
