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

// --- Helper: create a mock gnark server ---

func newAuctionMockServer(t *testing.T, expectedPath string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
}

func newPayloadCapturingServer(t *testing.T, expectedPath string) (*httptest.Server, *map[string]interface{}) {
	t.Helper()
	var captured map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.WriteHeader(200)
		w.Write([]byte(`{"status":200,"message":"ok"}`))
	}))
	return server, &captured
}

// --- Helper: build minimal MerkleProof ---

func makeMerkleProof(depth int) *core.MerkleProof {
	elements := make([]*big.Int, depth)
	for i := range elements {
		elements[i] = big.NewInt(int64(i + 100))
	}
	return &core.MerkleProof{
		Element:  big.NewInt(42),
		Elements: elements,
		Indices:  big.NewInt(3),
		Root:     big.NewInt(999),
	}
}

func makeKeyPair(privVal, pubVal int64) core.KeyPair {
	return core.KeyPair{
		PrivateKey: big.NewInt(privVal),
		PublicKey:  big.NewInt(pubVal),
	}
}

// --- AuctionInitProof Tests ---

func TestAuctionInitProof_Success(t *testing.T) {
	server := newAuctionMockServer(t, "/proof/auctionInit")
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	result, err := client.AuctionInitProof(
		big.NewInt(1),           // stBeacon
		big.NewInt(42),          // tokenId
		big.NewInt(0xABC),       // wtContractAddress
		makeKeyPair(10, 20),     // keyIn
		merkleDepth,             // merkleDepth
		makeMerkleProof(merkleDepth), // merkleProof
		big.NewInt(999),         // merkleRoot
		big.NewInt(0),           // stTreeNumber
		0,                       // vaultId (ERC20)
		big.NewInt(555),         // assetGroupMerkleRoot
		makeMerkleProof(merkleDepth), // assetGroupMerkleProof
		[]*big.Int{big.NewInt(100)},  // wtIdParams
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NumberOfInputs != 1 || result.NumberOfOutputs != 1 {
		t.Errorf("expected 1 input, 1 output, got %d, %d", result.NumberOfInputs, result.NumberOfOutputs)
	}
	// Statement: [beacon, vaultId, auctionId, treeNumber, merkleRoot, nullifier, assetGroupMerkleRoot]
	if len(result.Statement) != 7 {
		t.Errorf("expected 7 statement elements, got %d", len(result.Statement))
	}
	// First element should be beacon=1
	if result.Statement[0].Cmp(big.NewInt(1)) != 0 {
		t.Errorf("expected beacon=1, got %s", result.Statement[0])
	}
	// Second element should be vaultId=0
	if result.Statement[1].Cmp(big.NewInt(0)) != 0 {
		t.Errorf("expected vaultId=0, got %s", result.Statement[1])
	}
}

func TestAuctionInitProof_ZeroAssetGroupRoot(t *testing.T) {
	server := newAuctionMockServer(t, "/proof/auctionInit")
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	result, err := client.AuctionInitProof(
		big.NewInt(1),
		big.NewInt(42),
		big.NewInt(0xABC),
		makeKeyPair(10, 20),
		merkleDepth,
		makeMerkleProof(merkleDepth),
		big.NewInt(999),
		big.NewInt(0),
		0,
		big.NewInt(0), // zero asset group root → zeros for path elements
		nil,           // nil assetGroupMerkleProof
		[]*big.Int{big.NewInt(100)},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Last statement element should be assetGroupMerkleRoot=0
	last := result.Statement[len(result.Statement)-1]
	if last.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("expected assetGroupMerkleRoot=0, got %s", last)
	}
}

func TestAuctionInitProof_VaultIdErc721(t *testing.T) {
	server := newAuctionMockServer(t, "/proof/auctionInit")
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	_, err := client.AuctionInitProof(
		big.NewInt(1), big.NewInt(42), big.NewInt(0xABC),
		makeKeyPair(10, 20), merkleDepth, makeMerkleProof(merkleDepth),
		big.NewInt(999), big.NewInt(0),
		1, // vaultId=1 (ERC721)
		big.NewInt(555), makeMerkleProof(merkleDepth),
		[]*big.Int{big.NewInt(100)},
	)
	if err != nil {
		t.Fatalf("unexpected error for ERC721 vaultId: %v", err)
	}
}

func TestAuctionInitProof_VaultIdErc1155(t *testing.T) {
	server := newAuctionMockServer(t, "/proof/auctionInit")
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	_, err := client.AuctionInitProof(
		big.NewInt(1), big.NewInt(42), big.NewInt(0xABC),
		makeKeyPair(10, 20), merkleDepth, makeMerkleProof(merkleDepth),
		big.NewInt(999), big.NewInt(0),
		2, // vaultId=2 (ERC1155)
		big.NewInt(555), makeMerkleProof(merkleDepth),
		[]*big.Int{big.NewInt(100), big.NewInt(200)}, // idParams: [amount, tokenId]
	)
	if err != nil {
		t.Fatalf("unexpected error for ERC1155 vaultId: %v", err)
	}
}

func TestAuctionInitProof_InvalidVaultId(t *testing.T) {
	server := newAuctionMockServer(t, "/proof/auctionInit")
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	_, err := client.AuctionInitProof(
		big.NewInt(1), big.NewInt(42), big.NewInt(0xABC),
		makeKeyPair(10, 20), merkleDepth, makeMerkleProof(merkleDepth),
		big.NewInt(999), big.NewInt(0),
		99, // invalid vaultId
		big.NewInt(555), makeMerkleProof(merkleDepth),
		[]*big.Int{big.NewInt(100)},
	)
	if err == nil {
		t.Fatal("expected error for invalid vaultId")
	}
}

func TestAuctionInitProof_PayloadFields(t *testing.T) {
	server, captured := newPayloadCapturingServer(t, "/proof/auctionInit")
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	client.AuctionInitProof(
		big.NewInt(1), big.NewInt(42), big.NewInt(0xABC),
		makeKeyPair(10, 20), merkleDepth, makeMerkleProof(merkleDepth),
		big.NewInt(999), big.NewInt(0), 0,
		big.NewInt(555), makeMerkleProof(merkleDepth),
		[]*big.Int{big.NewInt(100)},
	)

	payload := *captured
	if payload["StBeacon"] != "1" {
		t.Errorf("expected StBeacon=1, got %v", payload["StBeacon"])
	}
	if payload["StVaultId"] != "0" {
		t.Errorf("expected StVaultId=0, got %v", payload["StVaultId"])
	}
	if payload["WtContractAddress"] != "2748" {
		t.Errorf("expected WtContractAddress=2748 (0xABC), got %v", payload["WtContractAddress"])
	}
}

// --- AuctionBidProof Tests ---

func TestAuctionBidProof_Success(t *testing.T) {
	server := newAuctionMockServer(t, "/proof/auctionBid")
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	keysIn := []core.KeyPair{makeKeyPair(10, 20), makeKeyPair(30, 40)}
	keysOut := []core.KeyPair{makeKeyPair(50, 60), makeKeyPair(70, 80)}

	result, err := client.AuctionBidProof(
		big.NewInt(1),                                      // stAuctionId
		big.NewInt(100),                                    // wtBidAmount
		big.NewInt(999),                                    // wtBidRandom
		big.NewInt(0xABC),                                  // assetAddress
		[]*big.Int{big.NewInt(50), big.NewInt(50)},         // wtValuesIn
		keysIn,
		[]*big.Int{big.NewInt(40), big.NewInt(60)},         // wtValuesOut
		keysOut,
		merkleDepth,
		MerkleProofPair(merkleDepth, 2),                    // merkleProofs
		[]*big.Int{big.NewInt(999), big.NewInt(999)},       // stMerkleRoots
		[]*big.Int{big.NewInt(0), big.NewInt(0)},           // stTreeNumbers
		0,                                                   // stVaultId
		[][]*big.Int{{big.NewInt(50)}, {big.NewInt(50)}},   // wtIdParamsIn
		[][]*big.Int{{big.NewInt(40)}, {big.NewInt(60)}},   // wtIdParamsOut
		big.NewInt(555),                                     // stAssetGroupMerkleRoot
		makeMerkleProof(merkleDepth),                        // assetGroupMerkleProof
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NumberOfInputs != 2 || result.NumberOfOutputs != 2 {
		t.Errorf("expected 2 inputs/2 outputs, got %d/%d", result.NumberOfInputs, result.NumberOfOutputs)
	}
	// Statement: [auctionId, blindedBid, vaultId, ...treeNumbers, ...merkleRoots, ...nullifiers, ...commitmentsOut, assetGroupMerkleRoot]
	// = 3 + 2 + 2 + 2 + 2 + 1 = 12
	if len(result.Statement) != 12 {
		t.Errorf("expected 12 statement elements, got %d", len(result.Statement))
	}
}

func TestAuctionBidProof_ZeroValueInput(t *testing.T) {
	server := newAuctionMockServer(t, "/proof/auctionBid")
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	// One zero-value input should use zero-filled path elements
	result, err := client.AuctionBidProof(
		big.NewInt(1), big.NewInt(100), big.NewInt(999), big.NewInt(0xABC),
		[]*big.Int{big.NewInt(0), big.NewInt(50)}, // first input is zero
		[]core.KeyPair{makeKeyPair(10, 20), makeKeyPair(30, 40)},
		[]*big.Int{big.NewInt(50)},
		[]core.KeyPair{makeKeyPair(50, 60)},
		merkleDepth,
		MerkleProofPair(merkleDepth, 2),
		[]*big.Int{big.NewInt(999), big.NewInt(999)},
		[]*big.Int{big.NewInt(0), big.NewInt(0)},
		0,
		[][]*big.Int{{big.NewInt(0)}, {big.NewInt(50)}},
		[][]*big.Int{{big.NewInt(50)}},
		big.NewInt(555),
		makeMerkleProof(merkleDepth),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// --- AuctionPrivateOpeningProof2 Tests ---

func TestAuctionPrivateOpeningProof2_Success(t *testing.T) {
	server, captured := newPayloadCapturingServer(t, "/proof/auctionPrivateOpening")
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	result, err := client.AuctionPrivateOpeningProof2(
		big.NewInt(42),    // stAuctionId
		big.NewInt(12345), // stBlindedBid
		big.NewInt(100),   // wtBidAmount
		big.NewInt(999),   // wtBidRandom
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := *captured
	if payload["StVaultId"] != "42" {
		t.Errorf("expected StVaultId=42, got %v", payload["StVaultId"])
	}
	if payload["StBlindedBid"] != "12345" {
		t.Errorf("expected StBlindedBid=12345, got %v", payload["StBlindedBid"])
	}

	if len(result.Statement) != 2 {
		t.Errorf("expected 2 statement elements, got %d", len(result.Statement))
	}
	if result.Statement[0].Cmp(big.NewInt(42)) != 0 {
		t.Errorf("expected auctionId=42 in statement, got %s", result.Statement[0])
	}
	if result.NumberOfInputs != 1 || result.NumberOfOutputs != 1 {
		t.Errorf("expected 1/1, got %d/%d", result.NumberOfInputs, result.NumberOfOutputs)
	}
}

// --- AuctionNotWinningBidProof2 Tests ---

func TestAuctionNotWinningBidProof2_Success(t *testing.T) {
	server := newAuctionMockServer(t, "/proof/auctionNotWinning")
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	result, err := client.AuctionNotWinningBidProof2(
		big.NewInt(1),   // stAuctionId
		big.NewInt(100), // stBidBlockNumber
		big.NewInt(101), // stWinningBidBlockNumber
		big.NewInt(200), // wtBidAmount
		big.NewInt(999), // wtBidRandom
		big.NewInt(250), // wtWinningBidAmount
		big.NewInt(888), // wtWinningBidRandom
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Statement: [auctionId, blindedBidDifference, bidBlockNumber, winningBidBlockNumber]
	if len(result.Statement) != 4 {
		t.Errorf("expected 4 statement elements, got %d", len(result.Statement))
	}
	if result.Statement[0].Cmp(big.NewInt(1)) != 0 {
		t.Errorf("expected auctionId=1, got %s", result.Statement[0])
	}
	// blindedBidDifference should be non-negative (computed mod SNARK_SCALAR_FIELD)
	if result.Statement[1].Sign() < 0 {
		t.Errorf("blindedBidDifference should be non-negative, got %s", result.Statement[1])
	}
}

func TestAuctionNotWinningBidProof2_BlindedBidDifference(t *testing.T) {
	server, captured := newPayloadCapturingServer(t, "/proof/auctionNotWinning")
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	client.AuctionNotWinningBidProof2(
		big.NewInt(1), big.NewInt(100), big.NewInt(101),
		big.NewInt(200), big.NewInt(999),
		big.NewInt(250), big.NewInt(888),
	)

	payload := *captured
	// StBlindedBidDifference should be present and non-empty
	diff, ok := payload["StBlindedBidDifference"].(string)
	if !ok || diff == "" {
		t.Errorf("expected StBlindedBidDifference to be a non-empty string, got %v", payload["StBlindedBidDifference"])
	}
}

// --- Error Handling Tests ---

func TestAuctionInitProof_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := core.NewGnarkClient(server.URL)
	merkleDepth := 8

	_, err := client.AuctionInitProof(
		big.NewInt(1), big.NewInt(42), big.NewInt(0xABC),
		makeKeyPair(10, 20), merkleDepth, makeMerkleProof(merkleDepth),
		big.NewInt(999), big.NewInt(0), 0,
		big.NewInt(555), makeMerkleProof(merkleDepth),
		[]*big.Int{big.NewInt(100)},
	)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// --- Helper to create multiple merkle proofs ---

func MerkleProofPair(depth, count int) []*core.MerkleProof {
	proofs := make([]*core.MerkleProof, count)
	for i := range proofs {
		proofs[i] = makeMerkleProof(depth)
	}
	return proofs
}
