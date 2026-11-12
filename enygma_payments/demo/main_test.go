package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPagesDoNotCarryTheAdminKey(t *testing.T) {
	srv := &Server{broker: newBroker(), adminKey: "k3y-that-must-not-leak"}
	for path, h := range map[string]http.HandlerFunc{"/": srv.handleIndex, "/bank": srv.handleBank} {
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(http.MethodGet, path, nil))
		if strings.Contains(w.Body.String(), srv.adminKey) {
			t.Errorf("%s serves the admin key", path)
		}
	}
}

func TestRequireAllowedHost(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := requireAllowedHost(allowedHosts("127.0.0.1:9090", "demo.internal, Other.Example"), ok)
	cases := map[string]int{
		"localhost:9090":      http.StatusOK,
		"127.0.0.1:9090":      http.StatusOK,
		"[::1]:9090":          http.StatusOK,
		"LOCALHOST:9090":      http.StatusOK,
		"demo.internal:9090":  http.StatusOK,
		"other.example":       http.StatusOK,
		"attacker.example":    http.StatusMisdirectedRequest,
		"localhost.evil:9090": http.StatusMisdirectedRequest,
		"":                    http.StatusMisdirectedRequest,
	}
	for host, want := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("Host %q: got %d, want %d", host, w.Code, want)
		}
	}
}

func TestAllowedHostsSkipsWildcardListen(t *testing.T) {
	if allowedHosts("0.0.0.0:9090", "")["0.0.0.0"] {
		t.Error("a wildcard listen address must not become an allowed Host")
	}
	if !allowedHosts("10.0.0.5:9090", "")["10.0.0.5"] {
		t.Error("the listen host should be allowed")
	}
}
