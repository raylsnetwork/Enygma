package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// VerificationKeyJSON is the circom-format JSON output by export_vk_init_auction.
type VerificationKeyJSON struct {
	Protocol    string       `json:"protocol"`
	Curve       string       `json:"curve"`
	VkAlpha1    []string     `json:"vk_alpha_1"`
	VkBeta2     [][]string   `json:"vk_beta_2"`
	VkGamma2    [][]string   `json:"vk_gamma_2"`
	VkDelta2    [][]string   `json:"vk_delta_2"`
	VkAlphabeta [][][]string `json:"vk_alphabeta_12"`
	IC          [][]string   `json:"IC"`
}

// G1Point / G2Point / VerifyingKey must match the Solidity struct ABI exactly.
type G1Point struct {
	X *big.Int `abi:"x"`
	Y *big.Int `abi:"y"`
}
type G2Point struct {
	X [2]*big.Int `abi:"x"`
	Y [2]*big.Int `abi:"y"`
}
type VerifyingKey struct {
	Alpha1 G1Point   `abi:"alpha1"`
	Beta2  G2Point   `abi:"beta2"`
	Gamma2 G2Point   `abi:"gamma2"`
	Delta2 G2Point   `abi:"delta2"`
	Ic     []G1Point `abi:"ic"`
}

// AuctionConfig mirrors the enygma_auction.config.json structure.
// Defined here so init.go can be compiled standalone (without deploy.go).
type AuctionConfig struct {
	Network struct {
		Host    string `json:"host"`
		Port    string `json:"port"`
		ChainID string `json:"chain-id"`
		Accounts []struct {
			Address string `json:"address"`
			Private string `json:"private"`
		} `json:"accounts"`
	} `json:"network"`
}

// initProjectRoot is set by init().
var initProjectRoot string

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get working directory:", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "enygma_auction.config.json")); err == nil {
			initProjectRoot = dir
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	initProjectRoot = cwd
}

func main() {
	if err := initContracts(); err != nil {
		log.Fatal("Init failed:", err)
	}
}

func initContracts() error {
	config, err := loadInitConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	receipts, err := loadInitReceipts()
	if err != nil {
		return fmt.Errorf("load receipts: %w", err)
	}

	rpcURL := fmt.Sprintf("http://%s:%s", config.Network.Host, config.Network.Port)
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	chainID, ok := new(big.Int).SetString(config.Network.ChainID, 10)
	if !ok {
		return fmt.Errorf("invalid chain-id: %s", config.Network.ChainID)
	}

	auth, err := getInitTransactOpts(config.Network.Accounts[0].Private, chainID)

	if err != nil {
		return fmt.Errorf("signer: %w", err)
	}

	verifierABI, err := loadContractABI("core/contracts/Verifier.sol/Verifier")
	if err != nil {
		return fmt.Errorf("load Verifier ABI: %w", err)
	}
	verifierAddr := common.HexToAddress(receipts["Verifier"])
	g16Addr      := common.HexToAddress(receipts["G16Verifier"])

	// Step 1: wire Verifier → GenericGroth16Verifier
	fmt.Println("Initializing Verifier with GenericGroth16Verifier...")
	if _, err := callMethod(client, auth, verifierABI, verifierAddr, "initializeVerifier", g16Addr); err != nil {
		return fmt.Errorf("initializeVerifier: %w", err)
	}

	// Step 2: register the 6 VKs in slot order (Lock=0, Bid=1, Batch=2, Final=3, Revert=4, Withdraw=5)
	circuitNames := []string{"AuctionLock", "AuctionBid", "AuctionBatch", "AuctionFinal", "AuctionRevert", "AuctionWithdraw"}
	for i, name := range circuitNames {
		vkPath := filepath.Join(initProjectRoot, "build", name+".json")
		vk, err := loadVK(vkPath)
		if err != nil {
			return fmt.Errorf("load VK %s: %w", name, err)
		}
		fmt.Printf("  Registering VK slot %d (%s)...\n", i, name)
		if _, err := callMethod(client, auth, verifierABI, verifierAddr, "addVerificationKey", vk); err != nil {
			return fmt.Errorf("addVerificationKey %s: %w", name, err)
		}
	}

	// Step 3: grant EnygmaAuction the AUCTION_ROLE on both vaults
	vaultABI, err := loadContractABI("core/contracts/AuctionCoinVault.sol/AuctionCoinVault")
	if err != nil {
		return fmt.Errorf("load AuctionCoinVault ABI: %w", err)
	}
	auctionAddr  := common.HexToAddress(receipts["EnygmaAuction"])
	nftVaultAddr  := common.HexToAddress(receipts["NftVault"])
	usdcVaultAddr := common.HexToAddress(receipts["UsdcVault"])

	fmt.Println("Granting AUCTION_ROLE to EnygmaAuction on NftVault...")
	if _, err := callMethod(client, auth, vaultABI, nftVaultAddr, "grantAuctionRole", auctionAddr); err != nil {
		return fmt.Errorf("grantAuctionRole NftVault: %w", err)
	}

	fmt.Println("Granting AUCTION_ROLE to EnygmaAuction on UsdcVault...")
	if _, err := callMethod(client, auth, vaultABI, usdcVaultAddr, "grantAuctionRole", auctionAddr); err != nil {
		return fmt.Errorf("grantAuctionRole UsdcVault: %w", err)
	}

	fmt.Println("Init complete.")
	return nil
}

// ---- helpers ----------------------------------------------------------------

func loadInitConfig() (*AuctionConfig, error) {
	data, err := os.ReadFile(filepath.Join(initProjectRoot, "enygma_auction.config.json"))
	if err != nil {
		return nil, err
	}
	var cfg AuctionConfig
	return &cfg, json.Unmarshal(data, &cfg)
}

// loadInitReceipts returns contractName→address string map from build/receipts.json.
func loadInitReceipts() (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(initProjectRoot, "build", "receipts.json"))
	if err != nil {
		return nil, err
	}
	var raw map[string]struct {
		ContractAddress string `json:"contractAddress"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	m := make(map[string]string, len(raw))
	for k, v := range raw {
		m[k] = v.ContractAddress
	}
	return m, nil
}

func loadContractABI(contractPath string) (abi.ABI, error) {
	artifactPath := filepath.Join(initProjectRoot, "artifacts", "contracts", contractPath+".json")
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return abi.ABI{}, err
	}
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		return abi.ABI{}, err
	}
	return abi.JSON(strings.NewReader(string(artifact.ABI)))
}

func getInitTransactOpts(privateKeyHex string, chainID *big.Int) (*bind.TransactOpts, error) {
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	pk, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, err
	}
	return bind.NewKeyedTransactorWithChainID(pk, chainID)
}

func callMethod(client *ethclient.Client, auth *bind.TransactOpts, contractABI abi.ABI, addr common.Address, method string, args ...interface{}) (common.Hash, error) {
	contract := bind.NewBoundContract(addr, contractABI, client, client, client)
	tx, err := contract.Transact(auth, method, args...)
	if err != nil {
		return common.Hash{}, fmt.Errorf("%s: %w", method, err)
	}
	rec, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("wait %s: %w", method, err)
	}
	if rec.Status == 0 {
		return common.Hash{}, fmt.Errorf("transaction %s reverted", method)
	}
	return tx.Hash(), nil
}

func loadVK(filePath string) (VerifyingKey, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return VerifyingKey{}, err
	}
	var vkJSON VerificationKeyJSON
	if err := json.Unmarshal(data, &vkJSON); err != nil {
		return VerifyingKey{}, err
	}
	return formatVK(vkJSON), nil
}

func formatVK(vkJSON VerificationKeyJSON) VerifyingKey {
	parseBI := func(s string) *big.Int {
		n, _ := new(big.Int).SetString(s, 10)
		return n
	}

	var ic []G1Point
	for _, pt := range vkJSON.IC {
		ic = append(ic, G1Point{X: parseBI(pt[0]), Y: parseBI(pt[1])})
	}

	// G2 coordinate convention: vk_beta_2[0] = [imaginary(A1), real(A0)],
	// vk_beta_2[1] = [imaginary(A1), real(A0)] — same as init.go in enygma_dvp.
	// EIP-197 expects x[0]=imaginary, x[1]=real, which is how they arrive in the JSON.
	return VerifyingKey{
		Alpha1: G1Point{X: parseBI(vkJSON.VkAlpha1[0]), Y: parseBI(vkJSON.VkAlpha1[1])},
		Beta2: G2Point{
			X: [2]*big.Int{parseBI(vkJSON.VkBeta2[0][0]), parseBI(vkJSON.VkBeta2[0][1])},
			Y: [2]*big.Int{parseBI(vkJSON.VkBeta2[1][0]), parseBI(vkJSON.VkBeta2[1][1])},
		},
		Gamma2: G2Point{
			X: [2]*big.Int{parseBI(vkJSON.VkGamma2[0][0]), parseBI(vkJSON.VkGamma2[0][1])},
			Y: [2]*big.Int{parseBI(vkJSON.VkGamma2[1][0]), parseBI(vkJSON.VkGamma2[1][1])},
		},
		Delta2: G2Point{
			X: [2]*big.Int{parseBI(vkJSON.VkDelta2[0][0]), parseBI(vkJSON.VkDelta2[0][1])},
			Y: [2]*big.Int{parseBI(vkJSON.VkDelta2[1][0]), parseBI(vkJSON.VkDelta2[1][1])},
		},
		Ic: ic,
	}
}
