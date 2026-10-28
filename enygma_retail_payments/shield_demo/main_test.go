package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
