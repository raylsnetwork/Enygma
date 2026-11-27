package server

// The tag and channel routes refuse, before signing, what TagRegistry and
// TagChannelRegistry would reject: the relayer pays the gas of a transaction
// that reverts (#480).

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// tagRouteRequests is one well-formed request per route.
var tagRouteRequests = map[string]interface{}{
	"/relay/tag":     RelayTagRequest{Tag: "0x" + strings.Repeat("e5", 32), Ctxt: "0x01"},
	"/relay/channel": RelayChannelRequest{C1: "0x" + strings.Repeat("e6", 1088), C2: "0x01", Bitmap: "0x01"},
}

// A request whose call would revert answers 422, sends nothing and leaves
// nothing claimed, as the payment routes do.
func TestTagRoutes_SendNothingWhenTheCallWouldRevert(t *testing.T) {
	for route, req := range tagRouteRequests {
		t.Run(route, func(t *testing.T) {
			var mined atomic.Bool
			mined.Store(true)
			h, r := tagHandler(t, &mined)
			txNodeReverts.Store(true)
			if w := postJSON(r, route, req); w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("relay of a reverting call: got %d, want 422: %s", w.Code, w.Body.String())
			}
			if data, _ := sentTxData.Load().([]byte); len(data) != 0 {
				t.Fatal("sent a call whose simulation reverted")
			}
			held := false
			h.inFlight.Range(func(_, _ interface{}) bool { held = true; return false })
			if held {
				t.Fatal("a refused request left its claim held")
			}
		})
	}
}

// A request over a registry's limits is refused with 400 before the relayer
// simulates or signs anything: TagRegistry caps ctxt at CTXT_MAX (4096 bytes),
// TagChannelRegistry caps c2 at C2_MAX (4096) and the bitmap at BITMAP_MAX
// (16384), and refuses a bitmap with no bit set.
func TestTagRoutes_RefuseWhatTheRegistryRejectsBeforeSimulating(t *testing.T) {
	c1 := "0x" + strings.Repeat("e7", 1088)
	for name, tc := range map[string]struct {
		route string
		req   interface{}
	}{
		"ctxt over CTXT_MAX":  {"/relay/tag", RelayTagRequest{Tag: "0x" + strings.Repeat("e8", 32), Ctxt: strings.Repeat("01", 4097)}},
		"c2 over C2_MAX":      {"/relay/channel", RelayChannelRequest{C1: c1, C2: strings.Repeat("01", 4097), Bitmap: "0x01"}},
		"bitmap over the max": {"/relay/channel", RelayChannelRequest{C1: c1, C2: "0x01", Bitmap: strings.Repeat("01", 16385)}},
		"an all-zero bitmap":  {"/relay/channel", RelayChannelRequest{C1: c1, C2: "0x01", Bitmap: "0x0000"}},
	} {
		t.Run(name, func(t *testing.T) {
			var mined atomic.Bool
			mined.Store(true)
			_, r := tagHandler(t, &mined)
			if w := postJSON(r, tc.route, tc.req); w.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400: %s", w.Code, w.Body.String())
			}
			if n := txNodeCalls.Load(); n != 0 {
				t.Fatalf("simulated a request the registry rejects (%d eth_calls)", n)
			}
			if data, _ := sentTxData.Load().([]byte); len(data) != 0 {
				t.Fatal("sent a request the registry rejects")
			}
		})
	}
}

// Requests at the registries' limits still go through.
func TestTagRoutes_RelayAtTheRegistryLimits(t *testing.T) {
	c1 := "0x" + strings.Repeat("e9", 1088)
	for name, tc := range map[string]struct {
		route string
		req   interface{}
	}{
		"ctxt at CTXT_MAX":  {"/relay/tag", RelayTagRequest{Tag: "0x" + strings.Repeat("ea", 32), Ctxt: strings.Repeat("01", 4096)}},
		"c2 and bitmap max": {"/relay/channel", RelayChannelRequest{C1: c1, C2: strings.Repeat("01", 4096), Bitmap: strings.Repeat("00", 16383) + "80"}},
	} {
		t.Run(name, func(t *testing.T) {
			var mined atomic.Bool
			mined.Store(true)
			_, r := tagHandler(t, &mined)
			if w := postJSON(r, tc.route, tc.req); w.Code != http.StatusOK {
				t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
			}
		})
	}
}
