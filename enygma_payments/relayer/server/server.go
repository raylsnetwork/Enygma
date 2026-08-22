package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"enygma_payments_relayer/config"

	"github.com/gin-gonic/gin"
)

// New creates the gin engine with all relayer routes attached, and returns
// the underlying Handler alongside it so the caller can call Handler.Close()
// on shutdown — releasing the persistent idempotency store's bbolt file
// lock cleanly instead of only on a hard exit.
//
// Route layout:
//
//	GET  /health         — liveness probe (public, no auth)
//	GET  /relay/info     — contract + relayer addresses (public, no auth)
//	POST /relay/transfer — relay Enygma.transfer() (requires Bearer token)
func New(cfg *config.Config) (*gin.Engine, *Handler, error) {
	h, err := NewHandler(cfg)
	if err != nil {
		return nil, nil, err
	}
	r := gin.Default()
	applyRoutes(r, cfg.APIKey, h)
	return r, h, nil
}

// NewWithHandler creates the gin engine with a pre-built Handler.
// Used in tests to inject mock backends without dialing a real chain.
func NewWithHandler(apiKey string, h *Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	applyRoutes(r, apiKey, h)
	return r
}

// apiTokenContextKey is where bearerAuth stashes the validated bearer token
// (parts[1] of "Bearer <token>") for downstream middleware — specifically
// the rate limiter, which needs to key on caller identity, not the raw
// Authorization header. Keying on the raw header would let one caller with
// the one valid token multiply their effective rate limit by varying the
// "Bearer" scheme's case ("Bearer"/"bearer"/"BEARER"/...), since bearerAuth
// itself accepts the scheme case-insensitively (strings.EqualFold) while
// the header string as a whole is case-sensitive.
const apiTokenContextKey = "relayer_api_token"

func applyRoutes(r *gin.Engine, apiKey string, h *Handler) {
	// Public endpoints — no authentication required.
	// /health is a pure liveness probe: no dependencies, so an RPC hiccup
	// can't make an orchestrator think the process itself is dead.
	// /health/ready actually checks whether the relayer can do its job.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/health/ready", h.Ready)
	r.GET("/relay/info", h.Info)

	// Protected endpoints — require Authorization: Bearer <RELAYER_API_KEY>,
	// then a per-caller rate limit (429 if exceeded) before any proof
	// verification, chain call, or gas is spent.
	rl := newRateLimiter(h.cfg.RateLimitRPS, h.cfg.RateLimitBurst)
	relay := r.Group("/relay", bearerAuth(apiKey), rl.middleware())
	{
		relay.POST("/transfer", h.RelayTransfer)
		relay.POST("/transfer_fee", h.RelayTransferFee)
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

		// Stash the validated token (not the raw header) for the rate
		// limiter to key on — see apiTokenContextKey.
		c.Set(apiTokenContextKey, parts[1])
		c.Next()
	}
}
