package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"

	"enygma_dvp/gnark_circuits/server/config"
	"github.com/gin-gonic/gin"

	serverutils "enygma_dvp/gnark_circuits/server/utils"

	// covered by test/01–04
	"enygma_dvp/gnark_circuits/server/circuits/privateMint"

	"enygma_dvp/gnark_circuits/server/circuits/dvpDestination"
	"enygma_dvp/gnark_circuits/server/circuits/dvpInit"

	// Payment-family circuits — dedicated Payment deployment, ported from
	// enygma_retail_payments. Covered by test/12.
	"enygma_dvp/gnark_circuits/server/circuits/payment"
	"enygma_dvp/gnark_circuits/server/circuits/payment2in"
	"enygma_dvp/gnark_circuits/server/circuits/paymentFee"
	"enygma_dvp/gnark_circuits/server/circuits/paymentRelayerFeePublic"
	"enygma_dvp/gnark_circuits/server/circuits/usdrFee"

	"enygma_gnark_shared/proofslots"
	"enygma_gnark_shared/proverhttp"
)

// The DvP integration suite peaked at 2702 bytes on /proof/payment2in.
const maxRequestBodyBytes = 64 << 10

func NewServer(cfg *config.Config) *gin.Engine {
	r := gin.Default()
	limitBody := func(c *gin.Context) {
		if err := proverhttp.BufferBody(c.Writer, c.Request, maxRequestBodyBytes); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Next()
	}

	// MEDIUM-4 fix: API key guard — if configured, every request must carry a
	// matching X-Gnark-Key header. This prevents any other local process from
	// using the proving keys without the shared secret.
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

	proof.POST("/privateMint", privateMint.NewHandler(cfg.PrivateMintPk, cfg.PrivateMintVk))
	proof.POST("/dvpInitiator", dvpInit.NewHandler(cfg.DvPInitiatorPk, cfg.DvPInitiatorVk))
	proof.POST("/dvpDestination", dvpDestination.NewHandler(cfg.DvPDestinationPk, cfg.DvPDestinationVk))

	proof.POST("/payment", payment.NewHandler(cfg.PaymentPk, cfg.PaymentVk))
	proof.POST("/payment2in", payment2in.NewHandler(cfg.Payment2inPk, cfg.Payment2inVk))
	proof.POST("/paymentFee", paymentFee.NewHandler(cfg.PaymentFeePk, cfg.PaymentFeeVk))
	proof.POST("/paymentRelayerFeePublic", paymentRelayerFeePublic.NewHandler(cfg.PaymentRelayerFeePublicPk, cfg.PaymentRelayerFeePublicVk))
	proof.POST("/usdrFee", usdrFee.NewHandler(cfg.UsdrFeePk, cfg.UsdrFeeVk))

	// Merkle tree
	util := r.Group("/util", limitBody)
	util.POST("/merkleStatus", serverutils.MerkleStatusHandler())
	util.POST("/merkleVault", serverutils.MerkleVaultHandler())

	return r
}
