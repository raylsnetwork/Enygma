package config


type Config struct {
    Port     string
    OwnershipERC721Pk  string
    OwnershipERC721Vk  string
    JoinSplitERC20Pk   string
    JoinSplitERC20Vk   string
    JoinSplitERC20_10_2Pk string
    JoinSplitERC20_10_2Vk string
    AuctionInitPk      string
    AuctionInitVk      string
    AuctionInitAuditorPk      string
    AuctionInitAuditorVk      string
    AuctionBidPk       string
    AuctionBidVk       string
    AuctionBidAuditorPk string
    AuctionBidAuditorVk string
    AuctionPrivateOpeningPk string
    AuctionPrivateOpeningVk string
    AuctionNotOpeningPk string
    AuctionNotOpeningVk string
    BrokerRegistrationPk string
    BrokerRegistrationVk string
    ERC1155FungiblePk string
    ERC1155FungibleVk string
    ERC1155FungibleAuditorPk string
    ERC1155FungibleAuditorVk string
    ERC1155FungibleWithBrokerPk string
    ERC1155FungibleWithBrokerVk string
    ERC1155NonFungiblePk    string
    ERC1155NonFungibleVk    string
    ERC1155NonFungibleAuditorPk    string
    ERC1155NonFungibleAuditorVk    string
    LegitBrokerPk string
    LegitBrokerVk string
    PrivateMintPk string
    PrivateMintVk string
	
}

func Load() *Config {
    return &Config{
        Port:    	   "8081",
        OwnershipERC721Pk:      "./scripts/keys/OwnershipERC721PK.key",
        OwnershipERC721Vk: 	    "./scripts/keys/OwnershipERC721VK.key",
        JoinSplitERC20Pk:       "./scripts/keys/JoinErc20PK.key",
        JoinSplitERC20Vk:       "./scripts/keys/JoinErc20VK.key",
        JoinSplitERC20_10_2Pk:       "./scripts/keys/JoinErc20_10_2PK.key",
        JoinSplitERC20_10_2Vk:       "./scripts/keys/JoinErc20_10_2VK.key",
        AuctionInitPk: 	        "./scripts/keys/AuctionInitPK.key",
        AuctionInitVk: 	        "./scripts/keys/AuctionInitVK.key",
        AuctionInitAuditorPk:   "./scripts/keys/AuctionInitAuditorPK.key",
        AuctionInitAuditorVk:   "./scripts/keys/AuctionInitAuditorVK.key",
        AuctionBidPk: 	        "./scripts/keys/AuctionBidPK.key",
        AuctionBidVk: 	        "./scripts/keys/AuctionBidVK.key",
        AuctionBidAuditorPk:    "./scripts/keys/AuctionBidAuditorPK.key",
        AuctionBidAuditorVk:    "./scripts/keys/AuctionBidAuditorVK.key",
        AuctionPrivateOpeningPk: "./scripts/keys/AuctionPrivateOpeningPK.key",
        AuctionPrivateOpeningVk: "./scripts/keys/AuctionPrivateOpeningVK.key",
        AuctionNotOpeningPk:   "./scripts/keys/AuctionNotWinningPK.key",
        AuctionNotOpeningVk:   "./scripts/keys/AuctionNotWinningVK.key",
        BrokerRegistrationPk: "./scripts/keys/BrokerRegistrationPK.key",
        BrokerRegistrationVk: "./scripts/keys/BrokerRegistrationVK.key",
        ERC1155FungiblePk:    "./scripts/keys/JoiSplitERC1155PK.key",
        ERC1155FungibleVk:    "./scripts/keys/JoiSplitERC1155VK.key",  
        ERC1155FungibleAuditorPk: "./scripts/keys/JoinSplitERC1155AuditorPK.key",
        ERC1155FungibleAuditorVk: "./scripts/keys/JoinSplitERC1155AuditorVK.key",
        ERC1155FungibleWithBrokerPk: "./scripts/keys/JoiSplitERC1155WithBrokerPK.key",
        ERC1155FungibleWithBrokerVk: "./scripts/keys/JoiSplitERC1155WithBrokerVK.key",
        ERC1155NonFungiblePk: "./scripts/keys/OwnershipERC1155NonFungiblePK.key",
        ERC1155NonFungibleVk: "./scripts/keys/OwnershipERC1155NonFungibleVK.key",
        ERC1155NonFungibleAuditorPk: "./scripts/keys/OwnershipERC1155NonFungibleAuditorPK.key",
        ERC1155NonFungibleAuditorVk: "./scripts/keys/OwnershipERC1155NonFungibleAuditorVK.key",
        LegitBrokerPk:  "./scripts/keys/LegitBrokerPK.key",
        LegitBrokerVk:  "./scripts/keys/LegitBrokerVK.key",
        PrivateMintPk: "./scripts/keys/PrivateMintPK.key",
        PrivateMintVk: "./scripts/keys/PrivateMintVK.key",
    }
}

