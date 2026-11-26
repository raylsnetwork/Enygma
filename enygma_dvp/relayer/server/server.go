package server

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"enygma_dvp/relayer/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// This cap leaves headroom above the largest relay body measured in the DvP suite.
const maxRequestBodyBytes = 256 * 1024

type ipLimiterStore struct {
	mu sync.Mutex
	m  map[string]*rate.Limiter
}

func (s *ipLimiterStore) get(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.m[ip]; ok {
		return l
	}
	l := rate.NewLimiter(rate.Limit(5), 10)
	s.m[ip] = l
	return l
}

// rateLimitMiddleware enforces both the global and per-IP rate limits.
// Returns HTTP 429 if either limit is exceeded.
func rateLimitMiddleware(globalLimiter *rate.Limiter, ipLimiters *ipLimiterStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ipLimiters.get(c.ClientIP()).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "per-IP rate limit exceeded"})
			return
		}
		if !globalLimiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "global rate limit exceeded"})
			return
		}
		c.Next()
	}
}

// New creates the gin engine with all DVP relayer routes attached.
func New(cfg *config.Config) (*gin.Engine, error) {
	h, err := NewHandler(cfg)
	if err != nil {
		return nil, err
	}

	return newRouter(cfg, h)
}

func newRouter(cfg *config.Config, h *Handler) (*gin.Engine, error) {
	r := gin.Default()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("invalid RELAYER_TRUSTED_PROXIES: %w", err)
	}
	// Preserve the relayer limits of 20 requests/s globally (burst 30)
	// and 5 requests/s per IP (burst 10).
	globalLimiter := rate.NewLimiter(20, 30)
	ipLimiters := &ipLimiterStore{m: make(map[string]*rate.Limiter)}

	// /health and /relay/info are public — no auth required.
	r.GET("/health", h.health)
	r.GET("/relay/info", h.Info)

	// All /relay/* routes require a valid Bearer token and are rate-limited.
	relay := r.Group("/relay", bearerAuth(cfg.APIKey), rateLimitMiddleware(globalLimiter, ipLimiters), limitBody)
	{
		relay.POST("/payment", h.RelayPayment)
		relay.POST("/payment_relayer_fee", h.RelayPaymentRelayerFee)
		relay.POST("/payment_usdr_fee", h.RelayPaymentUsdrFee)
		relay.POST("/partial_withdraw", h.RelayPartialWithdraw)
		relay.POST("/swap", h.RelaySwap)
		relay.POST("/exchange", h.RelayExchange)
	}

	return r, nil
}

// limitBody rejects oversized bodies before JSON decoding, including trailing
// bytes that a decoder could leave unread after the first JSON value.
func limitBody(c *gin.Context) {
	body := http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
	data, err := io.ReadAll(body)
	if err != nil {
		status := http.StatusBadRequest
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		c.AbortWithStatusJSON(status, gin.H{"error": http.StatusText(status)})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Next()
}

// bearerAuth returns a gin middleware that enforces Bearer token authentication.
//
// Every request to a protected route must include:
//
//	Authorization: Bearer <token>
//
// Responds with 401 if the header is missing or malformed.
// Responds with 403 if the token format is valid but the value is wrong.
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

		token := parts[1]
		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "invalid token",
			})
			return
		}

		c.Next()
	}
}
