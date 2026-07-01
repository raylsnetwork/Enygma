package tests

import (
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/core"
)

// makeViewEncapKey generates a fresh ML-KEM-768 encapsulation key for tests.
func makeViewEncapKey(t *testing.T) []byte {
	t.Helper()
	vkp, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("failed to generate view key pair: %v", err)
	}
	return vkp.EncapsKey
}

// --- Erc20JoinSplitProof Tests ---

func TestErc20JoinSplitProof_Success(t *testing.T) {
	var receivedPath string
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	keysIn := []core.KeyPair{makeKeyPair(10, 20), makeKeyPair(30, 40)}
	recipientSpendPks := []*big.Int{big.NewInt(60), big.NewInt(80)}
	recipientViewKeys := [][]byte{makeViewEncapKey(t), makeViewEncapKey(t)}

	result, err := client.Erc20JoinSplitProof(
		big.NewInt(1),                                    // stMessage
		[]*big.Int{big.NewInt(100), big.NewInt(200)},     // wtValuesIn
		keysIn,
		[]*big.Int{big.NewInt(150), big.NewInt(150)},     // wtSaltsIn
		[]*big.Int{big.NewInt(150), big.NewInt(150)},     // wtValuesOut
		recipientSpendPks,
		recipientViewKeys,
		merkleDepth,
		MerkleProofPair(merkleDepth, 2),
		[]*big.Int{big.NewInt(0), big.NewInt(1)},         // stTreeNumbers
		big.NewInt(0),                                     // wtTokenId
		false,                                             // use10_2
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify endpoint
	if receivedPath != "/proof/joinSplitERC20" {
		t.Errorf("expected endpoint /proof/joinSplitERC20, got %s", receivedPath)
	}

	// Verify payload fields
	if receivedBody["StMessage"] == nil {
		t.Error("expected StMessage in payload")
	}
	if receivedBody["WtTokenId"] == nil {
		t.Error("expected WtTokenId in payload")
	}
	if receivedBody["WtValuesIn"] == nil {
		t.Error("expected WtValuesIn in payload")
	}
	if receivedBody["StNullifiers"] == nil {
		t.Error("expected StNullifiers in payload")
	}
	if receivedBody["WtSaltsIn"] == nil {
		t.Error("expected WtSaltsIn in payload")
	}
	if receivedBody["WtSpendPublicKeysOut"] == nil {
		t.Error("expected WtSpendPublicKeysOut in payload")
	}

	// Statement order: [message, tree[0], root[0], null[0], tree[1], root[1], null[1], commit[0], commit[1]]
	// = 1 + 3*2 + 2 = 9
	if len(result.Statement) != 9 {
		t.Errorf("expected 9 statement elements, got %d", len(result.Statement))
	}

	// First element is the message
	if result.Statement[0].Cmp(big.NewInt(1)) != 0 {
		t.Errorf("expected statement[0]=message=1, got %s", result.Statement[0])
	}

	// Elements 1,2,3 are tree[0], root[0], null[0]
	// Element 1 should be tree number 0
	if result.Statement[1].Cmp(big.NewInt(0)) != 0 {
		t.Errorf("expected statement[1]=tree[0]=0, got %s", result.Statement[1])
	}

	if result.NumberOfInputs != 2 || result.NumberOfOutputs != 2 {
		t.Errorf("expected 2/2, got %d/%d", result.NumberOfInputs, result.NumberOfOutputs)
	}

	// V2: CipherText and EncTxData should be populated
	if len(result.CipherText) != 2 {
		t.Errorf("expected 2 CipherText entries, got %d", len(result.CipherText))
	}
	if len(result.EncTxData) != 2 {
		t.Errorf("expected 2 EncTxData entries, got %d", len(result.EncTxData))
	}
}

func TestErc20JoinSplitProof_ZeroValueInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	// First input has zero value — should use zero path elements
	result, err := client.Erc20JoinSplitProof(
		big.NewInt(1),
		[]*big.Int{big.NewInt(0), big.NewInt(200)},     // first input is zero
		[]core.KeyPair{makeKeyPair(10, 20), makeKeyPair(30, 40)},
		[]*big.Int{big.NewInt(0), big.NewInt(150)},     // wtSaltsIn
		[]*big.Int{big.NewInt(200)},                    // wtValuesOut
		[]*big.Int{big.NewInt(60)},                     // recipientSpendPks
		[][]byte{makeViewEncapKey(t)},                  // recipientViewEncapKeys
		merkleDepth,
		MerkleProofPair(merkleDepth, 2),
		[]*big.Int{big.NewInt(0), big.NewInt(0)},
		big.NewInt(0),
		false,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Zero-value input should have zero merkle root in statement
	// Statement: [message, tree[0], root[0], null[0], tree[1], root[1], null[1], commit[0]]
	// root[0] at index 2 should be 0
	if result.Statement[2].Sign() != 0 {
		t.Errorf("expected zero merkle root for zero-value input, got %s", result.Statement[2])
	}
}

func TestErc20JoinSplitProof_10_2Variant(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	_, err := client.Erc20JoinSplitProof(
		big.NewInt(1),
		[]*big.Int{big.NewInt(100), big.NewInt(200)},
		[]core.KeyPair{makeKeyPair(10, 20), makeKeyPair(30, 40)},
		[]*big.Int{big.NewInt(150), big.NewInt(150)},     // wtSaltsIn
		[]*big.Int{big.NewInt(150), big.NewInt(150)},
		[]*big.Int{big.NewInt(60), big.NewInt(80)},       // recipientSpendPks
		[][]byte{makeViewEncapKey(t), makeViewEncapKey(t)}, // recipientViewEncapKeys
		merkleDepth,
		MerkleProofPair(merkleDepth, 2),
		[]*big.Int{big.NewInt(0), big.NewInt(0)},
		big.NewInt(0),
		true, // use10_2 = true
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/proof/joinSplitERC20_10_2" {
		t.Errorf("expected endpoint /proof/joinSplitERC20_10_2, got %s", receivedPath)
	}
}

// --- Erc20WithdrawProof Tests ---

func TestErc20WithdrawProof_Success(t *testing.T) {
	var receivedPath string
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	// recipient as uint160 (simulates an Ethereum address cast to big.Int)
	recipientAddr := big.NewInt(0xDEADBEEF)
	dummyPk := big.NewInt(0x1234)
	withdrawAmount := big.NewInt(100)
	tokenId := big.NewInt(0)

	keysIn := []core.KeyPair{makeKeyPair(10, 20), makeKeyPair(30, 40)}

	result, err := client.Erc20WithdrawProof(
		big.NewInt(0),                                   // stMessage
		[]*big.Int{withdrawAmount, big.NewInt(0)},       // wtValuesIn
		keysIn,
		[]*big.Int{big.NewInt(77), big.NewInt(0)},       // wtSaltsIn
		withdrawAmount,
		recipientAddr,
		dummyPk,
		merkleDepth,
		MerkleProofPair(merkleDepth, 2),
		[]*big.Int{big.NewInt(0), big.NewInt(0)},        // stTreeNumbers
		tokenId,
		false, // use10_2
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify endpoint
	if receivedPath != "/proof/joinSplitERC20" {
		t.Errorf("expected endpoint /proof/joinSplitERC20, got %s", receivedPath)
	}

	// WtSaltsOut must be ["0", "0"] — fixed zero salt for withdrawal
	saltsOut, ok := receivedBody["WtSaltsOut"].([]interface{})
	if !ok || len(saltsOut) != 2 {
		t.Fatalf("expected WtSaltsOut to be a slice of length 2, got %v", receivedBody["WtSaltsOut"])
	}
	if saltsOut[0] != "0" || saltsOut[1] != "0" {
		t.Errorf("expected WtSaltsOut=[\"0\",\"0\"], got %v", saltsOut)
	}

	// WtSpendPublicKeysOut[0] must equal recipient address as decimal string
	spendPksOut, ok := receivedBody["WtSpendPublicKeysOut"].([]interface{})
	if !ok || len(spendPksOut) != 2 {
		t.Fatalf("expected WtSpendPublicKeysOut to be a slice of length 2, got %v", receivedBody["WtSpendPublicKeysOut"])
	}
	if spendPksOut[0] != recipientAddr.String() {
		t.Errorf("expected WtSpendPublicKeysOut[0]=%s (recipient), got %v", recipientAddr.String(), spendPksOut[0])
	}

	// Statement length: 1 + 3*nIn + nOut = 1 + 3*2 + 2 = 9
	if len(result.Statement) != 9 {
		t.Errorf("expected 9 statement elements, got %d", len(result.Statement))
	}

	// NumberOfInputs=2, NumberOfOutputs=2
	if result.NumberOfInputs != 2 || result.NumberOfOutputs != 2 {
		t.Errorf("expected 2/2, got %d/%d", result.NumberOfInputs, result.NumberOfOutputs)
	}

	// No ciphertexts for withdrawal (no KEM)
	if len(result.CipherText) != 0 {
		t.Errorf("expected empty CipherText for withdrawal, got %d entries", len(result.CipherText))
	}
}

func TestErc20WithdrawProof_10_2Variant(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	_, err := client.Erc20WithdrawProof(
		big.NewInt(0),
		[]*big.Int{big.NewInt(100), big.NewInt(0)},
		[]core.KeyPair{makeKeyPair(10, 20), makeKeyPair(30, 40)},
		[]*big.Int{big.NewInt(77), big.NewInt(0)},
		big.NewInt(100),
		big.NewInt(0xADD),
		big.NewInt(0x1234),
		merkleDepth,
		MerkleProofPair(merkleDepth, 2),
		[]*big.Int{big.NewInt(0), big.NewInt(0)},
		big.NewInt(0),
		true, // use10_2
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/proof/joinSplitERC20_10_2" {
		t.Errorf("expected endpoint /proof/joinSplitERC20_10_2, got %s", receivedPath)
	}
}

// --- Erc721OwnershipProof Tests ---

func TestErc721OwnershipProof_Success(t *testing.T) {
	var receivedPath string
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	result, err := client.Erc721OwnershipProof(
		big.NewInt(1),                 // stMessage
		big.NewInt(42),                // wtValue (tokenId)
		makeKeyPair(10, 20),           // keyIn
		big.NewInt(77),                // wtSaltIn
		makeKeyPair(50, 60),           // keyOut
		nil,                           // recipientViewEncapKey (nil → random salt fallback)
		merkleDepth,
		makeMerkleProof(merkleDepth),
		big.NewInt(0),                 // stTreeNumber
		big.NewInt(0xABC),             // wtErc721ContractAddress
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify endpoint
	if receivedPath != "/proof/ownershipERC721" {
		t.Errorf("expected endpoint /proof/ownershipERC721, got %s", receivedPath)
	}

	// Verify raw tokenId is sent as WtValues (ERC721 circuit uses raw tokenIds)
	wtValues, ok := receivedBody["WtValues"].([]interface{})
	if !ok || len(wtValues) != 1 {
		t.Fatalf("expected WtValues to be a slice of length 1, got %v", receivedBody["WtValues"])
	}
	if wtValues[0] != "42" {
		t.Errorf("expected WtValues[0]=42 (raw tokenId), got %v", wtValues[0])
	}

	// Statement: [message, treeNumber, merkleRoot, nullifier, commitmentOut]
	if len(result.Statement) != 5 {
		t.Errorf("expected 5 statement elements, got %d", len(result.Statement))
	}
	if result.Statement[0].Cmp(big.NewInt(1)) != 0 {
		t.Errorf("expected statement[0]=message=1, got %s", result.Statement[0])
	}
	if result.Statement[1].Cmp(big.NewInt(0)) != 0 {
		t.Errorf("expected statement[1]=treeNumber=0, got %s", result.Statement[1])
	}
	if result.NumberOfInputs != 1 || result.NumberOfOutputs != 1 {
		t.Errorf("expected 1/1, got %d/%d", result.NumberOfInputs, result.NumberOfOutputs)
	}
}

// --- Erc1155FungibleJoinSplitProof Tests ---

func TestErc1155FungibleJoinSplitProof_Success(t *testing.T) {
	var receivedPath string
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	keysIn := []core.KeyPair{makeKeyPair(10, 20), makeKeyPair(30, 40)}
	keysOut := []core.KeyPair{makeKeyPair(50, 60), makeKeyPair(70, 80)}
	assetGroupProof := makeMerkleProof(merkleDepth)

	result, err := client.Erc1155FungibleJoinSplitProof(
		big.NewInt(1),                                    // stMessage
		[]*big.Int{big.NewInt(100), big.NewInt(200)},     // wtValuesIn
		keysIn,
		[]*big.Int{big.NewInt(50), big.NewInt(50)},       // wtSaltsIn
		[]*big.Int{big.NewInt(150), big.NewInt(150)},     // wtValuesOut
		keysOut,
		[][]byte{nil, nil},                                // recipientViewEncapKeys (nil → random salt fallback)
		merkleDepth,
		MerkleProofPair(merkleDepth, 2),
		[]*big.Int{big.NewInt(0), big.NewInt(1)},         // stTreeNumbers
		big.NewInt(0xDEF),                                 // wtErc1155ContractAddress
		big.NewInt(42),                                    // wtErc1155TokenId
		big.NewInt(0),                                     // stAssetGroupTreeNumber
		assetGroupProof,                                   // assetGroupMerkleProof
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify endpoint (note: server has typo "erc155" not "erc1155")
	if receivedPath != "/proof/erc155Fungible" {
		t.Errorf("expected endpoint /proof/erc155Fungible, got %s", receivedPath)
	}

	// Verify asset group fields in payload
	if receivedBody["StAssetGroupTreeNumber"] == nil {
		t.Error("expected StAssetGroupTreeNumber in payload")
	}
	if receivedBody["StAssetGroupMerkleRoot"] == nil {
		t.Error("expected StAssetGroupMerkleRoot in payload")
	}
	if receivedBody["WtAssetGroupPathElements"] == nil {
		t.Error("expected WtAssetGroupPathElements in payload")
	}
	if receivedBody["WtErc1155ContractAddress"] == nil {
		t.Error("expected WtErc1155ContractAddress in payload")
	}
	if receivedBody["WtErc1155TokenId"] == nil {
		t.Error("expected WtErc1155TokenId in payload")
	}

	// Statement (interleaved, no asset group):
	// [message, tree[0], root[0], null[0], tree[1], root[1], null[1], commit[0], commit[1]]
	// = 1 + 3*2 + 2 = 9
	if len(result.Statement) != 9 {
		t.Errorf("expected 9 statement elements, got %d", len(result.Statement))
	}

	if result.NumberOfInputs != 2 || result.NumberOfOutputs != 2 {
		t.Errorf("expected 2/2, got %d/%d", result.NumberOfInputs, result.NumberOfOutputs)
	}
}

// --- Erc1155NonFungibleOwnershipProof Tests ---

func TestErc1155NonFungibleOwnershipProof_Success(t *testing.T) {
	var receivedPath string
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8
	assetGroupProof := makeMerkleProof(merkleDepth)

	result, err := client.Erc1155NonFungibleOwnershipProof(
		big.NewInt(1),                 // stMessage
		big.NewInt(1),                 // wtValue (amount, typically 1 for NFT)
		makeKeyPair(10, 20),           // keyIn
		big.NewInt(77),                // wtSaltIn
		makeKeyPair(50, 60),           // keyOut
		nil,                           // recipientViewEncapKey (nil → random salt fallback)
		merkleDepth,
		makeMerkleProof(merkleDepth),
		big.NewInt(0),                 // stTreeNumber
		big.NewInt(0xDEF),             // wtErc1155ContractAddress
		big.NewInt(42),                // wtErc1155TokenId
		big.NewInt(0),                 // stAssetGroupTreeNumber
		assetGroupProof,               // assetGroupMerkleProof
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify endpoint
	if receivedPath != "/proof/erc1155NonFungible" {
		t.Errorf("expected endpoint /proof/erc1155NonFungible, got %s", receivedPath)
	}

	// Statement includes asset group: [message, treeNumber, merkleRoot, nullifier,
	//   commitmentOut, assetGroupTreeNumber, assetGroupMerkleRoot]
	if len(result.Statement) != 7 {
		t.Errorf("expected 7 statement elements, got %d", len(result.Statement))
	}

	// Verify asset group fields are in statement
	// assetGroupTreeNumber at index 5
	if result.Statement[5].Cmp(big.NewInt(0)) != 0 {
		t.Errorf("expected statement[5]=assetGroupTreeNumber=0, got %s", result.Statement[5])
	}
	// assetGroupMerkleRoot at index 6 should be the proof root (999 from makeMerkleProof)
	if result.Statement[6].Cmp(big.NewInt(999)) != 0 {
		t.Errorf("expected statement[6]=assetGroupMerkleRoot=999, got %s", result.Statement[6])
	}

	if result.NumberOfInputs != 1 || result.NumberOfOutputs != 1 {
		t.Errorf("expected 1/1, got %d/%d", result.NumberOfInputs, result.NumberOfOutputs)
	}
}

// --- Bug Regression Tests ---

// Bug 1 regression: AuctionPrivateOpeningProof2 should send StVaultId, not StAuctionId
func TestAuctionPrivateOpeningProof2_SendsStVaultId(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	_, err := client.AuctionPrivateOpeningProof2(
		big.NewInt(42),
		big.NewInt(12345),
		big.NewInt(100),
		big.NewInt(999),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use StVaultId, not StAuctionId
	if _, hasOld := receivedBody["StAuctionId"]; hasOld {
		t.Error("payload should not contain StAuctionId (bug: should be StVaultId)")
	}
	if receivedBody["StVaultId"] != "42" {
		t.Errorf("expected StVaultId=42, got %v", receivedBody["StVaultId"])
	}
}

// Bug 4 regression: GnarkProver dispatcher should route PrivateMint
func TestGnarkProverDispatcher_PrivateMint(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte(`{"proof":["0x1"],"publicSignal":["0xa"]}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	inputs := map[string]interface{}{
		"commitment":      "0xcommit",
		"contractAddress": "0xcontract",
		"tokenId":         "1",
		"salt":            "0",
		"amount":          "100",
		"publicKey":       "0xpk",
		"cipherText":      "0xcipher",
	}

	resp, privateMint, err := client.GnarkProver(inputs, "path/to/PrivateMint.zkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/proof/privateMint" {
		t.Errorf("expected path /proof/privateMint, got %s", receivedPath)
	}

	// PrivateMint returns via the second return value
	if resp != nil {
		t.Error("expected nil ProofResponse for PrivateMint")
	}
	if privateMint == nil {
		t.Fatal("expected non-nil PrivateMintProofResponse")
	}
	if len(privateMint.Proof) != 1 || privateMint.Proof[0] != "0x1" {
		t.Errorf("unexpected proof: %v", privateMint.Proof)
	}
}

// Bug 4 regression: GnarkProver dispatcher should route non-auditor AuctionInit
func TestGnarkProverDispatcher_AuctionInit(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	inputs := map[string]interface{}{
		"st_beacon":                  "1",
		"st_vaultId":                 "0",
		"st_auctionId":              "42",
		"st_treeNumber":             "0",
		"st_merkleRoot":             "root0",
		"st_nullifier":              "null0",
		"st_assetGroup_merkleRoot":  "agRoot",
		"wt_commitment":             "cmt0",
		"wt_pathElements":           []interface{}{"elem0"},
		"wt_pathIndices":            "0",
		"wt_privateKey":             "pk0",
		"wt_idParams":               []interface{}{"id0"},
		"wt_contractAddress":        "0xABC",
		"wt_assetGroup_pathElements": []interface{}{"agEl0"},
		"wt_assetGroup_pathIndices":  "0",
	}

	// Non-auditor variant: path contains "AuctionInit" but NOT "AuctionInit_Auditor"
	resp, _, err := client.GnarkProver(inputs, "path/to/AuctionInit.zkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/proof/auctionInit" {
		t.Errorf("expected path /proof/auctionInit, got %s", receivedPath)
	}
	if resp.Status != 200 {
		t.Errorf("expected status 200, got %d", resp.Status)
	}
}

// Bug 4 regression: GnarkProver dispatcher should route non-auditor AuctionBid
func TestGnarkProverDispatcher_AuctionBid(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	elements := make([]interface{}, 16)
	for i := range elements {
		elements[i] = "el"
	}
	inputs := map[string]interface{}{
		"st_auctionId":               "1",
		"st_blindedBid":              "bid123",
		"st_vaultId":                 "0",
		"st_treeNumbers":             []interface{}{"0", "1"},
		"st_merkleRoots":             []interface{}{"root0", "root1"},
		"st_nullifiers":              []interface{}{"null0", "null1"},
		"st_commitmentsOut":          []interface{}{"cmt0", "cmt1"},
		"st_assetGroup_merkleRoot":   "agRoot",
		"wt_bidAmount":               "100",
		"wt_bidRandom":               "999",
		"wt_privateKeysIn":           []interface{}{"pk0", "pk1"},
		"wt_pathElements":            elements,
		"wt_pathIndices":             []interface{}{"0", "1"},
		"wt_contractAddress":         "0xABC",
		"wt_publicKeysOut":           []interface{}{"pubk0", "pubk1"},
		"wt_valuesOut":               []interface{}{"50", "50"},
		"wt_assetGroup_pathElements": []interface{}{"agEl0", "agEl1"},
		"wt_assetGroup_pathIndices":  "0",
		"wt_idParamsIn":              []interface{}{[]interface{}{"p0"}, []interface{}{"p1"}},
		"wt_idParamsOut":             []interface{}{[]interface{}{"q0"}, []interface{}{"q1"}},
	}

	// Non-auditor variant: path contains "AuctionBid" but NOT "AuctionBid_Auditor"
	resp, _, err := client.GnarkProver(inputs, "path/to/AuctionBid.zkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/proof/auctionBid" {
		t.Errorf("expected path /proof/auctionBid, got %s", receivedPath)
	}
	if resp.Status != 200 {
		t.Errorf("expected status 200, got %d", resp.Status)
	}
}

// TestErc20PrivateMintProof_CommitmentFormat verifies that Erc20PrivateMintProof
// sends the correct V2 commitment and cipherText to the server, and that the
// commitment matches what the JoinSplit circuit expects on the input side.
func TestErc20PrivateMintProof_CommitmentFormat(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"proof":["0x1"],"publicSignal":["0xa"]}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)

	pkSpend := big.NewInt(0xABCD)
	salt := big.NewInt(0x1234)
	amount := big.NewInt(5)
	tokenId := big.NewInt(10)
	contractAddress := big.NewInt(0xDEAD)

	result, err := client.Erc20PrivateMintProof(pkSpend, salt, amount, tokenId, contractAddress)
	if err != nil {
		t.Fatalf("Erc20PrivateMintProof: %v", err)
	}

	// Commitment must match V2 format: Poseidon(pkSpend, salt, amount, tokenId)
	expected, err := core.Erc20CommitmentV2(pkSpend, salt, amount, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2: %v", err)
	}
	if result.Commitment.Cmp(expected) != 0 {
		t.Errorf("commitment mismatch: got %s, want %s", result.Commitment, expected)
	}

	// Salt is preserved for later spending
	if result.Salt.Cmp(salt) != 0 {
		t.Errorf("salt mismatch: got %s, want %s", result.Salt, salt)
	}

	// Payload must include the right fields
	if receivedBody["commitment"] != expected.String() {
		t.Errorf("payload commitment: got %v, want %s", receivedBody["commitment"], expected)
	}
	if receivedBody["publicKey"] != pkSpend.String() {
		t.Errorf("payload publicKey: got %v, want %s", receivedBody["publicKey"], pkSpend)
	}
	if receivedBody["salt"] != salt.String() {
		t.Errorf("payload salt: got %v, want %s", receivedBody["salt"], salt)
	}
	if receivedBody["amount"] != amount.String() {
		t.Errorf("payload amount: got %v, want %s", receivedBody["amount"], amount)
	}
	if receivedBody["tokenId"] != tokenId.String() {
		t.Errorf("payload tokenId: got %v, want %s", receivedBody["tokenId"], tokenId)
	}
}

// TestErc20PrivateMintProof_CommitmentUsableAsJoinSplitInput verifies that the
// commitment produced by PrivateMint is accepted by the JoinSplit proof path —
// i.e. the same (pkSpend, salt, amount, tokenId) tuple satisfies Erc20CommitmentV2.
func TestErc20PrivateMintProof_CommitmentUsableAsJoinSplitInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"proof":["0x1"],"publicSignal":["0xa"]}`))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)

	spendKP, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}
	salt, err := core.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField: %v", err)
	}
	amount := big.NewInt(5)
	tokenId := big.NewInt(10)
	contractAddress := big.NewInt(0xDEAD)

	result, err := client.Erc20PrivateMintProof(spendKP.PublicKey, salt, amount, tokenId, contractAddress)
	if err != nil {
		t.Fatalf("Erc20PrivateMintProof: %v", err)
	}

	// The JoinSplit circuit will verify:
	//   Erc20CommitmentV2(pkSpend, saltIn, amountIn, tokenId) == commitment in Merkle tree
	// Confirm that recomputing with the same inputs reproduces the exact commitment.
	recomputed, err := core.Erc20CommitmentV2(spendKP.PublicKey, result.Salt, amount, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 recompute: %v", err)
	}
	if recomputed.Cmp(result.Commitment) != 0 {
		t.Errorf("recomputed commitment mismatch: got %s, want %s", recomputed, result.Commitment)
	}
}

