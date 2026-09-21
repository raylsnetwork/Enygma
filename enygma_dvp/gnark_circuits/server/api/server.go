package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gnark_server/server/config"

	serverutils "gnark_server/server/utils"

	// covered by test/01–04
	"gnark_server/server/circuits/privateMint"

	"gnark_server/server/circuits/dvpDestination"
	"gnark_server/server/circuits/dvpInit"

	// Payment-family circuits — dedicated Payment deployment, ported from
	// enygma_retail_payments. Covered by test/12.
	"gnark_server/server/circuits/payment"
	"gnark_server/server/circuits/payment2in"
	"gnark_server/server/circuits/paymentFee"
	"gnark_server/server/circuits/paymentRelayerFeePublic"
	"gnark_server/server/circuits/usdrFee"
)

func NewServer(cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// MEDIUM-4 fix: API key guard — if configured, every request must carry a
	// matching X-Gnark-Key header. This prevents any other local process from
	// using the proving keys without the shared secret.
	if cfg.APIKey != "" {
		r.Use(func(c *gin.Context) {
			if c.GetHeader("X-Gnark-Key") != cfg.APIKey {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			}
		})
	}

	r.POST("/proof/privateMint", privateMint.NewHandler(cfg.PrivateMintPk, cfg.PrivateMintVk))
	r.POST("/proof/dvpInitiator", dvpInit.NewHandler(cfg.DvPInitiatorPk, cfg.DvPInitiatorVk))
	r.POST("/proof/dvpDestination", dvpDestination.NewHandler(cfg.DvPDestinationPk, cfg.DvPDestinationVk))

	r.POST("/proof/payment", payment.NewHandler(cfg.PaymentPk, cfg.PaymentVk))
	r.POST("/proof/payment2in", payment2in.NewHandler(cfg.Payment2inPk, cfg.Payment2inVk))
	r.POST("/proof/paymentFee", paymentFee.NewHandler(cfg.PaymentFeePk, cfg.PaymentFeeVk))
	r.POST("/proof/paymentRelayerFeePublic", paymentRelayerFeePublic.NewHandler(cfg.PaymentRelayerFeePublicPk, cfg.PaymentRelayerFeePublicVk))
	r.POST("/proof/usdrFee", usdrFee.NewHandler(cfg.UsdrFeePk, cfg.UsdrFeeVk))

	// Merkle tree
	r.POST("/util/merkleStatus", serverutils.MerkleStatusHandler())
	r.POST("/util/merkleVault", serverutils.MerkleVaultHandler())

	return r
}
