// cmd/deploy_payment — self-contained deployment for a dedicated "Payment"
// deployment (own Verifier/EnygmaDvp/vaults, separate from the main
// deployment's 25-circuit Verifier). Ported from enygma_retail_payments'
// Payment-family circuits, plus a genuine second-asset USDr pair.
//
// Deliberately does not share cmd/deploy's helpers (own package main, own
// config/receipt types) — each command under cmd/ is self-contained.
//
// Build & run (from enygma_dvp/):
//
//	CC=/usr/bin/clang go build -C scripts -o /tmp/deploy_payment ./cmd/deploy_payment
//	/tmp/deploy_payment
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

type paymentContractArtifact struct {
	ContractName string          `json:"contractName"`
	ABI          json.RawMessage `json:"abi"`
	Bytecode     string          `json:"bytecode"`
}

type paymentConfig struct {
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

type paymentReceiptData struct {
	ContractAddress string `json:"contractAddress"`
	TransactionHash string `json:"transactionHash"`
	BlockNumber     uint64 `json:"blockNumber"`
	GasUsed         uint64 `json:"gasUsed"`
}

type paymentDeploymentReceipts map[string]paymentReceiptData

var paymentProjectRoot string

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get working directory:", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "enygmadvp_payment.config.json")); err == nil {
			paymentProjectRoot = dir
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	paymentProjectRoot = cwd
}

func main() {
	if err := deployPayment(); err != nil {
		log.Fatal("Deployment failed:", err)
	}
}

func deployPayment() error {
	config, err := loadPaymentConfig()
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
	if len(config.Network.Accounts) < 1 {
		return fmt.Errorf("need at least 1 account in config")
	}

	owner, err := paymentTransactOpts(config.Network.Accounts[0].Private, chainID)
	if err != nil {
		return fmt.Errorf("failed to create owner transact opts: %w", err)
	}
	fmt.Printf("owner: %s\n", owner.From.Hex())

	receipts := make(paymentDeploymentReceipts)

	fmt.Println("Deploying PoseidonT3...")
	poseidonT3Address, receipt, err := paymentDeployContract(client, owner, "core/contracts/Poseidon.sol/PoseidonT3")
	if err != nil {
		return fmt.Errorf("failed to deploy PoseidonT3: %w", err)
	}
	fmt.Printf("PoseidonT3 -> %s\n", poseidonT3Address.Hex())
	receipts["Poseidon"] = paymentReceiptToData(receipt, poseidonT3Address)

	fmt.Println("Deploying PoseidonT5...")
	poseidonT5Address, receipt, err := paymentDeployContract(client, owner, "core/contracts/Poseidon.sol/PoseidonT5")
	if err != nil {
		return fmt.Errorf("failed to deploy PoseidonT5: %w", err)
	}
	fmt.Printf("PoseidonT5 -> %s\n", poseidonT5Address.Hex())

	fmt.Println("Deploying PoseidonWrapper...")
	poseidonWrapperAddress, receipt, err := paymentDeployContractWithLibraries(
		client, owner,
		"core/contracts/PoseidonWrapper.sol/PoseidonWrapper",
		map[string]common.Address{
			"PoseidonT3": poseidonT3Address,
			"PoseidonT5": poseidonT5Address,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to deploy PoseidonWrapper: %w", err)
	}
	fmt.Printf("PoseidonWrapper -> %s\n", poseidonWrapperAddress.Hex())
	receipts["PoseidonWrapper"] = paymentReceiptToData(receipt, poseidonWrapperAddress)

	fmt.Println("Deploying GenericGroth16Verifier...")
	g16VerifierAddress, receipt, err := paymentDeployContract(client, owner, "core/contracts/GenericGroth16Verifier.sol/GenericGroth16Verifier")
	if err != nil {
		return fmt.Errorf("failed to deploy GenericGroth16Verifier: %w", err)
	}
	fmt.Printf("GenericGroth16Verifier -> %s\n", g16VerifierAddress.Hex())
	receipts["G16Verifier"] = paymentReceiptToData(receipt, g16VerifierAddress)

	fmt.Println("Deploying Verifier...")
	verifierAddress, receipt, err := paymentDeployContract(client, owner, "core/contracts/Verifier.sol/Verifier")
	if err != nil {
		return fmt.Errorf("failed to deploy Verifier: %w", err)
	}
	fmt.Printf("Verifier -> %s\n", verifierAddress.Hex())
	receipts["Verifier"] = paymentReceiptToData(receipt, verifierAddress)

	fmt.Println("Deploying PrivateMintVerifier...")
	privateMintVerifierAddress, receipt, err := paymentDeployContract(client, owner, "core/contracts/PrivateMintVerifier.sol/PrivateMintVerifier")
	if err != nil {
		return fmt.Errorf("failed to deploy PrivateMintVerifier: %w", err)
	}
	fmt.Printf("PrivateMintVerifier -> %s\n", privateMintVerifierAddress.Hex())
	receipts["PrivateMintVerifier"] = paymentReceiptToData(receipt, privateMintVerifierAddress)

	fmt.Println("Deploying EnygmaDvp...")
	enygmaDvpAddress, receipt, err := paymentDeployContractWithArgs(
		client, owner,
		"core/contracts/EnygmaDvp.sol/EnygmaDvp",
		poseidonWrapperAddress, g16VerifierAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy EnygmaDvp: %w", err)
	}
	fmt.Printf("EnygmaDvp -> %s\n", enygmaDvpAddress.Hex())
	receipts["EnygmaDvp"] = paymentReceiptToData(receipt, enygmaDvpAddress)

	fmt.Println("Deploying RaylsERC20 (main payment token)...")
	erc20Address, receipt, err := paymentDeployContractWithArgs(
		client, owner,
		"erc20/contracts/RaylsERC20.sol/RaylsERC20",
		"PaymentERC20", "PAY20",
	)
	if err != nil {
		return fmt.Errorf("failed to deploy RaylsERC20: %w", err)
	}
	fmt.Printf("RaylsERC20 -> %s\n", erc20Address.Hex())
	receipts["ERC20"] = paymentReceiptToData(receipt, erc20Address)

	fmt.Println("Deploying Erc20CoinVault (main payment vault)...")
	erc20CoinVaultAddress, receipt, err := paymentDeployContractWithArgs(
		client, owner,
		"core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault",
		enygmaDvpAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy Erc20CoinVault: %w", err)
	}
	fmt.Printf("Erc20CoinVault -> %s\n", erc20CoinVaultAddress.Hex())
	receipts["Erc20CoinVault"] = paymentReceiptToData(receipt, erc20CoinVaultAddress)

	// USDr — a second, independent relayer-fee asset: its own token and vault,
	// registered on the same EnygmaDvp instance (vaultId=1, auto-assigned by
	// registerVault in cmd/init_payment). See UsdrFeeCircuit /
	// EnygmaDvp.paymentWithUsdrFee.
	fmt.Println("Deploying UsdrERC20...")
	usdrErc20Address, receipt, err := paymentDeployContractWithArgs(
		client, owner,
		"erc20/contracts/RaylsERC20.sol/RaylsERC20",
		"USDr", "USDR",
	)
	if err != nil {
		return fmt.Errorf("failed to deploy UsdrERC20: %w", err)
	}
	fmt.Printf("UsdrERC20 -> %s\n", usdrErc20Address.Hex())
	receipts["UsdrERC20"] = paymentReceiptToData(receipt, usdrErc20Address)

	fmt.Println("Deploying UsdrCoinVault...")
	usdrCoinVaultAddress, receipt, err := paymentDeployContractWithArgs(
		client, owner,
		"core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault",
		enygmaDvpAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy UsdrCoinVault: %w", err)
	}
	fmt.Printf("UsdrCoinVault -> %s\n", usdrCoinVaultAddress.Hex())
	receipts["UsdrCoinVault"] = paymentReceiptToData(receipt, usdrCoinVaultAddress)

	if err := savePaymentReceipts(receipts); err != nil {
		return fmt.Errorf("failed to save receipts: %w", err)
	}
	fmt.Println("Receipts saved to ./build/payment_receipts.json")
	return nil
}

func loadPaymentConfig() (*paymentConfig, error) {
	configPath := filepath.Join(paymentProjectRoot, "enygmadvp_payment.config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config paymentConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func loadPaymentArtifact(contractPath string) (*paymentContractArtifact, error) {
	artifactPath := filepath.Join(paymentProjectRoot, "artifacts", "contracts", contractPath+".json")
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, err
	}
	var artifact paymentContractArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func paymentTransactOpts(privateKeyHex string, chainID *big.Int) (*bind.TransactOpts, error) {
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, err
	}
	return bind.NewKeyedTransactorWithChainID(privateKey, chainID)
}

func paymentDeployContract(client *ethclient.Client, auth *bind.TransactOpts, contractPath string) (common.Address, *types.Receipt, error) {
	return paymentDeployContractWithArgs(client, auth, contractPath)
}

func paymentDeployContractWithArgs(client *ethclient.Client, auth *bind.TransactOpts, contractPath string, args ...interface{}) (common.Address, *types.Receipt, error) {
	artifact, err := loadPaymentArtifact(contractPath)
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

func paymentDeployContractWithLibraries(client *ethclient.Client, auth *bind.TransactOpts, contractPath string, libMap map[string]common.Address) (common.Address, *types.Receipt, error) {
	artifactPath := filepath.Join(paymentProjectRoot, "artifacts/contracts", contractPath+".json")
	rawData, err := os.ReadFile(artifactPath)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to read artifact %s: %w", contractPath, err)
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
		return common.Address{}, nil, fmt.Errorf("failed to parse artifact JSON: %w", err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(fullArtifact.ABI)))
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to parse ABI: %w", err)
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

	bytecode := common.FromHex("0x" + bytecodeHex)
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

func paymentReceiptToData(receipt *types.Receipt, contractAddress common.Address) paymentReceiptData {
	return paymentReceiptData{
		ContractAddress: contractAddress.Hex(),
		TransactionHash: receipt.TxHash.Hex(),
		BlockNumber:     receipt.BlockNumber.Uint64(),
		GasUsed:         receipt.GasUsed,
	}
}

func savePaymentReceipts(receipts paymentDeploymentReceipts) error {
	buildDir := filepath.Join(paymentProjectRoot, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(receipts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(buildDir, "payment_receipts.json"), data, 0644)
}
