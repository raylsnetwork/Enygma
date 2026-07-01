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

	// Merkle tree
	r.POST("/util/merkleStatus", serverutils.MerkleStatusHandler())
	r.POST("/util/merkleVault", serverutils.MerkleVaultHandler())

	return r
}

