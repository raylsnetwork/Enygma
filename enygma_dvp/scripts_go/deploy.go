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

// ContractArtifact represents the structure of a Hardhat artifact JSON file
type ContractArtifact struct {
	ContractName string          `json:"contractName"`
	ABI          json.RawMessage `json:"abi"`
	Bytecode     string          `json:"bytecode"`
}

// Config represents the enygmadvp.config.json structure
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

// ReceiptData represents deployment receipt information
type ReceiptData struct {
	ContractAddress string `json:"contractAddress"`
	TransactionHash string `json:"transactionHash"`
	BlockNumber     uint64 `json:"blockNumber"`
	GasUsed         uint64 `json:"gasUsed"`
}

// DeploymentReceipts holds all deployment receipts
type DeploymentReceipts map[string]ReceiptData

var (
	projectRoot string
)

func init() {
	// Get the project root directory (parent of scripts_go)
	execPath, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get working directory:", err)
	}
	projectRoot = filepath.Dir(execPath)
	if filepath.Base(execPath) != "scripts_go" {
		projectRoot = execPath
	}
}

func main() {
	if err := deploy(); err != nil {
		log.Fatal("Deployment failed:", err)
	}
}

func deploy() error {
	// Load config
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to Ethereum client
	rpcURL := fmt.Sprintf("http://%s:%s", config.Network.Host, config.Network.Port)
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}
	defer client.Close()

	// Get chain ID
	chainID, ok := new(big.Int).SetString(config.Network.ChainID, 10)
	if !ok {
		return fmt.Errorf("invalid chain ID: %s", config.Network.ChainID)
	}

	// Set up signers (owner, alice, bob)
	if len(config.Network.Accounts) < 3 {
		return fmt.Errorf("need at least 3 accounts in config")
	}

	owner, err := getTransactOpts(config.Network.Accounts[0].Private, chainID)
	if err != nil {
		return fmt.Errorf("failed to create owner transact opts: %w", err)
	}

	fmt.Printf("owner: %s\n", owner.From.Hex())
	fmt.Printf("alice: %s\n", config.Network.Accounts[1].Address)
	fmt.Printf("bob: %s\n", config.Network.Accounts[2].Address)

	receipts := make(DeploymentReceipts)

	// Deploy PoseidonT3
	fmt.Println("Deploying poseidonT3 smart contract...")
	poseidonT3Address, receipt, err := deployContract(client, owner, "core/contracts/Poseidon.sol/PoseidonT3")
	if err != nil {
		return fmt.Errorf("failed to deploy PoseidonT3: %w", err)
	}
	fmt.Printf("poseidonT3 has been deployed to %s\n", poseidonT3Address.Hex())
	receipts["Poseidon"] = receiptToData(receipt, poseidonT3Address)

	// Deploy GenericGroth16Verifier
	fmt.Println("Deploying GenericGroth16Verifier smart contract...")
	g16VerifierAddress, receipt, err := deployContract(client, owner, "core/contracts/GenericGroth16Verifier.sol/GenericGroth16Verifier")
	if err != nil {
		return fmt.Errorf("failed to deploy GenericGroth16Verifier: %w", err)
	}
	fmt.Printf("GenericGroth16Verifier has been deployed to %s\n", g16VerifierAddress.Hex())
	receipts["G16Verifier"] = receiptToData(receipt, g16VerifierAddress)

	// Deploy Verifier
	fmt.Println("Deploying Verifier smart contract...")
	verifierAddress, receipt, err := deployContract(client, owner, "core/contracts/Verifier.sol/Verifier")
	if err != nil {
		return fmt.Errorf("failed to deploy Verifier: %w", err)
	}
	fmt.Printf("Verifier has been deployed to %s\n", verifierAddress.Hex())
	receipts["Verifier"] = receiptToData(receipt, verifierAddress)

	// Deploy PrivateMintVerifier
	fmt.Println("Deploying PrivateMintVerifier smart contract...")
	privateMintVerifierAddress, receipt, err := deployContract(client, owner, "core/contracts/PrivateMintVerifier.sol/PrivateMintVerifier")
	if err != nil {
		return fmt.Errorf("failed to deploy PrivateMintVerifier: %w", err)
	}
	fmt.Printf("PrivateMintVerifier has been deployed to %s\n", privateMintVerifierAddress.Hex())
	receipts["PrivateMintVerifier"] = receiptToData(receipt, privateMintVerifierAddress)

	// Deploy PoseidonWrapper (with library linking)
	fmt.Println("Deploying PoseidonWrapper smart contract...")
	poseidonWrapperAddress, receipt, err := deployContractWithLibrary(
		client, owner,
		"core/contracts/PoseidonWrapper.sol/PoseidonWrapper",
		"PoseidonT3", poseidonT3Address,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy PoseidonWrapper: %w", err)
	}
	fmt.Printf("poseidonWrapper has been deployed to %s\n", poseidonWrapperAddress.Hex())
	receipts["PoseidonWrapper"] = receiptToData(receipt, poseidonWrapperAddress)

	// Deploy EnygmaDvp
	fmt.Println("Deploying EnygmaDvp smart contract...")
	enygmaDvpAddress, receipt, err := deployContractWithArgs(
		client, owner,
		"core/contracts/EnygmaDvp.sol/EnygmaDvp",
		poseidonWrapperAddress, g16VerifierAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy EnygmaDvp: %w", err)
	}
	fmt.Printf("enygmaDvp has been deployed to %s\n", enygmaDvpAddress.Hex())
	receipts["EnygmaDvp"] = receiptToData(receipt, enygmaDvpAddress)

	// Deploy RaylsERC20
	fmt.Println("Deploying RaylsERC20 smart contract...")
	erc20Address, receipt, err := deployContractWithArgs(
		client, owner,
		"erc20/contracts/RaylsERC20.sol/RaylsERC20",
		"TestERC20", "Rayls20",
	)
	if err != nil {
		return fmt.Errorf("failed to deploy RaylsERC20: %w", err)
	}
	fmt.Printf("RaylsERC20 has been deployed to %s\n", erc20Address.Hex())
	receipts["ERC20"] = receiptToData(receipt, erc20Address)

	// Deploy RaylsERC721
	fmt.Println("Deploying RaylsERC721 smart contract...")
	erc721Address, receipt, err := deployContractWithArgs(
		client, owner,
		"erc721/contracts/RaylsERC721.sol/RaylsERC721",
		"TestERC721", "Rayls721",
	)
	if err != nil {
		return fmt.Errorf("failed to deploy RaylsERC721: %w", err)
	}
	fmt.Printf("RaylsERC721 has been deployed to %s\n", erc721Address.Hex())
	receipts["ERC721"] = receiptToData(receipt, erc721Address)

	// Deploy RaylsERC1155
	fmt.Println("Deploying RaylsERC1155 smart contract...")
	erc1155Address, receipt, err := deployContractWithArgs(
		client, owner,
		"erc1155/contracts/RaylsERC1155.sol/RaylsERC1155",
		"Rayls1155",
	)
	if err != nil {
		return fmt.Errorf("failed to deploy RaylsERC1155: %w", err)
	}
	fmt.Printf("RaylsERC1155 has been deployed to %s\n", erc1155Address.Hex())
	receipts["ERC1155"] = receiptToData(receipt, erc1155Address)

	// Deploy Erc20CoinVault
	fmt.Println("Deploying Erc20CoinVault smart contract...")
	erc20CoinVaultAddress, receipt, err := deployContractWithArgs(
		client, owner,
		"core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault",
		enygmaDvpAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy Erc20CoinVault: %w", err)
	}
	fmt.Printf("Erc20CoinVault has been deployed to %s\n", erc20CoinVaultAddress.Hex())
	receipts["Erc20CoinVault"] = receiptToData(receipt, erc20CoinVaultAddress)

	// Deploy Erc721CoinVault
	fmt.Println("Deploying Erc721CoinVault smart contract...")
	erc721CoinVaultAddress, receipt, err := deployContractWithArgs(
		client, owner,
		"core/contracts/vaults/Erc721CoinVault.sol/Erc721CoinVault",
		enygmaDvpAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy Erc721CoinVault: %w", err)
	}
	fmt.Printf("Erc721CoinVault has been deployed to %s\n", erc721CoinVaultAddress.Hex())
	receipts["Erc721CoinVault"] = receiptToData(receipt, erc721CoinVaultAddress)

	// Deploy Erc1155CoinVault
	fmt.Println("Deploying Erc1155CoinVault smart contract...")
	erc1155CoinVaultAddress, receipt, err := deployContractWithArgs(
		client, owner,
		"core/contracts/vaults/Erc1155CoinVault.sol/Erc1155CoinVault",
		enygmaDvpAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy Erc1155CoinVault: %w", err)
	}
	fmt.Printf("Erc1155CoinVault has been deployed to %s\n", erc1155CoinVaultAddress.Hex())
	receipts["Erc1155CoinVault"] = receiptToData(receipt, erc1155CoinVaultAddress)

	// Deploy EnygmaErc20CoinVault
	fmt.Println("Deploying EnygmaErc20CoinVault smart contract...")
	enygmaCoinVaultAddress, receipt, err := deployContractWithArgs(
		client, owner,
		"core/contracts/vaults/EnygmaErc20CoinVault.sol/EnygmaErc20CoinVault",
		enygmaDvpAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy EnygmaErc20CoinVault: %w", err)
	}
	fmt.Printf("EnygmaErc20CoinVault has been deployed to %s\n", enygmaCoinVaultAddress.Hex())
	receipts["EnygmaErc20CoinVault"] = receiptToData(receipt, enygmaCoinVaultAddress)

	// Deploy FungibleAssetGroup
	fmt.Println("Deploying FungibleAssetGroup smart contract...")
	fungibleGroupAddress, receipt, err := deployContractWithArgs(
		client, owner,
		"core/contracts/vaults/AssetGroup.sol/AssetGroup",
		enygmaDvpAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy FungibleAssetGroup: %w", err)
	}
	fmt.Printf("FungibleAssetGroup has been deployed to %s\n", fungibleGroupAddress.Hex())
	receipts["FungibleAssetGroup"] = receiptToData(receipt, fungibleGroupAddress)

	// Deploy NonFungibleAssetGroup
	fmt.Println("Deploying NonFungibleAssetGroup smart contract...")
	nonFungibleGroupAddress, receipt, err := deployContractWithArgs(
		client, owner,
		"core/contracts/vaults/AssetGroup.sol/AssetGroup",
		enygmaDvpAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy NonFungibleAssetGroup: %w", err)
	}
	fmt.Printf("NonFungibleAssetGroup has been deployed to %s\n", nonFungibleGroupAddress.Hex())
	receipts["NonFungibleAssetGroup"] = receiptToData(receipt, nonFungibleGroupAddress)

	// Save receipts to JSON
	if err := saveReceipts(receipts); err != nil {
		return fmt.Errorf("failed to save receipts: %w", err)
	}

	fmt.Println("receipts has been saved to ./build/receipts.json.")
	return nil
}

func loadConfig() (*Config, error) {
	configPath := filepath.Join(projectRoot, "enygmadvp.config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func loadArtifact(contractPath string) (*ContractArtifact, error) {
	artifactPath := filepath.Join(projectRoot, "artifacts", "contracts", contractPath+".json")
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, err
	}

	var artifact ContractArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, err
	}

	return &artifact, nil
}

func getTransactOpts(privateKeyHex string, chainID *big.Int) (*bind.TransactOpts, error) {
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, err
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return nil, err
	}

	return auth, nil
}

func deployContract(client *ethclient.Client, auth *bind.TransactOpts, contractPath string) (common.Address, *types.Receipt, error) {
	artifact, err := loadArtifact(contractPath)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to load artifact: %w", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	bytecode := common.FromHex(artifact.Bytecode)

	address, tx, _, err := bind.DeployContract(auth, parsedABI, bytecode, client)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to deploy contract: %w", err)
	}

	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to wait for deployment: %w", err)
	}

	return address, receipt, nil
}

func deployContractWithArgs(client *ethclient.Client, auth *bind.TransactOpts, contractPath string, args ...interface{}) (common.Address, *types.Receipt, error) {
	artifact, err := loadArtifact(contractPath)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to load artifact: %w", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	bytecode := common.FromHex(artifact.Bytecode)

	address, tx, _, err := bind.DeployContract(auth, parsedABI, bytecode, client, args...)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to deploy contract: %w", err)
	}

	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to wait for deployment: %w", err)
	}

	return address, receipt, nil
}

func deployContractWithLibrary(client *ethclient.Client, auth *bind.TransactOpts, contractPath string, libName string, libAddress common.Address) (common.Address, *types.Receipt, error) {
	artifact, err := loadArtifact(contractPath)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to load artifact: %w", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Link library in bytecode
	// Hardhat uses placeholder format: __$<hash>$__
	// We need to replace it with the actual library address
	bytecodeHex := artifact.Bytecode
	libAddressHex := strings.ToLower(strings.TrimPrefix(libAddress.Hex(), "0x"))

	// Find and replace library placeholder
	// The placeholder is typically __$<34-char-hash>$__ (total 40 chars)
	// We'll search for any placeholder pattern and replace it
	for {
		startIdx := strings.Index(bytecodeHex, "__$")
		if startIdx == -1 {
			break
		}
		endIdx := strings.Index(bytecodeHex[startIdx+3:], "$__")
		if endIdx == -1 {
			break
		}
		endIdx = startIdx + 3 + endIdx + 3
		bytecodeHex = bytecodeHex[:startIdx] + libAddressHex + bytecodeHex[endIdx:]
	}

	bytecode := common.FromHex(bytecodeHex)

	address, tx, _, err := bind.DeployContract(auth, parsedABI, bytecode, client)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to deploy contract: %w", err)
	}

	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to wait for deployment: %w", err)
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
