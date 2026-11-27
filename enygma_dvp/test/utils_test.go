package tests

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/core"
)

// --- Buffer2BigInt ---

func TestBuffer2BigInt(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected *big.Int
	}{
		{"nil input", nil, big.NewInt(0)},
		{"empty slice", []byte{}, big.NewInt(0)},
		{"single byte", []byte{0x0a}, big.NewInt(10)},
		{"two bytes", []byte{0x01, 0x00}, big.NewInt(256)},
		{"larger value", []byte{0xff, 0xff}, big.NewInt(65535)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := core.Buffer2BigInt(tt.input)
			if result.Cmp(tt.expected) != 0 {
				t.Errorf("Buffer2BigInt(%v) = %s, want %s", tt.input, result.String(), tt.expected.String())
			}
		})
	}
}

// --- StringifyBigInts ---

func TestStringifyBigInts(t *testing.T) {
	input := map[string]interface{}{
		"a": big.NewInt(123),
		"b": []*big.Int{big.NewInt(1), big.NewInt(2)},
		"c": "plain string",
		"d": map[string]interface{}{
			"nested": big.NewInt(456),
		},
	}

	result := core.StringifyBigInts(input)

	if result["a"] != "123" {
		t.Errorf("Expected a=123, got %v", result["a"])
	}

	bSlice, ok := result["b"].([]string)
	if !ok || len(bSlice) != 2 || bSlice[0] != "1" || bSlice[1] != "2" {
		t.Errorf("Expected b=[1, 2], got %v", result["b"])
	}

	if result["c"] != "plain string" {
		t.Errorf("Expected c=plain string, got %v", result["c"])
	}

	nested, ok := result["d"].(map[string]interface{})
	if !ok || nested["nested"] != "456" {
		t.Errorf("Expected d.nested=456, got %v", result["d"])
	}
}

func TestStringifyBigIntsEmpty(t *testing.T) {
	result := core.StringifyBigInts(map[string]interface{}{})
	if len(result) != 0 {
		t.Errorf("Expected empty map, got %v", result)
	}
}

// --- RandomInField ---

func TestRandomInField(t *testing.T) {
	val, err := core.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField failed: %v", err)
	}

	if val == nil {
		t.Fatal("RandomInField returned nil")
	}

	if val.Cmp(core.SNARK_SCALAR_FIELD) >= 0 {
		t.Errorf("Value %s should be less than SNARK_SCALAR_FIELD", val.String())
	}

	if val.Sign() < 0 {
		t.Error("Value should be non-negative")
	}
}

func TestRandomInFieldUniqueness(t *testing.T) {
	val1, _ := core.RandomInField()
	val2, _ := core.RandomInField()

	if val1.Cmp(val2) == 0 {
		t.Error("Two random values should not be equal (extremely unlikely)")
	}
}

// --- GetPublicKey ---

func TestGetPublicKey(t *testing.T) {
	privKey := big.NewInt(12345)

	pub1, err := core.GetPublicKey(privKey)
	if err != nil {
		t.Fatalf("GetPublicKey failed: %v", err)
	}

	pub2, err := core.GetPublicKey(privKey)
	if err != nil {
		t.Fatalf("GetPublicKey failed: %v", err)
	}

	if pub1.Cmp(pub2) != 0 {
		t.Error("GetPublicKey should be deterministic")
	}
}

// --- GetNullifier ---

func TestGetNullifier(t *testing.T) {
	privKey := big.NewInt(111)
	pathIndices := big.NewInt(5)

	nullifier, err := core.GetNullifier(privKey, pathIndices)
	if err != nil {
		t.Fatalf("GetNullifier failed: %v", err)
	}

	if nullifier == nil {
		t.Fatal("Nullifier should not be nil")
	}

	nullifier2, _ := core.GetNullifier(privKey, pathIndices)
	if nullifier.Cmp(nullifier2) != 0 {
		t.Error("GetNullifier should be deterministic")
	}

	nullifier3, _ := core.GetNullifier(privKey, big.NewInt(6))
	if nullifier.Cmp(nullifier3) == 0 {
		t.Error("Different path indices should produce different nullifiers")
	}

	fmt.Printf("Nullifier: %s\n", nullifier.String())
}

// --- GetCommitment ---

func TestGetCommitment(t *testing.T) {
	uniqueId := big.NewInt(42)
	pubKey := big.NewInt(9999)

	commitment, err := core.GetCommitment(uniqueId, pubKey)
	if err != nil {
		t.Fatalf("GetCommitment failed: %v", err)
	}

	if commitment == nil {
		t.Fatal("Commitment should not be nil")
	}

	commitment2, _ := core.GetCommitment(uniqueId, pubKey)
	if commitment.Cmp(commitment2) != 0 {
		t.Error("GetCommitment should be deterministic")
	}

	fmt.Printf("Commitment: %s\n", commitment.String())
}

// --- GetAuctionId ---

func TestGetAuctionId(t *testing.T) {
	commitment := big.NewInt(777)

	auctionId, err := core.GetAuctionId(commitment)
	if err != nil {
		t.Fatalf("GetAuctionId failed: %v", err)
	}

	if auctionId == nil {
		t.Fatal("AuctionId should not be nil")
	}

	auctionId2, _ := core.GetAuctionId(commitment)
	if auctionId.Cmp(auctionId2) != 0 {
		t.Error("GetAuctionId should be deterministic")
	}

	fmt.Printf("AuctionId: %s\n", auctionId.String())
}

// --- BlindedPublicKey ---

func TestBlindedPublicKey(t *testing.T) {
	pubKey := big.NewInt(12345)

	blinded, err := core.BlindedPublicKey(pubKey)
	if err != nil {
		t.Fatalf("BlindedPublicKey failed: %v", err)
	}

	if blinded == nil {
		t.Fatal("BlindedPublicKey should not be nil")
	}

	if blinded.Cmp(pubKey) == 0 {
		t.Error("Blinded key should differ from original")
	}

	fmt.Printf("Blinded public key: %s\n", blinded.String())
}

// --- Token unique IDs ---

func TestErc20UniqueId(t *testing.T) {
	contractAddr := big.NewInt(1000)
	amount := big.NewInt(500)

	uid, err := core.Erc20UniqueId(contractAddr, amount)
	if err != nil {
		t.Fatalf("Erc20UniqueId failed: %v", err)
	}

	if uid == nil {
		t.Fatal("UID should not be nil")
	}

	uid2, _ := core.Erc20UniqueId(contractAddr, amount)
	if uid.Cmp(uid2) != 0 {
		t.Error("Erc20UniqueId should be deterministic")
	}

	uid3, _ := core.Erc20UniqueId(contractAddr, big.NewInt(501))
	if uid.Cmp(uid3) == 0 {
		t.Error("Different amounts should produce different UIDs")
	}

	fmt.Printf("ERC20 UID: %s\n", uid.String())
}

func TestErc721UniqueId(t *testing.T) {
	contractAddr := big.NewInt(2000)
	tokenId := big.NewInt(1)

	uid, err := core.Erc721UniqueId(contractAddr, tokenId)
	if err != nil {
		t.Fatalf("Erc721UniqueId failed: %v", err)
	}

	if uid == nil {
		t.Fatal("UID should not be nil")
	}

	uid2, _ := core.Erc721UniqueId(contractAddr, tokenId)
	if uid.Cmp(uid2) != 0 {
		t.Error("Erc721UniqueId should be deterministic")
	}

	fmt.Printf("ERC721 UID: %s\n", uid.String())
}

func TestErc1155UniqueId(t *testing.T) {
	contractAddr := big.NewInt(3000)
	tokenId := big.NewInt(10)
	amount := big.NewInt(50)

	uid, err := core.Erc1155UniqueId(contractAddr, tokenId, amount)
	if err != nil {
		t.Fatalf("Erc1155UniqueId failed: %v", err)
	}

	if uid == nil {
		t.Fatal("UID should not be nil")
	}

	uid2, _ := core.Erc1155UniqueId(contractAddr, tokenId, amount)
	if uid.Cmp(uid2) != 0 {
		t.Error("Erc1155UniqueId should be deterministic")
	}

	uid3, _ := core.Erc1155UniqueId(contractAddr, tokenId, big.NewInt(51))
	if uid.Cmp(uid3) == 0 {
		t.Error("Different amounts should produce different UIDs")
	}

	fmt.Printf("ERC1155 UID: %s\n", uid.String())
}

// --- Pedersen ---

func TestPedersen(t *testing.T) {
	amount := big.NewInt(100)
	random := big.NewInt(999)

	hash, err := core.Pedersen(amount, random)
	if err != nil {
		t.Fatalf("Pedersen failed: %v", err)
	}

	if hash == nil {
		t.Fatal("Hash should not be nil")
	}

	hash2, _ := core.Pedersen(amount, random)
	if hash.Cmp(hash2) != 0 {
		t.Error("Pedersen should be deterministic")
	}

	hash3, _ := core.Pedersen(amount, big.NewInt(1000))
	if hash.Cmp(hash3) == 0 {
		t.Error("Different random values should produce different hashes")
	}

	fmt.Printf("Pedersen hash: %s\n", hash.String())
}

// --- Keccak256 ---

func TestKeccak256(t *testing.T) {
	result := core.Keccak256([]byte("ZkDvp"))

	if result == nil {
		t.Fatal("Keccak256 result should not be nil")
	}

	if result.Cmp(core.SNARK_SCALAR_FIELD) >= 0 {
		t.Error("Result should be less than SNARK_SCALAR_FIELD")
	}

	zeroVal := core.GetZeroValue()
	if result.Cmp(zeroVal) != 0 {
		t.Error("Keccak256('ZkDvp') should match GetZeroValue()")
	}

	fmt.Printf("Keccak256('ZkDvp'): %s\n", result.String())
}

func TestKeccak256Deterministic(t *testing.T) {
	h1 := core.Keccak256([]byte("hello"))
	h2 := core.Keccak256([]byte("hello"))

	if h1.Cmp(h2) != 0 {
		t.Error("Keccak256 should be deterministic")
	}
}

func TestKeccak256DifferentInputs(t *testing.T) {
	h1 := core.Keccak256([]byte("hello"))
	h2 := core.Keccak256([]byte("world"))

	if h1.Cmp(h2) == 0 {
		t.Error("Different inputs should produce different hashes")
	}
}

// --- WriteToJSON ---

func TestWriteToJSON(t *testing.T) {
	tmpFile := t.TempDir() + "/test_output.json"

	data := map[string]interface{}{
		"key": "value",
		"num": 42,
	}

	err := core.WriteToJSON(tmpFile, data)
	if err != nil {
		t.Fatalf("WriteToJSON failed: %v", err)
	}

	raw, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read back JSON: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("Expected key=value, got %v", result["key"])
	}
}

func TestWriteToJSONIndented(t *testing.T) {
	tmpFile := t.TempDir() + "/test_indent.json"

	data := map[string]string{"hello": "world"}

	err := core.WriteToJSON(tmpFile, data)
	if err != nil {
		t.Fatalf("WriteToJSON failed: %v", err)
	}

	raw, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	content := string(raw)
	if len(content) < 10 {
		t.Error("JSON output seems too short, expected indented output")
	}
}
