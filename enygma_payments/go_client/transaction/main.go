package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"

	"enygma/config"
	enygma "enygma/contracts"
	"enygma/agreement"
	"enygma/internal/curve"
	"enygma/internal/proof"
	"enygma/internal/randomness"
	"enygma/internal/types"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// chainRPCURL is used only for read-only chain queries (GetPublicValues, GetBlockHash).
// Write operations (Transfer) go through the relayer.
const chainRPCURL = "http://127.0.0.1:8545"

// ── Relay types ───────────────────────────────────────────────────────────────

type relayTransferRequest struct {
	Proof        [8]string  `json:"proof"`
	PublicSignal []string   `json:"publicSignal"`
	Commitments  [][]string `json:"commitments"`
	KIndex       []int64    `json:"kIndex"`
}

type relayTxResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
}

// ── Entry point ───────────────────────────────────────────────────────────────

type Address struct {
	Address string `json:"address"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	args, err := parseArguments()
	if err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	cfg, err := config.Load("./config/address.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	secrets, err := initializeSecrets(args.SenderId, "./keys", args.QtyBanks)
	if err != nil {
		return fmt.Errorf("initialize ML-KEM secrets: %w", err)
	}
	senderSecret, _ := poseidon.Hash([]*big.Int{args.PreviousR, args.Sk})
	senderSecret.Mod(senderSecret, curve.P)
	secrets[args.SenderId] = senderSecret

	return executeTransaction(cfg, args, secrets)
}

func parseArguments() (*types.TransactionArgs, error) {
	if len(os.Args) < 7 {
		return nil, fmt.Errorf("usage: %s <qtyBank> <value> <senderId> <sk> <previousV> <previousR>", os.Args[0])
	}

	qtyBanks, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return nil, fmt.Errorf("invalid qtyBank: %w", err)
	}

	value := new(big.Int)
	if _, ok := value.SetString(os.Args[2], 10); !ok {
		return nil, fmt.Errorf("invalid value")
	}

	senderId, err := strconv.Atoi(os.Args[3])
	if err != nil {
		return nil, fmt.Errorf("invalid senderId: %w", err)
	}

	sk := new(big.Int)
	if _, ok := sk.SetString(os.Args[4], 10); !ok {
		return nil, fmt.Errorf("invalid sk")
	}

	previousV := new(big.Int)
	if _, ok := previousV.SetString(os.Args[5], 10); !ok {
		return nil, fmt.Errorf("invalid previousV")
	}

	previousR := new(big.Int)
	if _, ok := previousR.SetString(os.Args[6], 10); !ok {
		return nil, fmt.Errorf("invalid previousR")
	}

	return &types.TransactionArgs{
		QtyBanks:  qtyBanks,
		Value:     value,
		SenderId:  senderId,
		Sk:        sk,
		PreviousV: previousV,
		PreviousR: previousR,
	}, nil
}

// ── Transaction execution ─────────────────────────────────────────────────────

func executeTransaction(cfg *config.Config, args *types.TransactionArgs, secrets []*big.Int) error {
	// Dial chain for read-only state queries only.
	client, err := ethclient.Dial(chainRPCURL)
	if err != nil {
		return fmt.Errorf("dial chain: %w", err)
	}
	defer client.Close()

	instance, err := enygma.NewEnygma(common.HexToAddress(cfg.ContractAddress), client)
	if err != nil {
		return fmt.Errorf("bind contract: %w", err)
	}

	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(int64(args.QtyBanks)))
	if err != nil {
		return fmt.Errorf("GetPublicValues: %w", err)
	}
	ReferenceBalance := pubVals.Balances
	PublicKeys := pubVals.Keys

	BlockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		return fmt.Errorf("GetBlockHash: %w", err)
	}

	kIndex := generateKIndex()
	HashArray := randomness.HashArrayGen(secrets, kIndex)
	TagMessage := randomness.TagMessageGen(args.SenderId, secrets, BlockHash, kIndex)
	TxValue := GenerateTxValues(args.Value)
	Nullifier, _ := poseidon.Hash([]*big.Int{HashArray[args.SenderId], BlockHash})
	TxCommit, TxRandom := randomness.GenCommitmentAndRandom(args.QtyBanks, args.Value, args.SenderId, TxValue, BlockHash, kIndex, secrets)

	proofResponse := proof.GenerateProof(args,
		Nullifier, BlockHash, PublicKeys,
		ReferenceBalance, TxCommit,
		TxValue, TxRandom, secrets,
		kIndex, HashArray, TagMessage, cfg,
	)

	return sendTransferViaRelayer(cfg.RelayerURL, TxCommit, proofResponse, kIndex)
}

// sendTransferViaRelayer serialises the proof and commitments and POSTs them to
// the relayer's /relay/transfer endpoint. The relayer signs and submits on-chain.
func sendTransferViaRelayer(relayerURL string, commitments []enygma.IEnygmaPoint, resp *types.Response, kIndex []*big.Int) error {
	commFinal := make([][]string, len(commitments))
	for i, c := range commitments {
		commFinal[i] = []string{c.C1.String(), c.C2.String()}
	}

	var proof8 [8]string
	for i := 0; i < 8 && i < len(resp.Proof); i++ {
		proof8[i] = resp.Proof[i].String()
	}

	pubSig := make([]string, len(resp.PublicSignal))
	for i, v := range resp.PublicSignal {
		pubSig[i] = v.String()
	}

	kIdx64 := make([]int64, len(kIndex))
	for i, k := range kIndex {
		kIdx64[i] = k.Int64()
	}

	reqBody := relayTransferRequest{
		Proof:        proof8,
		PublicSignal: pubSig,
		Commitments:  commFinal,
		KIndex:       kIdx64,
	}

	data, _ := json.Marshal(reqBody)

	apiKey := os.Getenv("RELAYER_API_KEY")
	if apiKey == "" {
		apiKey = "change-me"
	}

	req, err := http.NewRequest(http.MethodPost, relayerURL+"/relay/transfer", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("contact relayer: %w", err)
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("relayer returned %d: %s", httpResp.StatusCode, body)
	}

	var relayResp relayTxResponse
	if err := json.Unmarshal(body, &relayResp); err != nil {
		return fmt.Errorf("parse relay response: %w", err)
	}

	log.Printf("Transfer successful: tx=%s block=%d gas=%d",
		relayResp.TxHash, relayResp.BlockNumber, relayResp.GasUsed)
	return nil
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
/////////// DEMO PURPOSE ONLY /////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func GenerateTxValues(value *big.Int) []*big.Int {
	vNegate := curve.GetNegative(value)
	return []*big.Int{
		vNegate,
		big.NewInt(60),
		big.NewInt(40),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
	}
}

func generateKIndex() []*big.Int {
	return []*big.Int{
		big.NewInt(0), big.NewInt(1), big.NewInt(2),
		big.NewInt(3), big.NewInt(4), big.NewInt(5),
	}
}

// initializeSecrets creates ML-KEM-768 pairwise shared secrets for the sender.
//
// In this demo all bank view keys live under storeDir on the same machine.
// The sender (senderID) is the "leader": it encapsulates to each peer's public
// key on first call, producing a ciphertext that the peer can later decapsulate
// to recover the same shared secret.  Subsequent calls read from disk cache.
func initializeSecrets(senderID int, storeDir string, nBanks int) ([]*big.Int, error) {
	mgrs := make([]*agreement.Manager, nBanks)
	for i := 0; i < nBanks; i++ {
		m, err := agreement.New(i, storeDir)
		if err != nil {
			return nil, fmt.Errorf("bank %d manager: %w", i, err)
		}
		mgrs[i] = m
	}

	sender := mgrs[senderID]
	secrets := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		if i == senderID {
			continue // set by caller from Poseidon(prevR, sk)
		}
		peerEK := mgrs[i].EncapsulationKey()
		ss, err := sender.GetOrEstablish(i, peerEK)
		if err != nil {
			return nil, fmt.Errorf("establish secret with bank %d: %w", i, err)
		}
		secrets[i] = ss
	}
	return secrets, nil
}
