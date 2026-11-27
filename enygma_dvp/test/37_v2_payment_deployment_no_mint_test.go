package tests

// 37_v2_payment_deployment_no_mint_test.go
//
// The dedicated Payment deployment (scripts/cmd/deploy_payment and
// init_payment) has two vaults, the main token's and USDr's, and both take
// deposits. A private mint adds a note that no deposit backs, so the
// deployment ships no PrivateMintVerifier unless enygmadvp_payment.config.json
// turns "private-mint" on, which the committed config does not. Needs the
// chain make dvp-stack brings up; no test mints on this deployment.

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestV2PaymentDeployment_RegistersNoMintVerifier(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping onchain test")
	}
	data, err := os.ReadFile("../build/payment_receipts.json")
	if err != nil {
		t.Fatalf("read build/payment_receipts.json: %v — run deploy_payment+init_payment first", err)
	}
	var receipts map[string]onchainReceiptEntry
	if err := json.Unmarshal(data, &receipts); err != nil {
		t.Fatalf("parse payment_receipts.json: %v", err)
	}
	dvp, ok := receipts["EnygmaDvp"]
	if !ok {
		t.Fatalf("payment_receipts.json has no EnygmaDvp: %s", data)
	}
	if pm, ok := receipts["PrivateMintVerifier"]; ok {
		t.Errorf("deploy_payment deployed a PrivateMintVerifier (%s)", pm.ContractAddress)
	}

	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	defer client.Close()
	dvpAddr := common.HexToAddress(dvp.ContractAddress)
	logs, err := client.FilterLogs(context.Background(), ethereum.FilterQuery{Addresses: []common.Address{dvpAddr}})
	if err != nil {
		t.Fatalf("logs of the payment EnygmaDvp %s: %v", dvpAddr.Hex(), err)
	}
	// Its constructor logs RoleGranted, so a query that misses the
	// deployment's blocks finds nothing at all.
	if len(logs) == 0 {
		t.Fatalf("no logs at all from the payment EnygmaDvp %s", dvpAddr.Hex())
	}
	registered := loadOnchainABI(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json").Events["PrivateMintVerifierRegistered"]
	if registered.ID == (common.Hash{}) {
		t.Fatal("EnygmaDvp's ABI has no PrivateMintVerifierRegistered event")
	}
	for _, l := range logs {
		if len(l.Topics) > 0 && l.Topics[0] == registered.ID {
			t.Errorf("the payment EnygmaDvp logged PrivateMintVerifierRegistered(%s) in block %d",
				common.BytesToAddress(l.Topics[1].Bytes()).Hex(), l.BlockNumber)
		}
	}
}
