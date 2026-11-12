package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"enygma_retail_payments/gnark_circuits/server/config"
)

func init() { gin.SetMode(gin.TestMode) }

// echoEngine puts h in front of a handler that reads the whole body.
func echoEngine(h ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(h...)
	r.POST("/echo", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusOK)
	})
	return r
}

// post sends body to /echo with an optional Authorization header.
func post(r http.Handler, body []byte, auth string) int {
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestMaxBodySize(t *testing.T) {
	r := echoEngine(maxBodySize(16))
	if got := post(r, bytes.Repeat([]byte("a"), 16), ""); got != http.StatusOK {
		t.Errorf("body at the limit: got %d, want 200", got)
	}
	if got := post(r, bytes.Repeat([]byte("a"), 17), ""); got != http.StatusRequestEntityTooLarge {
		t.Errorf("body over the limit: got %d, want 413", got)
	}
}

func TestBearerToken(t *testing.T) {
	r := echoEngine(bearerToken("s3cret"))
	cases := map[string]int{
		"":              http.StatusUnauthorized,
		"Bearer wrong":  http.StatusUnauthorized,
		"s3cret":        http.StatusUnauthorized,
		"Bearer s3cret": http.StatusOK,
	}
	for auth, want := range cases {
		if got := post(r, nil, auth); got != want {
			t.Errorf("Authorization %q: got %d, want %d", auth, got, want)
		}
	}
}

func TestBearerTokenOffWhenEmpty(t *testing.T) {
	if got := post(echoEngine(bearerToken("")), nil, ""); got != http.StatusOK {
		t.Errorf("no token configured: got %d, want 200", got)
	}
}

func TestConcurrencyLimit(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 4)
	r := gin.New()
	r.Use(concurrencyLimit(1, 50*time.Millisecond))
	r.POST("/echo", func(c *gin.Context) {
		started <- struct{}{}
		<-release
		c.Status(http.StatusOK)
	})

	var wg sync.WaitGroup
	wg.Add(1)
	var first int
	go func() { defer wg.Done(); first = post(r, nil, "") }()
	<-started

	second := make(chan int, 1)
	go func() { second <- post(r, nil, "") }()
	select {
	case got := <-second:
		if got != http.StatusServiceUnavailable {
			t.Errorf("second request while the slot is held: got %d, want 503", got)
		}
	case <-started:
		t.Error("second request ran while the only slot was held")
		close(release)
		wg.Wait()
		<-second
		return
	case <-time.After(5 * time.Second):
		t.Fatal("second request neither ran nor was turned away")
	}
	close(release)
	wg.Wait()
	if first != http.StatusOK {
		t.Errorf("first request: got %d, want 200", first)
	}
	if got := post(r, nil, ""); got != http.StatusOK {
		t.Errorf("request after the slot is freed: got %d, want 200", got)
	}
}

func TestLoadDefaultsToLoopback(t *testing.T) {
	t.Setenv("GNARK_BIND_HOST", "")
	t.Setenv("GNARK_AUTH_TOKEN", "")
	t.Setenv("GNARK_MAX_CONCURRENT_PROOFS", "")
	cfg := config.Load()
	if cfg.BindHost != "127.0.0.1" {
		t.Errorf("BindHost = %q, want 127.0.0.1", cfg.BindHost)
	}
	if cfg.AuthToken != "" || cfg.MaxConcurrentProofs < 1 {
		t.Errorf("AuthToken = %q, MaxConcurrentProofs = %d", cfg.AuthToken, cfg.MaxConcurrentProofs)
	}
}
