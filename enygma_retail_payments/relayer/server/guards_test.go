package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"enygma_retail_payments/relayer/config"
)

func TestTrustedProxies(t *testing.T) {
	for _, tc := range []struct {
		name        string
		proxies     string
		wantLimited bool
	}{
		{name: "empty", wantLimited: true},
		{name: "untrusted peer", proxies: "192.0.2.1, 192.0.2.2", wantLimited: true},
		{name: "trusted peer", proxies: "192.0.2.1, 198.51.100.0/24"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RELAYER_TRUSTED_PROXIES", tc.proxies)
			t.Setenv("RELAYER_PRIVATE_KEY", "unused")
			t.Setenv("RELAYER_API_KEY", "test")
			cfg, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			router, err := newRouter(cfg, &Handler{})
			if err != nil {
				t.Fatal(err)
			}
			limited := false
			for i := 1; i <= 15; i++ {
				req := httptest.NewRequest(http.MethodPost, "/relay/payment", strings.NewReader("{"))
				req.RemoteAddr = "198.51.100.1:1234"
				req.Header.Set("Authorization", "Bearer test")
				req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i))
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				switch {
				case w.Code == http.StatusBadRequest:
				case w.Code == http.StatusTooManyRequests && strings.Contains(w.Body.String(), "per-IP"):
					limited = true
				default:
					t.Fatalf("unexpected response %d %s", w.Code, w.Body)
				}
			}
			if limited != tc.wantLimited {
				t.Errorf("per-IP limit triggered = %v, want %v", limited, tc.wantLimited)
			}
		})
	}
}

func TestInvalidTrustedProxyConfig(t *testing.T) {
	_, err := newRouter(&config.Config{TrustedProxies: []string{"not-an-ip"}}, &Handler{})
	if err == nil || !strings.Contains(err.Error(), "RELAYER_TRUSTED_PROXIES") {
		t.Fatalf("invalid proxy configuration returned %v", err)
	}
}

func TestPerIPRejectionPreservesGlobalToken(t *testing.T) {
	const limitedPeer = "198.51.100.1"
	ips := &ipLimiterStore{m: map[string]*rate.Limiter{limitedPeer: rate.NewLimiter(0, 0)}}
	router := gin.New()
	router.GET("/relay", rateLimitMiddleware(rate.NewLimiter(0, 1), ips), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	for _, step := range []struct {
		peer    string
		status  int
		message string
	}{
		{limitedPeer, http.StatusTooManyRequests, "per-IP"},
		{"198.51.100.2", http.StatusNoContent, ""},
		{"198.51.100.3", http.StatusTooManyRequests, "global"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/relay", nil)
		req.RemoteAddr = step.peer + ":1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != step.status || !strings.Contains(w.Body.String(), step.message) {
			t.Fatalf("peer %s: got %d %s, want %d %s", step.peer, w.Code, w.Body, step.status, step.message)
		}
	}
}

func TestRelayBodyLimit(t *testing.T) {
	router, err := newRouter(&config.Config{APIKey: "test"}, &Handler{})
	if err != nil {
		t.Fatal(err)
	}

	const capBytes = 256 * 1024
	for _, tc := range []struct {
		name   string
		body   io.Reader
		status int
		detail string
	}{
		{"at limit", strings.NewReader("{}" + strings.Repeat(" ", capBytes-2)), http.StatusBadRequest, "RelayPaymentRequest.VaultId"},
		{"over limit", strings.NewReader("{}" + strings.Repeat(" ", capBytes-1)), http.StatusRequestEntityTooLarge, "Request Entity Too Large"},
		{"read failure", iotest.ErrReader(errors.New("read failed")), http.StatusBadRequest, "Bad Request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/relay/payment", tc.body)
			req.Header.Set("Authorization", "Bearer test")
			req.ContentLength = -1 // The limit must also apply to a streamed body.
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d", w.Code, tc.status)
			}
			if !strings.Contains(w.Body.String(), tc.detail) {
				t.Fatalf("got response %s, want detail %q", w.Body, tc.detail)
			}
		})
	}
}
