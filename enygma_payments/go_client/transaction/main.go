package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"

	"enygma/agreement"
	"enygma/config"
	enygma "enygma/contracts"
	"enygma/internal/curve"
	"enygma/internal/proof"
	"enygma/internal/randomness"
	"enygma/internal/types"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
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

// parseArguments reads the transaction's non-secret shape from argv and its
// three secrets — sk (the whole spend authority), previousV and previousR
// (which de-blind the sender's current balance) — from environment
// variables instead.
//
// Fix M-10: these three used to be positional argv (os.Args[4..6]).
// os.Args is world-readable via `ps aux` and `/proc/<pid>/cmdline` on any
// shared host, and persists in shell history, CI logs and container
// specs, with no rotation path for a leaked sk. Environment variables
// aren't a complete fix for a genuinely hostile host (they're visible via
// /proc/<pid>/environ to anyone who can already read cmdline), but they
// don't appear in `ps aux`, don't get echoed into shell history by the
// invocation itself, and are the same mechanism this codebase already
// uses for its other secrets (RELAYER_PRIVATE_KEY, RELAYER_API_KEY,
// MY_KEY) — consistent with the audit's remediation ("read sk from a
// file, from stdin, or from an environment variable").
func parseArguments() (*types.TransactionArgs, error) {
	if len(os.Args) < 4 {
		return nil, fmt.Errorf("usage: ENYGMA_SK=<sk> ENYGMA_PREVIOUS_V=<balance> ENYGMA_PREVIOUS_R=<blinding> %s <qtyBank> <value> <senderId>", os.Args[0])
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

	sk, err := bigIntFromEnv("ENYGMA_SK")
	if err != nil {
		return nil, err
	}
	previousV, err := bigIntFromEnv("ENYGMA_PREVIOUS_V")
	if err != nil {
		return nil, err
	}
	previousR, err := bigIntFromEnv("ENYGMA_PREVIOUS_R")
	if err != nil {
		return nil, err
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

func bigIntFromEnv(name string) (*big.Int, error) {
	s := os.Getenv(name)
	if s == "" {
		return nil, fmt.Errorf("%s not set — export it before running (never pass secrets as command-line arguments, see M-10)", name)
	}
	v := new(big.Int)
	if _, ok := v.SetString(s, 10); !ok {
		return nil, fmt.Errorf("%s: invalid decimal value", name)
	}
	return v, nil
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

	// Fix L-10: getPublicValues is indexed by on-chain accountId, and
	// accountId 0 is the permanent unregistered sentinel (never a real
	// bank) — fetching qtyBanks+1 and slicing off index 0 is what makes
	// ReferenceBalance[i]/PublicKeys[i] (circuit slot i, 0-based) actually
	// correspond to accountId i+1. The old unsliced GetPublicValues(qtyBanks)
	// call was internally self-consistent (both the fetch and the eventual
	// on-chain participantIds treated position == accountId), which is
	// exactly why it never reverted — it silently read and later credited
	// accountId 0, an ownerless sink check() cannot even see, while bank
	// qtyBanks (the last real bank) was never queried at all.
	pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(int64(args.QtyBanks+1)))
	if err != nil {
		return fmt.Errorf("GetPublicValues: %w", err)
	}
	ReferenceBalance := pubVals.Balances[1:]
	PublicKeys := pubVals.Keys[1:]

	BlockHash, err := instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		return fmt.Errorf("GetBlockHash: %w", err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return fmt.Errorf("ChainID: %w", err)
	}
	DomainId := computeDomainId(chainID, common.HexToAddress(cfg.ContractAddress))

	kIndex := generateKIndex()
	// Fix L-08 (blockers 1 and 2): FingerPrintGen replaces HashArrayGen —
	// see internal/randomness/operation.go's doc comment.
	FingerPrint := randomness.FingerPrintGen(secrets, args.SenderId)
	TxValue := GenerateTxValues(args.Value)
	// Fix L-08 (blocker 3): the nullifier is Poseidon(secrets[senderId],
	// BlockHash) directly — secrets[args.SenderId] is already
	// Poseidon(previousR, sk) mod P (set in run(), matching what the
	// circuit calls secretRemain). The old code hashed HashArray[senderId]
	// here instead — an extra Poseidon(s,s) layer never applied by the
	// circuit's own Nullifier assertion, which is one of the three
	// independent reasons this path never produced a valid proof.
	//
	// Must be computed before TagMessageGen/GenCommitmentAndRandom: Fix
	// H-01/H-02 use it (not BlockHash, the epoch anchor) as the
	// per-transaction value mixed into both derivations — see
	// internal/randomness/operation.go.
	Nullifier, _ := poseidon.Hash([]*big.Int{secrets[args.SenderId], BlockHash})
	TagMessage := randomness.TagMessageGen(args.SenderId, secrets, Nullifier, kIndex)
	TxCommit, TxRandom := randomness.GenCommitmentAndRandom(args.QtyBanks, args.Value, args.SenderId, TxValue, Nullifier, kIndex, secrets)

	proofResponse, err := proof.GenerateProof(args,
		Nullifier, BlockHash, PublicKeys,
		ReferenceBalance, TxCommit,
		TxValue, TxRandom, secrets,
		kIndex, FingerPrint, TagMessage, DomainId, cfg,
	)
	if err != nil {
		// Fix L-09: GenerateProof now reports every prover failure as an
		// error instead of a nil-Proof "success" — surfacing it here,
		// before ever reaching the relayer, is the whole point of the fix.
		return fmt.Errorf("generate proof: %w", err)
	}

	// Fix H-09 (item 2 of the remediation list): ENYGMA_SUBMIT_MODE=direct
	// submits transfer() on chain using the bank's own key, bypassing the
	// relayer entirely, instead of the default relayer-assisted path.
	// registerAccount already accepts an arbitrary addr (never bound to
	// msg.sender), and onlyRegistered only checks "is msg.sender SOME
	// registered participant" — not "is msg.sender the specific bank this
	// proof moves value for" (that's the ZK proof's job, by design, since
	// msg.sender was never meant to identify the real sender within the
	// k-anonymity set). So a bank registered under its own address (see
	// cmd/register_bank) can always have submitted this way; this flag is
	// what actually removes the relayer as a mandatory dependency for a
	// bank that chooses to pay its own gas — a censored or unresponsive
	// relayer is no longer this bank's only option.
	if os.Getenv("ENYGMA_SUBMIT_MODE") == "direct" {
		if err := sendTransferDirect(client, instance, chainID, kIndex, TxCommit, proofResponse); err != nil {
			return err
		}
	} else {
		if err := sendTransferViaRelayer(cfg.RelayerURL, TxCommit, proofResponse, kIndex); err != nil {
			return err
		}
	}

	// Fix H-09 (item 5 of the remediation list): verify the relayer's
	// claimed success against the chain directly, on this process's own
	// RPC connection, exactly as demo/main.go's reference flow already
	// does. Before this, transaction/main.go was the ONE caller H-09's
	// own audit text names as not doing this — every other real client
	// (demo/main.go, every go_client/enygma_test integration test)
	// independently re-derives the expected new balance and compares.
	// A relayer that returns 200 without actually submitting (or that
	// gets front-run/reordered into a different outcome) leaves the
	// sender's commitment unchanged; this catches that here, at the
	// client, rather than trusting the relayer's response at face value.
	// It does not defend against a relayer that submits nothing at all
	// AND never responds (that surfaces as a network error above, or a
	// timeout the caller must set its own limit on) — only against one
	// that claims success without the state actually having moved.
	return verifyTransferOnChain(instance, args.SenderId, ReferenceBalance[args.SenderId], TxCommit[args.SenderId], args.Value)
}

// verifyTransferOnChain re-reads the sender's on-chain balance and
// confirms it equals prevBalance homomorphically added to the sender's
// own delta commitment (senderDelta) — the same check, using the same
// contract-native addPedComm arithmetic, that demo/main.go's reference
// flow performs after every relay call (Fix H-09).
func verifyTransferOnChain(instance *enygma.Enygma, senderIdx int, prevBalance, senderDelta enygma.IEnygmaPoint, amount *big.Int) error {
	newBal, err := instance.GetBalance(&bind.CallOpts{}, big.NewInt(int64(senderIdx+1)))
	if err != nil {
		return fmt.Errorf("verify: GetBalance: %w", err)
	}
	expX, expY, err := instance.AddPedComm(&bind.CallOpts{}, prevBalance.C1, prevBalance.C2, senderDelta.C1, senderDelta.C2)
	if err != nil {
		return fmt.Errorf("verify: addPedComm: %w", err)
	}
	if newBal.X.Cmp(expX) != 0 || newBal.Y.Cmp(expY) != 0 {
		return fmt.Errorf("verify FAILED: on-chain balance (%s, %s) does not match prevBalance+delta (%s, %s) — "+
			"the relayer reported success but the sender's commitment did not actually move as expected",
			newBal.X, newBal.Y, expX, expY)
	}
	log.Printf("verify: on-chain balance matches prevBalance + TxCommit[sender] ✓ (-%s tokens confirmed)", amount)
	return nil
}

// computeDomainId mirrors Enygma.sol's _expectedDomainId() (Fix L-01):
// (block.chainid << 160) | uint256(uint160(address(this))).
func computeDomainId(chainID *big.Int, contractAddr common.Address) *big.Int {
	addr := new(big.Int).SetBytes(contractAddr.Bytes())
	chain := new(big.Int).Lsh(chainID, 160)
	return new(big.Int).Or(chain, addr)
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

	// Fix L-10: kIndex holds the circuit's internal 0-based slot values
	// (also the AnonymitySet signal baked into the proof); the on-chain
	// participantIds this relay request's "kIndex" field actually becomes
	// (Enygma.transfer's third argument) are real 1-based accountIds. The
	// contract would have happily accepted the unmapped 0-based array —
	// H-07 rejects accountId 0 as unregistered, but nothing on chain
	// would have stopped positions 1-5 silently crediting/debiting
	// accounts 1-5 while the real intended participants (accounts 2-6,
	// since accountId = position+1) never moved — the old code's fetch
	// bug above happened to make that internally consistent instead of a
	// revert, which is why it never surfaced as an error.
	kIdx64 := make([]int64, len(kIndex))
	for i, k := range kIndex {
		kIdx64[i] = k.Int64() + 1
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

// sendTransferDirect submits transfer() on chain directly, signed by the
// bank's own key (BANK_ETH_PRIVATE_KEY), with no relayer involved at all
// (Fix H-09, item 2). bankTag is "" — there is no relayer credential to
// attribute when the bank is submitting for itself.
func sendTransferDirect(client *ethclient.Client, instance *enygma.Enygma, chainID *big.Int, kIndex []*big.Int, commitments []enygma.IEnygmaPoint, resp *types.Response) error {
	bankKeyHex := strings.TrimPrefix(os.Getenv("BANK_ETH_PRIVATE_KEY"), "0x")
	if bankKeyHex == "" {
		return fmt.Errorf("ENYGMA_SUBMIT_MODE=direct requires BANK_ETH_PRIVATE_KEY (the bank's own registered signing key — see cmd/register_bank)")
	}
	bankKey, err := crypto.HexToECDSA(bankKeyHex)
	if err != nil {
		return fmt.Errorf("parse BANK_ETH_PRIVATE_KEY: %w", err)
	}
	bankAddr := crypto.PubkeyToAddress(bankKey.PublicKey)

	var proof8 [8]*big.Int
	for i := 0; i < 8 && i < len(resp.Proof); i++ {
		proof8[i] = resp.Proof[i]
	}
	var pubSig81 [81]*big.Int
	for i, v := range resp.PublicSignal {
		pubSig81[i] = v
	}
	transferProof := enygma.IEnygmaProof{Proof: proof8, PublicSignal: pubSig81}

	// Fix L-10: same slot->accountId mapping sendTransferViaRelayer uses
	// — kIndex holds the circuit's internal 0-based AnonymitySet values;
	// the on-chain participantIds are the real 1-based accountIds.
	participantIds := make([]*big.Int, len(kIndex))
	for i, k := range kIndex {
		participantIds[i] = new(big.Int).Add(k, big.NewInt(1))
	}

	ctx := context.Background()
	nonce, err := client.PendingNonceAt(ctx, bankAddr)
	if err != nil {
		return fmt.Errorf("nonce for %s: %w", bankAddr, err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("suggest gas price: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(bankKey, chainID)
	if err != nil {
		return fmt.Errorf("build transactor: %w", err)
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.GasLimit = 16_000_000
	auth.GasPrice = gasPrice

	log.Printf("submitting Transfer directly as %s (no relayer)...", bankAddr.Hex())
	tx, err := instance.Transfer(auth, commitments, transferProof, participantIds, "")
	if err != nil {
		return fmt.Errorf("Transfer(): %w", err)
	}
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return fmt.Errorf("wait mined: %w", err)
	}
	if receipt.Status != 1 {
		return fmt.Errorf("transfer transaction reverted on-chain (tx=%s)", tx.Hash().Hex())
	}
	log.Printf("Transfer successful (direct, no relayer): tx=%s block=%d gas=%d",
		tx.Hash().Hex(), receipt.BlockNumber.Uint64(), receipt.GasUsed)
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

// generateKIndex returns the circuit's internal 0-based anonymity-set slot
// values — this is what SenderId/AnonymitySet in the proof itself use, and
// what secrets/PublicKeys/ReferenceBalance are indexed by (position i ↔
// on-chain accountId i+1, per the slicing above). It is NOT the array
// submitted on chain — sendTransferViaRelayer maps slot i to accountId
// i+1 (Fix L-10) when building that request; conflating the two was the
// bug.
func generateKIndex() []*big.Int {
	return []*big.Int{
		big.NewInt(0), big.NewInt(1), big.NewInt(2),
		big.NewInt(3), big.NewInt(4), big.NewInt(5),
	}
}

// initializeSecrets creates ML-KEM-768 pairwise shared secrets for the sender.
//
// Fix M-11 defect 3 ("the most serious"): this used to call agreement.New
// for every OTHER bank too, which — because New's loadOrCreate silently
// generates a fresh keypair for any bankID with no seed file on disk —
// meant the sender's own process fabricated every peer's "private" key
// locally and then encapsulated to public keys it had invented moments
// earlier. On any host that is not a single shared-storeDir demo
// machine, every pairwise secret was unilaterally decided by the sender
// alone, with no counterparty able to derive its own blinding factor or
// tag, and the failure was completely silent. Only the sender's own
// identity is loaded via agreement.New now; every peer's key is read via
// LoadPeerEncapsulationKey, which never generates one and fails loudly if
// the peer hasn't published it (i.e. hasn't run its own agreement.New).
//
// Fix M-11 defect 1: the sender only leads (encapsulates) for peers with
// a strictly higher bankID; for a peer with a lower id, the sender is the
// follower and must call GetOrAccept instead — enforced inside Manager
// itself (GetOrEstablish/GetOrAccept both reject the wrong direction), so
// this loop just tries the side that applies and lets the peer's own
// process have already run GetOrEstablish for the other direction. This
// only actually matters once senderID is not always the lowest id in the
// set, which requires participation from more than this one process —
// out of scope for what this single-process CLI can exercise on its own,
// but the guard rejects the wrong call cleanly rather than silently
// producing a divergent secret, which is the property that matters here.
func initializeSecrets(senderID int, storeDir string, nBanks int) ([]*big.Int, error) {
	sender, err := agreement.New(senderID, storeDir)
	if err != nil {
		return nil, fmt.Errorf("bank %d manager: %w", senderID, err)
	}

	secrets := make([]*big.Int, nBanks)
	for i := 0; i < nBanks; i++ {
		if i == senderID {
			continue // set by caller from Poseidon(prevR, sk)
		}
		var ss *big.Int
		if senderID < i {
			peerEK, err := agreement.LoadPeerEncapsulationKey(storeDir, i)
			if err != nil {
				return nil, fmt.Errorf("load bank %d's published key: %w", i, err)
			}
			ss, err = sender.GetOrEstablish(i, peerEK)
			if err != nil {
				return nil, fmt.Errorf("establish secret with bank %d: %w", i, err)
			}
		} else {
			ss, err = sender.GetOrAccept(i)
			if err != nil {
				return nil, fmt.Errorf("accept secret from bank %d: %w", i, err)
			}
		}
		secrets[i] = ss
	}
	return secrets, nil
}
