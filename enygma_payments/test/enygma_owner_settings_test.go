package enygma_test

// Owner settings on Enygma.sol: replacing an already-set verifier (or the
// ZkDvp bridge) waits verifierChangeDelay between proposal and execution,
// while first-time setup applies at once; setUsdrFixedFee is bounded below
// 2^64; addZkDvp needs a contract; registerAccount refuses the zero address
// and an address that already has an account.
//
// Runs on go-ethereum's simulated backend against the Hardhat artifacts:
//
//	cd enygma_payments/contracts/enygma && npx hardhat compile

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

// VerifierKind values (Enygma.VerifierKind).
const (
	kindTransfer uint8 = 0
	kindWithdraw uint8 = 1
	kindZkDvp    uint8 = 6
)

type ownerEnv struct {
	t       *testing.T
	backend *simulated.Backend
	key     *ecdsa.PrivateKey
	enygma  common.Address
	abi     abi.ABI
}

func loadEnygmaArtifact(t *testing.T, rel string) (abi.ABI, []byte) {
	t.Helper()
	data, err := os.ReadFile("../contracts/enygma/artifacts/contracts/" + rel)
	if err != nil {
		t.Skipf("artifact %s missing (run `npx hardhat compile` in contracts/enygma): %v", rel, err)
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

func newOwnerEnv(t *testing.T) *ownerEnv {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	backend := simulated.NewBackend(types.GenesisAlloc{
		crypto.PubkeyToAddress(key.PublicKey): {Balance: new(big.Int).Lsh(big.NewInt(1), 100)},
	})
	t.Cleanup(func() { backend.Close() })
	e := &ownerEnv{t: t, backend: backend, key: key}
	e.abi, _ = loadEnygmaArtifact(t, "Enygma.sol/Enygma.json")
	return e
}

func (e *ownerEnv) auth() *bind.TransactOpts {
	auth, err := bind.NewKeyedTransactorWithChainID(e.key, big.NewInt(1337))
	if err != nil {
		e.t.Fatal(err)
	}
	auth.GasLimit = 15_000_000
	return auth
}

// deploy deploys a contract and returns its address, or the revert reason.
func (e *ownerEnv) deploy(rel string, args ...interface{}) (common.Address, error) {
	e.t.Helper()
	a, bin := loadEnygmaArtifact(e.t, rel)
	addr, tx, _, err := bind.DeployContract(e.auth(), a, bin, e.backend.Client(), args...)
	if err != nil {
		return common.Address{}, err
	}
	e.backend.Commit()
	rcpt, err := e.backend.Client().TransactionReceipt(context.Background(), tx.Hash())
	if err != nil {
		e.t.Fatal(err)
	}
	if rcpt.Status != types.ReceiptStatusSuccessful {
		return common.Address{}, errors.New("deployment reverted")
	}
	return addr, nil
}

func (e *ownerEnv) mustDeploy(rel string, args ...interface{}) common.Address {
	e.t.Helper()
	addr, err := e.deploy(rel, args...)
	if err != nil {
		e.t.Fatalf("deploy %s: %v", rel, err)
	}
	return addr
}

// send simulates the call to read a custom error, then mines it. It returns
// the custom error's name, or "" on success.
func (e *ownerEnv) send(method string, args ...interface{}) string {
	e.t.Helper()
	data, err := e.abi.Pack(method, args...)
	if err != nil {
		e.t.Fatalf("pack %s: %v", method, err)
	}
	auth := e.auth()
	msg := ethereum.CallMsg{From: auth.From, To: &e.enygma, Gas: auth.GasLimit, Data: data}
	if _, err := e.backend.Client().CallContract(context.Background(), msg, nil); err != nil {
		var de rpc.DataError
		if errors.As(err, &de) {
			if s, ok := de.ErrorData().(string); ok {
				raw, _ := hex.DecodeString(strings.TrimPrefix(s, "0x"))
				for name, ce := range e.abi.Errors {
					if len(raw) >= 4 && string(ce.ID[:4]) == string(raw[:4]) {
						return name
					}
				}
			}
		}
		return err.Error()
	}
	tx, err := bind.NewBoundContract(e.enygma, e.abi, nil, e.backend.Client(), nil).RawTransact(auth, data)
	if err != nil {
		e.t.Fatalf("send %s: %v", method, err)
	}
	e.backend.Commit()
	rcpt, err := e.backend.Client().TransactionReceipt(context.Background(), tx.Hash())
	if err != nil || rcpt.Status != types.ReceiptStatusSuccessful {
		e.t.Fatalf("%s mined but reverted (%v)", method, err)
	}
	return ""
}

func (e *ownerEnv) mustSend(method string, args ...interface{}) {
	e.t.Helper()
	if r := e.send(method, args...); r != "" {
		e.t.Fatalf("%s: %s", method, r)
	}
}

func (e *ownerEnv) call(method string, args ...interface{}) []interface{} {
	e.t.Helper()
	var out []interface{}
	if err := bind.NewBoundContract(e.enygma, e.abi, e.backend.Client(), nil, nil).Call(nil, &out, method, args...); err != nil {
		e.t.Fatalf("call %s: %v", method, err)
	}
	return out
}

func (e *ownerEnv) addr(method string, args ...interface{}) common.Address {
	e.t.Helper()
	return e.call(method, args...)[0].(common.Address)
}

func (e *ownerEnv) wait(d time.Duration) {
	e.t.Helper()
	if err := e.backend.AdjustTime(d); err != nil {
		e.t.Fatal(err)
	}
	e.backend.Commit()
}

const hour = int64(3600)

func TestEnygmaOwner_VerifierReplacementWaitsForDelay(t *testing.T) {
	e := newOwnerEnv(t)
	e.enygma = e.mustDeploy("Enygma.sol/Enygma.json", big.NewInt(1), big.NewInt(hour))
	first := e.mustDeploy("mocks/MockTransferVerifier.sol/MockTransferVerifier.json")
	second := e.mustDeploy("mocks/MockTransferVerifier.sol/MockTransferVerifier.json")
	third := e.mustDeploy("mocks/MockTransferVerifier.sol/MockTransferVerifier.json")

	// First-time setup applies at once.
	e.mustSend("addVerifier", first)
	if got := e.addr("VerifierAddress"); got != first {
		t.Fatalf("first addVerifier: verifier %s, want %s", got.Hex(), first.Hex())
	}

	// A replacement is only proposed.
	e.mustSend("addVerifier", second)
	if got := e.addr("VerifierAddress"); got != first {
		t.Fatalf("replacement applied at once: verifier %s, want %s until the delay passes", got.Hex(), first.Hex())
	}
	pending := e.call("pendingVerifierChange", kindTransfer, big.NewInt(0))
	if pending[0].(common.Address) != second {
		t.Fatalf("pending target %s, want %s", pending[0].(common.Address).Hex(), second.Hex())
	}
	if r := e.send("executeVerifierChange", kindTransfer, big.NewInt(0)); r != "VerifierChangeNotReady" {
		t.Fatalf("execute before the delay: got %q, want VerifierChangeNotReady", r)
	}

	e.wait(time.Duration(hour+1) * time.Second)
	e.mustSend("executeVerifierChange", kindTransfer, big.NewInt(0))
	if got := e.addr("VerifierAddress"); got != second {
		t.Fatalf("after execute: verifier %s, want %s", got.Hex(), second.Hex())
	}
	if r := e.send("executeVerifierChange", kindTransfer, big.NewInt(0)); r != "NoPendingVerifierChange" {
		t.Fatalf("second execute: got %q, want NoPendingVerifierChange", r)
	}

	// A cancelled proposal never applies.
	e.mustSend("addVerifier", third)
	e.mustSend("cancelVerifierChange", kindTransfer, big.NewInt(0))
	e.wait(time.Duration(hour+1) * time.Second)
	if r := e.send("executeVerifierChange", kindTransfer, big.NewInt(0)); r != "NoPendingVerifierChange" {
		t.Fatalf("execute after cancel: got %q, want NoPendingVerifierChange", r)
	}
	if got := e.addr("VerifierAddress"); got != second {
		t.Fatalf("after cancel: verifier %s, want %s", got.Hex(), second.Hex())
	}

	// Withdraw slots are per split count: a new split count is first-time setup.
	w1 := e.mustDeploy("mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
	w2 := e.mustDeploy("mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
	e.mustSend("addWithdrawVerifier", w1, big.NewInt(6))
	e.mustSend("addWithdrawVerifier", w2, big.NewInt(6))
	if got := e.addr("WithdrawVerifierAddress"); got != w1 {
		t.Fatalf("withdraw replacement applied at once: %s, want %s", got.Hex(), w1.Hex())
	}
	if p := e.call("pendingVerifierChange", kindWithdraw, big.NewInt(6)); p[0].(common.Address) != w2 {
		t.Fatalf("pending withdraw target %s, want %s", p[0].(common.Address).Hex(), w2.Hex())
	}

	// The ZkDvp bridge is delayed the same way, and must be a contract.
	if r := e.send("addZkDvp", common.HexToAddress("0x00000000000000000000000000000000000000d1")); r != "VerifierHasNoCode" {
		t.Fatalf("addZkDvp with no code: got %q, want VerifierHasNoCode", r)
	}
	z1 := e.mustDeploy("mocks/MockZkDvp.sol/MockZkDvp.json")
	z2 := e.mustDeploy("mocks/MockZkDvp.sol/MockZkDvp.json")
	e.mustSend("addZkDvp", z1)
	e.mustSend("addZkDvp", z2)
	if got := e.addr("ZkdvpAddress"); got != z1 {
		t.Fatalf("ZkDvp replacement applied at once: %s, want %s", got.Hex(), z1.Hex())
	}
	if p := e.call("pendingVerifierChange", kindZkDvp, big.NewInt(0)); p[0].(common.Address) != z2 {
		t.Fatalf("pending ZkDvp target %s, want %s", p[0].(common.Address).Hex(), z2.Hex())
	}
}

func TestEnygmaOwner_DelayIsSetAtDeploy(t *testing.T) {
	e := newOwnerEnv(t)
	if _, err := e.deploy("Enygma.sol/Enygma.json", big.NewInt(1), big.NewInt(31*24*hour)); err == nil {
		t.Fatal("deployed with a verifierChangeDelay above MAX_VERIFIER_CHANGE_DELAY")
	}

	// A zero delay (local development) applies replacements at once.
	e.enygma = e.mustDeploy("Enygma.sol/Enygma.json", big.NewInt(1), big.NewInt(0))
	first := e.mustDeploy("mocks/MockTransferVerifier.sol/MockTransferVerifier.json")
	second := e.mustDeploy("mocks/MockTransferVerifier.sol/MockTransferVerifier.json")
	e.mustSend("addVerifier", first)
	e.mustSend("addVerifier", second)
	if got := e.addr("VerifierAddress"); got != second {
		t.Fatalf("zero delay: verifier %s, want %s", got.Hex(), second.Hex())
	}
}

func TestEnygmaOwner_UsdrFixedFeeBound(t *testing.T) {
	e := newOwnerEnv(t)
	e.enygma = e.mustDeploy("Enygma.sol/Enygma.json", big.NewInt(1), big.NewInt(hour))
	limit := new(big.Int).Lsh(big.NewInt(1), 64)
	if r := e.send("setUsdrFixedFee", limit); r != "InvalidFeeAmount" {
		t.Fatalf("setUsdrFixedFee(2^64): got %q, want InvalidFeeAmount", r)
	}
	max := new(big.Int).Sub(limit, big.NewInt(1))
	e.mustSend("setUsdrFixedFee", max)
	if got := e.call("usdrFixedFeeAmount")[0].(*big.Int); got.Cmp(max) != 0 {
		t.Fatalf("usdrFixedFeeAmount %s, want %s", got, max)
	}
}

func TestEnygmaOwner_RegisterAccountAddressChecks(t *testing.T) {
	e := newOwnerEnv(t)
	e.enygma = e.mustDeploy("Enygma.sol/Enygma.json", big.NewInt(1), big.NewInt(hour))
	e.mustSend("initialize")

	register := func(addr common.Address, id int64) string {
		return e.send("registerAccount", addr, big.NewInt(id), big.NewInt(1000+id), testG.X, testG.Y, []byte{})
	}
	if r := register(common.Address{}, 1); r != "ZeroAddress" {
		t.Fatalf("registerAccount(0x0): got %q, want ZeroAddress", r)
	}
	bank := common.HexToAddress("0x00000000000000000000000000000000000000b1")
	if r := register(bank, 1); r != "" {
		t.Fatalf("registerAccount: %s", r)
	}
	if r := register(bank, 2); r != "AlreadyRegistered" {
		t.Fatalf("second account for the same address: got %q, want AlreadyRegistered", r)
	}
	if got := e.call("addressToAccountId", bank)[0].(*big.Int); got.Int64() != 1 {
		t.Fatalf("addressToAccountId = %s, want 1", got)
	}
	if r := register(common.HexToAddress("0x00000000000000000000000000000000000000b2"), 2); r != "" {
		t.Fatalf("registerAccount for another address: %s", r)
	}
}
