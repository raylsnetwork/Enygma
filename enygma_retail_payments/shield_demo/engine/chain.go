package engine

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// chain is the run's connection to the deployment.
type chain struct {
	client  *ethclient.Client
	chainID *big.Int

	dvpAddr      common.Address
	vaultAddr    common.Address
	erc20Addr    common.Address
	registryAddr common.Address

	vault *bind.BoundContract
	erc20 *bind.BoundContract

	relayerAddr common.Address
}

func dialChain(cfg Config) (*chain, error) {
	receipts, err := loadReceipts(cfg.RetailRoot)
	if err != nil {
		return nil, err
	}
	addr := func(name string) (common.Address, error) {
		r, ok := receipts[name]
		if !ok || r.ContractAddress == "" {
			return common.Address{}, fmt.Errorf("%s missing from build/receipts.json (run deploy+init)", name)
		}
		return common.HexToAddress(r.ContractAddress), nil
	}
	c := &chain{chainID: big.NewInt(cfg.ChainID)}
	for name, dst := range map[string]*common.Address{
		"EnygmaDvp":      &c.dvpAddr,
		"Erc20CoinVault": &c.vaultAddr,
		"ERC20":          &c.erc20Addr,
		"UserRegistry":   &c.registryAddr,
	} {
		if *dst, err = addr(name); err != nil {
			return nil, err
		}
	}

	vaultABI, err := loadABI(cfg.RetailRoot, "Erc20CoinVault")
	if err != nil {
		return nil, err
	}
	erc20ABI, err := loadABI(cfg.RetailRoot, "RaylsERC20")
	if err != nil {
		return nil, err
	}
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", cfg.RPCURL, err)
	}
	c.client = client
	c.vault = bind.NewBoundContract(c.vaultAddr, vaultABI, client, client, client)
	c.erc20 = bind.NewBoundContract(c.erc20Addr, erc20ABI, client, client, client)
	return c, nil
}

func (c *chain) close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

type receiptEntry struct {
	ContractAddress string `json:"contractAddress"`
}

func loadReceipts(root string) (map[string]receiptEntry, error) {
	data, err := os.ReadFile(filepath.Join(root, "build", "receipts.json"))
	if err != nil {
		return nil, fmt.Errorf("read build/receipts.json: %w (run deploy+init first)", err)
	}
	var r map[string]receiptEntry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse build/receipts.json: %w", err)
	}
	return r, nil
}

func loadABI(root, contract string) (abi.ABI, error) {
	data, err := os.ReadFile(filepath.Join(root, "contracts", "abis", contract+".json"))
	if err != nil {
		return abi.ABI{}, fmt.Errorf("read %s ABI: %w", contract, err)
	}
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		return abi.ABI{}, fmt.Errorf("parse %s artifact: %w", contract, err)
	}
	return abi.JSON(strings.NewReader(string(artifact.ABI)))
}

// auth returns a transactor for the account keyHex controls.
func (c *chain) auth(keyHex string) (*bind.TransactOpts, error) {
	key, err := crypto.HexToECDSA(keyHex)
	if err != nil {
		return nil, fmt.Errorf("parse account key: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, c.chainID)
	if err != nil {
		return nil, err
	}
	auth.GasLimit = 8_000_000
	return auth, nil
}

// newAccount generates a fresh account key, so a run never collides with
// registrations already on the chain.
func newAccount() (keyHex string, addr common.Address, err error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return "", common.Address{}, err
	}
	return common.Bytes2Hex(crypto.FromECDSA(key)), crypto.PubkeyToAddress(key.PublicKey), nil
}

// fund sends wei from the funder's account to each of to, one mined
// transaction at a time.
func (c *chain) fund(ctx context.Context, funderHex string, wei *big.Int, to []common.Address) error {
	key, err := crypto.HexToECDSA(funderHex)
	if err != nil {
		return fmt.Errorf("parse funder key: %w", err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	signer := ethtypes.LatestSignerForChainID(c.chainID)
	for _, addr := range to {
		if err := c.sendValue(ctx, key, from, signer, addr, wei); err != nil {
			return fmt.Errorf("fund %s: %w", addr.Hex(), err)
		}
	}
	return nil
}

func (c *chain) sendValue(ctx context.Context, key *ecdsa.PrivateKey, from common.Address, signer ethtypes.Signer, to common.Address, wei *big.Int) error {
	nonce, err := c.client.PendingNonceAt(ctx, from)
	if err != nil {
		return err
	}
	gasPrice, err := c.client.SuggestGasPrice(ctx)
	if err != nil {
		return err
	}
	tx, err := ethtypes.SignNewTx(key, signer, &ethtypes.LegacyTx{
		Nonce: nonce, To: &to, Value: wei, Gas: 21_000, GasPrice: gasPrice,
	})
	if err != nil {
		return err
	}
	if err := c.client.SendTransaction(ctx, tx); err != nil {
		return err
	}
	_, err = c.waitMined(ctx, tx)
	return err
}

// waitMined waits for tx and fails if it reverted.
func (c *chain) waitMined(ctx context.Context, tx *ethtypes.Transaction) (*ethtypes.Receipt, error) {
	receipt, err := bind.WaitMined(ctx, c.client, tx)
	if err != nil {
		return nil, err
	}
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("tx %s reverted (block %d)", tx.Hash().Hex(), receipt.BlockNumber)
	}
	return receipt, nil
}

// transact sends method on contract from the account keyHex controls and
// waits for it to be mined.
func (c *chain) transact(ctx context.Context, keyHex string, contract *bind.BoundContract, method string, args ...any) (*ethtypes.Receipt, error) {
	auth, err := c.auth(keyHex)
	if err != nil {
		return nil, err
	}
	tx, err := contract.Transact(auth, method, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	receipt, err := c.waitMined(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	return receipt, nil
}

// hasCommitmentLog reports whether logs include vault's Commitment event for commitment.
func hasCommitmentLog(logs []*ethtypes.Log, vault common.Address, commitment *big.Int) bool {
	for _, l := range logs {
		if l.Address == vault && len(l.Topics) >= 3 && l.Topics[0] == commitmentTopic && l.Topics[2].Big().Cmp(commitment) == 0 {
			return true
		}
	}
	return false
}

// abiBytesPair decodes the (bytes, bytes) data of a Payment or EncryptedNote event.
func abiBytesPair() abi.Arguments {
	bytesT, _ := abi.NewType("bytes", "", nil)
	return abi.Arguments{{Type: bytesT}, {Type: bytesT}}
}

// formatNum formats n with comma separators, e.g. 243018 → "243,018".
func formatNum(n uint64) string {
	s := fmt.Sprint(n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}
