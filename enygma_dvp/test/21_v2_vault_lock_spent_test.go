package tests

// 21_v2_vault_lock_spent_test.go
//
// Merkle.lock (behind every DvP vault's lockCoin) must refuse a spent note.
// Every current caller checks the nullifier first, but lock() itself did not,
// so a caller that skipped that check (the legacy EnygmaAuction, a future one)
// could lock a spent note into a swap or bid that can never settle. Runs on
// go-ethereum's simulated backend against MerkleLockHarness; needs
// `npx hardhat compile`.

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
)

func TestV2VaultLock_RejectsSpentNote(t *testing.T) {
	harnessABI, harnessBin := loadArtifact(t, "test/MerkleLockHarness.sol/MerkleLockHarness.json")
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	backend := simulated.NewBackend(types.GenesisAlloc{from: {Balance: new(big.Int).Lsh(big.NewInt(1), 100)}})
	defer backend.Close()
	client := backend.Client()
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	auth.GasLimit = 3_000_000
	addr, _, harness, err := bind.DeployContract(auth, harnessABI, harnessBin, client)
	if err != nil {
		t.Fatal(err)
	}
	backend.Commit()

	// call simulates first (to read a revert reason), then mines.
	call := func(method string, args ...interface{}) string {
		t.Helper()
		data, err := harnessABI.Pack(method, args...)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.CallContract(context.Background(), ethereum.CallMsg{From: from, To: &addr, Gas: auth.GasLimit, Data: data}, nil); err != nil {
			return err.Error()
		}
		if _, err := harness.RawTransact(auth, data); err != nil {
			t.Fatal(err)
		}
		backend.Commit()
		return ""
	}
	tree := big.NewInt(0)

	if r := call("lockNote", tree, big.NewInt(7)); r != "" {
		t.Fatalf("locking an unspent note: %s", r)
	}
	if r := call("lockNote", tree, big.NewInt(7)); !strings.Contains(r, "already locked") {
		t.Fatalf("locking a locked note: got %q, want 'already locked'", r)
	}
	if r := call("spend", tree, big.NewInt(8)); r != "" {
		t.Fatalf("spend: %s", r)
	}
	if r := call("lockNote", tree, big.NewInt(8)); !strings.Contains(r, "already spent") {
		t.Fatalf("locking a spent note: got %q, want 'Merkle: Nullifier already spent.'", r)
	}
}
