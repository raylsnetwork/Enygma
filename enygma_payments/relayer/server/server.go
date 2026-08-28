package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"enygma_payments_relayer/config"

	"github.com/gin-gonic/gin"
)

// maxRequestBodyBytes bounds every request body read by the relayer.
//
// Fix M-09: previously nothing limited request body size — only
// len(req.PublicSignal) > 80 was checked, and only after the whole body had
// already been read into memory. A large body from an unauthenticated or
// authenticated caller could exhaust memory before that check ever ran.
// 256KiB is generous headroom over a genuine request (proof: 8 decimals,
// publicSignal: <=80 decimals, commitments: 6 pairs, kIndex: 6 ints — a few
// KB at most) while still bounding the worst case tightly.
const maxRequestBodyBytes = 256 * 1024

// New creates the gin engine with all relayer routes attached.
//
// Route layout:
//
//	GET  /health          — liveness + chain/balance probe (public, no auth)
//	GET  /relay/info      — contract + relayer addresses (public, no auth)
//	POST /relay/transfer  — relay Enygma.transfer() (requires Bearer token)
//	POST /relay/transfer_fee — relay Enygma.transferWithFee() (requires Bearer token)
func New(cfg *config.Config) (*gin.Engine, error) {
	h, err := NewHandler(cfg)
	if err != nil {
		return nil, err
	}
	r := gin.Default()
	applyRoutes(r, cfg.APIKeys, h)
	return r, nil
}

// NewWithHandler creates the gin engine with a pre-built Handler.
// Used in tests to inject mock backends without dialing a real chain.
//
// apiKeys maps each accepted Bearer token to the bank identifier it was
// issued to (Fix H-06) — see config.Config.APIKeys.
func NewWithHandler(apiKeys map[string]string, h *Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	applyRoutes(r, apiKeys, h)
	return r
}

func applyRoutes(r *gin.Engine, apiKeys map[string]string, h *Handler) {
	// Fix M-09: bound every request body, not just PublicSignal's length
	// after the fact.
	r.Use(maxBodySize(maxRequestBodyBytes))

	// Public endpoints — no authentication required.
	r.GET("/health", h.Health)
	r.GET("/relay/info", h.Info)

	// Protected endpoints — require Authorization: Bearer <token>.
	relay := r.Group("/relay", bearerAuth(apiKeys))
	{
		relay.POST("/transfer", h.RelayTransfer)
		relay.POST("/transfer_fee", h.RelayTransferFee)
	}
}

// maxBodySize wraps every request body in http.MaxBytesReader so a caller
// cannot force the server to buffer an unbounded body into memory before
// any handler-level validation runs (Fix M-09).
func maxBodySize(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}

// bearerAuth returns a gin middleware enforcing per-bank Bearer token
// authentication (Fix H-06).
//
// Responds 401 if the Authorization header is missing or malformed.
// Responds 403 if the token does not match any issued credential.
// On success, attaches the token's bank identifier to the request context
// under "bankID" so handlers can attribute logs to the calling bank.
func bearerAuth(apiKeys map[string]string) gin.HandlerFunc {
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
		token := parts[1]

		// Constant-time lookup across every issued token: comparing against
		// each candidate with subtle.ConstantTimeCompare (rather than a
		// plain map index, which short-circuits on the first differing
		// byte) keeps the check's timing independent of which token, if
		// any, is closest to a match.
		var bankID string
		var ok bool
		for tok, id := range apiKeys {
			if subtle.ConstantTimeCompare([]byte(token), []byte(tok)) == 1 {
				bankID, ok = id, true
			}
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "invalid API key",
			})
			return
		}

		c.Set("bankID", bankID)
		c.Next()
	}
}
