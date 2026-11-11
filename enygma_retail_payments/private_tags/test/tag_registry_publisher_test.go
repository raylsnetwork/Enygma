package tags_test

// TagRegistry's duplicate guard is per publisher: one address publishing a
// tag must not stop another address from publishing the same tag in the same
// block, while one address still can't publish the same tag twice in a block.
// Needs a local node (hardhat node or anvil, chain id 1337, the hardhat
// accounts) on :8545, or at TAGS_TEST_RPC. It pauses automine to land three
// publishes in one block, then turns it back on.

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestTagRegistry_DuplicateGuardIsPerPublisher(t *testing.T) {
	ctx := context.Background()
	url := hardhatRPC
	if v := os.Getenv("TAGS_TEST_RPC"); v != "" {
		url = v
	}
	client, err := ethclient.DialContext(ctx, url)
	if err != nil {
		t.Skipf("no node at %s: %v", url, err)
	}
	defer client.Close()
	if _, err := client.ChainID(ctx); err != nil {
		t.Skipf("no node at %s: %v", url, err)
	}
	authA := hardhatAuthFromKey(t, alicePrivKeyHex)
	authB := hardhatAuthFromKey(t, bobPrivKeyHex)

	addr, err := tags.DeployTagRegistry(client, authA, "../contracts/TagRegistry.json")
	if err != nil {
		t.Fatalf("DeployTagRegistry: %v", err)
	}
	data, err := os.ReadFile("../contracts/TagRegistry.json")
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	registryABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		t.Fatal(err)
	}
	registry := bind.NewBoundContract(addr, registryABI, client, client, client)

	rpc := client.Client()
	if err := rpc.CallContext(ctx, nil, "evm_setAutomine", false); err != nil {
		t.Skipf("node can't pause automine: %v", err)
	}
	defer func() { _ = rpc.CallContext(ctx, nil, "evm_setAutomine", true) }()

	tag := [32]byte{1, 2, 3}
	txA, err := registry.Transact(authA, "publishTag", tag, []byte("from A"))
	if err != nil {
		t.Fatal(err)
	}
	txB, err := registry.Transact(authB, "publishTag", tag, []byte("from B"))
	if err != nil {
		t.Fatal(err)
	}
	txA2, err := registry.Transact(authA, "publishTag", tag, []byte("from A again"))
	if err != nil {
		t.Fatal(err)
	}
	if err := rpc.CallContext(ctx, nil, "evm_mine"); err != nil {
		t.Fatal(err)
	}

	receipt := func(tx *types.Transaction) *types.Receipt {
		r, err := bind.WaitMined(ctx, client, tx)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	rA, rB, rA2 := receipt(txA), receipt(txB), receipt(txA2)
	block := rA.BlockNumber
	if block.Cmp(rB.BlockNumber) != 0 || block.Cmp(rA2.BlockNumber) != 0 {
		t.Fatalf("publishes landed in different blocks (%v, %v, %v)", block, rB.BlockNumber, rA2.BlockNumber)
	}
	if rA.Status != types.ReceiptStatusSuccessful {
		t.Error("first publish by A reverted")
	}
	if rB.Status != types.ReceiptStatusSuccessful {
		t.Error("B publishing a tag A already published in this block reverted")
	}
	if rA2.Status != types.ReceiptStatusFailed {
		t.Error("A publishing the same tag twice in one block succeeded")
	}

	var out []interface{}
	if err := registry.Call(&bind.CallOpts{}, &out, "getCount", block); err != nil {
		t.Fatal(err)
	}
	if n := out[0].(*big.Int); n.Int64() != 2 {
		t.Errorf("getCount = %v, want 2", n)
	}
}
