package main

import(
	

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/constraint/solver"
	
	"gnark_server/templates"
	"gnark_server/primitives"
	script "gnark_server/scripts"
)


func GenerationVkPk (){
		
	solver.RegisterHint(primitives.ModHint)
	solver.RegisterHint(primitives.ERC155UniqueIdNative)
	solver.RegisterHint(primitives.PoseidonNative)
	solver.RegisterHint(primitives.PoseidonPrivateKeyNative)


	
	auctionbidConfig := templates.AuctionBidCircuitConfig{
		TmNInputs:    2,
		TmMOutputs:   2,
		TmNumOfIdParams:5,
		TmDepthMerkle: 8,
		TmRange: frontend.Variable("1000000000000000000000000000000000000"),
		TmGroupMerkleTreeDepth: 8,
	}	
	
	auctionBidAuditorConfig := templates.TmAuctionBidAuditorCircuitConfig{
		TmInputs:	2,
		TmOutputs:	2,   		
		TmNumOfIdParms:5,
		TmMerkleTreeDepth: 8,
		TmRange:frontend.Variable("1000000000000000000000000000000000000"),
		TmAssetGroupMerkleTreeDepth:8,

	}

	auctionInitAuditorConfig := templates.TmAuctionInitAuditorCircuitConfig{

		TmNumOfIdParms :5,
		TmMerkleTreeDepth:    8,		
		TmAssetGroupMerkleTreeDepth:8,

	}

	auctionInitConfig := templates.AuctionInitCircuitConfig{
			TmNumOfIdParms:5,
			TmMerkleTreeDepth:8,
			TmGroupMerkleTreeDepth:8,
	}

	auctionNotWinningConfig := templates.AuctionNotWinningCircuitConfig{
		TmRange:   frontend.Variable("1000000000000000000000000000000000000"),
	}

	privateOpeningConfig := templates.AuctionPrivateOpeningCircuitConfig{
		TmRange:   frontend.Variable("1000000000000000000000000000000000000"),
	}


	brokerRegistration := templates.BrokerageRegistrationConfig{
		TmNumOfInputs :		      2,
		TmMerkleTreeDepth: 	  8,
		TmGroupMerkleTreeDepth:  8,
		TmMaxPermittedCommissionRate:10,
		TmComissionRateDecimals:2,
		TmRange: "1000000000000000000000000000000000000",
	}

	erc1155Batch := templates.ERC1155NonFungibleCircuitConfig{
		TmNumOfTokens: 10,
		TmMerkleTreeDepth: 8,
		TmAssetGroupMerkleTreeDepth:8,
	}

	erc1155BatchNonFungible := templates.ERC1155NonFungibleWithAuditorCircuitConfig{
		TmNumOfTokens: 10,
		TmMerkleTreeDepth: 8,
		TmAssetGroupMerkleTree:8,
	}

	erc20_join_split := templates.Erc20CircuitConfig{
		TmNInputs: 2,
		TmMOutputs:  2,
		TmMerkleTreeDepth:8,
		TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	}

	erc20Auditor_join_split_10_2 := templates.Erc20WithAuditorConfig{
		TmNInputs: 10,
		TmMOutputs: 2,
		TmMerkleTreeDepth:8,
		TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	}

	erc20_join_split_10_2 := templates.Erc20CircuitConfig{
		TmNInputs: 10,
		TmMOutputs:  2,
		TmMerkleTreeDepth:8,
		TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	}

	// // erc20_join_split := templates.Erc20CircuitConfig{
	// // 	TmNInputs: 2,
	// // 	TmMOutputs:  2,
	// // 	TmMerkleTreeDepth:8,
	// // 	Range: frontend.Variable("1000000000000000000000000000000000000"),
	// // }

	erc20_auditor_join_split := templates.Erc20WithAuditorConfig{
		TmNInputs: 2,
		TmMOutputs:  2,
		TmMerkleTreeDepth:8,
		TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	}


	erc20_join_with_broker := templates.Erc20BrokerConfig{
		TmNInputs: 2,
		TmMOutputs: 3,
		TmMerkleTree:8,
		TmRange: frontend.Variable("1000000000000000000000000000000000000"),
		TmMaxComissionPercentage:10,
		TmComissionPercentageDecimals:0,
	}

	erc1155_join_split := templates.ERC1155FungibleCircuitConfig{
		TmNInputs: 2,
		TmMOutputs: 2,
		TmMerkleTreeDepth:8,
		TmAssetGroupMerkleTree: 8,
		TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	}

	erc1155_join_split_with_auditor := templates.ERC1155FungibleWithAuditorCircuitConfig{
		TmNInputs: 2,
		TmMOutputs: 2,
		TmMerkleTreeDepth:8,
		TmAssetGroupMerkleTree: 8,
		TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	}	

	erc1155_join_split_with_broker := templates.ERC1155FungibleWithBrokerCircuitConfig{
		NInputs: 2,
		MOutputs: 3,
		MerkleTreeDepth:8,
		AssetGroupMerkleTreeDepth: 8,
		MaxPermittedCommissionRate:10,
		ComissionRateDecimals:2,
		Range: frontend.Variable("1000000000000000000000000000000000000"),
	}

	legit_broker := templates.LegitBrokerCircuitConfig{}

	ownership_erc721_config := templates.Erc721CircuitConfig{
		TmNumOfTokens: 1,
		TmMerkleTreeDepth: 8,
	
	}

	ownership_erc721_config_with_auditor := templates.Erc721WithAuditorCircuitConfig{
		TmNumOfTokens: 1,
		TmMerkleTreeDepth: 8,
	}

	ownership_erc1155_Fungible_config := templates.ERC1155FungibleCircuitConfig{
		TmNInputs: 1,
		TmMOutputs: 1,
		TmMerkleTreeDepth:8,
		TmAssetGroupMerkleTree: 8,
		TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	}

	ownership_erc1155_Non_Fungible_config := templates.ERC1155NonFungibleCircuitConfig{
		TmNumOfTokens:1,
		TmMerkleTreeDepth:8,
		TmAssetGroupMerkleTreeDepth:8,
		}
	ownership_erc1155_Non_Fungible_config_with_auditor := templates.ERC1155NonFungibleWithAuditorCircuitConfig{
		TmNumOfTokens:1,
		TmMerkleTreeDepth:8,
		TmAssetGroupMerkleTree:8,
		}

	private_mint_config := templates.PrivateMintConfig{

	}

	script.SetupAuctionBid(auctionbidConfig,"AuctionBid")
	script.SetupAuctionBidAuditor(auctionBidAuditorConfig,"AuctionBidAuditor")
	script.SetupAuctionInit(auctionInitConfig,"AuctionInit")
	script.SetupAuctionInitAuditor(auctionInitAuditorConfig,"AuctionInitAuditor")
	
	script.SetupAuctionNotWinning(auctionNotWinningConfig,"AuctionNotWinning")
	script.SetupAuctionPrivateOpenning(privateOpeningConfig,"AuctionPrivateOpening")

	script.SetupBrokerRegistration(brokerRegistration, "BrokerRegistration")
	script.SetupBatchERC1155(erc1155Batch, "ERC1155Batch")
	script.SetupBatchErc1155WithAuditor(erc1155BatchNonFungible,"ERC1155BatchAuditor")
	
	script.SetupJoinSplitERC20(erc20_join_split,"JoinErc20")
	script.SetupJoinSplitERC20WithAuditor(erc20_auditor_join_split,"JoinERC20Auditor")

	script.SetupJoinSplitERC20(erc20_join_split_10_2,"JoinErc20_10_2")
	script.SetupJoinSplitERC20WithAuditor(erc20Auditor_join_split_10_2,"JoinERC20_10_2Auditor")

	script.SetupJoinSplitERC20WithBroker(erc20_join_with_broker,"JoinERC20WithBroker")
	
	script.SetupJoinSplitERC1155(erc1155_join_split, "JoiSplitERC1155")
	script.SetupJoinSplitERC1155Auditor(erc1155_join_split_with_auditor, "JoinSplitERC1155Auditor")

	script.SetupJoinSplitERC1155WithBroker(erc1155_join_split_with_broker, "JoiSplitERC1155WithBroker")

	script.SetupLegitBroker(legit_broker, "LegitBroker")
	
	script.SetupOwnershipERC721(ownership_erc721_config,"OwnershipERC721")
	script.SetupOwnershipERC721Auditor(ownership_erc721_config_with_auditor, "OwnershipERC721Auditor")
	
	script.SetupOwnershipERC1155Fungible(ownership_erc1155_Fungible_config, "OwnershipERC1155Fungible")
	
	script.SetupOwnershipERC1155NonFungible(ownership_erc1155_Non_Fungible_config, "OwnershipERC1155NonFungible")
	script.SetupOwnershipERC1155NonFungibleAuditor(ownership_erc1155_Non_Fungible_config_with_auditor,"OwnershipERC1155NonFungibleAuditor")
	
	script.SetupOwnershipERC1155NonFungibleAuditor(ownership_erc1155_Non_Fungible_config_with_auditor,"OwnershipERC1155NonFungibleAuditor")

	script.SetupPrivateMint(private_mint_config, "PrivateMint")
}

func main(){
	GenerationVkPk()
}
