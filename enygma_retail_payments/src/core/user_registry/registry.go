package userregistry

import (
	"context"
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

// UserKeys holds the public keys a sender needs to pay a recipient, plus the
// audit ciphertexts published at registration for the auditor to recover sk_view.
type UserKeys struct {
	SpendKey    *big.Int // pk_spend    — Poseidon(sk_spend)
	ViewKey     []byte   // pk_view     — ML-KEM-768 encapsulation key (1184 bytes)
	MlKemAuditCt []byte // mlKemAuditCt — ML-KEM capsule for auditor (1088 bytes)
	AesAuditCt   []byte // aesAuditCt   — AES-GCM encrypted sk_view seed (92 bytes)
}

// Register calls UserRegistry.register(pkSpend, pkView, mlKemAuditCt, aesAuditCt) on-chain.
// Each address can only register once — returns AlreadyRegistered on repeat calls.
//
// mlKemAuditCt and aesAuditCt are produced by auditor.EncryptViewKeyForAuditor and
// bind the user's sk_view to the auditor's public key via AEAD (mlKemAuditCt as AAD).
//
// Sybil protection: the function automatically queries the current registrationFee
// and sends the exact amount with the transaction. When the fee is 0 (default),
// no ETH is sent and existing behaviour is unchanged.
func Register(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	registryAddr common.Address,
	pkSpend *big.Int,
	pkView []byte,
	mlKemAuditCt []byte,
	aesAuditCt []byte,
) error {
	a, err := loadABI()
	if err != nil {
		return err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)

	// Query current fee — send it with the transaction so the call never
	// underpays even when the owner raises the fee between builds.
	fee, err := GetRegistrationFee(client, registryAddr)
	if err != nil {
		return fmt.Errorf("registrationFee query: %w", err)
	}

	// Use a local copy of auth to avoid mutating the caller's transactor.
	authCopy := *auth
	authCopy.Value = fee

	tx, err := c.Transact(&authCopy, "register", pkSpend, pkView, mlKemAuditCt, aesAuditCt)
	if err != nil {
		return fmt.Errorf("UserRegistry.register: %w", err)
	}
	_, err = bind.WaitMined(context.Background(), client, tx)
	return err
}

// GetRegistrationFee returns the current ETH fee required per registration.
// Returns zero when Sybil protection is disabled (default state).
func GetRegistrationFee(client *ethclient.Client, registryAddr common.Address) (*big.Int, error) {
	a, err := loadABI()
	if err != nil {
		return nil, err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)
	var out []interface{}
	if err := c.Call(&bind.CallOpts{}, &out, "registrationFee"); err != nil {
		return nil, fmt.Errorf("UserRegistry.registrationFee: %w", err)
	}
	fee, ok := out[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected type for registrationFee")
	}
	return fee, nil
}

// SetRegistrationFee sets the ETH fee required per registration.
// Only the contract owner (deployer) can call this.
// Set fee to 0 to re-enable free registration.
func SetRegistrationFee(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	registryAddr common.Address,
	fee *big.Int,
) error {
	a, err := loadABI()
	if err != nil {
		return err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)
	tx, err := c.Transact(auth, "setRegistrationFee", fee)
	if err != nil {
		return fmt.Errorf("UserRegistry.setRegistrationFee: %w", err)
	}
	_, err = bind.WaitMined(context.Background(), client, tx)
	return err
}

// WithdrawFees transfers all accumulated registration fees to the owner.
// Only the contract owner can call this.
func WithdrawFees(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	registryAddr common.Address,
) error {
	a, err := loadABI()
	if err != nil {
		return err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)
	tx, err := c.Transact(auth, "withdrawFees")
	if err != nil {
		return fmt.Errorf("UserRegistry.withdrawFees: %w", err)
	}
	_, err = bind.WaitMined(context.Background(), client, tx)
	return err
}

// LookupKeys reads the spend and view keys for a registered address.
func LookupKeys(
	client *ethclient.Client,
	registryAddr common.Address,
	user common.Address,
) (*UserKeys, error) {
	a, err := loadABI()
	if err != nil {
		return nil, err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)
	var result []interface{}
	if err := c.Call(&bind.CallOpts{}, &result, "getKeys", user); err != nil {
		return nil, fmt.Errorf("UserRegistry.getKeys(%s): %w", user.Hex(), err)
	}
	pkSpend, ok := result[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected type for pkSpend")
	}
	pkView, ok := result[1].([]byte)
	if !ok {
		return nil, fmt.Errorf("unexpected type for pkView")
	}
	return &UserKeys{SpendKey: pkSpend, ViewKey: pkView}, nil
}

// LookupAuditCiphertexts reads the audit ciphertext pair for a registered address.
// The auditor calls this to recover the user's sk_view via AuditorDecryptViewKey.
func LookupAuditCiphertexts(
	client *ethclient.Client,
	registryAddr common.Address,
	user common.Address,
) (mlKemAuditCt, aesAuditCt []byte, err error) {
	a, err := loadABI()
	if err != nil {
		return nil, nil, err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)
	var result []interface{}
	if err := c.Call(&bind.CallOpts{}, &result, "getAuditCiphertexts", user); err != nil {
		return nil, nil, fmt.Errorf("UserRegistry.getAuditCiphertexts(%s): %w", user.Hex(), err)
	}
	mlKemCt, ok := result[0].([]byte)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected type for mlKemAuditCt")
	}
	aesCt, ok := result[1].([]byte)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected type for aesAuditCt")
	}
	return mlKemCt, aesCt, nil
}

// GetUserCount returns the total number of registered users.
func GetUserCount(client *ethclient.Client, registryAddr common.Address) (int, error) {
	a, err := loadABI()
	if err != nil {
		return 0, err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)
	var result []interface{}
	if err := c.Call(&bind.CallOpts{}, &result, "getUserCount"); err != nil {
		return 0, fmt.Errorf("UserRegistry.getUserCount: %w", err)
	}
	count, ok := result[0].(*big.Int)
	if !ok {
		return 0, fmt.Errorf("unexpected type for getUserCount result")
	}
	return int(count.Int64()), nil
}

// GetUserIndex returns the 0-based registration index for a registered user.
func GetUserIndex(
	client *ethclient.Client,
	registryAddr common.Address,
	user common.Address,
) (int, error) {
	a, err := loadABI()
	if err != nil {
		return 0, err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)
	var result []interface{}
	if err := c.Call(&bind.CallOpts{}, &result, "getUserIndex", user); err != nil {
		return 0, fmt.Errorf("UserRegistry.getUserIndex(%s): %w", user.Hex(), err)
	}
	idx, ok := result[0].(uint32)
	if !ok {
		return 0, fmt.Errorf("unexpected type for getUserIndex result")
	}
	return int(idx), nil
}

// GetUserAt returns the address at the given 0-based registration index.
func GetUserAt(
	client *ethclient.Client,
	registryAddr common.Address,
	index int,
) (common.Address, error) {
	a, err := loadABI()
	if err != nil {
		return common.Address{}, err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)
	var result []interface{}
	if err := c.Call(&bind.CallOpts{}, &result, "getUserAt", big.NewInt(int64(index))); err != nil {
		return common.Address{}, fmt.Errorf("UserRegistry.getUserAt(%d): %w", index, err)
	}
	addr, ok := result[0].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("unexpected type for getUserAt result")
	}
	return addr, nil
}

// ── ABI loader ─────────────────────────────────────────────────────────────────

func loadABI() (abi.ABI, error) {
	dir, err := os.Getwd()
	if err != nil {
		return abi.ABI{}, err
	}
	for {
		candidate := filepath.Join(dir, "contracts", "user_registry", "UserRegistry.json")
		if data, err := os.ReadFile(candidate); err == nil {
			var artifact struct {
				ABI json.RawMessage `json:"abi"`
			}
			if err := json.Unmarshal(data, &artifact); err != nil {
				return abi.ABI{}, fmt.Errorf("parse UserRegistry artifact: %w", err)
			}
			return abi.JSON(strings.NewReader(string(artifact.ABI)))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return abi.ABI{}, fmt.Errorf("UserRegistry.json not found — compile UserRegistry.sol first")
}
