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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ContractArtifact represents a Hardhat artifact JSON file.
type ContractArtifact struct {
	ContractName string          `json:"contractName"`
	ABI          json.RawMessage `json:"abi"`
	Bytecode     string          `json:"bytecode"`
}

// Config is the enygma_auction.config.json structure.
type Config struct {
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

// ReceiptData is per-contract deployment info.
type ReceiptData struct {
	ContractAddress string `json:"contractAddress"`
	TransactionHash string `json:"transactionHash"`
	BlockNumber     uint64 `json:"blockNumber"`
	GasUsed         uint64 `json:"gasUsed"`
}

type DeploymentReceipts map[string]ReceiptData

var projectRoot string

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get working directory:", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "enygma_auction.config.json")); err == nil {
			projectRoot = dir
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	projectRoot = cwd
}

func main() {
	if err := deploy(); err != nil {
		log.Fatal("Deployment failed:", err)
	}
}

func deploy() error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	rpcURL := fmt.Sprintf("http://%s:%s", config.Network.Host, config.Network.Port)
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("connect to node: %w", err)
	}
	defer client.Close()

	chainID, ok := new(big.Int).SetString(config.Network.ChainID, 10)
	if !ok {
		return fmt.Errorf("invalid chain-id: %s", config.Network.ChainID)
	}

	if len(config.Network.Accounts) < 1 {
		return fmt.Errorf("need at least 1 account in config")
	}
	owner, err := getTransactOpts(config.Network.Accounts[0].Private, chainID)
	if err != nil {
		return fmt.Errorf("create owner signer: %w", err)
	}
	fmt.Printf("deployer: %s\n", owner.From.Hex())

	receipts := make(DeploymentReceipts)

	// 1. PoseidonT3 (real bytecode from circomlibjs must be in artifacts)
	fmt.Println("Deploying PoseidonT3...")
	poseidonT3Addr, r, err := deployContract(client, owner, "core/contracts/Poseidon.sol/PoseidonT3")
	if err != nil {
		return fmt.Errorf("deploy PoseidonT3: %w", err)
	}
	fmt.Printf("  PoseidonT3: %s\n", poseidonT3Addr.Hex())
	receipts["PoseidonT3"] = receiptToData(r, poseidonT3Addr)

	// 2. PoseidonT5 (needed by PoseidonWrapper even if auction only uses T3)
	fmt.Println("Deploying PoseidonT5...")
	poseidonT5Addr, r, err := deployContract(client, owner, "core/contracts/Poseidon.sol/PoseidonT5")
	if err != nil {
		return fmt.Errorf("deploy PoseidonT5: %w", err)
	}
	fmt.Printf("  PoseidonT5: %s\n", poseidonT5Addr.Hex())
	receipts["PoseidonT5"] = receiptToData(r, poseidonT5Addr)

	// 3. PoseidonWrapper (links T3 + T5)
	fmt.Println("Deploying PoseidonWrapper...")
	poseidonWrapperAddr, r, err := deployContractWithLibraries(
		client, owner,
		"core/contracts/PoseidonWrapper.sol/PoseidonWrapper",
		map[string]common.Address{
			"PoseidonT3": poseidonT3Addr,
			"PoseidonT5": poseidonT5Addr,
		},
	)
	if err != nil {
		return fmt.Errorf("deploy PoseidonWrapper: %w", err)
	}
	fmt.Printf("  PoseidonWrapper: %s\n", poseidonWrapperAddr.Hex())
	receipts["PoseidonWrapper"] = receiptToData(r, poseidonWrapperAddr)

	// 4. GenericGroth16Verifier (stateless BN254 pairing)
	fmt.Println("Deploying GenericGroth16Verifier...")
	g16Addr, r, err := deployContract(client, owner, "core/contracts/GenericGroth16Verifier.sol/GenericGroth16Verifier")
	if err != nil {
		return fmt.Errorf("deploy GenericGroth16Verifier: %w", err)
	}
	fmt.Printf("  GenericGroth16Verifier: %s\n", g16Addr.Hex())
	receipts["G16Verifier"] = receiptToData(r, g16Addr)

	// 5. Verifier (VK registry)
	fmt.Println("Deploying Verifier...")
	verifierAddr, r, err := deployContract(client, owner, "core/contracts/Verifier.sol/Verifier")
	if err != nil {
		return fmt.Errorf("deploy Verifier: %w", err)
	}
	fmt.Printf("  Verifier: %s\n", verifierAddr.Hex())
	receipts["Verifier"] = receiptToData(r, verifierAddr)

	// 6. AuctionCoinVault for NFT (treeDepth = 8)
	fmt.Println("Deploying NftVault (AuctionCoinVault)...")
	nftVaultAddr, r, err := deployContractWithArgs(
		client, owner,
		"core/contracts/AuctionCoinVault.sol/AuctionCoinVault",
		poseidonWrapperAddr,
		big.NewInt(8),
	)
	if err != nil {
		return fmt.Errorf("deploy NftVault: %w", err)
	}
	fmt.Printf("  NftVault: %s\n", nftVaultAddr.Hex())
	receipts["NftVault"] = receiptToData(r, nftVaultAddr)

	// 7. AuctionCoinVault for USDC (treeDepth = 8)
	fmt.Println("Deploying UsdcVault (AuctionCoinVault)...")
	usdcVaultAddr, r, err := deployContractWithArgs(
		client, owner,
		"core/contracts/AuctionCoinVault.sol/AuctionCoinVault",
		poseidonWrapperAddr,
		big.NewInt(8),
	)
	if err != nil {
		return fmt.Errorf("deploy UsdcVault: %w", err)
	}
	fmt.Printf("  UsdcVault: %s\n", usdcVaultAddr.Hex())
	receipts["UsdcVault"] = receiptToData(r, usdcVaultAddr)

	// 8. EnygmaAuction (verifier, usdcVault, nftVault, vkIds 0..5)
	fmt.Println("Deploying EnygmaAuction...")
	auctionAddr, r, err := deployContractWithArgs(
		client, owner,
		"core/contracts/EnygmaAuction.sol/EnygmaAuction",
		verifierAddr,
		usdcVaultAddr,
		nftVaultAddr,
		big.NewInt(0), // VK_LOCK
		big.NewInt(1), // VK_BID
		big.NewInt(2), // VK_BATCH
		big.NewInt(3), // VK_FINAL
		big.NewInt(4), // VK_REVERT
		big.NewInt(5), // VK_WITHDRAW
	)
	if err != nil {
		return fmt.Errorf("deploy EnygmaAuction: %w", err)
	}
	fmt.Printf("  EnygmaAuction: %s\n", auctionAddr.Hex())
	receipts["EnygmaAuction"] = receiptToData(r, auctionAddr)

	if err := saveReceipts(receipts); err != nil {
		return fmt.Errorf("save receipts: %w", err)
	}
	fmt.Printf("Receipts saved to %s/build/receipts.json\n", projectRoot)
	return nil
}

// ---- helpers ----------------------------------------------------------------

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, "enygma_auction.config.json"))
	if err != nil {
		return nil, err
	}
	var cfg Config
	return &cfg, json.Unmarshal(data, &cfg)
}

func loadArtifact(contractPath string) (*ContractArtifact, error) {
	p := filepath.Join(projectRoot, "artifacts", "contracts", contractPath+".json")
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var a ContractArtifact
	return &a, json.Unmarshal(data, &a)
}

func getTransactOpts(privateKeyHex string, chainID *big.Int) (*bind.TransactOpts, error) {
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	pk, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, err
	}
	return bind.NewKeyedTransactorWithChainID(pk, chainID)
}

func deployContract(client *ethclient.Client, auth *bind.TransactOpts, contractPath string) (common.Address, *types.Receipt, error) {
	a, err := loadArtifact(contractPath)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("load artifact %s: %w", contractPath, err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(a.ABI)))
	if err != nil {
		return common.Address{}, nil, err
	}
	addr, tx, _, err := bind.DeployContract(auth, parsedABI, common.FromHex(a.Bytecode), client)
	if err != nil {
		return common.Address{}, nil, err
	}
	rec, err := bind.WaitMined(context.Background(), client, tx)
	return addr, rec, err
}

func deployContractWithArgs(client *ethclient.Client, auth *bind.TransactOpts, contractPath string, args ...interface{}) (common.Address, *types.Receipt, error) {
	a, err := loadArtifact(contractPath)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("load artifact %s: %w", contractPath, err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(a.ABI)))
	if err != nil {
		return common.Address{}, nil, err
	}
	addr, tx, _, err := bind.DeployContract(auth, parsedABI, common.FromHex(a.Bytecode), client, args...)
	if err != nil {
		return common.Address{}, nil, err
	}
	rec, err := bind.WaitMined(context.Background(), client, tx)
	return addr, rec, err
}

func deployContractWithLibraries(client *ethclient.Client, auth *bind.TransactOpts, contractPath string, libMap map[string]common.Address) (common.Address, *types.Receipt, error) {
	artifactPath := filepath.Join(projectRoot, "artifacts/contracts", contractPath+".json")
	rawData, err := os.ReadFile(artifactPath)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("read artifact %s: %w", contractPath, err)
	}

	var fullArtifact struct {
		ABI            json.RawMessage                                `json:"abi"`
		Bytecode       string                                         `json:"bytecode"`
		LinkReferences map[string]map[string][]struct {
			Start  int `json:"start"`
			Length int `json:"length"`
		} `json:"linkReferences"`
	}
	if err := json.Unmarshal(rawData, &fullArtifact); err != nil {
		return common.Address{}, nil, err
	}

	parsedABI, err := abi.JSON(strings.NewReader(string(fullArtifact.ABI)))
	if err != nil {
		return common.Address{}, nil, err
	}

	bytecodeHex := fullArtifact.Bytecode[2:] // strip 0x
	for _, libs := range fullArtifact.LinkReferences {
		for libName, positions := range libs {
			addr, ok := libMap[libName]
			if !ok {
				return common.Address{}, nil, fmt.Errorf("missing address for library %s", libName)
			}
			addrHex := strings.ToLower(strings.TrimPrefix(addr.Hex(), "0x"))
			for _, pos := range positions {
				start := pos.Start * 2
				end := start + pos.Length*2
				bytecodeHex = bytecodeHex[:start] + addrHex + bytecodeHex[end:]
			}
		}
	}

	addr, tx, _, err := bind.DeployContract(auth, parsedABI, common.FromHex("0x"+bytecodeHex), client)
	if err != nil {
		return common.Address{}, nil, err
	}
	rec, err := bind.WaitMined(context.Background(), client, tx)
	return addr, rec, err
}

func receiptToData(r *types.Receipt, addr common.Address) ReceiptData {
	return ReceiptData{
		ContractAddress: addr.Hex(),
		TransactionHash: r.TxHash.Hex(),
		BlockNumber:     r.BlockNumber.Uint64(),
		GasUsed:         r.GasUsed,
	}
}

func saveReceipts(receipts DeploymentReceipts) error {
	buildDir := filepath.Join(projectRoot, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(receipts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(buildDir, "receipts.json"), data, 0644)
}
