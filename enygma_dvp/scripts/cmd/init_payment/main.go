// cmd/init_payment — self-contained initializer for the dedicated Payment
// deployment (see cmd/deploy_payment). Registers the 5 Payment-family VKs
// (Payment/Payment2in/PaymentFee/PaymentRelayerFeePublic/UsdrFee) and both
// vaults (main payment token + USDr) on the freshly deployed EnygmaDvp.
//
// Deliberately does not share cmd/init's helpers (own package main, own
// copies of the VK-parsing types/functions) — same convention as
// cmd/deploy_payment.
//
// Build & run (from enygma_dvp/, after cmd/deploy_payment and
// `go run ./gnark_circuits/cmd/export_vk_payment ./build`):
//
//	CC=/usr/bin/clang go build -C scripts -o /tmp/init_payment ./cmd/init_payment
//	/tmp/init_payment
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

type initPaymentConfig struct {
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

type initPaymentReceiptData struct {
	ContractAddress string `json:"contractAddress"`
	TransactionHash string `json:"transactionHash"`
	BlockNumber     uint64 `json:"blockNumber"`
	GasUsed         uint64 `json:"gasUsed"`
}

type initPaymentReceipts map[string]initPaymentReceiptData

// VerificationKeyJSON / G1Point / G2Point / VerifyingKey / formatVKey /
// mustParseBigInt are copied verbatim from scripts/cmd/init — same circom-
// export shape produced by cmd/export_vk_payment, same on-chain struct
// layout expected by IEnygmaDvp.VerifyingKey.

type initPaymentVKJSON struct {
	Protocol    string       `json:"protocol"`
	Curve       string       `json:"curve"`
	VkAlpha1    []string     `json:"vk_alpha_1"`
	VkBeta2     [][]string   `json:"vk_beta_2"`
	VkGamma2    [][]string   `json:"vk_gamma_2"`
	VkDelta2    [][]string   `json:"vk_delta_2"`
	VkAlphabeta [][][]string `json:"vk_alphabeta_12"`
	IC          [][]string   `json:"IC"`
}

type initPaymentG1Point struct {
	X *big.Int `abi:"x"`
	Y *big.Int `abi:"y"`
}

type initPaymentG2Point struct {
	X [2]*big.Int `abi:"x"`
	Y [2]*big.Int `abi:"y"`
}

type initPaymentVerifyingKey struct {
	Alpha1 initPaymentG1Point   `abi:"alpha1"`
	Beta2  initPaymentG2Point   `abi:"beta2"`
	Gamma2 initPaymentG2Point   `abi:"gamma2"`
	Delta2 initPaymentG2Point   `abi:"delta2"`
	Ic     []initPaymentG1Point `abi:"ic"`
}

var initPaymentProjectRoot string

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get working directory:", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "enygmadvp_payment.config.json")); err == nil {
			initPaymentProjectRoot = dir
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	initPaymentProjectRoot = cwd
}

func main() {
	if err := initializePayment(); err != nil {
		log.Fatal("Initialization failed:", err)
	}
}

func initializePayment() error {
	config, err := loadInitPaymentConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	treeDepth := big.NewInt(int64(config.Circom.MetaParameters.TreeDepth))
	fmt.Printf("Merkle tree depth: %d\n", treeDepth)

	receipts, err := loadPaymentInitReceipts()
	if err != nil {
		return fmt.Errorf("failed to load receipts: %w", err)
	}

	vkeys, err := getPaymentVerificationKeys(config.Circom.Circuits)
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
	auth, err := getPaymentInitTransactOpts(config.Network.Accounts[0].Private, chainID)
	if err != nil {
		return fmt.Errorf("failed to create transact opts: %w", err)
	}

	enygmaDvpABI, err := loadPaymentInitContractABI("core/contracts/EnygmaDvp.sol/EnygmaDvp")
	if err != nil {
		return fmt.Errorf("failed to load EnygmaDvp ABI: %w", err)
	}
	verifierABI, err := loadPaymentInitContractABI("core/contracts/Verifier.sol/Verifier")
	if err != nil {
		return fmt.Errorf("failed to load Verifier ABI: %w", err)
	}

	enygmaDvpAddress := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	verifierAddress := common.HexToAddress(receipts["Verifier"].ContractAddress)
	g16VerifierAddress := common.HexToAddress(receipts["G16Verifier"].ContractAddress)
	fmt.Printf("EnygmaDvp:  %s\n", enygmaDvpAddress.Hex())
	fmt.Printf("Verifier:   %s\n", verifierAddress.Hex())

	fmt.Println("Initializing Verifier (groth16 backend)...")
	if _, err := callPaymentInitMethod(client, auth, verifierABI, verifierAddress, "initializeVerifier", g16VerifierAddress); err != nil {
		return fmt.Errorf("failed to initialize Verifier: %w", err)
	}

	dvpOwnerRole := crypto.Keccak256Hash([]byte("ownerRole"))
	fmt.Println("Granting DEFAULT_OWNER_ROLE on Verifier to EnygmaDvp...")
	if _, err := callPaymentInitMethod(client, auth, verifierABI, verifierAddress, "grantRole", dvpOwnerRole, enygmaDvpAddress); err != nil {
		return fmt.Errorf("failed to grant Verifier owner role to EnygmaDvp: %w", err)
	}

	fmt.Println("Initializing EnygmaDvp...")
	if _, err := callPaymentInitMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "initializeDvp", verifierAddress); err != nil {
		return fmt.Errorf("failed to initialize EnygmaDvp: %w", err)
	}

	for i, vkey := range vkeys {
		fmt.Printf("Registering verification key %d...\n", i+1)
		if _, err := callPaymentInitMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerNewVerificationKey", vkey); err != nil {
			return fmt.Errorf("failed to register verification key %d: %w", i, err)
		}
	}

	privateMintVerifierAddress := common.HexToAddress(receipts["PrivateMintVerifier"].ContractAddress)
	fmt.Printf("Registering PrivateMintVerifier (%s)...\n", privateMintVerifierAddress.Hex())
	if _, err := callPaymentInitMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerPrivateMintVerifier", privateMintVerifierAddress); err != nil {
		return fmt.Errorf("failed to register PrivateMintVerifier: %w", err)
	}

	fmt.Println("Registering Erc20CoinVault (main payment token, vaultId 0)...")
	if _, err := callPaymentInitMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerVault",
		common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress),
		common.HexToAddress(receipts["ERC20"].ContractAddress),
		big.NewInt(1),
		treeDepth,
	); err != nil {
		return fmt.Errorf("failed to register Erc20CoinVault: %w", err)
	}

	// USDr — a second, independent relayer-fee asset. registerVault assigns
	// vaultId=1 automatically (2nd vault on this EnygmaDvp instance). See
	// EnygmaDvp.paymentWithUsdrFee.
	fmt.Println("Registering UsdrCoinVault (vaultId 1)...")
	if _, err := callPaymentInitMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "registerVault",
		common.HexToAddress(receipts["UsdrCoinVault"].ContractAddress),
		common.HexToAddress(receipts["UsdrERC20"].ContractAddress),
		big.NewInt(1),
		treeDepth,
	); err != nil {
		return fmt.Errorf("failed to register UsdrCoinVault: %w", err)
	}

	// usdrTokenId defaults to 0 — the same fixed convention value every
	// Payment-family circuit's WtTokenId/StTokenId uses in this codebase.
	fmt.Println("Setting usdrTokenId = 0...")
	if _, err := callPaymentInitMethod(client, auth, enygmaDvpABI, enygmaDvpAddress, "setUsdrTokenId", big.NewInt(0)); err != nil {
		return fmt.Errorf("failed to set usdrTokenId: %w", err)
	}

	fmt.Println("EnygmaDvp initialized for the dedicated Payment deployment.")
	return nil
}

func loadInitPaymentConfig() (*initPaymentConfig, error) {
	configPath := filepath.Join(initPaymentProjectRoot, "enygmadvp_payment.config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config initPaymentConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func loadPaymentInitReceipts() (initPaymentReceipts, error) {
	receiptsPath := filepath.Join(initPaymentProjectRoot, "build", "payment_receipts.json")
	data, err := os.ReadFile(receiptsPath)
	if err != nil {
		return nil, err
	}
	var receipts initPaymentReceipts
	if err := json.Unmarshal(data, &receipts); err != nil {
		return nil, err
	}
	return receipts, nil
}

func loadPaymentInitContractABI(contractPath string) (abi.ABI, error) {
	artifactPath := filepath.Join(initPaymentProjectRoot, "artifacts", "contracts", contractPath+".json")
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

func getPaymentInitTransactOpts(privateKeyHex string, chainID *big.Int) (*bind.TransactOpts, error) {
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, err
	}
	return bind.NewKeyedTransactorWithChainID(privateKey, chainID)
}

func getPaymentVerificationKeys(circuits []struct {
	ID       int      `json:"id"`
	Filename string   `json:"filename"`
	Tags     []string `json:"tags"`
}) ([]initPaymentVerifyingKey, error) {
	var verificationKeys []initPaymentVerifyingKey
	for _, circuit := range circuits {
		filePath := filepath.Join(initPaymentProjectRoot, "build", circuit.Filename+".json")
		fmt.Println("Loading VK from:", filePath)

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read verification key file %s: %w", filePath, err)
		}
		var vkJSON initPaymentVKJSON
		if err := json.Unmarshal(data, &vkJSON); err != nil {
			return nil, fmt.Errorf("failed to parse verification key JSON %s: %w", filePath, err)
		}
		verificationKeys = append(verificationKeys, formatPaymentVKey(vkJSON))
	}
	return verificationKeys, nil
}

func mustParsePaymentBigInt(s, field string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic(fmt.Sprintf("formatPaymentVKey: invalid decimal integer for %s: %q", field, s))
	}
	return v
}

func formatPaymentVKey(vkey initPaymentVKJSON) initPaymentVerifyingKey {
	var ic []initPaymentG1Point
	for i, point := range vkey.IC {
		x := mustParsePaymentBigInt(point[0], fmt.Sprintf("IC[%d].X", i))
		y := mustParsePaymentBigInt(point[1], fmt.Sprintf("IC[%d].Y", i))
		ic = append(ic, initPaymentG1Point{X: x, Y: y})
	}

	alpha1X := mustParsePaymentBigInt(vkey.VkAlpha1[0], "VkAlpha1.X")
	alpha1Y := mustParsePaymentBigInt(vkey.VkAlpha1[1], "VkAlpha1.Y")

	beta2X0 := mustParsePaymentBigInt(vkey.VkBeta2[0][0], "VkBeta2[0][0]")
	beta2X1 := mustParsePaymentBigInt(vkey.VkBeta2[0][1], "VkBeta2[0][1]")
	beta2Y0 := mustParsePaymentBigInt(vkey.VkBeta2[1][0], "VkBeta2[1][0]")
	beta2Y1 := mustParsePaymentBigInt(vkey.VkBeta2[1][1], "VkBeta2[1][1]")

	gamma2X0 := mustParsePaymentBigInt(vkey.VkGamma2[0][0], "VkGamma2[0][0]")
	gamma2X1 := mustParsePaymentBigInt(vkey.VkGamma2[0][1], "VkGamma2[0][1]")
	gamma2Y0 := mustParsePaymentBigInt(vkey.VkGamma2[1][0], "VkGamma2[1][0]")
	gamma2Y1 := mustParsePaymentBigInt(vkey.VkGamma2[1][1], "VkGamma2[1][1]")

	delta2X0 := mustParsePaymentBigInt(vkey.VkDelta2[0][0], "VkDelta2[0][0]")
	delta2X1 := mustParsePaymentBigInt(vkey.VkDelta2[0][1], "VkDelta2[0][1]")
	delta2Y0 := mustParsePaymentBigInt(vkey.VkDelta2[1][0], "VkDelta2[1][0]")
	delta2Y1 := mustParsePaymentBigInt(vkey.VkDelta2[1][1], "VkDelta2[1][1]")

	// VkBeta2[0][0] = A1 (imaginary), VkBeta2[0][1] = A0 (real) — EIP-197 /
	// GenericGroth16Verifier expects x[0]=imaginary, x[1]=real.
	return initPaymentVerifyingKey{
		Alpha1: initPaymentG1Point{X: alpha1X, Y: alpha1Y},
		Beta2: initPaymentG2Point{
			X: [2]*big.Int{beta2X0, beta2X1},
			Y: [2]*big.Int{beta2Y0, beta2Y1},
		},
		Gamma2: initPaymentG2Point{
			X: [2]*big.Int{gamma2X0, gamma2X1},
			Y: [2]*big.Int{gamma2Y0, gamma2Y1},
		},
		Delta2: initPaymentG2Point{
			X: [2]*big.Int{delta2X0, delta2X1},
			Y: [2]*big.Int{delta2Y0, delta2Y1},
		},
		Ic: ic,
	}
}

func callPaymentInitMethod(client *ethclient.Client, auth *bind.TransactOpts, contractABI abi.ABI, contractAddress common.Address, method string, args ...interface{}) (common.Hash, error) {
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
