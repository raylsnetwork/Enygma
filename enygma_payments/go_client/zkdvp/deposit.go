package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"

	interfacezkdvp "enygma/interfacezkdvp"
	utils "enygma/utils"

	"github.com/iden3/go-iden3-crypto/poseidon"
)

/*
Script for Enygma + ZkDVP integration.

Deposit: Enygma balance update via relayer; ZkDVP withdraw proof attached.
The client generates commitments and proofs locally then hands them to the
relayer, which holds the signing key and submits on-chain.
*/

// ── Relayer types ─────────────────────────────────────────────────────────────

// zkdvpTxJSON is the relayer-compatible encoding of a ZkDVP join-split
// transaction embedded in a deposit. Matches the relayer's ZkDvpTxJSON.
type zkdvpTxJSON struct {
	ProofA     [2]string    `json:"proofA"`
	ProofB     [2][2]string `json:"proofB"`
	ProofC     [2]string    `json:"proofC"`
	Statement  []string     `json:"statement"`
	NumInputs  int64        `json:"numInputs"`
	NumOutputs int64        `json:"numOutputs"`
}

// relayDepositRequest matches the relayer's POST /relay/deposit body.
type relayDepositRequest struct {
	Proof        [8]string    `json:"proof"`
	PublicSignal [2]string    `json:"publicSignal"`
	Commitments  [][]string   `json:"commitments"`
	KIndex       []int64      `json:"kIndex"`
	ZkDvpTx      *zkdvpTxJSON `json:"zkdvpTx"`
}

// relayResponse is the success body returned by all relay endpoints.
type relayResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
}

// ── ZkDVP helpers ─────────────────────────────────────────────────────────────

func GenerateProofAtZkDvP(pk *big.Int, amount *big.Int, withdrawKey interfacezkdvp.Key, enygmaAddress *big.Int, merkleTree interfacezkdvp.MerkleTree) (interfacezkdvp.ZkDvpProofToWithdrawResponse, error) {
	jsonInfo := interfacezkdvp.ZkDvpProofToWithdrawRequest{
		PK:            pk.String(),
		Amount:        amount.String(),
		WithdrawKey:   withdrawKey,
		EnygmaAddress: enygmaAddress.String(),
		MerkleTree:    merkleTree,
	}
	response, err := utils.PostJSON(utils.ZkdvpGetProof, jsonInfo)
	if err != nil {
		return interfacezkdvp.ZkDvpProofToWithdrawResponse{}, fmt.Errorf("failed to generate ZkDvP proof: %v", err)
	}

	var result interfacezkdvp.ZkDvpProofToWithdrawResponse
	if err = json.Unmarshal(response, &result); err != nil {
		return interfacezkdvp.ZkDvpProofToWithdrawResponse{}, fmt.Errorf("failed to parse ZkDvP proof JSON: %v", err)
	}
	return result, nil
}

func getMerkleTreeZkDvp() (interfacezkdvp.MerkleTreeResponse, error) {
	response, err := utils.GetJSON(utils.ZkdvpGetMerkleTree)
	if err != nil {
		return interfacezkdvp.MerkleTreeResponse{}, fmt.Errorf("failed to retrieve Merkle tree: %v", err)
	}

	var result interfacezkdvp.MerkleTreeResponse
	if err = json.Unmarshal(response, &result); err != nil {
		return interfacezkdvp.MerkleTreeResponse{}, fmt.Errorf("failed to parse Merkle tree JSON: %v", err)
	}
	return result, nil
}

func InsertLeveMerkleTreeZkDvp(commitment *big.Int) (interfacezkdvp.InsertLeafResponse, error) {
	jsonInfo := interfacezkdvp.InsertLeaf{Commitment: commitment.String()}
	response, err := utils.PostJSON(utils.ZkdvpAddLeaveURL, jsonInfo)
	if err != nil {
		return interfacezkdvp.InsertLeafResponse{}, fmt.Errorf("failed to insert leaf: %v", err)
	}

	var result interfacezkdvp.InsertLeafResponse
	if err = json.Unmarshal(response, &result); err != nil {
		return interfacezkdvp.InsertLeafResponse{}, fmt.Errorf("failed to parse insert leaf JSON: %v", err)
	}
	return result, nil
}

func ProccessGenerateProofAtZkDvp(amount *big.Int, amountToDeposit *big.Int, pk *big.Int, secretDeposit string, enygmaAddress *big.Int) (interfacezkdvp.ZkDvpProofToWithdrawResponse, error) {
	withdrawSecretKey, _ := new(big.Int).SetString(secretDeposit, 10)
	withdrawPublicKey := utils.GetPkZkDvP(withdrawSecretKey)

	withdrawKey := interfacezkdvp.Key{
		PublicKey:  withdrawPublicKey.String(),
		PrivateKey: withdrawSecretKey.String(),
	}

	merkleTree, _ := getMerkleTreeZkDvp()
	tx, err := GenerateProofAtZkDvP(pk, amountToDeposit, withdrawKey, enygmaAddress, merkleTree.MerkleTree)
	return tx, err
}

// buildZkDvpTxJSON converts a ZkDVP proof response into the relayer JSON format.
func buildZkDvpTxJSON(tx interfacezkdvp.ZkDvpProofToWithdrawResponse) *zkdvpTxJSON {
	p := tx.ProofWithdraw.Proof
	return &zkdvpTxJSON{
		ProofA:     [2]string{p.A[0], p.A[1]},
		ProofB:     [2][2]string{{p.B[0][0], p.B[0][1]}, {p.B[1][0], p.B[1][1]}},
		ProofC:     [2]string{p.C[0], p.C[1]},
		Statement:  tx.ProofWithdraw.Statement,
		NumInputs:  int64(tx.ProofWithdraw.NumberOfInputs),
		NumOutputs: int64(tx.ProofWithdraw.NumberOfOutputs),
	}
}

// ── Relayer helper ────────────────────────────────────────────────────────────

// postRelayer sends an authenticated POST request to the relayer and returns
// the response body on HTTP 200. Any non-200 response is returned as an error.
func postRelayer(path string, payload interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, utils.RelayerURL+path, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+utils.RelayerAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contact relayer: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("relayer returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// ── Deposit ───────────────────────────────────────────────────────────────────

func deposit(senderId int, amount *big.Int, amountToDeposit *big.Int, kIndex []int, secrets []*big.Int, depositSecret string, block_number string) error {
	blockNumber, _ := new(big.Int).SetString(block_number, 10)

	// Generate Pedersen commitments locally.
	commitments, txRandom := utils.GenerateDepositCommitments(amountToDeposit, senderId, blockNumber, kIndex, secrets)

	addressBigInt, _ := new(big.Int).SetString(utils.Address[2:], 16)
	depositSecretBigInt, _ := new(big.Int).SetString(depositSecret, 10)
	pk := utils.GetPkZkDvP(depositSecretBigInt)

	// Compute the public hash committed inside the proof.
	inputs := []*big.Int{addressBigInt, amountToDeposit}
	poseidonHash, _ := poseidon.Hash(inputs)
	inputsWithPubKey := []*big.Int{poseidonHash, pk}
	hash, _ := poseidon.Hash(inputsWithPubKey)

	// Encode commitments as [[C1x, C1y], ...] for JSON.
	var commFinal [][]string
	for _, c := range commitments {
		commFinal = append(commFinal, []string{c.C1.String(), c.C2.String()})
	}

	// Request Groth16 deposit proof from the gnark server.
	proofData := struct {
		SenderID    string     `json:"senderId"`
		Address     string     `json:"address"`
		Hash        string     `json:"hash"`
		VInit       string     `json:"vInit"`
		VDeposit    string     `json:"vDeposit"`
		Secret      string     `json:"secret"`
		Pk          string     `json:"pk"`
		Commitments [][]string `json:"txCommit"`
		TxRandom    []string   `json:"txRandom"`
	}{
		SenderID:    strconv.FormatInt(int64(senderId), 10),
		Address:     addressBigInt.String(),
		Hash:        hash.String(),
		VInit:       amount.String(),
		VDeposit:    amountToDeposit.String(),
		Secret:      depositSecretBigInt.String(),
		Pk:          pk.String(),
		Commitments: commFinal,
		TxRandom:    utils.ConvertBigIntsToStrings(txRandom),
	}

	body, err := utils.PostJSON(utils.DepositProofURL, proofData)
	if err != nil {
		return fmt.Errorf("failed to generate proof: %v", err)
	}

	var proofResp interfacezkdvp.SnarkRespose
	if err = json.Unmarshal(body, &proofResp); err != nil {
		return fmt.Errorf("failed to parse proof response: %v", err)
	}

	// Encode proof elements as decimal strings for the relayer.
	var proof8 [8]string
	for i := 0; i < 8 && i < len(proofResp.Proof); i++ {
		proof8[i] = proofResp.Proof[i].String()
	}
	var pubSig2 [2]string
	for i := 0; i < 2 && i < len(proofResp.PublicSignal); i++ {
		pubSig2[i] = proofResp.PublicSignal[i].String()
	}

	// Encode kIndex as []int64 for the relayer.
	kIdx64 := make([]int64, len(kIndex))
	for i, k := range kIndex {
		kIdx64[i] = int64(k)
	}

	// Obtain the ZkDVP withdraw proof that travels with the deposit.
	zkDvpTx, err := ProccessGenerateProofAtZkDvp(amount, amountToDeposit, pk, depositSecret, addressBigInt)
	if err != nil {
		return fmt.Errorf("failed to generate ZkDVP proof: %v", err)
	}

	// Build and POST the relay request. The relayer signs and submits on-chain.
	relayReq := relayDepositRequest{
		Proof:        proof8,
		PublicSignal: pubSig2,
		Commitments:  commFinal,
		KIndex:       kIdx64,
		ZkDvpTx:      buildZkDvpTxJSON(zkDvpTx),
	}

	respBody, err := postRelayer("/relay/deposit", relayReq)
	if err != nil {
		return fmt.Errorf("relayer deposit: %v", err)
	}

	var relayResp relayResponse
	if err = json.Unmarshal(respBody, &relayResp); err != nil {
		return fmt.Errorf("failed to parse relay response: %v", err)
	}

	fmt.Printf("Deposit successful: tx=%s block=%d gas=%d\n",
		relayResp.TxHash, relayResp.BlockNumber, relayResp.GasUsed)
	return nil
}

func main() {
	senderId := 0
	originalAmount := big.NewInt(50)
	amountToDeposit := big.NewInt(5)
	kIndex := []int{0, 1, 2, 3, 4, 5}
	secrets := []*big.Int{
		big.NewInt(1234567890),
		big.NewInt(9876543210),
		big.NewInt(112233445566),
		big.NewInt(1234567890),
		big.NewInt(1234567890),
		big.NewInt(41241261412),
	}
	block_number := "1294712897901284091"
	depositSecret := "94"

	if err := deposit(senderId, originalAmount, amountToDeposit, kIndex, secrets, depositSecret, block_number); err != nil {
		fmt.Println("Deposit error:", err)
	}
}
