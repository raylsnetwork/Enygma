package main

import (
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"

	"gnark_server/templates"
	"gnark_server/primitives"
	script "gnark_server/scripts"
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
func GenerationVkPk (){

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
	// erc20_join_split := templates.Erc20CircuitConfig{
	// 	TmNInputs: 2,
	// 	TmMOutputs:  2,
	// 	TmMerkleTreeDepth:8,
	// 	TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	// }

	// NOT covered by integration tests (test/01–04)
	// erc20Auditor_join_split_10_2 := templates.Erc20WithAuditorConfig{
	// 	TmNInputs: 10,
	// 	TmMOutputs: 2,
	// 	TmMerkleTreeDepth:8,
	// 	TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	// }

	// NOT covered by integration tests (test/01–04)
	// erc20_join_split_10_2 := templates.Erc20CircuitConfig{
	// 	TmNInputs: 10,
	// 	TmMOutputs:  2,
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

	// covered by test/01_v2_erc20_private_mint_test.go (TestV2Erc20OnChain_PrivateMint)
	private_mint_config := templates.PrivateMintConfig{
		TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	}

	// covered by test/03_v2_dvp_test.go and test/04_v2_dvp_deadline_test.go
	// H-2 fix: TmRange added — KEYS MUST BE REGENERATED after this change.
	dvp_initiator_config := templates.DvPInitiatorCircuitConfig{
		TmMerkleTreeDepth: 8,
		TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
	}

	// covered by test/03_v2_dvp_test.go and test/04_v2_dvp_deadline_test.go
	// H-2 fix: TmRange added — KEYS MUST BE REGENERATED after this change.
	dvp_destination_config := templates.DvPDestinationCircuitConfig{
		TmMerkleTreeDepth: 8,
		TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
	}

	script.SetupPrivateMint(private_mint_config, "PrivateMint")
	script.SetupDvPInitiator(dvp_initiator_config, "DvPInitiator")
	script.SetupDvPDestination(dvp_destination_config, "DvPDestination")
}

func main(){
	GenerationVkPk()
}
