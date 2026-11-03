package server

import (
	"bytes"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestValidateOffer(t *testing.T) {
	c1 := make([]byte, agreementC1Size)
	if err := validateOffer(c1, []byte("ct"), []byte{0, 4}); err != nil {
		t.Fatalf("valid offer rejected: %v", err)
	}
	cases := map[string]struct{ c1, c2, bm []byte }{
		"short c1":     {c1[:100], nil, []byte{1}},
		"large c2":     {c1, make([]byte, agreementCtxtMax+1), []byte{1}},
		"empty bitmap": {c1, nil, []byte{0, 0}},
		"no bitmap":    {c1, nil, nil},
		"large bitmap": {c1, nil, append(make([]byte, agreementBitmapMax), 1)},
	}
	for name, tc := range cases {
		if validateOffer(tc.c1, tc.c2, tc.bm) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseReply(t *testing.T) {
	tag := "0x" + strings.Repeat("ab", 32)
	tags, ctxt, err := parseReply(&RelayReplyRequest{StartBlock: 7, Tags: []string{tag, tag}, Ctxt: "0x0102"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[1][31] != 0xab || !bytes.Equal(ctxt, []byte{1, 2}) {
		t.Fatalf("parsed %x / %x", tags, ctxt)
	}
	bad := []*RelayReplyRequest{
		{Tags: nil, Ctxt: "00"},
		{Tags: make([]string, agreementTagWindowMax+1), Ctxt: "00"},
		{Tags: []string{"0x1234"}, Ctxt: "00"},
		{Tags: []string{"zz"}, Ctxt: "00"},
		{Tags: []string{tag}, Ctxt: hex.EncodeToString(make([]byte, agreementCtxtMax+1))},
	}
	for i, r := range bad {
		if _, _, err := parseReply(r); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}

// Without a configured registry the routes answer 503 instead of sending.
func TestAgreementRoutesUnavailableWithoutRegistry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	for _, route := range []struct {
		path string
		fn   gin.HandlerFunc
	}{{"/relay/offer", h.RelayOffer}, {"/relay/reply", h.RelayReply}} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(`{}`))
		route.fn(c)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status %d, want 503", route.path, w.Code)
		}
	}
}
