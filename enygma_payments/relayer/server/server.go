package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"enygma_payments_relayer/config"

	"github.com/gin-gonic/gin"
)

// New creates the gin engine with all relayer routes attached.
//
// Route layout:
//
//	GET  /health         — liveness probe (public, no auth)
//	GET  /relay/info     — contract + relayer addresses (public, no auth)
//	POST /relay/transfer — relay Enygma.transfer() (requires Bearer token)
func New(cfg *config.Config) (*gin.Engine, error) {
	h, err := NewHandler(cfg)
	if err != nil {
		return nil, err
	}
	r := gin.Default()
	applyRoutes(r, cfg.APIKey, h)
	return r, nil
}

// NewWithHandler creates the gin engine with a pre-built Handler.
// Used in tests to inject mock backends without dialing a real chain.
func NewWithHandler(apiKey string, h *Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	applyRoutes(r, apiKey, h)
	return r
}

func applyRoutes(r *gin.Engine, apiKey string, h *Handler) {
	// Public endpoints — no authentication required.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/relay/info", h.Info)

	// Protected endpoints — require Authorization: Bearer <RELAYER_API_KEY>.
	relay := r.Group("/relay", bearerAuth(apiKey))
	{
		relay.POST("/transfer", h.RelayTransfer)
	}
}

// bearerAuth returns a gin middleware enforcing Bearer token authentication.
//
// Responds 401 if the Authorization header is missing or malformed.
// Responds 403 if the token value does not match (constant-time comparison
// to prevent timing attacks).
func bearerAuth(expectedToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header is required",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header must use Bearer scheme: Authorization: Bearer <token>",
			})
			return
		}

		if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expectedToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "invalid API key",
			})
			return
		}

		c.Next()
	}
}
