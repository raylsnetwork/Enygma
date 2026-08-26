package enygma

// TestEnygmaRequest_Binding_* verifies the M-08 fix to EnygmaRequest's
// gin binding tags (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, Medium/LIVE,
// "remote panics" sub-finding): tags used to permit 1-6 element arrays
// ("min=1,max=6") while the handler always indexes [0..NCommitment-1]
// (NCommitment==6) — a 2-element PublicKey passed binding and then
// panicked on the first out-of-range index (recovered by gin.Recovery
// into a 500, but still an uncontrolled remote panic on the cheap,
// pre-proving path). Tags are now "len=6" (exactly what the handler
// requires), and FingerPrintofSharedSecrets additionally gets
// "dive,len=6" on its inner slices, which it previously had no
// per-element validation on at all.
//
// These exercise gin's binding validator directly (via ShouldBindJSON on
// a bare gin.Context), not the full HTTP handler — no proving keys needed.
//
// Run:
//
//	CC=/usr/bin/clang go test ./pkg/circuits/enygma/... -run TestEnygmaRequest_Binding -v

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// validEnygmaRequestJSON returns a well-formed request body: every slice
// exactly 6 elements (or 6x6 for the fingerprint matrix), matching what
// the handler actually requires.
func validEnygmaRequestJSON() map[string]any {
	six := []string{"1", "2", "3", "4", "5", "6"}
	sixPairs := make([][2]string, 6)
	fp := make([][]string, 6)
	for i := range fp {
		fp[i] = []string{"1", "2", "3", "4", "5", "6"}
	}
	for i := range sixPairs {
		sixPairs[i] = [2]string{"1", "2"}
	}
	return map[string]any{
		"fingerprint_shared_secrets":   fp,
		"public_keys":                  six,
		"previous_commits":             sixPairs,
		"tx_commits":                   sixPairs,
		"block_number":                 "1",
		"anonymity_set":                six,
		"message_tags":                 six,
		"nullifier":                    "1",
		"sender_id":                    "1",
		"shared_secrets":               six,
		"secret_key":                   "1",
		"previous_sender_balance":      "1",
		"previous_sender_random_value": "1",
		"tx_values":                    six,
		"tx_random_values":             six,
		"sender_tx_value":              "1",
		"domain_id":                    "1", // Fix L-01
	}
}

func bindEnygmaRequest(t *testing.T, body map[string]any) (int, error) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/proof/enygma", bytes.NewReader(data))
	c.Request.Header.Set("Content-Type", "application/json")

	var req EnygmaRequest
	bindErr := c.ShouldBindJSON(&req)
	return len(req.PublicKey), bindErr
}

func TestEnygmaRequest_Binding_ValidPasses(t *testing.T) {
	n, err := bindEnygmaRequest(t, validEnygmaRequestJSON())
	if err != nil {
		t.Fatalf("unexpected bind error: %v", err)
	}
	if n != 6 {
		t.Errorf("PublicKey length: got %d, want 6", n)
	}
}

// Fix M-08: this is the exact class of request that used to pass binding
// ("min=1,max=6" allows 2) and then panic in the handler on
// request.PublicKey[2] (index out of range, since the loop always runs
// to NCommitment-1 == 5).
func TestEnygmaRequest_Binding_ShortPublicKeyRejected(t *testing.T) {
	body := validEnygmaRequestJSON()
	body["public_keys"] = []string{"1", "2"} // was accepted pre-fix
	if _, err := bindEnygmaRequest(t, body); err == nil {
		t.Fatal("FAIL (M-08 regressed): a 2-element public_keys array passed binding (handler would panic indexing [2..5])")
	}
}

func TestEnygmaRequest_Binding_LongPublicKeyRejected(t *testing.T) {
	body := validEnygmaRequestJSON()
	body["public_keys"] = []string{"1", "2", "3", "4", "5", "6", "7"}
	if _, err := bindEnygmaRequest(t, body); err == nil {
		t.Fatal("expected a 7-element public_keys array to be rejected")
	}
}

// FingerPrintofSharedSecrets had NO per-element (dive) validation at all
// before this fix — an inner slice of the wrong length passed binding
// silently.
func TestEnygmaRequest_Binding_ShortFingerprintRowRejected(t *testing.T) {
	body := validEnygmaRequestJSON()
	fp := body["fingerprint_shared_secrets"].([][]string)
	fp[2] = []string{"1", "2"} // one row too short
	body["fingerprint_shared_secrets"] = fp
	if _, err := bindEnygmaRequest(t, body); err == nil {
		t.Fatal("FAIL (M-08 regressed): a short inner fingerprint row passed binding")
	}
}

func TestEnygmaRequest_Binding_ShortPreviousCommitsArrayRejected(t *testing.T) {
	body := validEnygmaRequestJSON()
	// The outer array's own length constraint (len=6, unaffected by the
	// [2]string-per-entry question below) must still reject a short list.
	body["previous_commits"] = [][2]string{{"1", "2"}, {"1", "2"}}
	if _, err := bindEnygmaRequest(t, body); err == nil {
		t.Fatal("expected a 2-entry previous_commits array to be rejected")
	}
}
