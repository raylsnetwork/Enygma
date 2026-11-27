package tags

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type registryEntry struct {
	addr    common.Address
	pkSpend int64
	pkView  []byte
}

type registryRPC struct {
	abi        abi.ABI
	entries    []registryEntry
	failMethod string
}

// Call serves UserRegistry reads over an in-process Ethereum RPC client.
func (r *registryRPC) Call(_ context.Context, call struct{ Input hexutil.Bytes }, _ string) (hexutil.Bytes, error) {
	method, err := r.abi.MethodById(call.Input)
	if err != nil {
		return nil, err
	}
	args, err := method.Inputs.Unpack(call.Input[4:])
	if err != nil {
		return nil, err
	}
	switch method.Name {
	case "getUserCount":
		return method.Outputs.Pack(big.NewInt(int64(len(r.entries))))
	case "getUserAt":
		index := args[0].(*big.Int).Int64()
		if r.failMethod == method.Name && index == 1 {
			return nil, errors.New("test registry read failed")
		}
		return method.Outputs.Pack(r.entries[index].addr)
	case "getKeys":
		for i, entry := range r.entries {
			if entry.addr != args[0].(common.Address) {
				continue
			}
			if r.failMethod == method.Name && i == 1 {
				return nil, errors.New("test registry read failed")
			}
			return method.Outputs.Pack(big.NewInt(entry.pkSpend), entry.pkView)
		}
	}
	return nil, fmt.Errorf("unexpected registry call: %s", method.Name)
}

func registryTestClient(t *testing.T, entries []registryEntry, failMethod string) *ethclient.Client {
	t.Helper()
	contractABI, err := loadUserRegistryABI()
	if err != nil {
		t.Fatal(err)
	}
	server := rpc.NewServer()
	t.Cleanup(server.Stop)
	if err := server.RegisterName("eth", &registryRPC{contractABI, entries, failMethod}); err != nil {
		t.Fatal(err)
	}
	client := ethclient.NewClient(rpc.DialInProc(server))
	t.Cleanup(client.Close)
	return client
}

func TestFindRecipientByPkSpendReportsEveryMatchingAddress(t *testing.T) {
	entries := []registryEntry{
		{common.HexToAddress("0x12"), 42, []byte{1}},
		{common.HexToAddress("0x34"), 15, []byte{2}},
		{common.HexToAddress("0x56"), 42, []byte{3}},
		{common.HexToAddress("0x78"), 42, []byte{4}},
	}
	registry := common.HexToAddress("0x100")
	for _, tc := range []struct {
		name    string
		entries []registryEntry
	}{
		{"two_matches", entries[:3]},
		{"three_matches", entries},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := registryTestClient(t, tc.entries, "")
			addr, pkView, index, err := FindRecipientByPkSpend(client, registry, big.NewInt(42))
			if err == nil || !strings.Contains(err.Error(), "ambiguous pkSpend") {
				t.Fatalf("want ambiguity error, got address %s and error %v", addr, err)
			}
			for _, entry := range tc.entries {
				named := strings.Contains(err.Error(), entry.addr.Hex())
				if want := entry.pkSpend == 42; named != want {
					t.Errorf("address %s named = %t, want %t: %v", entry.addr, named, want, err)
				}
			}
			if addr != (common.Address{}) || pkView != nil || index != 0 {
				t.Errorf("ambiguous lookup returned recipient data: %s, %x, %d", addr, pkView, index)
			}

			// A caller with a known address gets that user's view key.
			pkView, err = GetRecipientPkView(client, registry, tc.entries[2].addr)
			if err != nil || !bytes.Equal(pkView, tc.entries[2].pkView) {
				t.Fatalf("address lookup got view key %x, error %v", pkView, err)
			}
		})
	}
}

func TestFindRecipientByPkSpendReturnsUniqueMatch(t *testing.T) {
	entries := []registryEntry{
		{common.HexToAddress("0x12"), 15, []byte{1}},
		{common.HexToAddress("0x34"), 42, []byte{2}},
	}
	client := registryTestClient(t, entries, "")
	addr, pkView, index, err := FindRecipientByPkSpend(client, common.HexToAddress("0x100"), big.NewInt(42))
	if err != nil || addr != entries[1].addr || !bytes.Equal(pkView, entries[1].pkView) || index != 1 {
		t.Fatalf("got address %s, view key %x, index %d, error %v", addr, pkView, index, err)
	}
}

func TestFindRecipientByPkSpendFailsOnAnIncompleteScan(t *testing.T) {
	entries := []registryEntry{
		{common.HexToAddress("0x12"), 42, []byte{1}},
		{common.HexToAddress("0x34"), 15, []byte{2}},
	}
	for _, method := range []string{"getUserAt", "getKeys"} {
		t.Run(method, func(t *testing.T) {
			client := registryTestClient(t, entries, method)
			_, _, _, err := FindRecipientByPkSpend(client, common.HexToAddress("0x100"), big.NewInt(42))
			if err == nil || !strings.Contains(err.Error(), method) || !strings.Contains(err.Error(), "test registry read failed") {
				t.Fatalf("want %s read error, got %v", method, err)
			}
		})
	}
}
