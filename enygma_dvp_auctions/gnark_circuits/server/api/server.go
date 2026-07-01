package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gnark_server/server/circuits/auctionBatch"
	"gnark_server/server/circuits/auctionBid"
	"gnark_server/server/circuits/auctionFinal"
	"gnark_server/server/circuits/auctionLock"
	"gnark_server/server/circuits/auctionRevert"
	"gnark_server/server/circuits/auctionWithdraw"
	"gnark_server/server/config"
)

// NewServer wires the six auction proof endpoints.
func NewServer(cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// API key guard: if configured, every request must carry a matching
	// X-Gnark-Key header. This prevents any other local process from using
	// the proving keys without the shared secret.
	if cfg.APIKey != "" {
		r.Use(func(c *gin.Context) {
			if c.GetHeader("X-Gnark-Key") != cfg.APIKey {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			}
		})
	}

	// Phase 0a — Bob locks his ERC-721 NFT for auction.
	r.POST("/proof/auctionLock", auctionLock.NewHandler(cfg.AuctionLockPk, cfg.AuctionLockVk))

	// Phase 0b — Alice submits a sealed USDC bid.
	r.POST("/proof/auctionBid", auctionBid.NewHandler(cfg.AuctionBidPk, cfg.AuctionBidVk))

	// Phase 0c — Alice withdraws a bid before the deadline.
	r.POST("/proof/auctionWithdraw", auctionWithdraw.NewHandler(cfg.AuctionWithdrawPk, cfg.AuctionWithdrawVk))

	// Phase 1 — Auctioneer opens up to 100 bids, proves batch winner.
	r.POST("/proof/auctionBatch", auctionBatch.NewHandler(cfg.AuctionBatchPk, cfg.AuctionBatchVk))

	// Phase 2 — Auctioneer finds overall winner across batches and settles.
	r.POST("/proof/auctionFinal", auctionFinal.NewHandler(cfg.AuctionFinalPk, cfg.AuctionFinalVk))

	// Revert path — Bob reclaims his locked NFT under a fresh commitment when
	// the auction is canceled (deadline passed with no settlement, or zero bids).
	r.POST("/proof/auctionRevert", auctionRevert.NewHandler(cfg.AuctionRevertPk, cfg.AuctionRevertVk))

	return r
}
