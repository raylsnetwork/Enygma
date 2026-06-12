package api

import (
	"github.com/gin-gonic/gin"

	"gnark_server/server/circuits/auctionBid"
	"gnark_server/server/circuits/auctionLock"
	"gnark_server/server/circuits/auctionNotWinning"
	"gnark_server/server/circuits/auctionSettle"
	"gnark_server/server/config"
)

// NewServer wires the four auction proof endpoints.
func NewServer(cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// Bob locks his ERC-721 NFT for auction.
	r.POST("/proof/auctionLock", auctionLock.NewHandler(cfg.AuctionLockPk, cfg.AuctionLockVk))

	// Alice submits a sealed USDC bid.
	r.POST("/proof/auctionBid", auctionBid.NewHandler(cfg.AuctionBidPk, cfg.AuctionBidVk))

	// Auctioneer finalizes the winning bid (publishes stWinningAmount).
	r.POST("/proof/auctionSettle", auctionSettle.NewHandler(cfg.AuctionSettlePk, cfg.AuctionSettleVk))

	// Losing bidder proves bid < winning amount to reclaim their locked USDC.
	r.POST("/proof/auctionNotWinning", auctionNotWinning.NewHandler(cfg.AuctionNotWinningPk, cfg.AuctionNotWinningVk))

	return r
}
