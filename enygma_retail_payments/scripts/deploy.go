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

// ContractArtifact represents the structure of a Hardhat artifact JSON file.
type ContractArtifact struct {
	ContractName string          `json:"contractName"`
	ABI          json.RawMessage `json:"abi"`
	Bytecode     string          `json:"bytecode"`
}

// Config represents enygmapayment.config.json.
type Config struct {
	Network struct {
		Host     string `json:"host"`
		Port     string `json:"port"`
		ChainID  string `json:"chain-id"`
		Accounts []struct {
			Address string `json:"address"`
			Private string `json:"private"`
		} `json:"accounts"`
	} `json:"network"`
}

// ReceiptData holds deployment receipt information.
type ReceiptData struct {
	ContractAddress string `json:"contractAddress"`
	TransactionHash string `json:"transactionHash"`
	BlockNumber     uint64 `json:"blockNumber"`
	GasUsed         uint64 `json:"gasUsed"`
}

// DeploymentReceipts holds all deployment receipts.
type DeploymentReceipts map[string]ReceiptData

var projectRoot string

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get working directory:", err)
	}
	// Walk up until we find enygmapayment.config.json (handles running from scripts/).
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "enygmapayment.config.json")); err == nil {
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
		return fmt.Errorf("failed to load config: %w", err)
	}

	rpcURL := fmt.Sprintf("http://%s:%s", config.Network.Host, config.Network.Port)
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}
	defer client.Close()

	chainID, ok := new(big.Int).SetString(config.Network.ChainID, 10)
	if !ok {
		return fmt.Errorf("invalid chain ID: %s", config.Network.ChainID)
	}

	if len(config.Network.Accounts) < 3 {
		return fmt.Errorf("need at least 3 accounts in config")
	}

	owner, err := getTransactOpts(config.Network.Accounts[0].Private, chainID)
	if err != nil {
		return fmt.Errorf("failed to create owner transact opts: %w", err)
	}

	fmt.Printf("owner: %s\n", owner.From.Hex())
	fmt.Printf("alice: %s\n", config.Network.Accounts[1].Address)
	fmt.Printf("bob:   %s\n", config.Network.Accounts[2].Address)

	receipts := make(DeploymentReceipts)

	fmt.Println("Deploying PoseidonT3...")
	poseidonT3Address, receipt, err := deployContract(client, owner, "PoseidonT3")
	if err != nil {
		return fmt.Errorf("failed to deploy PoseidonT3: %w", err)
	}
	fmt.Printf("PoseidonT3 → %s\n", poseidonT3Address.Hex())
	receipts["Poseidon"] = receiptToData(receipt, poseidonT3Address)

	fmt.Println("Deploying PoseidonT5...")
	poseidonT5Address, receipt, err := deployContract(client, owner, "PoseidonT5")
	if err != nil {
		return fmt.Errorf("failed to deploy PoseidonT5: %w", err)
	}
	fmt.Printf("PoseidonT5 → %s\n", poseidonT5Address.Hex())

	fmt.Println("Deploying PoseidonWrapper...")
	poseidonWrapperAddress, receipt, err := deployContractWithLibraries(
		client, owner, "PoseidonWrapper",
		map[string]common.Address{
			"PoseidonT3": poseidonT3Address,
			"PoseidonT5": poseidonT5Address,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to deploy PoseidonWrapper: %w", err)
	}
	fmt.Printf("PoseidonWrapper → %s\n", poseidonWrapperAddress.Hex())
	receipts["PoseidonWrapper"] = receiptToData(receipt, poseidonWrapperAddress)

	fmt.Println("Deploying GenericGroth16Verifier...")
	g16VerifierAddress, receipt, err := deployContract(client, owner, "GenericGroth16Verifier")
	if err != nil {
		return fmt.Errorf("failed to deploy GenericGroth16Verifier: %w", err)
	}
	fmt.Printf("GenericGroth16Verifier → %s\n", g16VerifierAddress.Hex())
	receipts["G16Verifier"] = receiptToData(receipt, g16VerifierAddress)

	fmt.Println("Deploying Verifier...")
	verifierAddress, receipt, err := deployContract(client, owner, "Verifier")
	if err != nil {
		return fmt.Errorf("failed to deploy Verifier: %w", err)
	}
	fmt.Printf("Verifier → %s\n", verifierAddress.Hex())
	receipts["Verifier"] = receiptToData(receipt, verifierAddress)

	fmt.Println("Deploying PrivateMintVerifier...")
	privateMintVerifierAddress, receipt, err := deployContract(client, owner, "PrivateMintVerifier")
	if err != nil {
		return fmt.Errorf("failed to deploy PrivateMintVerifier: %w", err)
	}
	fmt.Printf("PrivateMintVerifier → %s\n", privateMintVerifierAddress.Hex())
	receipts["PrivateMintVerifier"] = receiptToData(receipt, privateMintVerifierAddress)

	fmt.Println("Deploying EnygmaDvp...")
	enygmaDvpAddress, receipt, err := deployContractWithArgs(
		client, owner, "EnygmaDvp",
		poseidonWrapperAddress, g16VerifierAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy EnygmaDvp: %w", err)
	}
	fmt.Printf("EnygmaDvp → %s\n", enygmaDvpAddress.Hex())
	receipts["EnygmaDvp"] = receiptToData(receipt, enygmaDvpAddress)

	fmt.Println("Deploying RaylsERC20...")
	erc20Address, receipt, err := deployContractWithArgs(
		client, owner, "RaylsERC20",
		"TestERC20", "Rayls20",
	)
	if err != nil {
		return fmt.Errorf("failed to deploy RaylsERC20: %w", err)
	}
	fmt.Printf("RaylsERC20 → %s\n", erc20Address.Hex())
	receipts["ERC20"] = receiptToData(receipt, erc20Address)

	fmt.Println("Deploying Erc20CoinVault...")
	erc20CoinVaultAddress, receipt, err := deployContractWithArgs(
		client, owner, "Erc20CoinVault",
		enygmaDvpAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy Erc20CoinVault: %w", err)
	}
	fmt.Printf("Erc20CoinVault → %s\n", erc20CoinVaultAddress.Hex())
	receipts["Erc20CoinVault"] = receiptToData(receipt, erc20CoinVaultAddress)

	fmt.Println("Deploying UserRegistry...")
	userRegistryAddress, receipt, err := deployContractFromPath(
		client, owner,
		filepath.Join(projectRoot, "contracts", "user_registry", "UserRegistry.json"),
	)
	if err != nil {
		return fmt.Errorf("failed to deploy UserRegistry: %w", err)
	}
	fmt.Printf("UserRegistry → %s\n", userRegistryAddress.Hex())
	receipts["UserRegistry"] = receiptToData(receipt, userRegistryAddress)

	if err := saveReceipts(receipts); err != nil {
		return fmt.Errorf("failed to save receipts: %w", err)
	}

	fmt.Println("Receipts saved to ./build/receipts.json")
	return nil
}

func loadConfig() (*Config, error) {
	configPath := filepath.Join(projectRoot, "enygmapayment.config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config Config
	return &config, json.Unmarshal(data, &config)
}

// loadArtifact reads from contracts/abis/<name>.json (flat layout).
func loadArtifact(contractName string) (*ContractArtifact, error) {
	artifactPath := filepath.Join(projectRoot, "contracts", "abis", contractName+".json")
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, err
	}
	var artifact ContractArtifact
	return &artifact, json.Unmarshal(data, &artifact)
}

func getTransactOpts(privateKeyHex string, chainID *big.Int) (*bind.TransactOpts, error) {
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, err
	}
	return bind.NewKeyedTransactorWithChainID(privateKey, chainID)
}

func deployContract(client *ethclient.Client, auth *bind.TransactOpts, contractName string) (common.Address, *types.Receipt, error) {
	artifact, err := loadArtifact(contractName)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("load artifact %s: %w", contractName, err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("parse ABI: %w", err)
	}
	address, tx, _, err := bind.DeployContract(auth, parsedABI, common.FromHex(artifact.Bytecode), client)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("deploy: %w", err)
	}
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("wait mined: %w", err)
	}
	return address, receipt, nil
}

// deployContractFromPath deploys a contract from an explicit artifact file path.
func deployContractFromPath(client *ethclient.Client, auth *bind.TransactOpts, artifactPath string) (common.Address, *types.Receipt, error) {
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("read artifact %s: %w", artifactPath, err)
	}
	var artifact ContractArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return common.Address{}, nil, fmt.Errorf("parse artifact %s: %w", artifactPath, err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("parse ABI: %w", err)
	}
	address, tx, _, err := bind.DeployContract(auth, parsedABI, common.FromHex(artifact.Bytecode), client)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("deploy: %w", err)
	}
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("wait mined: %w", err)
	}
	return address, receipt, nil
}

func deployContractWithArgs(client *ethclient.Client, auth *bind.TransactOpts, contractName string, args ...interface{}) (common.Address, *types.Receipt, error) {
	artifact, err := loadArtifact(contractName)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("load artifact %s: %w", contractName, err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("parse ABI: %w", err)
	}
	address, tx, _, err := bind.DeployContract(auth, parsedABI, common.FromHex(artifact.Bytecode), client, args...)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("deploy: %w", err)
	}
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("wait mined: %w", err)
	}
	return address, receipt, nil
}

func deployContractWithLibraries(client *ethclient.Client, auth *bind.TransactOpts, contractName string, libMap map[string]common.Address) (common.Address, *types.Receipt, error) {
	artifactPath := filepath.Join(projectRoot, "contracts", "abis", contractName+".json")
	rawData, err := os.ReadFile(artifactPath)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("read artifact %s: %w", contractName, err)
	}

	var fullArtifact struct {
		ABI            json.RawMessage `json:"abi"`
		Bytecode       string          `json:"bytecode"`
		LinkReferences map[string]map[string][]struct {
			Start  int `json:"start"`
			Length int `json:"length"`
		} `json:"linkReferences"`
	}
	if err := json.Unmarshal(rawData, &fullArtifact); err != nil {
		return common.Address{}, nil, fmt.Errorf("parse artifact: %w", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(string(fullArtifact.ABI)))
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("parse ABI: %w", err)
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

	address, tx, _, err := bind.DeployContract(auth, parsedABI, common.FromHex("0x"+bytecodeHex), client)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("deploy: %w", err)
	}
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("wait mined: %w", err)
	}
	return address, receipt, nil
}

func receiptToData(receipt *types.Receipt, contractAddress common.Address) ReceiptData {
	return ReceiptData{
		ContractAddress: contractAddress.Hex(),
		TransactionHash: receipt.TxHash.Hex(),
		BlockNumber:     receipt.BlockNumber.Uint64(),
		GasUsed:         receipt.GasUsed,
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
