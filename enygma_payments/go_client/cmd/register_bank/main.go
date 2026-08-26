// register_bank is a one-time setup tool that registers a BANK's OWN
// Ethereum address as an Enygma participant, distinct from the relayer's
// self-registration (relayer/cmd/register) and from the owner's address
// every deployment script otherwise reuses for every bank.
//
// Fix H-09 (item 2 of 5): registerAccount already accepts an arbitrary
// addr parameter — it was never bound to msg.sender — so nothing about
// the contract prevented per-bank identity; every deployment simply
// registered every bank under the SAME (owner's) address as a
// convenience. Since onlyRegistered only checks "is msg.sender ANY
// registered participant" (not "is msg.sender the specific bank this
// proof moves value for" — that's the ZK proof's job, by design), a bank
// registered under its OWN address can call transfer()/transferWithFee()
// directly with its own key, with no dependency on the relayer at all.
// This tool is what makes that registration actually happen; see
// transaction/main.go's ENYGMA_SUBMIT_MODE=direct for the client side.
//
// Usage:
//
//	OWNER_PRIVATE_KEY=<hex>       \
//	BANK_ETH_PRIVATE_KEY=<hex>    \
//	BANK_SK=<decimal>             \
//	BANK_REG_R=<decimal>          \
//	go run ./cmd/register_bank --account-id 1 [--view-key-file ./keys/bank_1_ek.bin]
//
// Required env vars (never as command-line arguments — Fix M-10):
//
//	OWNER_PRIVATE_KEY    — hex ECDSA key authorised to call registerAccount
//	BANK_ETH_PRIVATE_KEY — hex ECDSA key the bank will sign its own
//	                       transfer()/transferWithFee() calls with; its
//	                       address is what gets bound to --account-id
//	BANK_SK              — the bank's ZK spend secret key (a decimal Baby
//	                       Jubjub subgroup scalar); publicKey =
//	                       Poseidon(sk, sk) mod P is derived from it and
//	                       registered on-chain — the same derivation
//	                       every circuit and go_client/enygma_test helper
//	                       already uses
//	BANK_REG_R           — the registration blinding factor; the initial
//	                       commitment Com(0, r) is derived from it and
//	                       registered on-chain (matches
//	                       registerAccount's existing "caller supplies a
//	                       pre-computed commitment, not raw randomness"
//	                       convention — Fix H-02 residual)
//
// Optional:
//
//	--view-key-file — path to the bank's published ML-KEM-768
//	                  encapsulation key (1184 bytes, e.g. as written by
//	                  agreement.New to <storeDir>/bank_<id>_ek.bin).
//	                  Omit only for an account that will never
//	                  participate in a ZK circuit — registerAccount
//	                  rejects any other length.
//	RELAYER_RPC_URL, RELAYER_CHAIN_ID, RELAYER_CONTRACT_ADDR,
//	RELAYER_ADDRESS_JSON — same defaults as relayer/cmd/register.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	enygma "enygma/contracts"
	"enygma/internal/curve"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

func main() {
	accountID := flag.Int64("account-id", 0,
		"accountId to register the bank under (must be non-zero, unique across all participants)")
	viewKeyFile := flag.String("view-key-file", "",
		"path to the bank's published ML-KEM-768 encapsulation key (1184 bytes); omit for an account that will never participate in a ZK circuit")
	flag.Parse()

	if *accountID == 0 {
		log.Fatal("--account-id must be set and non-zero")
	}

	rpcURL := envOr("RELAYER_RPC_URL", "http://127.0.0.1:8545")
	chainIDStr := envOr("RELAYER_CHAIN_ID", "1337")
	contractAddrStr := os.Getenv("RELAYER_CONTRACT_ADDR")
	addressJSON := envOr("RELAYER_ADDRESS_JSON", "./config/address.json")

	ownerKeyHex := requireEnv("OWNER_PRIVATE_KEY")
	bankKeyHex := strings.TrimPrefix(requireEnv("BANK_ETH_PRIVATE_KEY"), "0x")
	sk := requireBigIntEnv("BANK_SK")
	regR := requireBigIntEnv("BANK_REG_R")

	var viewKey []byte
	if *viewKeyFile != "" {
		var err error
		viewKey, err = os.ReadFile(*viewKeyFile)
		if err != nil {
			log.Fatalf("read --view-key-file %s: %v", *viewKeyFile, err)
		}
		if len(viewKey) != 1184 {
			log.Fatalf("--view-key-file %s: %d bytes, want exactly 1184 (a real ML-KEM-768 encapsulation key) — registerAccount will reject anything else", *viewKeyFile, len(viewKey))
		}
	} else {
		log.Println("no --view-key-file given: registering with an empty viewKey — this account will not be able to participate as a k-anonymity-set recipient in any circuit until it publishes one")
	}

	chainID, ok := new(big.Int).SetString(chainIDStr, 10)
	if !ok {
		log.Fatalf("invalid RELAYER_CHAIN_ID: %q", chainIDStr)
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("dial %s: %v", rpcURL, err)
	}
	defer client.Close()

	ownerKey, err := crypto.HexToECDSA(strings.TrimPrefix(ownerKeyHex, "0x"))
	if err != nil {
		log.Fatalf("parse OWNER_PRIVATE_KEY: %v", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(ownerKey, chainID)
	if err != nil {
		log.Fatalf("build transactor: %v", err)
	}
	auth.Value = big.NewInt(0)
	auth.GasLimit = 16_000_000

	bankKey, err := crypto.HexToECDSA(bankKeyHex)
	if err != nil {
		log.Fatalf("parse BANK_ETH_PRIVATE_KEY: %v", err)
	}
	bankAddr := crypto.PubkeyToAddress(bankKey.PublicKey)

	if contractAddrStr == "" {
		contractAddrStr, err = readAddressJSON(addressJSON)
		if err != nil {
			log.Fatalf("resolve contract address: %v", err)
		}
	}

	instance, err := enygma.NewEnygma(common.HexToAddress(contractAddrStr), client)
	if err != nil {
		log.Fatalf("bind contract at %s: %v", contractAddrStr, err)
	}

	// publicKey = Poseidon(sk, sk) mod P — the same derivation every
	// circuit's "knowledge of SecretKey" check and every go_client helper
	// (e.g. enygma_test's bankSks/pks loop) already uses.
	pkHash, err := poseidon.Hash([]*big.Int{sk, sk})
	if err != nil {
		log.Fatalf("derive public key: %v", err)
	}
	publicKey := new(big.Int).Mod(pkHash, curve.P)

	// Initial commitment Com(0, regR) — matches registerAccount's
	// existing convention (Fix H-02 residual): the caller supplies a
	// pre-computed commitment point, never raw randomness in calldata.
	commitPt := curve.PedersenCommitment(big.NewInt(0), regR)

	log.Printf("registering bank address %s as accountId=%d (publicKey=%s)", bankAddr.Hex(), *accountID, publicKey)

	tx, err := instance.RegisterAccount(auth, bankAddr, big.NewInt(*accountID), publicKey, commitPt.X, commitPt.Y, viewKey)
	if err != nil {
		log.Fatalf("registerAccount: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		log.Fatalf("wait mined: %v", err)
	}
	if receipt.Status != 1 {
		log.Fatalf("registerAccount reverted (tx=%s)", tx.Hash().Hex())
	}

	log.Printf("bank registered: address=%s accountId=%d tx=%s", bankAddr.Hex(), *accountID, tx.Hash().Hex())
	log.Printf("this bank can now submit transfer()/transferWithFee() directly with BANK_ETH_PRIVATE_KEY — see transaction/main.go's ENYGMA_SUBMIT_MODE=direct")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s must be set", key)
	}
	return v
}

func requireBigIntEnv(key string) *big.Int {
	s := requireEnv(key)
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		log.Fatalf("%s: invalid decimal value", key)
	}
	return v
}

func readAddressJSON(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var f struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	if f.Address == "" {
		return "", fmt.Errorf("%s: address field is empty", path)
	}
	return f.Address, nil
}
