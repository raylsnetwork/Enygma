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

// InitConfig represents enygmapayment.config.json for the init script.
type InitConfig struct {
	Network struct {
		Host     string `json:"host"`
		Port     string `json:"port"`
		ChainID  string `json:"chain-id"`
		Accounts []struct {
			Address string `json:"address"`
			Private string `json:"private"`
		} `json:"accounts"`
	} `json:"network"`
	Circom struct {
		MetaParameters struct {
			TreeDepth int `json:"tree-depth"`
		} `json:"meta-parameters"`
		Circuits []struct {
			ID       int      `json:"id"`
			Filename string   `json:"filename"`
			Tags     []string `json:"tags"`
		} `json:"circuits"`
	} `json:"circom"`
}

// ReceiptData represents deployment receipt information.
type InitReceiptData struct {
	ContractAddress string `json:"contractAddress"`
}

// InitReceipts holds all deployment receipts.
type InitReceipts map[string]InitReceiptData

// VerificationKeyJSON represents the circom-format VK JSON.
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

// G1Point represents a point on the G1 curve.
type G1Point struct {
	X *big.Int `abi:"x"`
	Y *big.Int `abi:"y"`
}

// G2Point represents a point on the G2 curve.
type G2Point struct {
	X [2]*big.Int `abi:"x"`
	Y [2]*big.Int `abi:"y"`
}

// VerifyingKey represents the formatted VK for on-chain submission.
type VerifyingKey struct {
	Alpha1 G1Point   `abi:"alpha1"`
	Beta2  G2Point   `abi:"beta2"`
	Gamma2 G2Point   `abi:"gamma2"`
	Delta2 G2Point   `abi:"delta2"`
	Ic     []G1Point `abi:"ic"`
}

var initProjectRoot string

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get working directory:", err)
	}
	// Walk up until we find enygmapayment.config.json (handles running from scripts/).
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "enygmapayment.config.json")); err == nil {
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
	if err := initializePayment(); err != nil {
		log.Fatal("Initialization failed:", err)
	}
}

func initializePayment() error {
	config, err := loadInitConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	treeDepth := big.NewInt(int64(config.Circom.MetaParameters.TreeDepth))
	fmt.Printf("Merkle tree depth: %d\n", treeDepth)

	receipts, err := loadReceipts()
	if err != nil {
		return fmt.Errorf("failed to load receipts: %w", err)
	}

	vkeys, err := getVerificationKeys(config.Circom.Circuits)
	if err != nil {
		return fmt.Errorf("failed to load verification keys: %w", err)
	}
	fmt.Printf("Loaded %d verification key(s)\n", len(vkeys))

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

	auth, err := getInitTransactOpts(config.Network.Accounts[0].Private, chainID)
	if err != nil {
		return fmt.Errorf("failed to create transact opts: %w", err)
	}

	enygmaDvpABI, err := loadContractABI("EnygmaDvp")
	if err != nil {
		return fmt.Errorf("failed to load EnygmaDvp ABI: %w", err)
	}
	verifierABI, err := loadContractABI("Verifier")
	if err != nil {
		return fmt.Errorf("failed to load Verifier ABI: %w", err)
	}

	enygmaDvpAddress := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	verifierAddress := common.HexToAddress(receipts["Verifier"].ContractAddress)
	g16VerifierAddress := common.HexToAddress(receipts["G16Verifier"].ContractAddress)
	fmt.Printf("EnygmaDvp:  %s\n", enygmaDvpAddress.Hex())
	fmt.Printf("Verifier:   %s\n", verifierAddress.Hex())

	// The deployed Verifier.json has initializeVerifier as a world-callable function
	// that stores the G16 verifier address AND grants DEFAULT_OWNER_ROLE to msg.sender.
	// Calling it as the deployer grants the deployer DEFAULT_OWNER_ROLE on Verifier,
	// which lets us call addVerificationKey directly (bypassing EnygmaDvp).
	fmt.Println("Initializing Verifier (groth16 backend)...")
	_, err = callContractMethod(client, auth, verifierABI, verifierAddress, "initializeVerifier", g16VerifierAddress)
	if err != nil {
		return fmt.Errorf("failed to initialize Verifier: %w", err)
	}

	fmt.Println("Initializing EnygmaDvp...")
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "initializeDvp", verifierAddress)
	if err != nil {
		return fmt.Errorf("failed to initialize EnygmaDvp: %w", err)
	}

	// Register VKs directly on Verifier — the deployer has DEFAULT_OWNER_ROLE after
	// calling initializeVerifier above.
	for i, vkey := range vkeys {
		fmt.Printf("Registering verification key %d...\n", i+1)
		_, err = callContractMethod(client, auth, verifierABI, verifierAddress, "addVerificationKey", vkey)
		if err != nil {
			return fmt.Errorf("failed to register verification key %d: %w", i, err)
		}
	}

	privateMintVerifierAddress := common.HexToAddress(receipts["PrivateMintVerifier"].ContractAddress)
	fmt.Printf("Registering PrivateMintVerifier (%s)...\n", privateMintVerifierAddress.Hex())
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerPrivateMintVerifier", privateMintVerifierAddress)
	if err != nil {
		return fmt.Errorf("failed to register PrivateMintVerifier: %w", err)
	}

	fmt.Println("Registering Erc20CoinVault...")
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerVault",
		common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress),
		common.HexToAddress(receipts["ERC20"].ContractAddress),
		big.NewInt(1),
		treeDepth,
	)
	if err != nil {
		return fmt.Errorf("failed to register Erc20CoinVault: %w", err)
	}

	fmt.Println("EnygmaDvp initialized for retail payments.")
	return nil
}

func loadInitConfig() (*InitConfig, error) {
	configPath := filepath.Join(initProjectRoot, "enygmapayment.config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config InitConfig
	return &config, json.Unmarshal(data, &config)
}

func loadReceipts() (InitReceipts, error) {
	receiptsPath := filepath.Join(initProjectRoot, "build", "receipts.json")
	data, err := os.ReadFile(receiptsPath)
	if err != nil {
		return nil, err
	}
	var receipts InitReceipts
	return receipts, json.Unmarshal(data, &receipts)
}

// loadContractABI reads from contracts/abis/<name>.json (flat layout).
func loadContractABI(contractName string) (abi.ABI, error) {
	artifactPath := filepath.Join(initProjectRoot, "contracts", "abis", contractName+".json")
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
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, err
	}
	return bind.NewKeyedTransactorWithChainID(privateKey, chainID)
}

func getVerificationKeys(circuits []struct {
	ID       int      `json:"id"`
	Filename string   `json:"filename"`
	Tags     []string `json:"tags"`
}) ([]VerifyingKey, error) {
	var vkeys []VerifyingKey
	for _, circuit := range circuits {
		filePath := filepath.Join(initProjectRoot, "build", circuit.Filename+".json")
		fmt.Println("Loading VK from:", filePath)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read VK file %s: %w", filePath, err)
		}
		var vkJSON VerificationKeyJSON
		if err := json.Unmarshal(data, &vkJSON); err != nil {
			return nil, fmt.Errorf("parse VK JSON %s: %w", filePath, err)
		}
		vkeys = append(vkeys, formatVKey(vkJSON))
	}
	return vkeys, nil
}

func formatVKey(vkey VerificationKeyJSON) VerifyingKey {
	var ic []G1Point
	for _, point := range vkey.IC {
		x, _ := new(big.Int).SetString(point[0], 10)
		y, _ := new(big.Int).SetString(point[1], 10)
		ic = append(ic, G1Point{X: x, Y: y})
	}
	alpha1X, _ := new(big.Int).SetString(vkey.VkAlpha1[0], 10)
	alpha1Y, _ := new(big.Int).SetString(vkey.VkAlpha1[1], 10)
	beta2X0, _ := new(big.Int).SetString(vkey.VkBeta2[0][0], 10)
	beta2X1, _ := new(big.Int).SetString(vkey.VkBeta2[0][1], 10)
	beta2Y0, _ := new(big.Int).SetString(vkey.VkBeta2[1][0], 10)
	beta2Y1, _ := new(big.Int).SetString(vkey.VkBeta2[1][1], 10)
	gamma2X0, _ := new(big.Int).SetString(vkey.VkGamma2[0][0], 10)
	gamma2X1, _ := new(big.Int).SetString(vkey.VkGamma2[0][1], 10)
	gamma2Y0, _ := new(big.Int).SetString(vkey.VkGamma2[1][0], 10)
	gamma2Y1, _ := new(big.Int).SetString(vkey.VkGamma2[1][1], 10)
	delta2X0, _ := new(big.Int).SetString(vkey.VkDelta2[0][0], 10)
	delta2X1, _ := new(big.Int).SetString(vkey.VkDelta2[0][1], 10)
	delta2Y0, _ := new(big.Int).SetString(vkey.VkDelta2[1][0], 10)
	delta2Y1, _ := new(big.Int).SetString(vkey.VkDelta2[1][1], 10)

	return VerifyingKey{
		Alpha1: G1Point{X: alpha1X, Y: alpha1Y},
		Beta2:  G2Point{X: [2]*big.Int{beta2X0, beta2X1}, Y: [2]*big.Int{beta2Y0, beta2Y1}},
		Gamma2: G2Point{X: [2]*big.Int{gamma2X0, gamma2X1}, Y: [2]*big.Int{gamma2Y0, gamma2Y1}},
		Delta2: G2Point{X: [2]*big.Int{delta2X0, delta2X1}, Y: [2]*big.Int{delta2Y0, delta2Y1}},
		Ic:     ic,
	}
}

func callContractMethod(client *ethclient.Client, auth *bind.TransactOpts, contractABI abi.ABI, contractAddress common.Address, method string, args ...interface{}) (common.Hash, error) {
	contract := bind.NewBoundContract(contractAddress, contractABI, client, client, client)
	tx, err := contract.Transact(auth, method, args...)
	if err != nil {
		return common.Hash{}, fmt.Errorf("call %s: %w", method, err)
	}
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("wait mined %s: %w", method, err)
	}
	if receipt.Status == 0 {
		return common.Hash{}, fmt.Errorf("transaction %s reverted", method)
	}
	return tx.Hash(), nil
}
