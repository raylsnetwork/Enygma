package tags

// UserRegistry helpers for the private-tag channel setup.
//
// The channel setup bitmap requires knowing the total number of registered users
// and the recipient's 0-based index in the registry. The sender also needs the
// recipient's ML-KEM-768 encapsulation key (pk_view) to encapsulate the shared
// secret. Both pieces of information live in the on-chain UserRegistry.
//
// Typical sender flow:
//
//	ss, c1, c2, bitmap, err := PrepareChannelSetupForRecipient(
//	    client, userRegistryAddr,
//	    recipientAddr,       // recipient's Ethereum address (known from prior contact)
//	    message, senderId,
//	    tags.PrivacySubset,
//	    nil,
//	)
//	// POST (c1, c2, bitmap) to POST /relay/channel
//
// If you only know the recipient's pkSpend (e.g., from scanning payment events),
// use FindRecipientByPkSpend first to resolve their Ethereum address.

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ── UserRegistry read helpers ─────────────────────────────────────────────────

// GetUserCount returns the total number of registered users in UserRegistry.
// Pass this as totalUsers to PrepareChannelSetup for bitmap construction.
func GetUserCount(client *ethclient.Client, registryAddr common.Address) (int, error) {
	a, err := loadUserRegistryABI()
	if err != nil {
		return 0, err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)
	var out []interface{}
	if err := c.Call(&bind.CallOpts{}, &out, "getUserCount"); err != nil {
		return 0, fmt.Errorf("UserRegistry.getUserCount: %w", err)
	}
	count, ok := out[0].(*big.Int)
	if !ok {
		return 0, fmt.Errorf("getUserCount: unexpected type %T", out[0])
	}
	return int(count.Int64()), nil
}

// GetRecipientIndex returns the 0-based registration index for a registered user.
// Pass this as recipientIdx to PrepareChannelSetup for bitmap construction.
func GetRecipientIndex(client *ethclient.Client, registryAddr common.Address, recipient common.Address) (int, error) {
	a, err := loadUserRegistryABI()
	if err != nil {
		return 0, err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)
	var out []interface{}
	if err := c.Call(&bind.CallOpts{}, &out, "getUserIndex", recipient); err != nil {
		return 0, fmt.Errorf("UserRegistry.getUserIndex(%s): %w", recipient.Hex(), err)
	}
	idx, ok := out[0].(uint32)
	if !ok {
		return 0, fmt.Errorf("getUserIndex: unexpected type %T", out[0])
	}
	return int(idx), nil
}

// GetRecipientPkView returns the ML-KEM-768 encapsulation key (pk_view, 1184 bytes)
// for a registered user. This is the key the sender must use in PrepareChannelSetup
// to encapsulate the shared secret for the recipient.
func GetRecipientPkView(client *ethclient.Client, registryAddr common.Address, recipient common.Address) ([]byte, error) {
	a, err := loadUserRegistryABI()
	if err != nil {
		return nil, err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)
	var out []interface{}
	if err := c.Call(&bind.CallOpts{}, &out, "getKeys", recipient); err != nil {
		return nil, fmt.Errorf("UserRegistry.getKeys(%s): %w", recipient.Hex(), err)
	}
	if len(out) < 2 {
		return nil, fmt.Errorf("getKeys: expected 2 return values, got %d", len(out))
	}
	pkView, ok := out[1].([]byte)
	if !ok {
		return nil, fmt.Errorf("getKeys: unexpected pkView type %T", out[1])
	}
	return pkView, nil
}

// FindRecipientByPkSpend scans all registered users and returns the Ethereum
// address, pk_view, and registration index of the user whose pkSpend matches.
//
// Use this when you know the recipient's ZK identity (pkSpend) — e.g., from
// a Commitment event — but not their Ethereum address.
//
// Complexity: O(N) where N = total registered users. For large registries this
// is an off-chain operation; it never sends a transaction.
func FindRecipientByPkSpend(
	client *ethclient.Client,
	registryAddr common.Address,
	pkSpend *big.Int,
) (addr common.Address, pkView []byte, index int, err error) {
	a, err := loadUserRegistryABI()
	if err != nil {
		return common.Address{}, nil, 0, err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)

	var countOut []interface{}
	if err := c.Call(&bind.CallOpts{}, &countOut, "getUserCount"); err != nil {
		return common.Address{}, nil, 0, fmt.Errorf("getUserCount: %w", err)
	}
	total := int(countOut[0].(*big.Int).Int64())

	for i := 0; i < total; i++ {
		var addrOut []interface{}
		if err := c.Call(&bind.CallOpts{}, &addrOut, "getUserAt", big.NewInt(int64(i))); err != nil {
			continue
		}
		candidate, ok := addrOut[0].(common.Address)
		if !ok {
			continue
		}

		var keysOut []interface{}
		if err := c.Call(&bind.CallOpts{}, &keysOut, "getKeys", candidate); err != nil {
			continue
		}
		sp, ok := keysOut[0].(*big.Int)
		if !ok {
			continue
		}
		if sp.Cmp(pkSpend) == 0 {
			pv, _ := keysOut[1].([]byte)
			return candidate, pv, i, nil
		}
	}
	return common.Address{}, nil, 0, fmt.Errorf("no user found with pkSpend=%s", pkSpend.String())
}

// ── Convenience wrapper ───────────────────────────────────────────────────────

// PrepareChannelSetupForRecipient is a convenience wrapper around PrepareChannelSetup
// that looks up totalUsers, recipientIdx, and pk_view from UserRegistry automatically.
//
// It resolves gaps 1 and 2 from the paper implementation:
//
//	Gap 1 — bitmap needs totalUsers and recipientIdx from the registry
//	Gap 2 — sender needs recipient's pk_view from the registry
//
// The caller only needs to know the recipient's Ethereum address (common.Address).
// Returns (sharedSecret, c1, c2, bitmap) ready to POST to /relay/channel.
func PrepareChannelSetupForRecipient(
	client *ethclient.Client,
	userRegistryAddr common.Address,
	recipientAddr common.Address,
	message []byte,
	senderId []byte,
	mode PrivacyMode,
	excludedIndices []int,
) (sharedSecret, c1, c2, bitmap []byte, err error) {
	totalUsers, err := GetUserCount(client, userRegistryAddr)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("GetUserCount: %w", err)
	}
	if totalUsers == 0 {
		return nil, nil, nil, nil, fmt.Errorf("UserRegistry is empty — recipient is not registered")
	}

	recipientIdx, err := GetRecipientIndex(client, userRegistryAddr, recipientAddr)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("GetRecipientIndex(%s): %w", recipientAddr.Hex(), err)
	}

	pkView, err := GetRecipientPkView(client, userRegistryAddr, recipientAddr)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("GetRecipientPkView(%s): %w", recipientAddr.Hex(), err)
	}

	return PrepareChannelSetup(pkView, message, senderId, mode, totalUsers, recipientIdx, excludedIndices)
}

// ── ABI loader ────────────────────────────────────────────────────────────────

func loadUserRegistryABI() (abi.ABI, error) {
	dir, err := os.Getwd()
	if err != nil {
		return abi.ABI{}, err
	}
	for {
		// Try the full artifact layout first (contracts/user_registry/UserRegistry.json)
		for _, candidate := range []string{
			filepath.Join(dir, "contracts", "user_registry", "UserRegistry.json"),
			filepath.Join(dir, "contracts", "abis", "UserRegistry.json"),
		} {
			if data, err := os.ReadFile(candidate); err == nil {
				var artifact struct {
					ABI json.RawMessage `json:"abi"`
				}
				if err := json.Unmarshal(data, &artifact); err != nil {
					return abi.ABI{}, fmt.Errorf("parse UserRegistry artifact: %w", err)
				}
				return abi.JSON(strings.NewReader(string(artifact.ABI)))
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return abi.ABI{}, fmt.Errorf("UserRegistry.json not found (searched contracts/user_registry/ and contracts/abis/ up from %s)", dir)
}
