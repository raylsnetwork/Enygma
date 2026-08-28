package api

// TestRequireJSONContentType_*, TestMaxBodySize_* verify the M-08 HTTP
// hardening middleware (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, Medium/LIVE):
// no MaxBytesReader, no Content-Type restriction, and no request-body
// limit existed anywhere before — "every route is drivable from a browser
// page as a CORS-simple text/plain POST."
//
// These test the middleware directly against a minimal gin engine + dummy
// downstream handler, not the real /proof/* routes — those need real
// Groth16 keys loaded via config.Load()'s paths (~141MB, resolved
// relative to the server's own working directory), impractical to wire
// into a fast unit test and not what this middleware itself is
// responsible for validating.
//
// Run:
//
//	CC=/usr/bin/clang go test ./pkg/api/... -v

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func testEngine(bodyLimit int64) *gin.Engine {
	r := gin.New()
	r.Use(maxBodySize(bodyLimit))
	r.Use(requireJSONContentType())
	r.POST("/echo", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"received": len(body)})
	})
	return r
}

// ── requireJSONContentType ───────────────────────────────────────────────────

func TestRequireJSONContentType_Accepts(t *testing.T) {
	r := testEngine(1024)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(`{"a":1}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200: %s", w.Code, w.Body.String())
	}
}

func TestRequireJSONContentType_AcceptsWithCharset(t *testing.T) {
	r := testEngine(1024)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(`{"a":1}`)))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200: %s", w.Code, w.Body.String())
	}
}

// Fix M-08: text/plain is one of the three CORS-simple content types a
// cross-origin browser page can send without a preflight — this is
// exactly the "drivable from a browser page as a CORS-simple text/plain
// POST" vector the audit demonstrated.
func TestRequireJSONContentType_RejectsTextPlain(t *testing.T) {
	r := testEngine(1024)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(`{"a":1}`)))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("FAIL (M-08 regressed): got %d, want 415", w.Code)
	}
}

func TestRequireJSONContentType_RejectsFormEncoded(t *testing.T) {
	r := testEngine(1024)
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(`a=1`))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("FAIL (M-08 regressed): got %d, want 415", w.Code)
	}
}

func TestRequireJSONContentType_RejectsMissing(t *testing.T) {
	r := testEngine(1024)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(`{"a":1}`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("got %d, want 415", w.Code)
	}
}

// ── maxBodySize ───────────────────────────────────────────────────────────────

func TestMaxBodySize_AllowsUnderLimit(t *testing.T) {
	r := testEngine(1024)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(make([]byte, 100)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200: %s", w.Code, w.Body.String())
	}
}

// Fix M-08: "no MaxBytesReader anywhere" — an oversized body must be
// rejected (the downstream handler's io.ReadAll returns an error) rather
// than fully buffered into memory with no limit.
func TestMaxBodySize_RejectsOverLimit(t *testing.T) {
	r := testEngine(100)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(make([]byte, 10_000)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("FAIL (M-08 regressed): got %d, want 400 (body too large)", w.Code)
	}
}
