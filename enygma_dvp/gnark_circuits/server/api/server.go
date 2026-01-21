package api

import (
    "github.com/gin-gonic/gin"
    "gnark_server/server/config"
    "gnark_server/server/circuits/ownershipERC721"
    "gnark_server/server/circuits/joinSplitERC20"
    "gnark_server/server/circuits/joinSplitERC20_10_2"
    "gnark_server/server/circuits/auctionInit"
    "gnark_server/server/circuits/auctionInitAuditor"
    "gnark_server/server/circuits/auctionBid"
    "gnark_server/server/circuits/auctionBidAuditor"
    "gnark_server/server/circuits/auctionPrivateOpening"
    "gnark_server/server/circuits/auctionNotWinning"
    "gnark_server/server/circuits/brokerRegistration"
    "gnark_server/server/circuits/erc1155FungbileWithBroker"
    "gnark_server/server/circuits/erc1155NonFungible"
    "gnark_server/server/circuits/erc1155NonFungibleAuditor"
    "gnark_server/server/circuits/legitBroker"
    "gnark_server/server/circuits/erc1155Fungible"
    "gnark_server/server/circuits/erc1155FungibleAuditor"
    "gnark_server/server/circuits/privateMint"


)


func NewServer(cfg *config.Config) *gin.Engine {
    r := gin.Default()
 
    r.POST("/proof/ownershipERC721", ownershipERC721.NewHandler(cfg.OwnershipERC721Pk, cfg.OwnershipERC721Vk))
    r.POST("/proof/joinSplitERC20",  joinSplitERC20.NewHandler(cfg.JoinSplitERC20Pk,    cfg.JoinSplitERC20Vk))
    r.POST("/proof/joinSplitERC20_10_2",  joinSplitERC20_10_2.NewHandler(cfg.JoinSplitERC20_10_2Pk,    cfg.JoinSplitERC20_10_2Vk))
    r.POST("/proof/auctionInit",    auctionInit.NewHandler(cfg.AuctionInitPk, cfg.AuctionInitVk))
    r.POST("/proof/auctionInitAuditor",   auctionInitAuditor.NewHandler(cfg.AuctionInitAuditorPk, cfg.AuctionInitAuditorVk))
    r.POST("/proof/auctionBid",  acutionBid.NewHandler(cfg.AuctionBidPk, cfg.AuctionBidVk))
    r.POST("/proof/auctionBidAuditor", auctionBidAuditor.NewHandler(cfg.AuctionBidAuditorPk, cfg.AuctionBidAuditorVk))
    r.POST("/proof/auctionPrivateOpening", auctionPrivateOpening.NewHandler(cfg.AuctionPrivateOpeningPk, cfg.AuctionPrivateOpeningVk))
    r.POST("/proof/auctionNotWinning", auctionNotWinning.NewHandler(cfg.AuctionNotOpeningPk, cfg.AuctionNotOpeningVk))
    r.POST("/proof/brokerRegistration", brokerRegistration.NewHandler(cfg.BrokerRegistrationPk, cfg.BrokerRegistrationVk))
    r.POST("./proof/erc155Fungible",erc1155Fungible.NewHandler(cfg.ERC1155FungiblePk, cfg.ERC1155FungibleVk) )
    r.POST("/proof/erc1155FungibleWithBroker",  erc1155FungibleWithBroker.NewHandler(cfg.ERC1155FungibleWithBrokerPk, cfg.ERC1155FungibleWithBrokerVk))
    r.POST("/proof/erc1155NonFungible",erc1155NonFungible.NewHandler(cfg.ERC1155NonFungiblePk, cfg.ERC1155NonFungibleVk))
    r.POST("/proof/erc1155NonFungibleAuditor",erc1155NonFungibleAuditor.NewHandler(cfg.ERC1155NonFungibleAuditorPk, cfg.ERC1155NonFungibleAuditorVk))
    r.POST("/proof/legitBroker",legitBroker.NewHandler(cfg.LegitBrokerPk, cfg.LegitBrokerVk))
    r.POST("/proof/erc1155FungibleAuditor", erc1155FungibleAuditor.NewHandler(cfg.ERC1155FungibleAuditorPk, cfg.ERC1155FungibleAuditorVk))
    r.POST("/proof/privateMint", privateMint.NewHandler(cfg.PrivateMintPk, cfg.PrivateMintVk))
    return r
}

