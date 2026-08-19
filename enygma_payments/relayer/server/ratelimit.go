package server

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// rateLimiter enforces a token-bucket limit per caller identity. Today's
// deployment has exactly one valid Bearer token (RELAYER_API_KEY), so keying
// on it is effectively a global limit — but it's keyed on caller identity,
// not IP or nothing, so the moment callers get distinct per-client API keys
// this becomes genuinely per-caller with no further change here.
type rateLimiter struct {
	rps   float64
	burst int

	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

func newRateLimiter(rps float64, burst int) *rateLimiter {
	return &rateLimiter{
		rps:      rps,
		burst:    burst,
		limiters: make(map[string]*rate.Limiter),
	}
}

// forKey returns the limiter for one caller identity, creating it on first use.
func (rl *rateLimiter) forKey(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	l, ok := rl.limiters[key]
	if !ok {
		l = rate.NewLimiter(rate.Limit(rl.rps), rl.burst)
		rl.limiters[key] = l
	}
	return l
}

// middleware returns gin middleware enforcing the limit. It must run after
// bearerAuth in the chain — it keys on the already-validated bearer token
// (stashed in the gin context under apiTokenContextKey, not the raw
// Authorization header — the header's "Bearer" scheme is case-insensitive
// but a raw-header key isn't, which would let one caller multiply its
// effective rate limit by varying the scheme's case), so an unauthenticated
// request never consumes budget from a real caller, and can't be used to
// exhaust another caller's allowance by guessing at keys (a wrong key is
// rejected by bearerAuth with 401/403 first).
//
// rps <= 0 disables rate limiting entirely (every call passes through).
func (rl *rateLimiter) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl.rps <= 0 {
			c.Next()
			return
		}
		key, _ := c.Get(apiTokenContextKey)
		keyStr, _ := key.(string)
		if !rl.forKey(keyStr).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded — slow down and retry",
			})
			return
		}
		c.Next()
	}
}
