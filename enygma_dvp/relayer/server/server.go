package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"

	"enygma_dvp_relayer/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// globalLimiter caps total relay throughput across all clients (20 req/s, burst 30).
// Protects the relayer's gas budget from being drained by a flood of valid requests.
var globalLimiter = rate.NewLimiter(rate.Limit(20), 30)

// ipLimiters holds per-IP token buckets (5 req/s per IP, burst 10).
// Note: the map grows unboundedly; in production replace with an LRU+TTL cache.
var ipLimiters = &ipLimiterStore{m: make(map[string]*rate.Limiter)}

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
func rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !globalLimiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "global rate limit exceeded"})
			return
		}
		if !ipLimiters.get(c.ClientIP()).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "per-IP rate limit exceeded"})
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

	r := gin.Default()

	// /health is public — no auth required.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// All /relay/* routes require a valid Bearer token and are rate-limited.
	relay := r.Group("/relay", bearerAuth(cfg.APIKey), rateLimitMiddleware())
	{
		relay.POST("/payment",  h.RelayPayment)
		relay.POST("/swap",     h.RelaySwap)
		relay.POST("/exchange", h.RelayExchange)
	}

	return r, nil
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
