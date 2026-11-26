package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"

	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionBatch"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionBid"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionFinal"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionLock"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionRevert"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionWithdraw"
	"enygma_dvp_auctions/gnark_circuits/server/config"

	"enygma_gnark_shared/proofslots"
	"enygma_gnark_shared/proverhttp"
)

// The auction integration runs peaked at 25762 bytes on /proof/auctionBatch.
const maxRequestBodyBytes = 256 << 10

// NewServer wires the six auction proof endpoints.
func NewServer(cfg *config.Config) *gin.Engine {
	r := gin.Default()
	limitBody := func(c *gin.Context) {
		if err := proverhttp.BufferBody(c.Writer, c.Request, maxRequestBodyBytes); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Next()
	}

	// API key guard: if configured, every request must carry a matching
	// X-Gnark-Key header. This prevents any other local process from using
	// the proving keys without the shared secret.
	if cfg.APIKey != "" {
		want := sha256.Sum256([]byte(cfg.APIKey))
		r.Use(func(c *gin.Context) {
			got := sha256.Sum256([]byte(c.GetHeader("X-Gnark-Key")))
			if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			}
		})
	}

	slots := proofslots.New()
	limitProofs := func(c *gin.Context) {
		if !slots.Acquire(c.Request.Context(), proofslots.Wait) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "prover busy, try again"})
			return
		}
		defer slots.Release()
		c.Next()
	}
	proof := r.Group("/proof", limitProofs, limitBody)

	// Phase 0a — Bob locks his ERC-721 NFT for auction.
	proof.POST("/auctionLock", auctionLock.NewHandler(cfg.AuctionLockPk, cfg.AuctionLockVk))

	// Phase 0b — Alice submits a sealed USDC bid.
	proof.POST("/auctionBid", auctionBid.NewHandler(cfg.AuctionBidPk, cfg.AuctionBidVk))

	// Phase 0c — Alice withdraws a bid before the deadline.
	proof.POST("/auctionWithdraw", auctionWithdraw.NewHandler(cfg.AuctionWithdrawPk, cfg.AuctionWithdrawVk))

	// Phase 1 — Auctioneer opens up to 100 bids, proves batch winner.
	proof.POST("/auctionBatch", auctionBatch.NewHandler(cfg.AuctionBatchPk, cfg.AuctionBatchVk))

	// Phase 2 — Auctioneer finds overall winner across batches and settles.
	proof.POST("/auctionFinal", auctionFinal.NewHandler(cfg.AuctionFinalPk, cfg.AuctionFinalVk))

	// Revert path — Bob reclaims his locked NFT under a fresh commitment when
	// the auction is canceled (deadline passed with no settlement, or zero bids).
	proof.POST("/auctionRevert", auctionRevert.NewHandler(cfg.AuctionRevertPk, cfg.AuctionRevertVk))

	return r
}
