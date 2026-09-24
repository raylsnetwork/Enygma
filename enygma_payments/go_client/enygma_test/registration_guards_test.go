package enygma_test

// Registration and mint guards.
//
// Every per-account loop in Enygma.sol (check(), checkUsdr(), getPublicValues,
// balance propagation) iterates account ids 1.._totalRegisteredParties, and
// every commitment must match a proof public signal, which the verifier
// forces below the base field. So the contract must refuse:
//
//   - registerAccount with an id that is not exactly the next one, since a
//     skipped id sits outside every loop and breaks check() until each
//     skipped id is filled in;
//   - commitments with a coordinate >= Q, which isOnCurve alone accepts
//     (it reduces with mulmod) but which can never match a proof;
//   - minting to an unregistered account, which adds to the supply
//     commitment without adding to any summed balance (for USDr there is no
//     burn to repair it);
//   - mintUsdrSupply while paused, like mintSupply.
//
// Run:
//   ENYGMA_CHAIN_URL=http://127.0.0.1:8545 ENYGMA_CHAIN_ID=1337 \
//   MY_KEY=<hardhat key> go test -run TestRegistrationAndMintGuards -v .

import (
	"math/big"
	"strings"
	"testing"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

type guardEnv struct {
	inst *enygma.Enygma
	mk   func() *bind.TransactOpts
	wait func(*ethtypes.Transaction, error) *ethtypes.Receipt
}

func guardSetup(t *testing.T) guardEnv {
	t.Helper()
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mk, wait := scenarioClient(t)
	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	addr := deployFromArtifact(t, client, mk(), artifactBase+"/Enygma.sol/Enygma.json", big.NewInt(30))
	inst, err := enygma.NewEnygma(addr, client)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	wait(inst.Initialize(mk()))
	return guardEnv{inst, mk, wait}
}

func guardPK(id int64) *big.Int {
	pk, _ := poseidon.Hash([]*big.Int{big.NewInt(3000 + id), big.NewInt(3000 + id)})
	return pk.Mod(pk, curveP)
}

// register sends registerAccount for id with a valid commitment (or the given
// override) and returns the send error, if any.
func (e guardEnv) register(id int64, cx, cy *big.Int) error {
	if cx == nil {
		cx, cy = regCommit(big.NewInt(100 + id))
	}
	var a common.Address
	a[19] = byte(id)
	tx, err := e.inst.RegisterAccount(e.mk(), a, big.NewInt(id), guardPK(id), cx, cy, []byte{})
	if err != nil {
		return err
	}
	e.wait(tx, nil)
	return nil
}

func wantErr(t *testing.T, what, name string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected revert %s, but the call succeeded", what, name)
		return
	}
	if !strings.Contains(err.Error(), name) {
		t.Errorf("%s: expected %s, got: %v", what, name, err)
		return
	}
	t.Logf("%s: correctly reverted with %s", what, name)
}

func TestRegistrationAndMintGuards(t *testing.T) {
	t.Run("AccountIdsMustBeSequential", func(t *testing.T) {
		e := guardSetup(t)
		wantErr(t, "first account with id 3", "InvalidAccountId", e.register(3, nil, nil))
		if err := e.register(1, nil, nil); err != nil {
			t.Fatalf("id 1 as the first account: %v", err)
		}
		wantErr(t, "id 3 after id 1 (skipping 2)", "InvalidAccountId", e.register(3, nil, nil))
		if err := e.register(2, nil, nil); err != nil {
			t.Fatalf("id 2 after id 1: %v", err)
		}
		if ok, err := e.inst.Check(&bind.CallOpts{}); err != nil || !ok {
			t.Fatalf("check() failed after sequential registration: ok=%v err=%v", ok, err)
		}
	})

	t.Run("UnreducedCommitmentRejected", func(t *testing.T) {
		e := guardSetup(t)
		q, _ := new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)
		x, y := regCommit(big.NewInt(7))
		wantErr(t, "registerAccount with x+Q", "InvalidCommitmentPoint",
			e.register(1, new(big.Int).Add(x, q), y))
		wantErr(t, "registerAccount with y+Q", "InvalidCommitmentPoint",
			e.register(1, x, new(big.Int).Add(y, q)))
		if err := e.register(1, x, y); err != nil {
			t.Fatalf("a reduced, valid commitment was rejected: %v", err)
		}
		mx, my := mintCommitPt(big.NewInt(5), big.NewInt(9))
		_, err := e.inst.MintSupply(e.mk(), big.NewInt(5), big.NewInt(1),
			new(big.Int).Add(mx, q), my)
		wantErr(t, "mintSupply with x+Q", "InvalidCommitmentPoint", err)
	})

	t.Run("MintToUnregisteredRejected", func(t *testing.T) {
		e := guardSetup(t)
		if err := e.register(1, nil, nil); err != nil {
			t.Fatalf("register 1: %v", err)
		}
		mx, my := mintCommitPt(big.NewInt(5), big.NewInt(9))
		_, err := e.inst.MintSupply(e.mk(), big.NewInt(5), big.NewInt(999), mx, my)
		wantErr(t, "mintSupply to an unregistered id", "UnregisteredParticipant", err)

		_, err = e.inst.MintUsdrSupply(e.mk(), big.NewInt(10), big.NewInt(999))
		wantErr(t, "mintUsdrSupply to an unregistered id", "UnregisteredParticipant", err)

		// The rejected mints must not have touched the supply invariants.
		if ok, err := e.inst.Check(&bind.CallOpts{}); err != nil || !ok {
			t.Fatalf("check() broken by a rejected mint: ok=%v err=%v", ok, err)
		}
		if ok, err := e.inst.CheckUsdr(&bind.CallOpts{}); err != nil || !ok {
			t.Fatalf("checkUsdr() broken by a rejected mint: ok=%v err=%v", ok, err)
		}
	})

	t.Run("MintUsdrHonorsPauseAndKeepsInvariant", func(t *testing.T) {
		e := guardSetup(t)
		if err := e.register(1, nil, nil); err != nil {
			t.Fatalf("register 1: %v", err)
		}
		tx, err := e.inst.MintUsdrSupply(e.mk(), big.NewInt(10), big.NewInt(1))
		if err != nil {
			t.Fatalf("mintUsdrSupply to a registered account failed: %v", err)
		}
		e.wait(tx, nil)
		if ok, err := e.inst.CheckUsdr(&bind.CallOpts{}); err != nil || !ok {
			t.Fatalf("checkUsdr() failed after a valid mint: ok=%v err=%v", ok, err)
		}

		e.wait(e.inst.Pause(e.mk()))
		_, err = e.inst.MintUsdrSupply(e.mk(), big.NewInt(10), big.NewInt(1))
		wantErr(t, "mintUsdrSupply while paused", "ContractIsPaused", err)
	})
}
