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

// Config represents the enygmadvp.config.json structure
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

// ReceiptData represents deployment receipt information
type InitReceiptData struct {
	ContractAddress string `json:"contractAddress"`
	TransactionHash string `json:"transactionHash"`
	BlockNumber     uint64 `json:"blockNumber"`
	GasUsed         uint64 `json:"gasUsed"`
}

// InitReceipts holds all deployment receipts
type InitReceipts map[string]InitReceiptData

// VerificationKeyJSON represents the JSON structure of a verification key file
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

// G1Point represents a point on the G1 curve (field names must match Solidity struct)
type G1Point struct {
	X *big.Int `abi:"x"`
	Y *big.Int `abi:"y"`
}

// G2Point represents a point on the G2 curve (field names must match Solidity struct)
type G2Point struct {
	X [2]*big.Int `abi:"x"`
	Y [2]*big.Int `abi:"y"`
}

// VerifyingKey represents the formatted verification key for the contract
type VerifyingKey struct {
	Alpha1 G1Point   `abi:"alpha1"`
	Beta2  G2Point   `abi:"beta2"`
	Gamma2 G2Point   `abi:"gamma2"`
	Delta2 G2Point   `abi:"delta2"`
	Ic     []G1Point `abi:"ic"`
}

var (
	initProjectRoot string
)

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get working directory:", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "enygmadvp.config.json")); err == nil {
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
	if err := initializeDvp(); err != nil {
		log.Fatal("Initialization failed:", err)
	}
}

func initializeDvp() error {
	// Load config
	config, err := loadInitConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	treeDepth := big.NewInt(int64(config.Circom.MetaParameters.TreeDepth))
	fmt.Printf("Merkle Tree depth: %d.\n", treeDepth)

	// Load receipts
	receipts, err := loadReceipts()
	if err != nil {
		return fmt.Errorf("failed to load receipts: %w", err)
	}

	// Load verification keys
	vkeys, err := getVerificationKeys(config.Circom.Circuits)
	if err != nil {
		return fmt.Errorf("failed to load verification keys: %w", err)
	}
	fmt.Printf("Loaded %d verification keys\n", len(vkeys))

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

	// Set up owner transact opts
	auth, err := getInitTransactOpts(config.Network.Accounts[0].Private, chainID)
	if err != nil {
		return fmt.Errorf("failed to create transact opts: %w", err)
	}

	// Load EnygmaDvp contract ABI
	enygmaDvpABI, err := loadContractABI("core/contracts/EnygmaDvp.sol/EnygmaDvp")
	if err != nil {
		return fmt.Errorf("failed to load EnygmaDvp ABI: %w", err)
	}

	enygmaDvpAddress := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	verifierAddress := common.HexToAddress(receipts["Verifier"].ContractAddress)
	g16VerifierAddress := common.HexToAddress(receipts["G16Verifier"].ContractAddress)

	fmt.Printf("Verifier Address: %s\n", verifierAddress.Hex())

	// DVP-1 fix: Verifier is now initialized directly by the deployer rather than
	// indirectly through EnygmaDvp.initializeDvp. This closes the front-running
	// window that existed when initializeVerifier was world-callable between Verifier
	// deployment and the first initializeDvp call.
	verifierABI, err := loadContractABI("core/contracts/Verifier.sol/Verifier")
	if err != nil {
		return fmt.Errorf("failed to load Verifier ABI: %w", err)
	}

	fmt.Println("Initializing Verifier with G16 verifier address...")
	_, err = callContractMethod(client, auth, verifierABI, verifierAddress, "initializeVerifier", g16VerifierAddress)
	if err != nil {
		return fmt.Errorf("failed to initialize Verifier: %w", err)
	}

	// DVP-1 fix: Grant DEFAULT_OWNER_ROLE on Verifier to EnygmaDvp so it can
	// register verification keys via Verifier.addVerificationKey.
	// DEFAULT_OWNER_ROLE = keccak256(abi.encodePacked("ownerRole"))
	dvpOwnerRole := crypto.Keccak256Hash([]byte("ownerRole"))
	fmt.Println("Granting DEFAULT_OWNER_ROLE on Verifier to EnygmaDvp...")
	_, err = callContractMethod(client, auth, verifierABI, verifierAddress, "grantRole", dvpOwnerRole, enygmaDvpAddress)
	if err != nil {
		return fmt.Errorf("failed to grant Verifier owner role to EnygmaDvp: %w", err)
	}

	// Initialize EnygmaDvp
	fmt.Println("initializing EnygmaDvp smart contract...")
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "initializeDvp", verifierAddress)
	if err != nil {
		return fmt.Errorf("failed to initialize EnygmaDvp: %w", err)
	}

	// Register verification keys
	for i, vkey := range vkeys {
		fmt.Printf("registering VerificationKey no %d\n", i)
		_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerNewVerificationKey", vkey)
		if err != nil {
			return fmt.Errorf("failed to register verification key %d: %w", i, err)
		}
	}

	// Register PrivateMintVerifier
	fmt.Println("Registering PrivateMintVerifier...")
	privateMintVerifierAddress := common.HexToAddress(receipts["PrivateMintVerifier"].ContractAddress)
	fmt.Printf("PrivateMintVerifier Address: %s\n", privateMintVerifierAddress.Hex())

	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerPrivateMintVerifier", privateMintVerifierAddress)
	if err != nil {
		return fmt.Errorf("failed to register PrivateMintVerifier: %w", err)
	}
	fmt.Println("... Registered PrivateMintVerifier")

	// Register CoinVaults
	fmt.Println("Registering CoinVaults to EnygmaDvp smart contract address.")

	// Erc20CoinVault
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerVault",
		common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress),
		common.HexToAddress(receipts["ERC20"].ContractAddress),
		big.NewInt(1),
		treeDepth,
	)
	if err != nil {
		return fmt.Errorf("failed to register Erc20CoinVault: %w", err)
	}
	fmt.Println("... Registered Erc20CoinVault")

	// Erc721CoinVault
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerVault",
		common.HexToAddress(receipts["Erc721CoinVault"].ContractAddress),
		common.HexToAddress(receipts["ERC721"].ContractAddress),
		big.NewInt(1),
		treeDepth,
	)
	if err != nil {
		return fmt.Errorf("failed to register Erc721CoinVault: %w", err)
	}
	fmt.Println("... Registered Erc721CoinVault")

	// Erc1155CoinVault
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerVault",
		common.HexToAddress(receipts["Erc1155CoinVault"].ContractAddress),
		common.HexToAddress(receipts["ERC1155"].ContractAddress),
		big.NewInt(2),
		treeDepth,
	)
	if err != nil {
		return fmt.Errorf("failed to register Erc1155CoinVault: %w", err)
	}
	fmt.Println("... Registered Erc1155CoinVault")

	// EnygmaErc20CoinVault
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerVault",
		common.HexToAddress(receipts["EnygmaErc20CoinVault"].ContractAddress),
		common.HexToAddress(receipts["ERC20"].ContractAddress),
		big.NewInt(1),
		treeDepth,
	)
	if err != nil {
		return fmt.Errorf("failed to register EnygmaErc20CoinVault: %w", err)
	}
	fmt.Println("... Registered EnygmaErc20CoinVault")

	// Register AssetGroups
	fmt.Println("Registering AssetGroups to EnygmaDvp smart contract.")

	// FungibleAssetGroup
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerAssetGroup",
		common.HexToAddress(receipts["FungibleAssetGroup"].ContractAddress),
		"Fungibles",
		true,
		treeDepth,
	)
	if err != nil {
		return fmt.Errorf("failed to register FungibleAssetGroup: %w", err)
	}
	fmt.Println("... Registered FungibleAssetGroup")

	// NonFungibleAssetGroup
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerAssetGroup",
		common.HexToAddress(receipts["NonFungibleAssetGroup"].ContractAddress),
		"NonFungibles",
		false,
		treeDepth,
	)
	if err != nil {
		return fmt.Errorf("failed to register NonFungibleAssetGroup: %w", err)
	}
	fmt.Println("... Registered NonFungibleAssetGroup")

	// Register Exchange Group Pair
	fmt.Println("Registering Fungible-Fungible groupPair to valid exchange groupPairs.")
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerExchangeGroupPair",
		big.NewInt(0),
		big.NewInt(0),
	)
	if err != nil {
		return fmt.Errorf("failed to register exchange group pair: %w", err)
	}

	// Register Swap Group Pair
	fmt.Println("Registering Fungible-nonFungible groupPair to valid swap groupPairs.")
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerSwapGroupPair",
		big.NewInt(0),
		big.NewInt(1),
	)
	if err != nil {
		return fmt.Errorf("failed to register swap group pair: %w", err)
	}

	// Add vaults to groups
	fmt.Println("Registering Erc20 vaultId in Fungibles assetGroup")
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "addVaultToGroup",
		big.NewInt(0),
		big.NewInt(0),
	)
	if err != nil {
		return fmt.Errorf("failed to add vault to group: %w", err)
	}

	fmt.Println("Registering Erc721 vaultId in NonFungibles assetGroup")
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "addVaultToGroup",
		big.NewInt(1),
		big.NewInt(1),
	)
	if err != nil {
		return fmt.Errorf("failed to add vault to group: %w", err)
	}

	fmt.Println("Registering Enygma ERC20 vaultId in Fungibles assetGroup")
	_, err = callContractMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "addVaultToGroup",
		big.NewInt(3),
		big.NewInt(0),
	)
	if err != nil {
		return fmt.Errorf("failed to add vault to group: %w", err)
	}

	// Add Enygma address to EnygmaErc20CoinVault
	enygmaVaultABI, err := loadContractABI("core/contracts/vaults/EnygmaErc20CoinVault.sol/EnygmaErc20CoinVault")
	if err != nil {
		return fmt.Errorf("failed to load EnygmaErc20CoinVault ABI: %w", err)
	}

	enygmaVaultAddress := common.HexToAddress(receipts["EnygmaErc20CoinVault"].ContractAddress)
	enygmaAddress := EnygmaAddress()

	_, err = callContractMethod(client, auth, enygmaVaultABI, enygmaVaultAddress, "addEnygma", enygmaAddress)
	if err != nil {
		return fmt.Errorf("failed to add Enygma to vault: %w", err)
	}
	fmt.Println("enygma was added into EnygmaErc20CoinVault")

	fmt.Println("EnygmaDvp has been initialized.")
	return nil
}

func loadInitConfig() (*InitConfig, error) {
	configPath := filepath.Join(initProjectRoot, "enygmadvp.config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config InitConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func loadReceipts() (InitReceipts, error) {
	receiptsPath := filepath.Join(initProjectRoot, "build", "receipts.json")
	data, err := os.ReadFile(receiptsPath)
	if err != nil {
		return nil, err
	}

	var receipts InitReceipts
	if err := json.Unmarshal(data, &receipts); err != nil {
		return nil, err
	}

	return receipts, nil
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

	parsedABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		return abi.ABI{}, err
	}

	return parsedABI, nil
}

func getInitTransactOpts(privateKeyHex string, chainID *big.Int) (*bind.TransactOpts, error) {
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

func getVerificationKeys(circuits []struct {
	ID       int      `json:"id"`
	Filename string   `json:"filename"`
	Tags     []string `json:"tags"`
}) ([]VerifyingKey, error) {
	var verificationKeys []VerifyingKey

	for _, circuit := range circuits {
		filePath := filepath.Join(initProjectRoot, "build", circuit.Filename+".json")
		fmt.Println(filePath)

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read verification key file %s: %w", filePath, err)
		}

		var vkJSON VerificationKeyJSON
		if err := json.Unmarshal(data, &vkJSON); err != nil {
			return nil, fmt.Errorf("failed to parse verification key JSON %s: %w", filePath, err)
		}

		vk := formatVKey(vkJSON)
		verificationKeys = append(verificationKeys, vk)
	}

	return verificationKeys, nil
}

func mustParseBigInt(s, field string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic(fmt.Sprintf("formatVKey: invalid decimal integer for %s: %q", field, s))
	}
	return v
}

func formatVKey(vkey VerificationKeyJSON) VerifyingKey {
	var ic []G1Point
	for i, point := range vkey.IC {
		x := mustParseBigInt(point[0], fmt.Sprintf("IC[%d].X", i))
		y := mustParseBigInt(point[1], fmt.Sprintf("IC[%d].Y", i))
		ic = append(ic, G1Point{X: x, Y: y})
	}

	alpha1X := mustParseBigInt(vkey.VkAlpha1[0], "VkAlpha1.X")
	alpha1Y := mustParseBigInt(vkey.VkAlpha1[1], "VkAlpha1.Y")

	beta2X0 := mustParseBigInt(vkey.VkBeta2[0][0], "VkBeta2[0][0]")
	beta2X1 := mustParseBigInt(vkey.VkBeta2[0][1], "VkBeta2[0][1]")
	beta2Y0 := mustParseBigInt(vkey.VkBeta2[1][0], "VkBeta2[1][0]")
	beta2Y1 := mustParseBigInt(vkey.VkBeta2[1][1], "VkBeta2[1][1]")

	gamma2X0 := mustParseBigInt(vkey.VkGamma2[0][0], "VkGamma2[0][0]")
	gamma2X1 := mustParseBigInt(vkey.VkGamma2[0][1], "VkGamma2[0][1]")
	gamma2Y0 := mustParseBigInt(vkey.VkGamma2[1][0], "VkGamma2[1][0]")
	gamma2Y1 := mustParseBigInt(vkey.VkGamma2[1][1], "VkGamma2[1][1]")

	delta2X0 := mustParseBigInt(vkey.VkDelta2[0][0], "VkDelta2[0][0]")
	delta2X1 := mustParseBigInt(vkey.VkDelta2[0][1], "VkDelta2[0][1]")
	delta2Y0 := mustParseBigInt(vkey.VkDelta2[1][0], "VkDelta2[1][0]")
	delta2Y1 := mustParseBigInt(vkey.VkDelta2[1][1], "VkDelta2[1][1]")

	// VkBeta2[0][0] = A1 (imaginary), VkBeta2[0][1] = A0 (real).
	// EIP-197 / GenericGroth16Verifier expects x[0]=imaginary, x[1]=real,
	// so use [beta2X0, beta2X1] = [imaginary, real] order.
	return VerifyingKey{
		Alpha1: G1Point{X: alpha1X, Y: alpha1Y},
		Beta2: G2Point{
			X: [2]*big.Int{beta2X0, beta2X1},
			Y: [2]*big.Int{beta2Y0, beta2Y1},
		},
		Gamma2: G2Point{
			X: [2]*big.Int{gamma2X0, gamma2X1},
			Y: [2]*big.Int{gamma2Y0, gamma2Y1},
		},
		Delta2: G2Point{
			X: [2]*big.Int{delta2X0, delta2X1},
			Y: [2]*big.Int{delta2Y0, delta2Y1},
		},
		Ic: ic,
	}
}

func callContractMethod(client *ethclient.Client, auth *bind.TransactOpts, contractABI abi.ABI, contractAddress common.Address, method string, args ...interface{}) (common.Hash, error) {
	contract := bind.NewBoundContract(contractAddress, contractABI, client, client, client)

	tx, err := contract.Transact(auth, method, args...)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to call %s: %w", method, err)
	}

	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to wait for %s: %w", method, err)
	}

	if receipt.Status == 0 {
		return common.Hash{}, fmt.Errorf("transaction %s failed", method)
	}

	return tx.Hash(), nil
}
