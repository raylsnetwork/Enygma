package main

import (
	"flag"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/consensys/gnark/constraint/solver"

	"enygma_dvp/gnark_circuits/primitives"
	script "enygma_dvp/gnark_circuits/scripts"
	"enygma_dvp/gnark_circuits/templates"
)

// GenerationVkPk generates proving keys (PK) and verification keys (VK) for all
// active circuits and writes them to scripts/keys/.
//
// Currently active circuits (3):
//   - PrivateMint      — private ERC-20 mint with commitment binding
//   - DvPInitiator     — Alice's side of a DvP swap (spend + 3 output commitments)
//   - DvPDestination   — Bob's side of a DvP swap (spend + cross-commitment check)
//
// The following circuits are implemented (templates + server routes exist) but are
// not yet part of the active deployment. Their key generation is commented out
// because they are not covered by integration tests and their VK slots are not
// registered on-chain. Uncomment the relevant block and re-run this file to
// activate a circuit:
//
//   - ERC20 JoinSplit (2-in/2-out, 10-in/2-out)
//   - ERC20 JoinSplit with Auditor
//   - ERC721 Ownership
//   - ERC721 Ownership with Auditor
//   - ERC1155 Fungible JoinSplit (1-in/1-out, 2-in/2-out)
//   - ERC1155 Fungible JoinSplit with Auditor
//   - ERC1155 NonFungible (1-token, 10-token batch)
//   - ERC1155 NonFungible with Auditor
//   - Auction Init, Auction Bid, Auction Private Opening, Auction Not Winning
//   - Auction Init with Auditor, Auction Bid with Auditor
//
// GenerationVkPk regenerates every circuit named in only, or all of them if only
// is empty. Each run draws fresh random keys, so regenerate only the circuits
// that changed: regenerating PrivateMint also requires re-patching
// PrivateMintVerifier.sol (cmd/patch_verifier) and the retail copies.
//
// It makes no keys for the Payment-family circuits (Payment, Payment2in,
// PaymentFee, PaymentRelayerFeePublic, UsdrFee): the Payment deployment
// proves enygma_retail_payments' templates with retail's committed keys,
// which make dvp-keygen copies in after it (as it does retail's PrivateMint
// keys), and cmd/export_vk_payment refuses any others.
func GenerationVkPk(only map[string]bool) {
	want := func(name string) bool { return len(only) == 0 || only[name] }

	solver.RegisterHint(primitives.ModHint)

	// NOT covered by integration tests (test/01–04)
	// auctionbidConfig := templates.AuctionBidCircuitConfig{
	// 	TmNInputs:    2,
	// 	TmMOutputs:   2,
	// 	TmNumOfIdParams:5,
	// 	TmDepthMerkle: 8,
	// 	TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	// 	TmGroupMerkleTreeDepth: 8,
	// }

	// NOT covered by integration tests (test/01–04)
	// auctionBidAuditorConfig := templates.TmAuctionBidAuditorCircuitConfig{
	// 	TmInputs:	2,
	// 	TmOutputs:	2,
	// 	TmNumOfIdParms:5,
	// 	TmMerkleTreeDepth: 8,
	// 	TmRange:frontend.Variable("1000000000000000000000000000000000000"),
	// 	TmAssetGroupMerkleTreeDepth:8,
	// }

	// NOT covered by integration tests (test/01–04)
	// auctionInitAuditorConfig := templates.TmAuctionInitAuditorCircuitConfig{
	// 	TmNumOfIdParms :5,
	// 	TmMerkleTreeDepth:    8,
	// 	TmAssetGroupMerkleTreeDepth:8,
	// }

	// NOT covered by integration tests (test/01–04)
	// auctionInitConfig := templates.AuctionInitCircuitConfig{
	// 	TmNumOfIdParms:5,
	// 	TmMerkleTreeDepth:8,
	// 	TmGroupMerkleTreeDepth:8,
	// }

	// NOT covered by integration tests (test/01–04)
	// auctionNotWinningConfig := templates.AuctionNotWinningCircuitConfig{
	// 	TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	// }

	// NOT covered by integration tests (test/01–04)
	// privateOpeningConfig := templates.AuctionPrivateOpeningCircuitConfig{
	// 	TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	// }

	// NOT covered by integration tests (test/01–04)
	// erc1155Batch := templates.ERC1155NonFungibleCircuitConfig{
	// 	TmNumOfTokens: 10,
	// 	TmMerkleTreeDepth: 8,
	// 	TmAssetGroupMerkleTreeDepth:8,
	// }

	// NOT covered by integration tests (test/01–04)
	// erc1155BatchNonFungible := templates.ERC1155NonFungibleWithAuditorCircuitConfig{
	// 	TmNumOfTokens: 10,
	// 	TmMerkleTreeDepth: 8,
	// 	TmAssetGroupMerkleTree:8,
	// }

	// NOT covered by integration tests (test/01–04)
	// erc20Auditor_join_split_10_2 := templates.Erc20WithAuditorConfig{
	// 	TmNInputs: 10,
	// 	TmMOutputs: 2,
	// 	TmMerkleTreeDepth:8,
	// 	TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	// }

	// NOT covered by integration tests (test/01–04)
	// erc20_auditor_join_split := templates.Erc20WithAuditorConfig{
	// 	TmNInputs: 2,
	// 	TmMOutputs:  2,
	// 	TmMerkleTreeDepth:8,
	// 	TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	// }

	// NOT covered by integration tests (test/01–04)
	// erc1155_join_split := templates.ERC1155FungibleCircuitConfig{
	// 	TmNInputs: 2,
	// 	TmMOutputs: 2,
	// 	TmMerkleTreeDepth:8,
	// 	TmAssetGroupMerkleTree: 8,
	// 	TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	// }

	// NOT covered by integration tests (test/01–04)
	// erc1155_join_split_with_auditor := templates.ERC1155FungibleWithAuditorCircuitConfig{
	// 	TmNInputs: 2,
	// 	TmMOutputs: 2,
	// 	TmMerkleTreeDepth:8,
	// 	TmAssetGroupMerkleTree: 8,
	// 	TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	// }

	// NOT covered by integration tests (test/01–04)
	// ownership_erc721_config := templates.Erc721CircuitConfig{
	// 	TmNumOfTokens: 1,
	// 	TmMerkleTreeDepth: 8,
	// }

	// NOT covered by integration tests (test/01–04)
	// ownership_erc721_config_with_auditor := templates.Erc721WithAuditorCircuitConfig{
	// 	TmNumOfTokens: 1,
	// 	TmMerkleTreeDepth: 8,
	// }

	// NOT covered by integration tests (test/01–04)
	// ownership_erc1155_Fungible_config := templates.ERC1155FungibleCircuitConfig{
	// 	TmNInputs: 1,
	// 	TmMOutputs: 1,
	// 	TmMerkleTreeDepth:8,
	// 	TmAssetGroupMerkleTree: 8,
	// 	TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	// }

	// NOT covered by integration tests (test/01–04)
	// ownership_erc1155_Non_Fungible_config := templates.ERC1155NonFungibleCircuitConfig{
	// 	TmNumOfTokens:1,
	// 	TmMerkleTreeDepth:8,
	// 	TmAssetGroupMerkleTreeDepth:8,
	// }

	// NOT covered by integration tests (test/01–04)
	// ownership_erc1155_Non_Fungible_config_with_auditor := templates.ERC1155NonFungibleWithAuditorCircuitConfig{
	// 	TmNumOfTokens:1,
	// 	TmMerkleTreeDepth:8,
	// 	TmAssetGroupMerkleTree:8,
	// }

	// covered by test/03_v2_dvp_test.go and test/04_v2_dvp_deadline_test.go
	dvp_initiator_config := templates.DvPInitiatorCircuitConfig{
		TmMerkleTreeDepth: 8,
	}

	// covered by test/03_v2_dvp_test.go and test/04_v2_dvp_deadline_test.go
	dvp_destination_config := templates.DvPDestinationCircuitConfig{
		TmMerkleTreeDepth: 8,
	}

	// covered by test/01_v2_erc20_private_mint_test.go (TestV2Erc20OnChain_PrivateMint)
	if want("PrivateMint") {
		script.SetupPrivateMint("PrivateMint")
	}
	if want("DvPInitiator") {
		script.SetupDvPInitiator(dvp_initiator_config, "DvPInitiator")
	}
	if want("DvPDestination") {
		script.SetupDvPDestination(dvp_destination_config, "DvPDestination")
	}
}

// circuits are the circuits GenerationVkPk makes keys for.
var circuits = []string{"PrivateMint", "DvPInitiator", "DvPDestination"}

// parseOnly reads -only's comma-separated circuit names. It refuses a name
// keygen makes no keys for, rather than make none: the Payment-family keys,
// say, are enygma_retail_payments', which make dvp-keygen copies.
func parseOnly(list string) (map[string]bool, error) {
	only := map[string]bool{}
	for _, name := range strings.Split(list, ",") {
		if name = strings.TrimSpace(name); name == "" {
			continue
		}
		if !slices.Contains(circuits, name) {
			return nil, fmt.Errorf("keygen makes no %s keys: it makes %s; make dvp-keygen copies the Payment-family keys from enygma_retail_payments",
				name, strings.Join(circuits, ", "))
		}
		only[name] = true
	}
	return only, nil
}

func main() {
	onlyFlag := flag.String("only", "", "comma-separated circuits to regenerate (default: all), e.g. DvPInitiator,DvPDestination")
	flag.Parse()
	only, err := parseOnly(*onlyFlag)
	if err != nil {
		log.Fatal(err)
	}
	GenerationVkPk(only)
}
