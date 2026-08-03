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

Withdraw: Enygma balance update via relayer; ZkDVP leaf insertion handled by
the relayer after on-chain confirmation. The client never touches the signing key.
*/

// ── Relayer types ─────────────────────────────────────────────────────────────

// depositParam mirrors the relayer's DepositParam struct.
// Each element describes one output split of a withdraw: amount, token contract,
// and the recipient's public key in the Enygma anonymity set.
type depositParam struct {
	Amount    string `json:"amount"`    // decimal *big.Int
	Erc20Addr string `json:"erc20Addr"` // hex address of the ERC-20 token
	PublicKey string `json:"publicKey"` // decimal *big.Int
}

// relayWithdrawRequest matches the relayer's POST /relay/withdraw body.
type relayWithdrawRequest struct {
	Proof         [8]string      `json:"proof"`
	PublicSignal  [1]string      `json:"publicSignal"`
	Commitments   [][]string     `json:"commitments"`
	KIndex        []int64        `json:"kIndex"`
	DepositParams []depositParam `json:"depositParams"`
	HashArray     []string       `json:"hashArray"`
}

// relayWithdrawResponse is the success body returned by /relay/withdraw.
// (Identical shape to the deposit response; defined separately to keep
// each file self-contained for independent go run usage.)
type relayWithdrawResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
}

// ── Relayer helper ────────────────────────────────────────────────────────────

// postWithdrawRelayer sends an authenticated POST to the relayer and returns
// the response body on HTTP 200. Any non-200 response is returned as an error.
// (Mirrors postRelayer in deposit.go; both files are run independently.)
func postWithdrawRelayer(path string, payload interface{}) ([]byte, error) {
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

// ── Withdraw ──────────────────────────────────────────────────────────────────

func withdraw(senderId int, amount *big.Int, vSplit []*big.Int, kIndex []int, depositKey []*big.Int, secrets []*big.Int, block_number string) error {
	blockNumber, _ := new(big.Int).SetString(block_number, 10)

	// Generate Pedersen commitments locally.
	commitments, txRandom := utils.GenerateWithdrawCommitments(amount, senderId, blockNumber, kIndex, secrets)

	addressBigInt, _ := new(big.Int).SetString(utils.Address[2:], 16)

	// Compute per-split hashes and public keys.
	// hashArray: Poseidon(Poseidon(addr, vSplit[i]), pk[i]) — inserted into ZkDVP by relayer.
	// pkArray:   Poseidon(depositKey[i]) — recipient public key for each split.
	var hashArray []*big.Int
	var pkArray []*big.Int
	for i := 0; i < len(vSplit); i++ {
		inputs := []*big.Int{addressBigInt, vSplit[i]}
		poseidonHash, _ := poseidon.Hash(inputs)
		pk := utils.GetPkZkDvP(depositKey[i])
		inputsWithPubKey := []*big.Int{poseidonHash, pk}
		hash, _ := poseidon.Hash(inputsWithPubKey)
		hashArray = append(hashArray, hash)
		pkArray = append(pkArray, pk)
	}

	// Encode commitments as [[C1x, C1y], ...] for JSON.
	var commFinal [][]string
	for _, c := range commitments {
		commFinal = append(commFinal, []string{c.C1.String(), c.C2.String()})
	}

	// Encode hash and value arrays as decimal strings.
	var hashArrayString []string
	for _, h := range hashArray {
		hashArrayString = append(hashArrayString, h.String())
	}
	var valueArray []string
	for _, v := range vSplit {
		valueArray = append(valueArray, v.String())
	}
	var pkArrayString []string
	for _, pk := range pkArray {
		pkArrayString = append(pkArrayString, pk.String())
	}

	// Request Groth16 withdraw proof from the gnark server.
	proofData := interfacezkdvp.SnarkWithdraw{
		SenderID:  strconv.FormatInt(int64(senderId), 10),
		Address:   addressBigInt.String(),
		HashArray: hashArrayString,
		VArray:    valueArray,
		V:         amount.String(),
		Pk:        pkArrayString,
		TxCommit:  commFinal,
		TxRandom:  utils.ConvertBigIntsToStrings(txRandom),
	}

	url := utils.WithdrawProofURL + "/" + strconv.Itoa(len(vSplit))
	body, err := utils.PostJSON(url, proofData)
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
	var pubSig1 [1]string
	if len(proofResp.PublicSignal) > 0 {
		pubSig1[0] = proofResp.PublicSignal[0].String()
	}

	// Encode kIndex as []int64 for the relayer.
	kIdx64 := make([]int64, len(kIndex))
	for i, k := range kIndex {
		kIdx64[i] = int64(k)
	}

	// Build deposit params: one entry per output split.
	// Amount and PublicKey are decimal strings; Erc20Addr is the token contract hex.
	depositParams := make([]depositParam, len(vSplit))
	for i := range vSplit {
		depositParams[i] = depositParam{
			Amount:    vSplit[i].String(),
			Erc20Addr: utils.Address,
			PublicKey: pkArray[i].String(),
		}
	}

	// Build and POST the relay request.
	// The relayer signs and submits on-chain, then inserts each HashArray entry
	// into the ZkDVP Merkle tree after confirmation.
	relayReq := relayWithdrawRequest{
		Proof:         proof8,
		PublicSignal:  pubSig1,
		Commitments:   commFinal,
		KIndex:        kIdx64,
		DepositParams: depositParams,
		HashArray:     hashArrayString,
	}

	respBody, err := postWithdrawRelayer("/relay/withdraw", relayReq)
	if err != nil {
		return fmt.Errorf("relayer withdraw: %v", err)
	}

	var relayResp relayWithdrawResponse
	if err = json.Unmarshal(respBody, &relayResp); err != nil {
		return fmt.Errorf("failed to parse relay response: %v", err)
	}

	fmt.Printf("Withdraw successful: tx=%s block=%d gas=%d\n",
		relayResp.TxHash, relayResp.BlockNumber, relayResp.GasUsed)
	return nil
}

func main() {
	senderId := 0
	amount := big.NewInt(50)
	kIndex := []int{0, 1, 2, 3, 4, 5}
	vSplit := []*big.Int{
		big.NewInt(25),
		big.NewInt(7),
		big.NewInt(4),
		big.NewInt(8),
		big.NewInt(1),
		big.NewInt(5),
	}
	secrets := []*big.Int{
		big.NewInt(1234567890),
		big.NewInt(1241251),
		big.NewInt(112233445566),
		big.NewInt(1234567890),
		big.NewInt(1234567890),
		big.NewInt(41241261412),
	}
	block_number := "1294012084012789"
	depositKey := []*big.Int{
		big.NewInt(99),
		big.NewInt(98),
		big.NewInt(97),
		big.NewInt(96),
		big.NewInt(95),
		big.NewInt(94),
	}

	if err := withdraw(senderId, amount, vSplit, kIndex, depositKey, secrets, block_number); err != nil {
		fmt.Println("Withdraw error:", err)
	}
}
