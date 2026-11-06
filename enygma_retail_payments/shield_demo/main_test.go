package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"enygma_retail_payments/shield_demo/engine"
)

// The demo's web server, driven through its event stream with no live stack:
// the service logs are files in a temporary directory the test writes to.

// startDemo serves the demo over logDir ("" for none) and returns its URL.
func startDemo(t *testing.T, logDir string) (*server, string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	s := newServer(ctx, engine.DefaultConfig(t.TempDir()), logDir)
	ts := httptest.NewServer(s.routes())
	t.Cleanup(func() { ts.Close(); cancel() })
	return s, ts.URL
}

// stream is one page's connection to /events.
type stream struct {
	t      *testing.T
	events chan map[string]any
}

func connect(t *testing.T, url string) *stream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url+"/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { cancel(); res.Body.Close() })
	s := &stream{t: t, events: make(chan map[string]any, 4096)}
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			data, ok := strings.CutPrefix(sc.Text(), "data: ")
			if !ok {
				continue
			}
			var e map[string]any
			if json.Unmarshal([]byte(data), &e) == nil {
				s.events <- e
			}
		}
		close(s.events)
	}()
	return s
}

// next returns the next event, failing the test if none arrives in time.
func (s *stream) next() map[string]any {
	s.t.Helper()
	select {
	case e, ok := <-s.events:
		if !ok {
			s.t.Fatal("event stream closed")
		}
		return e
	case <-time.After(5 * time.Second):
		s.t.Fatal("no event within 5s")
		return nil
	}
}

// within returns the next event if one arrives within d.
func (s *stream) within(d time.Duration) (map[string]any, bool) {
	select {
	case e, ok := <-s.events:
		return e, ok
	case <-time.After(d):
		return nil, false
	}
}

// nextTerm skips to the next service-log line.
func (s *stream) nextTerm() map[string]any {
	s.t.Helper()
	for {
		if e := s.next(); e["kind"] == "term" {
			return e
		}
	}
}

func appendLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, l := range lines {
		fmt.Fprintln(f, l)
	}
}

func TestServiceLogLineArrivesAsTerminalEvent(t *testing.T) {
	dir := t.TempDir()
	for _, svc := range services {
		appendLines(t, filepath.Join(dir, svc+".log"))
	}
	_, url := startDemo(t, dir)
	page := connect(t, url)

	appendLines(t, filepath.Join(dir, "relayer.log"), "relayed tx 0xabc")

	e := page.nextTerm()
	if e["service"] != "relayer" || e["line"] != "relayed tx 0xabc" {
		t.Fatalf("got %v, want the relayer's line", e)
	}
}

// numbered returns n lines "<svc> line 1" … "<svc> line n".
func numbered(svc string, n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("%s line %d", svc, i+1)
	}
	return lines
}

func TestNewPageGetsTheLast1000LinesOfEachService(t *testing.T) {
	dir := t.TempDir()
	appendLines(t, filepath.Join(dir, "hardhat.log"), numbered("hardhat", 1500)...)
	appendLines(t, filepath.Join(dir, "gnark.log"), numbered("gnark", 40)...)
	_, url := startDemo(t, dir)

	// Once the server has read the log, a new page's first event is the
	// oldest line kept.
	var page *stream
	deadline := time.Now().Add(5 * time.Second)
	for {
		page = connect(t, url)
		if e, ok := page.within(200 * time.Millisecond); ok && e["line"] == "hardhat line 501" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the replay never started at line 501")
		}
	}
	for want := 502; want <= 1500; want++ {
		e := page.nextTerm()
		if e["service"] != "hardhat" || e["line"] != fmt.Sprintf("hardhat line %d", want) {
			t.Fatalf("replay: got %v, want hardhat line %d", e, want)
		}
	}
	// A service with fewer lines is replayed whole.
	for want := 1; want <= 40; want++ {
		e := page.nextTerm()
		if e["service"] != "gnark" || e["line"] != fmt.Sprintf("gnark line %d", want) {
			t.Fatalf("replay: got %v, want gnark line %d", e, want)
		}
	}

	appendLines(t, filepath.Join(dir, "hardhat.log"), "after the replay")
	if e := page.nextTerm(); e["line"] != "after the replay" {
		t.Fatalf("after the replay: got %v", e)
	}
}

func TestWithoutALogDirectoryOnlyEngineEventsAreSent(t *testing.T) {
	// Logs in the working directory must not be picked up either.
	dir := t.TempDir()
	for _, svc := range services {
		appendLines(t, filepath.Join(dir, svc+".log"), svc+" line")
	}
	t.Chdir(dir)
	s, url := startDemo(t, "")
	page := connect(t, url)

	time.Sleep(3 * tailPoll)
	s.broker.Emit(engine.Event{Kind: engine.Log, Msg: "engine line"})

	if e := page.next(); e["kind"] != "log" || e["msg"] != "engine line" {
		t.Fatalf("got %v, want the engine's log line", e)
	}
	if e, ok := page.within(3 * tailPoll); ok {
		t.Fatalf("unexpected event %v", e)
	}
}

// getPage returns the page the demo serves at /.
func getPage(t *testing.T, url string) string {
	t.Helper()
	return getPath(t, url, "/")
}

// getPath returns what the demo serves at path, failing on a status other
// than 200 or a body that is not HTML.
func getPath(t *testing.T, url, path string) string {
	t.Helper()
	res, err := http.Get(url + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("GET %s: %s, %s", path, res.Status, res.Header.Get("Content-Type"))
	}
	b, _ := io.ReadAll(res.Body)
	return string(b)
}

func writePage(t *testing.T, path, html string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDevModeServesThePageFromDiskOnEveryRequest(t *testing.T) {
	page := filepath.Join(t.TempDir(), "index.html")
	writePage(t, page, "<p>first</p>")
	s, url := startDemo(t, "")
	s.pageFile = page

	if got := getPage(t, url); got != "<p>first</p>" {
		t.Fatalf("got %q, want the file's page", got)
	}
	writePage(t, page, "<p>edited</p>")
	if got := getPage(t, url); got != "<p>edited</p>" {
		t.Fatalf("after an edit: got %q, want the edited page", got)
	}
}

func TestWithoutDevModeTheEmbeddedPageIsServed(t *testing.T) {
	_, url := startDemo(t, "")
	if got := getPage(t, url); got != indexHTML {
		t.Fatalf("got %d bytes, want the embedded page", len(got))
	}
}

// The Protocol primer is served at /protocol like the demo page at /: the
// embedded copy normally, the file on disk on every request in -dev mode.
func TestDevModeServesThePrimerFromDiskOnEveryRequest(t *testing.T) {
	primer := filepath.Join(t.TempDir(), "protocol.html")
	writePage(t, primer, "<p>first</p>")
	s, url := startDemo(t, "")
	s.primerFile = primer

	if got := getPath(t, url, "/protocol"); got != "<p>first</p>" {
		t.Fatalf("got %q, want the file's primer", got)
	}
	writePage(t, primer, "<p>edited</p>")
	if got := getPath(t, url, "/protocol"); got != "<p>edited</p>" {
		t.Fatalf("after an edit: got %q, want the edited primer", got)
	}
}

func TestWithoutDevModeTheEmbeddedPrimerIsServed(t *testing.T) {
	_, url := startDemo(t, "")
	got := getPath(t, url, "/protocol")
	if got != protocolHTML {
		t.Fatalf("got %d bytes, want the embedded primer", len(got))
	}
	if !strings.Contains(got, "<title>Protocol primer") {
		t.Fatalf("the embedded primer has no Protocol primer title")
	}
}

// getDefaults returns the demo's /defaults, decoded.
func getDefaults(t *testing.T, url string) map[string]any {
	t.Helper()
	res, err := http.Get(url + "/defaults")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var d map[string]any
	if err := json.NewDecoder(res.Body).Decode(&d); err != nil {
		t.Fatalf("decode /defaults: %v", err)
	}
	return d
}

// /defaults gives the page what its on-chain proof check calls: the node's
// RPC URL, the deployment's Verifier and the Payment verifying key's slot
// (0, Erc20CoinVault's VK_ID_ERC20_JOINSPLIT).
func TestDefaultsNameTheVerifierThePageCalls(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePage(t, filepath.Join(root, "build", "receipts.json"),
		`{"Verifier": {"contractAddress": "0x6b01819C927b66eB1ED17a276fC60359B502d3De"}}`)
	s := newServer(context.Background(), engine.DefaultConfig(root), "")
	ts := httptest.NewServer(s.routes())
	t.Cleanup(ts.Close)

	d := getDefaults(t, ts.URL)
	chain, _ := d["chain"].(map[string]any)
	want := map[string]any{"rpcUrl": "http://127.0.0.1:8545", "verifier": "0x6b01819C927b66eB1ED17a276fC60359B502d3De", "paymentVk": float64(0)}
	for k, v := range want {
		if chain[k] != v {
			t.Errorf("/defaults chain.%s = %v, want %v (chain %v)", k, chain[k], v, d["chain"])
		}
	}
	if d["shieldTotal"] != float64(engine.DefaultOptions().ShieldTotal) {
		t.Errorf("/defaults lost the run options: shieldTotal %v", d["shieldTotal"])
	}
}

// Without a deployment, /defaults still gives the run options, and no chain.
func TestDefaultsWithoutADeploymentHaveNoChain(t *testing.T) {
	_, url := startDemo(t, "")
	d := getDefaults(t, url)
	if _, ok := d["chain"]; ok {
		t.Errorf("/defaults chain = %v with no build/receipts.json, want none", d["chain"])
	}
	if d["shieldTotal"] == nil {
		t.Error("/defaults has no run options")
	}
}
