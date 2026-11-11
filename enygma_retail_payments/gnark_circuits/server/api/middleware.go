package api

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// maxRequestBodyBytes bounds every request body. A proof request is a few KB
// of decimal-string fields; 512 KiB leaves ample headroom.
const maxRequestBodyBytes = 512 * 1024

// proofSlotWait is how long a request waits for a free proving slot before
// the server answers 503.
const proofSlotWait = 30 * time.Second

// maxBodySize caps the request body at n bytes, so binding fails on anything
// larger instead of buffering it.
func maxBodySize(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}

// bearerToken rejects requests whose Authorization header is not
// "Bearer <token>". An empty token turns the check off.
func bearerToken(token string) gin.HandlerFunc {
	want := []byte("Bearer " + token)
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		got := []byte(c.GetHeader("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or wrong bearer token"})
			return
		}
		c.Next()
	}
}

// concurrencyLimit lets at most n requests run at once; the rest wait up to
// wait for a slot, then get 503.
func concurrencyLimit(n int, wait time.Duration) gin.HandlerFunc {
	slots := make(chan struct{}, n)
	return func(c *gin.Context) {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case slots <- struct{}{}:
		case <-timer.C:
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "prover busy, try again"})
			return
		case <-c.Request.Context().Done():
			c.Abort()
			return
		}
		defer func() { <-slots }()
		c.Next()
	}
}
